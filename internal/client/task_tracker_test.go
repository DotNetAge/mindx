package client

import (
	"testing"

	"github.com/DotNetAge/mindx/internal/client/component/sidebar"
)

// renderTasks 是 sidebar 包内私有函数，渲染断言移至 sidebar 包测试覆盖；
// 此处仅保留 tracker 状态机测试。

const (
	createResult   = `{"task_id":"t-1","status":"pending","subject":"重构输入组件"}`
	updateResult   = `{"success":true,"message":"任务 \"t-1\" 已更新","task_id":"t-1","status":"in_progress"}`
	completeResult = `{"success":true,"task_id":"t-1","status":"completed"}`
)

func TestTaskTrackerCreateAndTransitions(t *testing.T) {
	tr := newTaskTracker()

	if !tr.applyToolResult("TaskCreate", createResult) {
		t.Fatal("TaskCreate result should be applied")
	}
	snap := tr.snapshot()
	if len(snap) != 1 || snap[0].Status != sidebar.TaskPending || snap[0].Subject != "重构输入组件" {
		t.Fatalf("unexpected snapshot after create: %+v", snap)
	}

	if !tr.applyToolResult("TaskUpdate", updateResult) {
		t.Fatal("TaskUpdate result should be applied")
	}
	if snap[0].Status = tr.snapshot()[0].Status; snap[0].Status != sidebar.TaskInProgress {
		t.Errorf("expected in_progress, got %s", snap[0].Status)
	}

	if !tr.applyToolResult("TaskUpdate", completeResult) {
		t.Fatal("completion should be applied")
	}
	if got := tr.snapshot()[0].Status; got != sidebar.TaskCompleted {
		t.Errorf("expected completed, got %s", got)
	}
}

func TestTaskTrackerIgnoresOtherTools(t *testing.T) {
	tr := newTaskTracker()
	if tr.applyToolResult("Bash", `{"exit_code":0,"stdout":"hi"}`) {
		t.Error("non-task tool must not affect tracker")
	}
	if len(tr.snapshot()) != 0 {
		t.Error("tracker should stay empty")
	}
}

func TestTaskTrackerMalformedJSONNoop(t *testing.T) {
	tr := newTaskTracker()
	for _, bad := range []string{"", "not json", `{"other":1}`, `{"task_id":""}`} {
		if tr.applyToolResult("TaskCreate", bad) {
			t.Errorf("malformed payload %q must be rejected", bad)
		}
	}
}

// Update 先于 Create 到达（乱序）时忽略，不产生幽灵任务。
func TestTaskTrackerUpdateBeforeCreateIgnored(t *testing.T) {
	tr := newTaskTracker()
	if tr.applyToolResult("TaskUpdate", updateResult) {
		t.Error("update for unknown task must be ignored")
	}
	if len(tr.snapshot()) != 0 {
		t.Error("no ghost task should be created")
	}
}

func TestTaskTrackerOrderPreservedAndReset(t *testing.T) {
	tr := newTaskTracker()
	tr.applyToolResult("TaskCreate", `{"task_id":"b","subject":"B","status":"pending"}`)
	tr.applyToolResult("TaskCreate", `{"task_id":"a","subject":"A","status":"pending"}`)

	snap := tr.snapshot()
	if len(snap) != 2 || snap[0].ID != "b" || snap[1].ID != "a" {
		t.Fatalf("creation order not preserved: %+v", snap)
	}

	tr.reset()
	if len(tr.snapshot()) != 0 {
		t.Error("reset should clear all tasks")
	}
}
