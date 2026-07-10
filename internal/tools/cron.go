package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/google/uuid"
)

// Cron manages scheduled tasks via the scheduler store.
// It provides create, read, update, delete, and list operations for cron entries.
type Cron struct {
	store *scheduler.FileSchedulerStore
}

// NewCron creates a Cron tool backed by the given scheduler store.
func NewCron(store *scheduler.FileSchedulerStore) tools.FuncTool {
	return &Cron{store: store}
}

func (t *Cron) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "Cron",
		Description: "Manage scheduled cron tasks. Create, list, update, and delete scheduled tasks that run periodically on a cron schedule.",
		Prompt: `Manage cron-scheduled tasks. Each task runs a prompt against an agent on a cron schedule.

Actions:
- **list**: List all scheduled tasks with their status and next run info.
- **create**: Create a new scheduled task. Requires id (optional — auto-generated if omitted), agent, content (the prompt to send), and cron_expr (6-field cron expression with seconds support, e.g. "0 0 9 * * *" for daily at 9am).
- **update**: Update an existing scheduled task by id. Only provided fields will be updated.
- **delete**: Delete a scheduled task by id.

Use this whenever the user wants to set up a recurring task like "send a status report every morning at 9am" or "check for updates every hour".`,
		IsReadOnly: false,
		Parameters: []tools.Parameter{
			{
				Name:        "action",
				Type:        "string",
				Description: "Operation to perform: \"list\", \"create\", \"update\", or \"delete\".",
				Required:    true,
				Enum:        []any{"list", "create", "update", "delete"},
			},
			{
				Name:        "id",
				Type:        "string",
				Description: "Schedule entry ID. Required for update and delete. Optional for create (auto-generated if omitted).",
				Required:    false,
			},
			{
				Name:        "agent",
				Type:        "string",
				Description: "Agent name to run the task against (e.g. \"myagent\"). Required for create.",
				Required:    false,
			},
			{
				Name:        "content",
				Type:        "string",
				Description: "The prompt/content to send to the agent when the task runs. Required for create.",
				Required:    false,
			},
			{
				Name:        "cron_expr",
				Type:        "string",
				Description: "6-field cron expression with seconds support (e.g. \"0 0 9 * * *\" for daily at 9am, \"0 */30 * * * *\" for every 30 minutes). Required for create.",
				Required:    false,
			},
			{
				Name:        "enabled",
				Type:        "boolean",
				Description: "Whether the task is enabled (default: true for new entries).",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "session_id",
				Type:        "string",
				Description: "Session ID to associate with the task. If empty or \"new\", a new session will be created on each run.",
				Required:    false,
			},
			{
				Name:        "project_dir",
				Type:        "string",
				Description: "Working directory for the task execution.",
				Required:    false,
			},
		},
	}
}

func (t *Cron) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.store == nil {
		return nil, fmt.Errorf("Cron: scheduler store is not available")
	}

	action, err := tools.ValidateRequiredString(params, "action")
	if err != nil {
		return nil, err
	}

	switch action {
	case "list":
		return t.listEntries(ctx)
	case "create":
		return t.createEntry(ctx, params)
	case "update":
		return t.updateEntry(ctx, params)
	case "delete":
		return t.deleteEntry(ctx, params)
	default:
		return nil, fmt.Errorf("Cron: unknown action %q", action)
	}
}

func (t *Cron) listEntries(ctx context.Context) (any, error) {
	entries, err := t.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cron: failed to list entries: %w", err)
	}
	if entries == nil {
		entries = []scheduler.ScheduleEntry{}
	}
	return entries, nil
}

func (t *Cron) createEntry(ctx context.Context, params map[string]any) (any, error) {
	agent, err := tools.ValidateRequiredString(params, "agent")
	if err != nil {
		return nil, fmt.Errorf("Cron: agent is required for create: %w", err)
	}
	content, err := tools.ValidateRequiredString(params, "content")
	if err != nil {
		return nil, fmt.Errorf("Cron: content is required for create: %w", err)
	}
	cronExpr, err := tools.ValidateRequiredString(params, "cron_expr")
	if err != nil {
		return nil, fmt.Errorf("Cron: cron_expr is required for create: %w", err)
	}

	id, _ := params["id"].(string)
	if id == "" {
		id = uuid.NewString()[:8]
	}

	enabled := true
	if raw, ok := params["enabled"]; ok {
		if v, ok := raw.(bool); ok {
			enabled = v
		}
	}

	sessionID, _ := params["session_id"].(string)
	projectDir, _ := params["project_dir"].(string)

	entry := &scheduler.ScheduleEntry{
		ID:         id,
		Agent:      agent,
		Content:    content,
		CronExpr:   cronExpr,
		Enabled:    enabled,
		SessionID:  sessionID,
		ProjectDir: projectDir,
		CreatedAt:  time.Now(),
	}

	if err := t.store.Save(ctx, entry); err != nil {
		return nil, fmt.Errorf("Cron: failed to save entry: %w", err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("Scheduled task %q created successfully", id),
		"entry":   entry,
	}, nil
}

func (t *Cron) updateEntry(ctx context.Context, params map[string]any) (any, error) {
	id, err := tools.ValidateRequiredString(params, "id")
	if err != nil {
		return nil, fmt.Errorf("Cron: id is required for update: %w", err)
	}

	existing, err := t.store.Load(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Cron: entry %q not found: %w", id, err)
	}

	if v, ok := params["agent"].(string); ok && v != "" {
		existing.Agent = v
	}
	if v, ok := params["content"].(string); ok && v != "" {
		existing.Content = v
	}
	if v, ok := params["cron_expr"].(string); ok && v != "" {
		existing.CronExpr = v
	}
	if v, ok := params["enabled"].(bool); ok {
		existing.Enabled = v
	}
	if v, ok := params["session_id"].(string); ok {
		existing.SessionID = v
	}
	if v, ok := params["project_dir"].(string); ok {
		existing.ProjectDir = v
	}

	if err := t.store.Save(ctx, existing); err != nil {
		return nil, fmt.Errorf("Cron: failed to update entry: %w", err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("Scheduled task %q updated successfully", id),
		"entry":   existing,
	}, nil
}

func (t *Cron) deleteEntry(ctx context.Context, params map[string]any) (any, error) {
	id, err := tools.ValidateRequiredString(params, "id")
	if err != nil {
		return nil, fmt.Errorf("Cron: id is required for delete: %w", err)
	}

	if err := t.store.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("Cron: failed to delete entry: %w", err)
	}

	return map[string]any{
		"id":      id,
		"message": fmt.Sprintf("Scheduled task %q deleted successfully", id),
	}, nil
}
