package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/google/uuid"
)

func (d *Daemon) handleScheduleList(_ context.Context, _ json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	entries, err := d.schedulerDB.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list schedules failed: %w", err)
	}

	if entries == nil {
		return []any{}, nil
	}

	return entries, nil
}

type scheduleAddParams struct {
	Agent      string `json:"agent"`
	SessionID  string `json:"session_id"`
	ProjectDir string `json:"project_dir"`
	Content    string `json:"content"`
	CronExpr   string `json:"cron_expr"`
	Enabled    bool   `json:"enabled"`
}

func (d *Daemon) handleScheduleAdd(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p scheduleAddParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// Strip @ prefix if present — agent names are stored without @ in the
	// agent registry (goreact convention). MindX uses @ as a display prefix.
	p.Agent = strings.TrimPrefix(p.Agent, "@")

	if p.Agent == "" {
		return nil, fmt.Errorf("agent is required")
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if p.CronExpr == "" {
		return nil, fmt.Errorf("cron_expr is required")
	}

	entry := &scheduler.ScheduleEntry{
		ID:         uuid.NewString()[:8],
		Agent:      p.Agent,
		SessionID:  p.SessionID,
		ProjectDir: p.ProjectDir,
		Content:    p.Content,
		CronExpr:   p.CronExpr,
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := d.schedulerDB.Save(context.Background(), entry); err != nil {
		return nil, fmt.Errorf("save schedule failed: %w", err)
	}

	return entry, nil
}

type scheduleDeleteParams struct {
	ID string `json:"id"`
}

func (d *Daemon) handleScheduleDelete(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p scheduleDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	if p.ID == "" {
		return nil, fmt.Errorf("schedule id is required")
	}

	if err := d.schedulerDB.Delete(context.Background(), p.ID); err != nil {
		return nil, fmt.Errorf("delete schedule failed: %w", err)
	}

	return map[string]string{"status": "deleted", "id": p.ID}, nil
}
