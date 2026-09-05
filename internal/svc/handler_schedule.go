package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
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

func (d *Daemon) handleScheduleAdd(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p rpc.ScheduleAddParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// Strip @ prefix if present — agent names are stored without @ in the
	// agent registry (goharness convention). MindX uses @ as a display prefix.
	p.Agent = strings.TrimPrefix(p.Agent, "@")

	if p.Agent == "" {
		return nil, fmt.Errorf("agent is required")
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if p.CronExpr == "" && p.ScheduledAt == "" {
		return nil, fmt.Errorf("cron_expr or scheduled_at is required")
	}

	// 雇佣校验：定时任务绑定的智能体必须已雇佣，防止未雇佣 Agent 被 cron 拉起执行
	if agents := d.app.Agents(); agents != nil {
		cfg := agents.Get(p.Agent)
		if cfg == nil {
			return nil, fmt.Errorf("未找到智能体 %q，请确认名称是否正确", p.Agent)
		}
		if !core.AgentIsHired(cfg) {
			return nil, fmt.Errorf("智能体 %q 尚未雇佣，无法绑定定时任务（可执行 mindx agent hire %s 启用）", p.Agent, p.Agent)
		}
	}

	// 一次性任务：scheduled_at → 一次性 cron（秒 分 时 日 月 *），到点执行后自动禁用。
	oneShot := false
	if p.ScheduledAt != "" {
		oneShotTime, parseErr := parseOneShotTime(p.ScheduledAt)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid scheduled_at %q: %w", p.ScheduledAt, parseErr)
		}
		if oneShotTime.Before(time.Now()) {
			return nil, fmt.Errorf("scheduled_at 时间已过，请重新选择")
		}
		p.CronExpr = fmt.Sprintf("%d %d %d %d %d *",
			oneShotTime.Second(), oneShotTime.Minute(), oneShotTime.Hour(),
			oneShotTime.Day(), int(oneShotTime.Month()))
		oneShot = true
	}

	// 前端创建调度任务时通常不传 session_id：回退到该智能体最近活动的会话，
	// 否则任务触发时无会话可用（load session 会因空 ID 失败）。
	if p.SessionID == "" {
		p.SessionID = d.resolveLatestSessionForAgent(p.Agent)
	}

	entry := &scheduler.ScheduleEntry{
		ID:         uuid.NewString()[:8],
		Agent:      p.Agent,
		SessionID:  p.SessionID,
		ProjectDir: p.ProjectDir,
		Content:    p.Content,
		CronExpr:   p.CronExpr,
		Enabled:    true,
		OneShot:    oneShot,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := d.schedulerDB.Save(context.Background(), entry); err != nil {
		return nil, fmt.Errorf("save schedule failed: %w", err)
	}

	return entry, nil
}

func (d *Daemon) handleScheduleDelete(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p rpc.ScheduleDeleteParams
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

// handleScheduleJobCancel 取消一次已触发但尚未执行的任务运行（前端通知框「取消」操作）。
// 仅当该运行仍处于 started 状态时生效并广播 missed，避免误标已完成/已失败的运行。
func (d *Daemon) handleScheduleJobCancel(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p rpc.ScheduleJobCancelParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.EntryID == "" || p.RunID == "" {
		return nil, fmt.Errorf("entry_id and run_id are required")
	}

	entry, err := d.schedulerDB.Load(context.Background(), p.EntryID)
	if err != nil {
		return nil, fmt.Errorf("load schedule entry failed: %w", err)
	}
	if entry.LastRunID != p.RunID || entry.LastStatus != "started" {
		// 该运行已结束或不存在：无需取消，幂等返回。
		return map[string]string{"status": "already_finished"}, nil
	}

	if err := d.schedulerDB.CancelRun(p.EntryID, p.RunID); err != nil {
		return nil, fmt.Errorf("cancel job run failed: %w", err)
	}

	// 广播 missed，使前端出队对应待执行任务并刷新生命周期记录。
	info := scheduler.JobLifecycleInfo{
		EntryID:   p.EntryID,
		RunID:     p.RunID,
		Agent:     entry.Agent,
		SessionID: entry.SessionID,
		Status:    "missed",
		Error:     "用户取消",
	}
	d.broadcastJobLifecycle("schedule.job_missed", info)

	return map[string]string{"status": "cancelled"}, nil
}

// parseOneShotTime 解析前端单次任务时间：优先本地时区格式
// "YYYY-MM-DDTHH:mm:ss"，其次 RFC3339；均失败则返回错误。
func parseOneShotTime(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
