package svc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/session"
)

func retryParams(sid string) json.RawMessage {
	p, _ := json.Marshal(map[string]string{"session_id": sid})
	return p
}

// TestHandleSessionRetryLast_NoUserRound 会话里没有用户消息时无可重试对象。
func TestHandleSessionRetryLast_NoUserRound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	info, err := session.CreateSession(context.Background(), d.app.SessDB(), "bound-agent",
		session.WithProjectDirOption(t.TempDir()))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = d.handleSessionRetryLast(context.Background(), retryParams(info.SessionID))
	if err != errRetryNoRound {
		t.Errorf("expected errRetryNoRound, got %v", err)
	}
}

// TestHandleSessionRetryLast_InFlight 队列有执行在跑时拒绝重试。
func TestHandleSessionRetryLast_InFlight(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "bound-agent")

	release := make(chan struct{})
	d.sessionQueueFor(sid).Enqueue(func() { <-release })
	defer close(release)

	_, err := d.handleSessionRetryLast(context.Background(), retryParams(sid))
	if err != errRetryInFlight {
		t.Errorf("expected errRetryInFlight, got %v", err)
	}
}

// TestHandleSessionRetryLast_Enqueued 有用户消息且队列空闲时，续跑任务入队，
// RPC 立即返回排队确认；后台闭包的 agent 解析失败由 onDone 吞掉，不影响本断言。
func TestHandleSessionRetryLast_Enqueued(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "bound-agent")

	res, err := d.handleSessionRetryLast(context.Background(), retryParams(sid))
	if err != nil {
		t.Fatalf("handleSessionRetryLast error = %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", res)
	}
	if m["resumed"] != true {
		t.Errorf("expected resumed=true, got %v", m["resumed"])
	}

	// 等待后台闭包退出，避免污染后续用例的队列状态
	deadline := time.Now().Add(5 * time.Second)
	for d.sessionQueueFor(sid).Busy() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
}
