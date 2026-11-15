package channel

import (
	"sync"
	"testing"
)

// TestConnEnqueueAfterClose 关闭后再次入队只能拿到错误，绝不能 panic。
//
// 回归点（P1 §4.6）：旧实现 close() 直接 close(c.send)，而「已关闭」信号
// （c.closed）要等读泵退出才关闭。窗口内的并发 enqueue 会向已关闭通道发送
// ——select 的 send 分支不会退化到 default，直接 panic，整个 daemon 崩溃。
// 客户端断连 + 并发广播是常态路径，不是边角场景。
func TestConnEnqueueAfterClose(t *testing.T) {
	c := &conn{
		send:   make(chan outMsg, 4),
		closed: make(chan struct{}),
		done:   make(chan struct{}),
	}
	c.close()

	for i := 0; i < 8; i++ {
		if err := c.enqueue(1, []byte("x")); err == nil {
			t.Fatal("关闭后入队应返回错误")
		}
	}
	// 幂等：重复 close 不 panic
	c.close()
}

// TestConnEnqueueConcurrentWithClose 关闭与并发送同时进行不得 panic（配合 -race）。
func TestConnEnqueueConcurrentWithClose(t *testing.T) {
	for round := 0; round < 200; round++ {
		c := &conn{
			send:   make(chan outMsg, 8),
			closed: make(chan struct{}),
			done:   make(chan struct{}),
		}
		var wg sync.WaitGroup
		wg.Add(5)
		for i := 0; i < 4; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_ = c.enqueue(1, []byte("payload"))
				}
			}()
		}
		go func() {
			defer wg.Done()
			c.close()
		}()
		wg.Wait()
	}
}

// TestConnDrainAndCloseOrder 关闭后队列里已入队的消息仍会被发完（写泵排空语义）。
// 这里只校验队列被消费干净，不涉及真实 websocket。
func TestConnDrainAndCloseOrder(t *testing.T) {
	c := &conn{
		send:   make(chan outMsg, 4),
		closed: make(chan struct{}),
		done:   make(chan struct{}),
	}
	if err := c.enqueue(1, []byte("a")); err != nil {
		t.Fatalf("关闭前入队应成功: %v", err)
	}
	c.close()
	// 关闭后入队被拒
	if err := c.enqueue(1, []byte("b")); err == nil {
		t.Fatal("关闭后入队应返回错误")
	}
	// 队列中的旧消息仍在，可被消费（写泵 drainSend 依赖此语义）
	select {
	case m := <-c.send:
		if string(m.data) != "a" {
			t.Fatalf("队列内容被破坏: %q", m.data)
		}
	default:
		t.Fatal("关闭不应丢弃已入队的消息")
	}
}
