package cron

import "time"

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusError   JobStatus = "error"
)

type Job struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Cron        string                 `json:"cron"`
	Skill       string                 `json:"skill"`
	Params      map[string]any          `json:"params"`
	Command     string                 `json:"command"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	LastStatus  JobStatus              `json:"last_status"`
	LastError   *string                `json:"last_error,omitempty"`
}
