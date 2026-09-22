package channel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// 进程内集成测试：以最小协议桩模拟中继服务与网关回声服务，
// 验证 Manager 全链路：连接 → 短码 → 同意闸 → 配对 → 桥接回环。
// ---------------------------------------------------------------------------

// memCertStore 内存证书存取桩
type memCertStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemCertStore() *memCertStore { return &memCertStore{m: map[string]string{}} }

func (s *memCertStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key], nil
}

func (s *memCertStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

// fakeRelay 最小中继桩：连接后下发 hello + 固定短码；
// 第二条连接提交正确短码后两端配对（下发证书），此后二进制帧互转。
type fakeRelay struct {
	srv  *httptest.Server
	code string

	mu    sync.Mutex
	conns []*websocket.Conn
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()
	f := &fakeRelay{code: "123456"}
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		f.mu.Lock()
		idx := len(f.conns)
		f.conns = append(f.conns, ws)
		f.mu.Unlock()

		_ = ws.WriteJSON(map[string]any{"type": "hello", "conn_id": idx})
		_ = ws.WriteJSON(map[string]any{"type": "code", "code": f.code, "expires_in": 60})

		go func() {
			defer ws.Close()
			for {
				mt, data, err := ws.ReadMessage()
				if err != nil {
					return
				}
				switch mt {
				case websocket.TextMessage:
					f.handleText(idx, ws, data)
				case websocket.BinaryMessage:
					f.forward(idx, data)
				}
			}
		}()
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// handleText 处理控制消息：仅实现 submit_code 配对路径。
func (f *fakeRelay) handleText(idx int, ws *websocket.Conn, data []byte) {
	var m struct {
		Type string `json:"type"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &m); err != nil || m.Type != "submit_code" {
		return
	}
	f.mu.Lock()
	paired := len(f.conns) >= 2
	f.mu.Unlock()
	if !paired || m.Code != f.code || idx != 1 {
		_ = ws.WriteJSON(map[string]any{"type": "error", "error": "invalid_code"})
		return
	}
	// 两端配对：桌面端（idx=0）随消息下发证书
	cert := "-----BEGIN CERTIFICATE-----TEST-----END CERTIFICATE-----"
	key := "-----BEGIN PRIVATE KEY-----TEST-----END PRIVATE KEY-----"
	_ = f.conns[0].WriteJSON(map[string]any{"type": "paired", "group_id": "g1", "device_id": "d0", "device_cert": cert, "device_key": key})
	_ = ws.WriteJSON(map[string]any{"type": "paired", "group_id": "g1", "device_id": "d1", "device_cert": cert, "device_key": key})
}

// forward 把一条连接的二进制帧转发给另一条（单组配对桩）。
func (f *fakeRelay) forward(from int, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.conns) < 2 {
		return
	}
	to := 1 - from
	_ = f.conns[to].WriteMessage(websocket.BinaryMessage, data)
}

// newFakeGateway 网关回声桩：文本帧原样回写，验证桥的双向通路。
func newFakeGateway(t *testing.T) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.TextMessage {
				if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
					return
				}
			}
		}
	}))
}

// waitFor 轮询等待条件成立，超时判定失败。
func waitFor(t *testing.T, timeout time.Duration, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", desc)
}

// TestManagerEndToEnd 覆盖：连接取码 → 引导同意闸 → 配对存证书 → 桥接回环。
func TestManagerEndToEnd(t *testing.T) {
	relay := newFakeRelay(t)
	wsURL := "ws" + strings.TrimPrefix(relay.srv.URL, "http") + "/ws"

	gwSrv := newFakeGateway(t)
	gwURL := "ws" + strings.TrimPrefix(gwSrv.URL, "http")

	certs := newMemCertStore()

	// 捕获同意闸广播
	var notifyMu sync.Mutex
	var notified []map[string]any
	mgr, err := NewManager(Config{
		URL:    wsURL,
		Certs:  certs,
		Logger: &nopLogger{},
		Notify: func(method string, params any) {
			if method != "channel.pair_request" {
				return
			}
			notifyMu.Lock()
			notified = append(notified, params.(map[string]any))
			notifyMu.Unlock()
		},
		GatewayDial: func() (*websocket.Conn, error) {
			ws, _, err := websocket.DefaultDialer.Dial(gwURL, nil)
			return ws, err
		},
	})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	mgr.Start(t.Context())
	t.Cleanup(mgr.Stop)

	// 1. 连接后收到短码，状态 waiting
	waitFor(t, 3*time.Second, "收到短码进入 waiting", func() bool {
		st := mgr.Status()
		return st["state"] == StateWaiting && st["code"] == relay.code
	})

	// 2. 手机引导：未批准 → pending 并广播 pair_request
	res := mgr.Bootstrap("iPhone-Ray")
	if res.Status != "pending" {
		t.Fatalf("首次引导应返回 pending，实际 %s", res.Status)
	}
	notifyMu.Lock()
	// 广播体为网关信封格式 {type,title,data}：业务字段在 data 内层
	envData, _ := notified[0]["data"].(map[string]any)
	reqID, _ := envData["request_id"].(string)
	notifyMu.Unlock()
	if reqID == "" {
		t.Fatal("pair_request 广播缺少 request_id")
	}
	// 重复引导仍为 pending（手机端 2s 重试语义）
	if res := mgr.Bootstrap("iPhone-Ray"); res.Status != "pending" {
		t.Fatalf("重复引导应保持 pending，实际 %s", res.Status)
	}

	// 3. 拒绝后再引导 → denied；过期语义由 TTL 控制，此处仅验证即时拒绝
	if err := mgr.Deny(reqID); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	if res := mgr.Bootstrap("iPhone-Ray"); res.Status != "denied" {
		t.Fatalf("拒绝后引导应返回 denied，实际 %s", res.Status)
	}

	// 4. TTL 内新请求（提示不同视为新设备）→ 批准 → 引导返回 ready + 短码
	res = mgr.Bootstrap("iPad-Ray")
	if res.Status != "pending" {
		t.Fatalf("新设备引导应返回 pending，实际 %s", res.Status)
	}
	notifyMu.Lock()
	envData2, _ := notified[len(notified)-1]["data"].(map[string]any)
	reqID2, _ := envData2["request_id"].(string)
	notifyMu.Unlock()
	if err := mgr.Approve(reqID2); err != nil {
		t.Fatalf("批准失败: %v", err)
	}
	res = mgr.Bootstrap("iPad-Ray")
	if res.Status != "ready" || res.ChannelURL != wsURL || res.Code != relay.code {
		t.Fatalf("批准后引导应返回 ready，实际 %+v", res)
	}

	// 5. 手机连接并提交短码 → 双方配对，管理器持久化证书
	phone, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("手机连接失败: %v", err)
	}
	t.Cleanup(func() { _ = phone.Close() })
	_ = phone.WriteJSON(map[string]any{"type": "submit_code", "code": relay.code})
	waitFor(t, 3*time.Second, "管理器进入 paired", func() bool {
		st := mgr.Status()
		return st["state"] == StatePaired && st["paired"] == true
	})
	waitFor(t, 3*time.Second, "证书已持久化", func() bool {
		c, _ := certs.Get(keyCert)
		k, _ := certs.Get(keyKey)
		return c != "" && k != ""
	})

	// 6. 桥接回环：手机发二进制 → 桥转网关文本 → 网关回声 → 桥转回手机二进制
	payload := []byte(`{"jsonrpc":"2.0","method":"ping","id":1}`)
	_ = phone.WriteMessage(websocket.BinaryMessage, payload)
	var got []byte
	waitFor(t, 5*time.Second, "手机收到网关回声数据", func() bool {
		if len(got) != 0 {
			return true
		}
		_ = phone.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		mt, data, err := phone.ReadMessage()
		if err == nil && mt == websocket.BinaryMessage {
			got = data
		}
		_ = phone.SetReadDeadline(time.Time{})
		return false
	})
	if string(got) != string(payload) {
		t.Fatalf("桥接回环数据不一致: %s", got)
	}

	// 7. 状态摘要关键字段
	st := mgr.Status()
	if st["connected"] != true || st["configured"] != true || st["group_id"] != "g1" {
		t.Fatalf("状态摘要异常: %+v", st)
	}
}

// nopLogger 空日志实现。
type nopLogger struct{}

func (l *nopLogger) Debug(_ string, _ ...any)          {}
func (l *nopLogger) Info(_ string, _ ...any)           {}
func (l *nopLogger) Warn(_ string, _ ...any)           {}
func (l *nopLogger) Error(_ string, _ error, _ ...any) {}
