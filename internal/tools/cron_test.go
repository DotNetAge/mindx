package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/mindx/pkg/scheduler"
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
