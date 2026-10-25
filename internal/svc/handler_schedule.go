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

	// 一次性任务：scheduled_at → 一次性 cron，到点执行后自动禁用。
	oneShot := false
	if p.ScheduledAt != "" {
		cronExpr, cronErr := scheduler.OneShotCron(p.ScheduledAt, time.Now())
		if cronErr != nil {
			return nil, cronErr
		}
		p.CronExpr = cronExpr
		oneShot = true
	}

	// 表达式合法性校验：引擎为 6 字段秒级解析，5 字段写法会保存成功却永不触发。
	if err := scheduler.ValidateCronExpr(p.CronExpr); err != nil {
		return nil, err
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

// handleScheduleUpdate 编辑既有任务：覆盖可变字段（内容/时刻/启停），
// 保留 ID、创建时间、执行统计与留痕。调度器 watchLoop（5 秒）感知
// cron 变更后自动按新表达式重建注册。
func (d *Daemon) handleScheduleUpdate(_ context.Context, params json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	var p rpc.ScheduleUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("schedule id is required")
	}
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

	entry, err := d.schedulerDB.Load(context.Background(), p.ID)
	if err != nil {
		return nil, fmt.Errorf("load schedule entry failed: %w", err)
	}

	// 雇佣校验与创建同口径：改绑的智能体必须已雇佣
	if agents := d.app.Agents(); agents != nil {
		cfg := agents.Get(p.Agent)
		if cfg == nil {
			return nil, fmt.Errorf("未找到智能体 %q，请确认名称是否正确", p.Agent)
		}
		if !core.AgentIsHired(cfg) {
			return nil, fmt.Errorf("智能体 %q 尚未雇佣，无法绑定定时任务（可执行 mindx agent hire %s 启用）", p.Agent, p.Agent)
		}
	}

	oneShot := false
	if p.ScheduledAt != "" {
		cronExpr, cronErr := scheduler.OneShotCron(p.ScheduledAt, time.Now())
		if cronErr != nil {
			return nil, cronErr
		}
		p.CronExpr = cronExpr
		oneShot = true
	}
	if err := scheduler.ValidateCronExpr(p.CronExpr); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		p.SessionID = d.resolveLatestSessionForAgent(p.Agent)
	}

	entry.Agent = p.Agent
	entry.SessionID = p.SessionID
	entry.ProjectDir = p.ProjectDir
	entry.Content = p.Content
	entry.CronExpr = p.CronExpr
	entry.Enabled = p.Enabled
	entry.UpdatedAt = time.Now()
	if oneShot {
		entry.OneShot = true
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

// enqueueOneJobAt 统一入队口：at 为零值 ≈ 10 秒后开工（普通信/即时场景），
// 非零 = 指定时刻开工（任务信按截止时刻排期）。调度器是唯一的执行主体：
// 雇佣校验、一次性条目生成、后台拉起、留痕全走既有 Scheduler 通路。
func (d *Daemon) enqueueOneJobAt(agent, content string, at time.Time) error {
	if d.schedulerDB == nil {
		return fmt.Errorf("本机调度器未就绪，无法叫醒智能体")
	}
	// 雇佣校验与 schedule.add 同口径：未雇佣的 Agent 不拉起
	if agents := d.app.Agents(); agents != nil {
		cfg := agents.Get(agent)
		if cfg == nil {
			return fmt.Errorf("未找到智能体 %q，请确认名称是否正确", agent)
		}
		if !core.AgentIsHired(cfg) {
			return fmt.Errorf("智能体 %q 尚未雇佣，无法开工", agent)
		}
	}
	// 即时场景给 10 秒余量：watchLoop 重载间隔 5 秒，留一倍余量避免恰好在重载间隙被跳过
	if at.IsZero() {
		at = time.Now().Add(10 * time.Second)
	}
	scheduledAt := at.Format(time.RFC3339)
	cronExpr, err := scheduler.OneShotCron(scheduledAt, time.Now())
	if err != nil {
		return err
	}
	if err := scheduler.ValidateCronExpr(cronExpr); err != nil {
		return err
	}
	entry := &scheduler.ScheduleEntry{
		ID:        uuid.NewString()[:8],
		Agent:     agent,
		SessionID: d.resolveLatestSessionForAgent(agent),
		Content:   content,
		CronExpr:  cronExpr,
		Enabled:   true,
		OneShot:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return d.schedulerDB.Save(context.Background(), entry)
}
