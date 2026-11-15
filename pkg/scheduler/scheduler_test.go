package scheduler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// soonCronExpr 返回「从 now 起 delay 之后那一秒」触发的 6 段 cron 表达式
// （秒级粒度，WithSeconds 调度器专用）。字段按目标时刻取值，跨分/时边界安全。
func soonCronExpr(delay time.Duration) string {
	t := time.Now().Add(delay)
	return fmt.Sprintf("%d %d %d %d %d *", t.Second(), t.Minute(), t.Hour(), t.Day(), t.Month())
}

func mustSaveEntry(t *testing.T, store *FileSchedulerStore, e *ScheduleEntry) {
	t.Helper()
	if err := store.Save(context.Background(), e); err != nil {
		t.Fatalf("save entry %s: %v", e.ID, err)
	}
}

func newTestEntry(id, cronExpr string, enabled bool) *ScheduleEntry {
	now := time.Now()
	return &ScheduleEntry{
		ID:        id,
		Agent:     "assistant",
		SessionID: "sess-1",
		Content:   "测试任务内容",
		CronExpr:  cronExpr,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// waitFor 轮询断言条件，超时 fail（避免测试里散落 sleep）。
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待超时：%s（%v 内未满足）", what, timeout)
}

// TestSchedulerStartRegistersEnabledOnly 验证启动装载：启用任务注册进 cron，
// 禁用任务不注册（reloadAll 对 Enabled=false 走 removeJob 分支）。
func TestSchedulerStartRegistersEnabledOnly(t *testing.T) {
	store, err := NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	mustSaveEntry(t, store, newTestEntry("job-on", "* * * * * *", true))
	mustSaveEntry(t, store, newTestEntry("job-off", "* * * * * *", false))

	// 传入已取消的 ctx：watchLoop 立即退出，不污染测试
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewScheduler(store, nil)
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	if _, ok := s.entries["job-on"]; !ok {
		t.Error("启用的任务应已注册进 cron")
	}
	if s.cronExprs["job-on"] != "* * * * * *" {
		t.Errorf("cronExprs = %q, want * * * * * *", s.cronExprs["job-on"])
	}
	if _, ok := s.entries["job-off"]; ok {
		t.Error("禁用的任务不应注册进 cron")
	}
}

// TestSchedulerJobFiresLifecycleAndOneShotDisable 验证到点触发全链：
// executeJob 发射 started 生命周期事件（透传 entry 字段），一次性任务触发后
// 自动禁用，不再二次触发。
func TestSchedulerJobFiresLifecycleAndOneShotDisable(t *testing.T) {
	store, err := NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	entry := newTestEntry("job-fire", soonCronExpr(2*time.Second), true)
	entry.OneShot = true
	entry.ProjectDir = t.TempDir()
	mustSaveEntry(t, store, entry)

	fired := make(chan JobLifecycleInfo, 4)
	s := NewScheduler(store, nil)
	s.OnLifecycle(func(info JobLifecycleInfo) {
		if info.Status == "started" {
			fired <- info
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	select {
	case info := <-fired:
		if info.EntryID != "job-fire" {
			t.Errorf("EntryID = %q, want job-fire", info.EntryID)
		}
		if info.RunID == "" {
			t.Error("RunID 不应为空")
		}
		if info.Agent != "assistant" || info.SessionID != "sess-1" || info.Content != "测试任务内容" {
			t.Errorf("生命周期事件字段透传不完整: %+v", info)
		}
		if info.ProjectDir == "" {
			t.Error("ProjectDir 不应为空")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("10s 内未收到 started 生命周期事件")
	}

	// 一次性任务：触发后自动禁用（executeJob 内 store.Disable）
	waitFor(t, 3*time.Second, "一次性任务自动禁用", func() bool {
		e, err := store.Load(context.Background(), "job-fire")
		return err == nil && !e.Enabled
	})
}

// TestSchedulerReportResultUpdatesStoreAndBroadcast 验证终态回写：
// ReportResult 更新 store 计数/状态并发射 completed/failed 生命周期事件，
// 事件字段从 store 回读补全。
func TestSchedulerReportResultUpdatesStoreAndBroadcast(t *testing.T) {
	store, err := NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	mustSaveEntry(t, store, newTestEntry("job-report", "* * * * * *", true))

	var events []JobLifecycleInfo
	s := NewScheduler(store, nil)
	s.OnLifecycle(func(info JobLifecycleInfo) { events = append(events, info) })

	s.ReportResult("job-report", "run-1", nil)
	if len(events) != 1 || events[0].Status != "completed" {
		t.Fatalf("成功回写应发射 completed, got %+v", events)
	}
	if events[0].Agent != "assistant" || events[0].SessionID != "sess-1" {
		t.Errorf("completed 事件应从 store 回读字段: %+v", events[0])
	}
	e, err := store.Load(context.Background(), "job-report")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e.LastStatus != "success" || e.SuccessCnt != 1 || e.LastRunID != "run-1" {
		t.Errorf("store 终态未落: %+v", e)
	}

	s.ReportResult("job-report", "run-2", errors.New("boom"))
	if len(events) != 2 || events[1].Status != "failed" || events[1].Error != "boom" {
		t.Fatalf("失败回写应发射 failed 且带错误: %+v", events)
	}
	e, err = store.Load(context.Background(), "job-report")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e.LastStatus != "failed" || e.FailureCnt != 1 || e.LastError != "boom" {
		t.Errorf("失败终态未落: %+v", e)
	}

	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != "job-report" {
		t.Errorf("List 应透传 store 全量条目: %v %+v", err, list)
	}
}

// TestSchedulerReloadAllDiffsRegistry 验证 watchLoop 的注册集比对逻辑：
// 新增注册 / 禁用摘除 / 表达式变更重建 / 删除清退，注册集无变化时不误报
// entriesChanged。
func TestSchedulerReloadAllDiffsRegistry(t *testing.T) {
	store, err := NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	s := NewScheduler(store, nil)

	changedCount := 0
	s.OnEntriesChanged(func() { changedCount++ })

	// 新增：注册 + 触发 entriesChanged
	mustSaveEntry(t, store, newTestEntry("job-a", "0 0 8 * * *", true))
	if err := s.reloadAll(); err != nil {
		t.Fatalf("reloadAll: %v", err)
	}
	if _, ok := s.entries["job-a"]; !ok {
		t.Fatal("新增任务应被注册")
	}
	if changedCount != 1 {
		t.Fatalf("新增后 entriesChanged 应触发 1 次, got %d", changedCount)
	}

	// 无变化：不触发 entriesChanged
	if err := s.reloadAll(); err != nil {
		t.Fatalf("reloadAll: %v", err)
	}
	if changedCount != 1 {
		t.Fatalf("无变化时不应触发 entriesChanged, got %d", changedCount)
	}

	// 禁用：摘除注册
	if err := store.Disable("job-a"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if err := s.reloadAll(); err != nil {
		t.Fatalf("reloadAll: %v", err)
	}
	if _, ok := s.entries["job-a"]; ok {
		t.Fatal("禁用后应从 cron 摘除")
	}

	// 重新启用且表达式变更：按新表达式重建
	updated := newTestEntry("job-a", "0 30 9 * * *", true)
	updated.UpdatedAt = time.Now()
	mustSaveEntry(t, store, updated)
	if err := s.reloadAll(); err != nil {
		t.Fatalf("reloadAll: %v", err)
	}
	if s.cronExprs["job-a"] != "0 30 9 * * *" {
		t.Errorf("表达式变更后应重建, got %q", s.cronExprs["job-a"])
	}

	// 从文件删除：清退注册
	if err := store.Delete(context.Background(), "job-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.reloadAll(); err != nil {
		t.Fatalf("reloadAll: %v", err)
	}
	if _, ok := s.entries["job-a"]; ok {
		t.Fatal("文件删除后应清退注册")
	}
}

// TestSchedulerExecuteJobWithoutCallback 验证无生命周期回调时到点执行不 panic
// （防御：Daemon 未注入回调的装配路径）。
func TestSchedulerExecuteJobWithoutCallback(t *testing.T) {
	store, err := NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSchedulerStore: %v", err)
	}
	entry := newTestEntry("job-plain", "* * * * * *", true)
	s := NewScheduler(store, nil)
	s.executeJob(entry) // 不应 panic
}
