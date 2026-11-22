package svc

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/pkg/scheduler"
)

// newSchedFixture 给 daemon 挂上带终态事件采集的调度器与同源存储，
// 返回存储与终态事件通道。
func newSchedFixture(t *testing.T, d *Daemon) (*scheduler.FileSchedulerStore, chan scheduler.JobLifecycleInfo) {
	t.Helper()
	store, err := scheduler.NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	s := scheduler.NewScheduler(store, nil)
	evt := make(chan scheduler.JobLifecycleInfo, 4)
	s.OnLifecycle(func(info scheduler.JobLifecycleInfo) {
		if info.Status == "failed" || info.Status == "completed" {
			evt <- info
		}
	})
	d.scheduler = s
	d.schedulerDB = store
	return store, evt
}

// TestRunScheduledJob_NilScheduler 验证无调度器时直接返回不 panic。
func TestRunScheduledJob_NilScheduler(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	d.scheduler = nil
	d.runScheduledJob(scheduler.JobLifecycleInfo{EntryID: "x", RunID: "r"}) // 不应 panic
}

// TestRunScheduledJob_LoadEntryFailed 验证任务加载失败按 failed 回写，
// 状态不留悬空 started。
func TestRunScheduledJob_LoadEntryFailed(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, evt := newSchedFixture(t, d)

	d.runScheduledJob(scheduler.JobLifecycleInfo{EntryID: "missing", RunID: "run-1"})

	select {
	case info := <-evt:
		if info.Status != "failed" || info.EntryID != "missing" {
			t.Errorf("终态事件 = %+v, want failed/missing", info)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("加载失败未回写终态")
	}
}

// TestRunScheduledJob_NewSentinelResolvesSession 验证 "new" 哨兵兜底：
// Agent 的 Cron 工具会把 "new" 原样落库，执行侧必须现场落实会话，
// 落实后再走执行链（本用例 agent 不可解析，按 failed 回写为终局）。
func TestRunScheduledJob_NewSentinelResolvesSession(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	store, evt := newSchedFixture(t, d)
	projectDir := t.TempDir()
	if err := store.Save(context.Background(), &scheduler.ScheduleEntry{
		ID:         "job-new",
		Agent:      "no-such-agent",
		SessionID:  "new",
		ProjectDir: projectDir,
		Content:    "执行一次巡检",
		CronExpr:   "* * * * * *",
		Enabled:    true,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	d.runScheduledJob(scheduler.JobLifecycleInfo{EntryID: "job-new", RunID: "run-9"})

	select {
	case info := <-evt:
		if info.Status != "failed" {
			t.Errorf("agent 不可解析应按 failed 回写, got %s", info.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到终态回写")
	}

	// 哨兵已落实：项目目录分片下应出现一个绑定该 agent 的真实会话
	sessions, err := d.app.SessDB().ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	found := false
	for _, si := range sessions {
		if si.AgentName == "no-such-agent" && si.ProjectDir == projectDir {
			found = true
		}
	}
	if !found {
		t.Errorf("new 哨兵未落实会话: sessions=%v", sessions)
	}
}

// TestRunScheduledJob_ExistingSessionBadAgent 验证已有会话路径：
// 复用绑定会话（已有消息的会话被标题门控跳过），agent 不可解析时按 failed 回写。
func TestRunScheduledJob_ExistingSessionBadAgent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	store, evt := newSchedFixture(t, d)
	sid := mustCreateSession(t, d.app.SessDB(), "bound-agent")
	if err := store.Save(context.Background(), &scheduler.ScheduleEntry{
		ID:        "job-existing",
		Agent:     "no-such-agent",
		SessionID: sid,
		Content:   "续跑任务",
		CronExpr:  "* * * * * *",
		Enabled:   true,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	d.runScheduledJob(scheduler.JobLifecycleInfo{EntryID: "job-existing", RunID: "run-2"})

	select {
	case info := <-evt:
		if info.Status != "failed" {
			t.Errorf("agent 不可解析应按 failed 回写, got %s", info.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到终态回写")
	}
}

// TestRunBackgroundTurn_ResolveRuntimeFails 验证运行时不可解析时
// onDone 携带错误同步回调（收割轮询器/定时任务依赖 onDone 计数重投，
// onDone 丢失会让收割意图永远停在 reporting）。
func TestRunBackgroundTurn_ResolveRuntimeFails(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	done := make(chan error, 1)
	d.runBackgroundTurn("sess-x", "no-such-agent", "内容", "内容", scheduledCancelClientID, func(err error) {
		done <- err
	}, false)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("运行时不可解析应回调错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("onDone 未被调用")
	}
}

// TestRunBackgroundTurn_CancelSetForVirtualClient 验证虚拟客户端取消集合
// 可按键取用（message.cancel 按会话取消时需一并覆盖调度执行，依赖此集合）。
func TestRunBackgroundTurn_CancelSetForVirtualClient(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if set := d.clientCancelSetFor(scheduledCancelClientID); set == nil {
		t.Fatal("虚拟客户端取消集合不应为 nil")
	}
}

// TestWithAskBypassSinks_ContextWired 验证旁路 sink 注入路径可用：
// gw 为 nil（测试环境）时冒泡回调只做挂起登记、不广播、不 panic。
// sink 本体是 goharness 闭包（无公开读取器），登记效果经
// markSessionBlock 的消费点 sessionBlockOf 断言闭环。
func TestWithAskBypassSinks_ContextWired(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if got := d.sessionBlockOf("sess-bypass"); got != "" {
		t.Fatalf("初始挂起登记应为空, got %q", got)
	}

	ctx := d.withAskBypassSinks(context.Background(), "sess-bypass", func() string { return "assistant" })
	if ctx == nil {
		t.Fatal("withAskBypassSinks 不应返回 nil ctx")
	}

	// 挂起登记与解除闭环：markSessionBlock → sessionBlockOf → clearSessionBlock
	d.markSessionBlock("sess-bypass", "ask")
	if got := d.sessionBlockOf("sess-bypass"); got != "ask" {
		t.Errorf("挂起登记 = %q, want ask", got)
	}
	d.clearSessionBlock("sess-bypass")
	if got := d.sessionBlockOf("sess-bypass"); got != "" {
		t.Errorf("解除后挂起登记应为空, got %q", got)
	}
}
