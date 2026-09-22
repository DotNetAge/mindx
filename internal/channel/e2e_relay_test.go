package channel

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestRelayEndToEndAgainstRealServer 真实 agent-hub 服务端到端验证
// （单测 TestManagerEndToEnd 用的是进程内假中继，本测试补齐真实协议栈）：
//
//	子进程拉起真实 agent-hub（personal 模式）→ daemon 侧真实 Manager 连接取短码 →
//	模拟手机读短码提交（手动短码路径 = 知情同意）→ 双方配对、证书下发 →
//	网关桥接回环（手机二进制帧 ⇄ 哑网关文本回声）→ 手机断开、daemon 感知解除配对。
func TestRelayEndToEndAgainstRealServer(t *testing.T) {
	// 1. 定位并构建真实 agent-hub 服务（同级目录 ../../agent-hub）
	hubDir, absErr := filepath.Abs("../../../agent-hub")
	if absErr != nil {
		t.Fatalf("定位 agent-hub 目录失败: %v", absErr)
	}
	if _, statErr := os.Stat(filepath.Join(hubDir, "go.mod")); statErr != nil {
		t.Skipf("agent-hub 模块不存在（%s），跳过真实服务端验证", hubDir)
	}
	bin := filepath.Join(t.TempDir(), "agent-hub-server")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "./cmd/server")
	build.Dir = hubDir
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("构建 agent-hub 服务失败: %v\n%s", buildErr, out)
	}

	// 2. 抢占空闲端口并启动服务子进程
	l, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("抢占端口失败: %v", listenErr)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close() // 释放到子进程启动之间存在极小竞争窗口，测试环境可接受

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"ADDR=127.0.0.1:"+strconv.Itoa(port),
		"DATA_DIR="+t.TempDir(),
		"SERVICE_MODE=personal",
	)
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("启动 agent-hub 服务失败: %v", startErr)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// 3. 等待健康检查就绪
	healthURL := "http://127.0.0.1:" + strconv.Itoa(port) + "/healthz"
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, getErr := http.Get(healthURL)
		if getErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		t.Fatal("等待 agent-hub 服务就绪超时")
	}

	// 4. daemon 侧真实 Manager 连接真实中继
	relayURL := "ws://127.0.0.1:" + strconv.Itoa(port) + "/ws"
	gwSrv := newFakeGateway(t)
	gwURL := "ws" + strings.TrimPrefix(gwSrv.URL, "http")
	certs := newMemCertStore()
	mgr, mgrErr := NewManager(Config{
		URL:    relayURL,
		Certs:  certs,
		Logger: &nopLogger{},
		Notify: func(string, any) {},
		GatewayDial: func() (*websocket.Conn, error) {
			ws, _, dialErr := websocket.DefaultDialer.Dial(gwURL, nil)
			return ws, dialErr
		},
	})
	if mgrErr != nil {
		t.Fatalf("创建管理器失败: %v", mgrErr)
	}
	mgr.Start(t.Context())
	t.Cleanup(mgr.Stop)

	// 5. daemon 从真实中继取得短码（服务端自动下发）
	waitFor(t, 10*time.Second, "daemon 从真实中继取得短码", func() bool {
		_, ok := mgr.Status()["code"]
		return ok
	})
	daemonCode, _ := mgr.Status()["code"].(string)
	if daemonCode == "" {
		t.Fatal("短码为空")
	}

	// 6. 模拟手机：连真实中继，读本端 hello+code，提交 daemon 短码
	phone, _, dialErr := websocket.DefaultDialer.Dial(relayURL, nil)
	if dialErr != nil {
		t.Fatalf("手机连接中继失败: %v", dialErr)
	}
	t.Cleanup(func() { _ = phone.Close() })
	_ = phone.SetReadDeadline(time.Now().Add(5 * time.Second))

	// hello → code（手机本端短码，仅校验顺序）
	var hello serverMsg
	if readErr := phone.ReadJSON(&hello); readErr != nil || hello.Type != msgHello {
		t.Fatalf("手机未收到 hello: %v", readErr)
	}
	var own serverMsg
	if readErr := phone.ReadJSON(&own); readErr != nil || own.Type != msgCode {
		t.Fatalf("手机未收到本端短码: %v", readErr)
	}
	if writeErr := phone.WriteJSON(clientMsg{Type: msgSubmitCode, Code: daemonCode}); writeErr != nil {
		t.Fatalf("手机提交短码失败: %v", writeErr)
	}

	// 7. 双方配对：手机收到 paired（含证书下发）
	var pairMsg serverMsg
	for {
		if readErr := phone.ReadJSON(&pairMsg); readErr != nil {
			t.Fatalf("手机等待配对结果失败: %v", readErr)
		}
		if pairMsg.Type == msgPaired {
			break
		}
	}
	if pairMsg.DeviceCert == "" || pairMsg.DeviceKey == "" {
		t.Fatal("配对消息缺少设备证书")
	}

	// daemon 侧同步进入配对态且证书已持久化
	waitFor(t, 5*time.Second, "daemon 进入配对态", func() bool {
		paired, _ := mgr.Status()["paired"].(bool)
		return paired
	})
	waitFor(t, 3*time.Second, "daemon 证书已持久化", func() bool {
		c, certErr := certs.Get(keyCert)
		return certErr == nil && c != ""
	})

	// 8. 桥接透传回环：手机发二进制帧 → 哑网关文本回声 → 桥接转回二进制帧
	_ = phone.SetWriteDeadline(time.Now().Add(5 * time.Second))
	payload := []byte("你好网关\n")
	if writeErr := phone.WriteMessage(websocket.BinaryMessage, payload); writeErr != nil {
		t.Fatalf("手机发送透传数据失败: %v", writeErr)
	}
	_ = phone.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		mt, data, readErr := phone.ReadMessage()
		if readErr != nil {
			t.Fatalf("手机等待网关回声失败: %v", readErr)
		}
		if mt == websocket.BinaryMessage {
			if string(data) != string(payload) {
				t.Fatalf("回声内容不符: %q", data)
			}
			break
		}
	}

	// 9. 手机断开：daemon 感知 peer_offline，解除配对
	_ = phone.Close()
	waitFor(t, 5*time.Second, "手机断开后 daemon 解除配对", func() bool {
		paired, _ := mgr.Status()["paired"].(bool)
		return !paired
	})
}
