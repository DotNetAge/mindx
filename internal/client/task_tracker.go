package client

import (
	"encoding/json"

	"github.com/DotNetAge/mindx/internal/client/component/sidebar"
)

// taskTracker 从 TaskCreate/TaskUpdate 工具的执行结果中提取任务状态，
// 维护当前会话的任务快照表并推送到侧栏。
//
// 数据源是现有的 tool_exec_end 事件流（goharness 任务工具本身会把
// task_id/status/subject 放进返回 JSON），因此零新增后端协议。
type taskTracker struct {
	order []string // 保持创建顺序
	tasks map[string]sidebar.Task
}

func newTaskTracker() *taskTracker {
	return &taskTracker{tasks: map[string]sidebar.Task{}}
}

// applyToolResult 解析工具执行结果；非任务工具或解析失败时返回 false（无副作用）。
// 工具结果格式见 goharness/tools/task_create.go 与 task_update.go 的返回值。
func (t *taskTracker) applyToolResult(toolName, result string) bool {
	switch toolName {
	case "TaskCreate":
		return t.applyCreate(result)
	case "TaskUpdate":
		return t.applyUpdate(result)
	default:
		return false
	}
}

func (t *taskTracker) applyCreate(result string) bool {
	var payload struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil || payload.TaskID == "" {
		return false
	}
	if _, exists := t.tasks[payload.TaskID]; !exists {
		t.order = append(t.order, payload.TaskID)
	}
	status := sidebar.TaskStatus(payload.Status)
	if status == "" {
		status = sidebar.TaskPending
	}
	t.tasks[payload.TaskID] = sidebar.Task{
		ID:      payload.TaskID,
		Subject: payload.Subject,
		Status:  status,
	}
	return true
}

func (t *taskTracker) applyUpdate(result string) bool {
	var payload struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil || payload.TaskID == "" {
		return false
	}
	task, ok := t.tasks[payload.TaskID]
	if !ok {
		return false
	}
	if payload.Status != "" {
		task.Status = sidebar.TaskStatus(payload.Status)
	}
	t.tasks[payload.TaskID] = task
	return true
}

// snapshot 返回按创建顺序排列的任务列表副本。
func (t *taskTracker) snapshot() []sidebar.Task {
	out := make([]sidebar.Task, 0, len(t.order))
	for _, id := range t.order {
		out = append(out, t.tasks[id])
	}
	return out
}

// reset 清空追踪状态（会话切换时调用）。
func (t *taskTracker) reset() {
	t.order = nil
	t.tasks = map[string]sidebar.Task{}
}
