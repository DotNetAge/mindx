package svc

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSessionQueue_SerialFIFO 验证任务按入队顺序串行执行：
// 前一个执行完全结束后才运行下一个，且全程无重叠。
func TestSessionQueue_SerialFIFO(t *testing.T) {
	timeout := time.After(10 * time.Second)

	var (
		mu      sync.Mutex
		order   []int
		overlap int32
		active  int32
	)

	q := &sessionQueue{}
	for i := 0; i < 5; i++ {
		i := i
		q.Enqueue(func() {
			if atomic.AddInt32(&active, 1) > 1 {
				atomic.AddInt32(&overlap, 1)
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		})
	}

	// 等待队列清空（执行者 goroutine 退出）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		q.mu.Lock()
		drained := !q.running && len(q.pending) == 0
		q.mu.Unlock()
		if drained {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue did not drain in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-timeout:
		t.Fatal("test timed out")
	default:
	}

	if overlap != 0 {
		t.Fatalf("tasks overlapped: %d", overlap)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 5 {
		t.Fatalf("expected 5 executed, got %d", len(order))
	}
	for i := 0; i < 5; i++ {
		if order[i] != i {
			t.Fatalf("FIFO order violated: got %v", order)
		}
	}
}

// TestSessionQueue_EnqueueNonBlocking 验证队列执行期间再次 Enqueue 不会阻塞调用方，
// 且任务会在上一轮结束后继续执行（不丢失）。
func TestSessionQueue_EnqueueNonBlocking(t *testing.T) {
	timeout := time.After(10 * time.Second)

	var (
		mu     sync.Mutex
		order  []int
		first  = make(chan struct{})
		second = make(chan struct{})
	)

	q := &sessionQueue{}
	q.Enqueue(func() {
		close(first)
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})

	// 等待第一个任务开始执行，然后在执行期间入队第二个任务。
	<-first
	q.Enqueue(func() {
		close(second)
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})

	select {
	case <-second:
	case <-timeout:
		t.Fatal("queued task was lost")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		q.mu.Lock()
		drained := !q.running && len(q.pending) == 0
		q.mu.Unlock()
		if drained {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue did not drain in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected execution order: %v", order)
	}
}

// TestClientCancelSet_CancelAll 验证 CancelAll 会批量取消集合内的全部条目。
func TestClientCancelSet_CancelAll(t *testing.T) {
	var cancelled int32
	set := &clientCancelSet{}
	set.Add(func() { atomic.AddInt32(&cancelled, 1) })
	set.Add(func() { atomic.AddInt32(&cancelled, 1) })

	set.CancelAll()
	if got := atomic.LoadInt32(&cancelled); got != 2 {
		t.Fatalf("expected 2 cancels, got %d", got)
	}
}

// TestClientCancelSet_Remove 验证 Remove 后 CancelAll 不再取消已移除的条目。
func TestClientCancelSet_Remove(t *testing.T) {
	var cancelled int32
	set := &clientCancelSet{}
	e := set.Add(func() { atomic.AddInt32(&cancelled, 1) })
	set.Remove(e)

	set.CancelAll()
	if got := atomic.LoadInt32(&cancelled); got != 0 {
		t.Fatalf("expected 0 cancels after remove, got %d", got)
	}
}

// TestClientCancelSet_ContextCancel 验证 CancelAll 确实通过 context.CancelFunc 生效。
func TestClientCancelSet_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	set := &clientCancelSet{}
	set.Add(cancel)

	set.CancelAll()
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}
