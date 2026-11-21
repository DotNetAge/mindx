package gateway

import (
	"context"
	"testing"
	"time"
)

// 回归测试：Close 必须幂等。
//
// 背景：Close 存在两条并发调用路径——调用方 defer Close（如 mindx skill add）
// 与 readLoop 退出时的 defer Close。未做 once 保护时 close(c.done) 会被执行
// 两次，触发 panic: close of closed channel（gort v0.1.6 实测可复现）。
func TestCloseIdempotent(t *testing.T) {
	env := setupTestEnv(t)

	cl := NewClient("ws://" + env.addr + "/ws")
	if err := cl.ConnectSync(); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// 模拟真实时序：主 goroutine defer Close 先执行并关闭底层连接，
	// 随后 readLoop 的 ReadMessage 报错退出，其 defer Close 第二次执行。
	// 修复前第二次 Close 必 panic。
	if err := cl.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := cl.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	// 给 readLoop 的 defer Close 留出执行窗口；未修复时 panic 在该 goroutine
	time.Sleep(100 * time.Millisecond)
}

// 真实断连场景：服务端主动断开 → 客户端 readLoop 退出并执行 defer Close →
// 调用方随后再 Close，不得 panic。
func TestCloseAfterServerDisconnect(t *testing.T) {
	env := setupTestEnv(t)

	cl := NewClient("ws://" + env.addr + "/ws")
	if err := cl.ConnectSync(); err != nil {
		t.Fatalf("connect: %v", err)
	}

	env.gw.Shutdown(context.Background())
	time.Sleep(200 * time.Millisecond)

	if err := cl.Close(); err != nil {
		t.Fatalf("close after server disconnect: %v", err)
	}
}

// DisconnectClient 主动断连：服务端按 clientID 强制关闭在线连接，
// 客户端感知断开、服务端摘除该连接（吊销即断连场景）。
func TestDisconnectClient(t *testing.T) {
	env := setupTestEnv(t)

	cl := NewClient("ws://" + env.addr + "/ws")
	if err := cl.ConnectSync(); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// 等客户端注册进服务端连接表（同包可读私有字段）
	var clientID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		env.gw.mu.RLock()
		for id := range env.gw.clients {
			clientID = id
		}
		env.gw.mu.RUnlock()
		if clientID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if clientID == "" {
		t.Fatal("客户端未注册到服务端连接表")
	}

	if !env.gw.DisconnectClient(clientID) {
		t.Fatal("连接存在时 DisconnectClient 应返回 true")
	}
	if env.gw.DisconnectClient("no-such-client") {
		t.Fatal("不存在的客户端应返回 false")
	}

	// 服务端应随后摘除该连接
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		env.gw.mu.RLock()
		_, still := env.gw.clients[clientID]
		env.gw.mu.RUnlock()
		if !still {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("断开后客户端应被从连接表摘除")
}
