package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gblogs "github.com/DotNetAge/goharness/logging"
	ghsession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

func newCronForTest(t *testing.T) tools.FuncTool {
	t.Helper()
	store, err := scheduler.NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatalf("初始化调度存储失败：%v", err)
	}
	return NewCron(store)
}

func TestCronCreateWithScheduledAt(t *testing.T) {
	cron := newCronForTest(t)
	ctx := context.Background()

	future := time.Now().Add(2 * time.Hour).Format("2006-01-02T15:04:05")
	out, err := cron.Execute(ctx, map[string]any{
		"action":       "create",
		"agent":        "writer",
		"content":      "巡检任务",
		"scheduled_at": future,
		"project_dir":  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("scheduled_at 创建失败：%v", err)
	}

	result := out.(map[string]any)
	id := result["id"].(string)
	entry := result["entry"].(*scheduler.ScheduleEntry)
	if !entry.OneShot {
		t.Fatalf("scheduled_at 创建的任务应为 OneShot，实际 %+v", entry)
	}
	fields := strings.Fields(entry.CronExpr)
	if len(fields) != 6 || fields[5] != "*" || fields[3] == "*" {
		t.Fatalf("一次性 cron 表达式不合预期：%q", entry.CronExpr)
	}

	// 落盘复核：从存储重读确认 OneShot 持久化。
	saved, err := cron.Execute(ctx, map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("list 失败：%v", err)
	}
	entries := saved.([]scheduler.ScheduleEntry)
	if len(entries) != 1 || entries[0].ID != id || !entries[0].OneShot {
		t.Fatalf("list 结果与创建不一致：%+v", entries)
	}
}

func TestCronCreateMissingPlan(t *testing.T) {
	cron := newCronForTest(t)

	_, err := cron.Execute(context.Background(), map[string]any{
		"action":  "create",
		"agent":   "writer",
		"content": "巡检任务",
	})
	if err == nil || !strings.Contains(err.Error(), "cron_expr 与 scheduled_at") {
		t.Fatalf("缺执行计划应报二选一错误，实际：%v", err)
	}
}

func TestCronCreateScheduledAtInPast(t *testing.T) {
	cron := newCronForTest(t)

	_, err := cron.Execute(context.Background(), map[string]any{
		"action":       "create",
		"agent":        "writer",
		"content":      "巡检任务",
		"scheduled_at": "2020-01-01T09:00:00",
	})
	if err == nil || !strings.Contains(err.Error(), "已过") {
		t.Fatalf("过去时间应报已过错误，实际：%v", err)
	}
}

func TestCronCreateWithoutProjectDirRejected(t *testing.T) {
	cron := newCronForTest(t)

	// 无工具上下文（拿不到当前会话工作目录）且未显式传 project_dir：
	// 必须在创建阶段拒绝，禁止静默落库到点才炸「project_dir 不能为空」
	_, err := cron.Execute(context.Background(), map[string]any{
		"action":    "create",
		"agent":     "writer",
		"content":   "巡检任务",
		"cron_expr": "0 0 9 * * *",
	})
	if err == nil || !strings.Contains(err.Error(), "工作目录") {
		t.Fatalf("缺工作目录应在创建阶段拒绝，实际：%v", err)
	}
}

func TestCronCreateKeepsProjectDir(t *testing.T) {
	cron := newCronForTest(t)
	dir := t.TempDir()

	out, err := cron.Execute(context.Background(), map[string]any{
		"action":      "create",
		"agent":       "writer",
		"content":     "巡检任务",
		"cron_expr":   "0 0 9 * * *",
		"project_dir": dir,
	})
	if err != nil {
		t.Fatalf("显式 project_dir 创建失败：%v", err)
	}
	entry := out.(map[string]any)["entry"].(*scheduler.ScheduleEntry)
	if entry.ProjectDir != dir {
		t.Fatalf("落库的工作目录不对：期望 %q，实际 %q", dir, entry.ProjectDir)
	}
}

// 未传 project_dir 时按当前会话的工作目录落库：这是大秘自建定时任务的常态路径，
// 缺省值必须在创建阶段就补齐，而不是留给到点执行时报「project_dir 不能为空」
func TestCronCreateDefaultsProjectDirFromSession(t *testing.T) {
	cron := newCronForTest(t)
	dir := t.TempDir()

	store, err := mindxses.NewRoutedSessionStore(filepath.Join(t.TempDir(), "session_dirs.json"))
	if err != nil {
		t.Fatalf("初始化会话存储失败：%v", err)
	}
	sess, err := ghsession.New("assistant", "", dir, store, gblogs.DefaultLogger())
	if err != nil {
		t.Fatalf("构造会话失败：%v", err)
	}
	ctx := tools.WithToolContext(context.Background(), &tools.ToolContext{Session: sess})

	out, err := cron.Execute(ctx, map[string]any{
		"action":    "create",
		"agent":     "writer",
		"content":   "巡检任务",
		"cron_expr": "0 0 9 * * *",
	})
	if err != nil {
		t.Fatalf("有会话上下文时创建失败：%v", err)
	}
	entry := out.(map[string]any)["entry"].(*scheduler.ScheduleEntry)
	if entry.ProjectDir != dir {
		t.Fatalf("未回填当前会话工作目录：期望 %q，实际 %q", dir, entry.ProjectDir)
	}
}
