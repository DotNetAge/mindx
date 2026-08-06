package rpc

import "encoding/json"

// ScheduleAddParams are the params for schedule.add.
type ScheduleAddParams struct {
	Agent      string `json:"agent"`
	SessionID  string `json:"session_id,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
	Content    string `json:"content"`
	CronExpr   string `json:"cron_expr"`
	Enabled    bool   `json:"enabled,omitempty"`
	// ScheduledAt 一次性任务的触发时间（如 "2026-08-06T14:30:00"）。
	// 提供时由服务端转为一次性 cron 表达式（秒 分 时 日 月 *），
	// 并在任务到点执行后自动禁用，与 CronExpr 二选一。
	ScheduledAt string `json:"scheduled_at,omitempty"`
}

// ScheduleDeleteParams are the params for schedule.del.
type ScheduleDeleteParams struct {
	ID string `json:"id"`
}

// ScheduleJobCancelParams are the params for schedule.job_cancel.
// 前端取消一次已触发但尚未执行（status=started）的任务运行。
type ScheduleJobCancelParams struct {
	EntryID string `json:"entry_id"`
	RunID   string `json:"run_id"`
}

func (c *Client) ScheduleList() (json.RawMessage, error) {
	return c.CallWithTimeout("schedule.list", nil)
}

func (c *Client) ScheduleAdd(params ScheduleAddParams) (json.RawMessage, error) {
	return c.CallWithTimeout("schedule.add", params)
}

func (c *Client) ScheduleDelete(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("schedule.del", ScheduleDeleteParams{ID: id})
}

func (c *Client) ScheduleJobCancel(entryID, runID string) (json.RawMessage, error) {
	return c.CallWithTimeout("schedule.job_cancel", ScheduleJobCancelParams{EntryID: entryID, RunID: runID})
}
