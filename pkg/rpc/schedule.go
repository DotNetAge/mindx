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
}

// ScheduleDeleteParams are the params for schedule.del.
type ScheduleDeleteParams struct {
	ID string `json:"id"`
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
