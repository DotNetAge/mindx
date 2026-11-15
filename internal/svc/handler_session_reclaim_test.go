package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/logging"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/stretchr/testify/require"
)

// loadTestSession 载入测试会话（与 mustCreateSession 使用同一 agent 名）。
func loadTestSession(t *testing.T, d *Daemon, sessionID, agentName string) *goharnesssession.Session {
	t.Helper()
	sess, err := goharnesssession.Load(context.Background(), sessionID, agentName, d.app.SessDB(), logging.DefaultLogger())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	return sess
}

// appendRound 追加一轮 user + assistant 消息，返回该轮 user 消息的时间戳。
func appendRound(t *testing.T, sess *goharnesssession.Session, userText string) int64 {
	t.Helper()
	ts := time.Now().Add(time.Duration(len(sess.All())) * time.Second).UnixMilli()
	require.NoError(t, sess.Append(context.Background(), goharnesssession.Message{
		Role: "user", Content: userText, Timestamp: ts,
	}))
	require.NoError(t, sess.Append(context.Background(), goharnesssession.Message{
		Role: "assistant", Content: "answer-" + userText, Timestamp: ts + 1,
	}))
	return ts
}

// TestReclaimLastRound_OnlyLastRound 回收只吃最后一轮，更早轮次原样保留。
//
// 回归点（P1 §4.5）：delete_round 原本按时间戳定位任意轮次做**中间删除**，
// 把已发给模型的历史删掉，导致上下文与磁盘分叉、KV 前缀失效。收窄后
// 截断点永远锚定在最后一条 user 消息上。
func TestReclaimLastRound_OnlyLastRound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sessionID := mustCreateSession(t, sessDB, "test-agent") // 第一轮：user "init"
	sess := loadTestSession(t, d, sessionID, "test-agent")
	secondTs := appendRound(t, sess, "second") // 第二轮

	// 回收第二轮
	res, err := d.handleSessionDeleteRound(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q,"id":%d}`, sessionID, secondTs)))
	require.NoError(t, err)
	require.Equal(t, true, res.(map[string]any)["deleted"])

	after := loadTestSession(t, d, sessionID, "test-agent").All()
	require.Len(t, after, 1, "只应删掉第二轮的两条消息")
	require.Equal(t, "init", after[0].Content, "第一轮必须原样保留")
}

// TestReclaimLastRound_RejectsEarlierRound 指向更早轮次必须被拒。
func TestReclaimLastRound_RejectsEarlierRound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sessionID := mustCreateSession(t, sessDB, "test-agent")
	sess := loadTestSession(t, d, sessionID, "test-agent")
	firstTs := sess.All()[0].Timestamp
	appendRound(t, sess, "second")

	// 试图回收第一轮（中间删除）
	_, err := d.handleSessionDeleteRound(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q,"id":%d}`, sessionID, firstTs)))
	require.ErrorIs(t, err, errLastRoundOnly)

	require.Len(t, loadTestSession(t, d, sessionID, "test-agent").All(), 3, "拒绝时不得改动任何消息")
}

// TestReclaimLastRound_TruncateHitsLastRound session.truncate 语义等价于回收最后一轮，
// 且不再出现「少删一条」的边界错位。
func TestReclaimLastRound_TruncateHitsLastRound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sessionID := mustCreateSession(t, sessDB, "test-agent")
	sess := loadTestSession(t, d, sessionID, "test-agent")
	appendRound(t, sess, "second")

	res, err := d.handleSessionTruncate(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
	require.NoError(t, err)
	require.Equal(t, 1, res.(map[string]any)["messages_kept"])

	after := loadTestSession(t, d, sessionID, "test-agent").All()
	require.Len(t, after, 1)
	require.Equal(t, "init", after[0].Content)
}

// TestReclaimLastRound_NoRound 会话里没有 user 消息时给出明确拒绝，而不是空操作。
func TestReclaimLastRound_NoRound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessDB := d.app.SessDB()
	sessionID := mustCreateSession(t, sessDB, "test-agent")
	sess := loadTestSession(t, d, sessionID, "test-agent")
	// 先回收唯一一轮，此时会话为空
	_, err := d.handleSessionTruncate(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
	require.NoError(t, err)
	require.Zero(t, len(sess.All()))

	_, err = d.handleSessionTruncate(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
	require.ErrorIs(t, err, errNoReclaimableRound)
}

// TestReclaimLastRound_RefusesRunningTurn 执行中的会话拒绝回收，避免与运行轮次交错。
func TestReclaimLastRound_RefusesRunningTurn(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sessionID := mustCreateSession(t, d.app.SessDB(), "test-agent")

	// 占住该会话的执行队列：running 期间 Busy() 为 true
	release := make(chan struct{})
	d.sessionQueueFor(sessionID).Enqueue(func() { <-release })

	_, err := d.handleSessionTruncate(context.Background(),
		json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
	require.ErrorIs(t, err, errTurnInFlight)

	close(release)
	// 等待队列清空后再回收，应恢复可用
	require.Eventually(t, func() bool {
		_, e := d.handleSessionTruncate(context.Background(),
			json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
		return e == nil
	}, 2*time.Second, 20*time.Millisecond)
}
