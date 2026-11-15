package svc

import (
	"testing"
	"time"

	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func TestEffectiveEventSession(t *testing.T) {
	if got := effectiveEventSession("main", nil); got != "main" {
		t.Errorf("nil getter = %q, want main", got)
	}
	// getter 返回空串 → 主会话
	if got := effectiveEventSession("main", func() string { return "" }); got != "main" {
		t.Errorf("empty sub = %q, want main", got)
	}
	// getter 返回主会话自身 → 主会话
	if got := effectiveEventSession("main", func() string { return "main" }); got != "main" {
		t.Errorf("sub==main = %q, want main", got)
	}
	// 子会话转发 → 子会话 ID
	if got := effectiveEventSession("main", func() string { return "sub-1" }); got != "sub-1" {
		t.Errorf("sub = %q, want sub-1", got)
	}
}

func TestBroadcastHelpers_NilGateway(t *testing.T) {
	// gw 为 nil 时三个广播助手都必须静默 no-op（调度后台执行无人在线场景）
	broadcastResp(nil, gateway.RespError, "t", "d")
	broadcastEvent(nil, "sid", gateway.RespError, "t", "d")
	broadcastExecutionSummary(nil, "sid", goharnessevents.ExecutionSummaryData{}, "agent")
}

// broadcastTestHandlers 构造带真实 gateway 的广播回调集合（无人在线，
// BroadcastNotification 自然丢弃，安全覆盖全部回调分支）。
func broadcastTestHandlers(t *testing.T) (*Daemon, askEventHandlers, string) {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")
	if sess == nil {
		t.Fatal("loadTestSession returned nil")
	}

	withAgent := func() gateway.ResponseOption {
		return gateway.WithResponseMeta(map[string]any{"agent_name": "tester"})
	}
	h := newBroadcastAskHandlers(d, d.Gateway(), sid, withAgent, sess,
		func() string { return "tester" },
		func() string { return "" },
	)
	return d, h, sid
}

func TestNewBroadcastAskHandlers_Callbacks(t *testing.T) {
	d, h, sid := broadcastTestHandlers(t)

	// 无 panic 即通过（gateway 无在线客户端，广播自然丢弃）
	h.Thinking("thinking...")
	h.Content("content...")
	h.ToolUseDelta(goharnessevents.ToolUseDeltaData{Index: 0, ID: "c1", Name: "fs.read"})
	h.ThinkingDone()
	h.ToolStart(goharnessevents.ToolExecStartData{ToolName: "fs.read"})
	h.ToolEnd(goharnessevents.ToolExecEndData{ToolName: "fs.read", ToolCallID: "c1", Success: true})
	h.Answer("final answer")
	h.ExecutionSummary(goharnessevents.ExecutionSummaryData{TotalIterations: 2, ToolCalls: 1})
	h.LoopEnd(goharnessevents.CycleInfo{Iteration: 2})
	h.Compaction(goharnessevents.CompactionData{SessionID: sid, MessagesSlid: 5})
	h.MaxTurnsReached(goharnessevents.MaxTurnsReachedData{TurnsCompleted: 10, MaxTurns: 10})
	h.Error("boom")
	h.SubtaskSpawned(goharnessevents.SubtaskInfo{SessionID: "sub-1", AgentName: "tester"})
	h.SubtaskCompleted(goharnessevents.SubtaskResult{SessionID: "sub-1", Success: true})
	h.SubtaskTitleUpdated(goharnessevents.SubtaskTitle{SessionID: "sub-1", Title: "t"})
	h.TaskSummary(goharnessevents.TaskSummaryData{})
	h.LLMTimeout(goharnessevents.LLMTimeoutData{Elapsed: time.Second, Error: "timeout"})
	h.LLMCancelled(goharnessevents.LLMCancelledData{Elapsed: time.Second})
	h.LLMRetry(goharnessevents.LLMRetryData{SessionID: sid, Provider: "p", Model: "m", Attempt: 1, MaxAttempts: 3})
	h.TokenUsageRecorded(goharnesssession.TokenUsageRecord{ID: "r1", SessionID: sid, TotalTokens: 100})
	h.UserMessageSaved(12345)

	// ToolEnd 子会话分支：evSid != sid 提前返回，不触发文件 diff 广播
	h.ToolEnd(goharnessevents.ToolExecEndData{ToolName: "fs.write"})

	_ = d
	_ = sid
}

func TestNewBroadcastAskHandlers_SessionBlocks(t *testing.T) {
	d, h, sid := broadcastTestHandlers(t)

	// AskUserPending：登记 ask 挂起
	h.AskUserPending(goharnessevents.AskUserPendingData{})
	if got := d.sessionBlockOf(sid); got != "ask" {
		t.Errorf("sessionBlockOf after AskUserPending = %q, want ask", got)
	}
	d.clearSessionBlock(sid)

	// PermissionPending：登记 permission 挂起
	h.PermissionPending(goharnessevents.PermissionPendingData{ToolName: "fs.write"})
	if got := d.sessionBlockOf(sid); got != "permission" {
		t.Errorf("sessionBlockOf after PermissionPending = %q, want permission", got)
	}
	d.clearSessionBlock(sid)

	// 子会话提问：挂起登记到父会话 + 反查线索
	h.AskUserPending(goharnessevents.AskUserPendingData{SessionID: "sub-7"})
	if got := d.sessionBlockOf(sid); got != "ask" {
		t.Errorf("parent block after sub AskUserPending = %q, want ask", got)
	}
	if !d.resolveSubBlockAnswer("sub-7") {
		t.Error("resolveSubBlockAnswer should hit for registered sub")
	}
	if got := d.sessionBlockOf(sid); got != "" {
		t.Errorf("parent ask block should be cleared by sub answer, got %q", got)
	}

	// markSessionBlock 空参防御
	d.markSessionBlock("", "ask")
	d.markSubBlockParent("", "p")
	if got := d.sessionBlockOf(""); got != "" {
		t.Errorf("empty session block = %q, want empty", got)
	}
}
