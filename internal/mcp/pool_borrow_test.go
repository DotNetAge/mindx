package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// blockingClient 是可观测的假 MCP 客户端：Call 会阻塞到 release 被关闭，
// 并记录 Close 被调用的次数与时点。
type blockingClient struct {
	enter   chan struct{}
	release chan struct{}

	mu     sync.Mutex
	closes int
	alive  bool
}

func newBlockingClient() *blockingClient {
	return &blockingClient{
		enter:   make(chan struct{}, 1),
		release: make(chan struct{}),
		alive:   true,
	}
}

func (c *blockingClient) Connect(context.Context) error { return nil }

func (c *blockingClient) Close() error {
	c.mu.Lock()
	c.closes++
	c.alive = false
	c.mu.Unlock()
	return nil
}

func (c *blockingClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

func (c *blockingClient) Call(context.Context, string, any) (json.RawMessage, error) {
	select {
	case c.enter <- struct{}{}:
	default:
	}
	<-c.release
	return json.RawMessage(`{}`), nil
}

func (c *blockingClient) Notify(context.Context, string, any) error { return nil }

func (c *blockingClient) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

// TestPoolDisconnectDoesNotPullConnInUse 工具调用进行中，Disconnect 不得关闭
// 在用的连接，归还后才真正关闭。
//
// 回归点（P1 §4.8）：Call 以前取出指针后立即放锁，Disconnect/reap/CloseAll
// 可以在窗口内关掉连接，随后对着已关闭的 client 发请求。
func TestPoolDisconnectDoesNotPullConnInUse(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	client := newBlockingClient()
	p.mu.Lock()
	p.connections["srv"] = &managedConn{client: client, lastUsed: time.Now(), ttl: time.Minute}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = p.Call(context.Background(), "srv", "tool", nil)
	}()

	// 等到调用真正进入 client
	select {
	case <-client.enter:
	case <-time.After(2 * time.Second):
		t.Fatal("工具调用没有进入 client")
	}

	p.Disconnect("srv")
	if n := client.closeCount(); n != 0 {
		t.Fatalf("调用进行中不应关闭连接，Close 被调用 %d 次", n)
	}

	close(client.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("调用未返回")
	}

	// 归还后必须关闭且只关一次
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if client.closeCount() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("归还后应关闭连接一次，实际 %d 次", client.closeCount())
}

// TestPoolReapSkipsConnInUse 回收循环不得动正在借用的连接。
func TestPoolReapSkipsConnInUse(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	client := newBlockingClient()
	p.mu.Lock()
	p.connections["srv"] = &managedConn{client: client, lastUsed: time.Now().Add(-time.Hour), ttl: time.Second}
	p.mu.Unlock()

	if !p.connections["srv"].acquire() {
		t.Fatal("借用失败")
	}
	p.reap()
	if n := client.closeCount(); n != 0 {
		t.Fatalf("借用中的连接被回收了（Close %d 次）", n)
	}
	if _, ok := p.connections["srv"]; !ok {
		t.Fatal("借用中的连接不应从索引移除")
	}
}
