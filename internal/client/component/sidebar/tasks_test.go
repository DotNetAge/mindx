package sidebar

import (
	"strings"
	"testing"
)

func TestRenderTasksEmpty(t *testing.T) {
	if out := renderTasks(nil, 0, 0, 30); out != "" {
		t.Errorf("empty task list should render nothing, got %q", out)
	}
}

func TestRenderTasksGlyphsByStatus(t *testing.T) {
	tasks := []Task{
		{ID: "1", Subject: "待办项", Status: TaskPending},
		{ID: "2", Subject: "进行中", Status: TaskInProgress},
		{ID: "3", Subject: "已完成", Status: TaskCompleted},
		{ID: "4", Subject: "已取消", Status: TaskCancelled},
	}
	out := renderTasks(tasks, 1, 4, 40)
	for _, want := range []string{"○", "●", "✓", "✕"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing glyph %q in:\n%s", want, out)
		}
	}
	// i18n 未初始化时标题退化为裸 key + EXTRA 标注，
	// 但 done/total 数值必须透传到渲染结果。
	if !strings.Contains(out, "1") || !strings.Contains(out, "4") {
		t.Errorf("missing progress numbers in:\n%s", out)
	}
}

func TestTaskProgressCounts(t *testing.T) {
	tasks := []Task{
		{Status: TaskCompleted},
		{Status: TaskCompleted},
		{Status: TaskInProgress},
		{Status: TaskPending},
	}
	done, total := taskProgress(tasks)
	if done != 2 || total != 4 {
		t.Errorf("progress = %d/%d, want 2/4", done, total)
	}
}
