package svc

import (
	"fmt"

	goharnessevents "github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string, opts ...gateway.ResponseOption) {
	if d.gw == nil {
		return
	}
	allOpts := append([]gateway.ResponseOption{gateway.WithSessionID(sessionID)}, opts...)
	_ = d.gw.SendResponse(clientID, respType, title, data, allOpts...)
}

// broadcastJobLifecycle 统一广播调度任务生命周期事件（job_started/completed/failed/missed）。
// payload 携带 entry_id/run_id/agent/session_id/content 等完整信息，
// meta.agent_name 供前端取智能体名。
func (d *Daemon) broadcastJobLifecycle(method string, info scheduler.JobLifecycleInfo) {
	if d.gw == nil {
		return
	}
	d.gw.BroadcastNotification(method, map[string]any{
		"session_id": info.SessionID,
		"agent":      info.Agent,
		"type":       info.Status,
		"data":       info,
		"meta":       map[string]any{"agent_name": info.Agent},
	})
}

// broadcastScheduleChanged 任务库增删改后广播（handler 保存后即时发，
// watchLoop 感知手写/CLI 落盘变更后兜底发）。payload 空体，前端收到即重拉
// schedule.list 刷新角标与列表；重复广播无害（拉取幂等）。
func (d *Daemon) broadcastScheduleChanged() {
	if d.gw == nil {
		return
	}
	d.gw.BroadcastNotification("schedule.entries_changed", map[string]any{})
}

func buildTaskSummaryMarkdown(ts goharnessevents.TaskSummaryData) string {
	return fmt.Sprintf("### %s\n\n%s\n",
		i18n.T("svc.md.task.summary"), ts.Summary)
}
