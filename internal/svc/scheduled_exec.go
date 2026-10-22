package svc

// 调度任务后台执行链（2026-10-21 定案回归，恢复 aa24dc8 删除前的设计意图）：
// Scheduler 供 Agent 用 Cron 工具创建定时任务；到点后由 daemon 直接拉起
// Runtime Ask Agent 在后台执行，不依赖任何客户端在线。执行事件经
// newBroadcastAskHandlers 广播到在线客户端仅作通知呈现（无人在线则事件
// 自然丢弃），会话历史照常落盘，前端打开绑定会话即可回放。执行结束
// ReportResult 回写任务状态并广播 completed/failed，前端日历据此刷新。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"

	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

// scheduledCancelClientID 是后台执行在 clientCancels 中的虚拟客户端键：
// 调度任务不属于任何客户端，message.cancel 按会话取消时需一并查此集合。
const scheduledCancelClientID = "__scheduled__"

// withAskBypassSinks 注入子智能体授权/提问冒泡旁路（前台与后台执行共用）。
//
// 子会话（spawn 的子 Agent 执行循环）触发授权请求或 AskUser 提问时，不依赖
// 父 exec EventBus 的存活（父 exec 结束/被取消后订阅销毁，parentEmit 转发
// 链路会静默丢事件），而是经此发送器直接广播到所有客户端。envelope 结构
// （session_id / meta.agent_name / title / data / type）与 wireAskEvents 中
// 对应 emitter 的发送一致，前端解析无差异；data.session_id 为发起方子会话 ID，
// 前端据此精确渲染/作答。主会话自身的授权与提问不走旁路（b.permissionCh ==
// nil / b.askCh == nil），仍经原 EventBus 转发链路。
// agentName 按闭包引用：闭包读取的是调用方 currentAgentName 的最新值
// （子会话事件会更新它）。
func (d *Daemon) withAskBypassSinks(ctx context.Context, sid string, agentName func() string) context.Context {
	ctx = agents.WithPermissionSink(ctx, func(data events.PermissionPendingData) {
		if d.gw == nil {
			return
		}
		d.gw.BroadcastNotification(string(gateway.RespPermissionRequest), map[string]any{
			"session_id": sid,
			"meta":       map[string]any{"agent_name": agentName()},
			"title":      i18n.T("svc.event.permission.request"),
			"data": map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
				"session_id":     data.SessionID,
			},
			"type": gateway.RespPermissionRequest,
		})
	})

	ctx = agents.WithAskSink(ctx, func(data events.AskUserPendingData) {
		if d.gw == nil {
			return
		}
		askType := gateway.ResponseType(events.AskUserRequest)
		d.gw.BroadcastNotification(string(askType), map[string]any{
			"session_id": sid,
			"meta":       map[string]any{"agent_name": agentName()},
			"title":      i18n.T("svc.event.ask.user"),
			"data": map[string]any{
				"questions":  data.Questions,
				"session_id": data.SessionID,
			},
			"type": askType,
		})
	})

	return ctx
}

// runScheduledJob 后台执行一次到点的调度任务：加载绑定会话 → 组装 AskBuilder
// （广播版事件链，与 defaultHandler 的会话装配同源）→ Run → ReportResult 收尾。
// 由 OnLifecycle 的 started 分支异步调用；panic 必须在此收敛，不能拖垮
// cron goroutine，且执行异常要按失败回写，任务状态不留悬空 started。
func (d *Daemon) runScheduledJob(info scheduler.JobLifecycleInfo) {
	if d.scheduler == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			d.scheduler.ReportResult(info.EntryID, info.RunID, fmt.Errorf("执行异常: %v", r))
		}
	}()

	entry, err := d.schedulerDB.Load(context.Background(), info.EntryID)
	if err != nil {
		d.logger.Error("scheduled job: load entry failed", err,
			"entry_id", info.EntryID, "run_id", info.RunID)
		d.scheduler.ReportResult(info.EntryID, info.RunID, err)
		return
	}
	// 会话缺失：无法定位执行落点，标记失败并广播（错误写明原因与补救）。
	if entry.SessionID == "" {
		err := errors.New("任务未绑定会话，无法执行（删除后重建该任务即可）")
		d.logger.Error("scheduled job: entry has no session", err, "entry_id", info.EntryID)
		d.scheduler.ReportResult(info.EntryID, info.RunID, err)
		return
	}
	rt, err := d.app.ResolveRuntime(entry.Agent)
	if err != nil {
		d.logger.Error("scheduled job: resolve runtime failed", err,
			"entry_id", info.EntryID, "agent", entry.Agent)
		d.scheduler.ReportResult(info.EntryID, info.RunID, err)
		return
	}

	sid := entry.SessionID
	content := entry.Content
	agentName := entry.Agent
	gw := d.gw
	currentAgentName := agentName
	var eventSubSessionID string
	getAgentName := func() string { return currentAgentName }
	getSubSessionID := func() string { return eventSubSessionID }
	withAgent := func() gateway.ResponseOption {
		return gateway.WithResponseMeta(map[string]any{"agent_name": currentAgentName})
	}

	d.logger.Info("scheduled job: background execution started",
		"entry_id", info.EntryID, "run_id", info.RunID,
		"agent", agentName, "session_id", sid)

	ctx, cancel := context.WithCancel(context.Background())
	// 后台执行登记在虚拟客户端集合：message.cancel 按会话取消时一并覆盖。
	cancelSet := d.clientCancelSetFor(scheduledCancelClientID)
	cancelEntry := cancelSet.Add(sid, cancel)
	// 子智能体授权/提问冒泡旁路：与前台执行共用实现。
	ctx = d.withAskBypassSinks(ctx, sid, getAgentName)

	d.sessionQueueFor(sid).Enqueue(func() {
		defer func() {
			cancelSet.Remove(cancelEntry)
			cancel()
		}()
		defer func() {
			d.activeSessions.Delete(sid)
			if r := recover(); r != nil {
				d.logger.Error("scheduled job: AskBuilder panic", fmt.Errorf("%v", r),
					"entry_id", info.EntryID, "session_id", sid)
			}
		}()

		// 排队等待期间被取消（停止按钮）：按失败回写终态。
		if ctx.Err() != nil {
			d.logger.Info("scheduled job: skipped, cancelled while queued",
				"entry_id", info.EntryID, "session_id", sid)
			d.scheduler.ReportResult(info.EntryID, info.RunID, ctx.Err())
			return
		}

		// 会话装配与 defaultHandler 同源：Runtime 统一注入压缩/沙箱配置，
		// 动态窗口 resolver，文件记忆 SummarySink 落到会话项目目录。
		var sessOpts []goharnesssession.SessionConfig
		sessOpts = append(sessOpts, rt.SessionConfigs()...)
		sessOpts = append(sessOpts,
			goharnesssession.WithModelContextResolver(d.resolveSessionModelContextLength),
		)
		memoryDir := d.app.FileMemory().MemoryDirFor(d.sessionProjectDir(sid))
		sessOpts = append(sessOpts, goharnesssession.WithSummarySink(func(ctx context.Context, summary string) error {
			return d.app.FileMemory().Write(ctx, memoryDir, agentName, sid, summary)
		}))
		s, err := goharnesssession.Load(context.Background(), sid, agentName, d.app.SessDB(), d.logger, sessOpts...)
		if err != nil {
			d.logger.Error("scheduled job: failed to load session", err, "session_id", sid)
			d.scheduler.ReportResult(info.EntryID, info.RunID, err)
			return
		}
		d.activeSessions.Store(sid, s)

		// 文件追踪事件广播（DiffView）：仅 tracked 动作，与前台同语义。
		s.SetFileModifyHandler(func(ev goharnesssession.FileModifyEvent) {
			if ev.Action != "tracked" {
				return
			}
			if ev.FilePath == "" {
				return
			}
			payload, _ := json.Marshal(map[string]any{
				"files":  []string{ev.FilePath},
				"action": ev.Action,
			})
			broadcastEvent(gw, sid, gateway.RespFileModified,
				i18n.T("svc.event.file_modified"), string(payload))
		})

		// 广播版事件链：事件以与定向版同构的信封广播到所有在线客户端。
		emitter := newBroadcastAskHandlers(gw, sid, withAgent, s, getAgentName, getSubSessionID)

		// 轮次日志：与前台同源采集（mindx report collect / daily-report 消费）。
		turnLogLenBefore := len(s.All())
		turnLogStart := time.Now()

		builder := rt.Ask(agentName, content, s).
			WithContext(ctx).
			OnEvent(func(ev events.ReactEvent) {
				currentAgentName = ev.AgentName
				eventSubSessionID = ev.SessionID
			})
		builder = d.withMetricHook(builder, sid, agentName, getSubSessionID, content).
			OnEvent(func(ev events.ReactEvent) {
				// Compact 事件广播：与 defaultHandler 同款（MicroCompact 已拆除）。
				switch ev.Type {
				case events.CompactStart:
					data, _ := ev.Data.(events.CompactStartData)
					if gw != nil {
						gw.BroadcastNotification("compact_start", map[string]any{
							"session_id": sid, "data": data,
						})
					}
				case events.CompactDone:
					data, _ := ev.Data.(events.CompactDoneData)
					if gw != nil {
						gw.BroadcastNotification("compact_done", map[string]any{
							"session_id": sid, "data": data,
						})
					}
				}
			})
		builder = wireAskEvents(builder, emitter)

		_, err = builder.
			OnPermissionDenied(func(reason string) {
				broadcastEvent(gw, sid, gateway.RespPermissionDenied, i18n.T("svc.event.permission.denied"), reason, withAgent())
			}).
			Run()

		// 终态回写：取消按 missed，其余失败按 failed，成功按 success；
		// ReportResult 内部广播 completed/failed/missed，前端日历据此刷新。
		d.scheduler.ReportResult(info.EntryID, info.RunID, err)

		if err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("scheduled job: execution failed", err,
				"entry_id", info.EntryID, "session_id", sid, "agent", agentName)
		} else {
			d.logger.Info("scheduled job: execution finished",
				"entry_id", info.EntryID, "session_id", sid, "agent", agentName)
		}

		// 轮次日志落盘（旁路采集，失败不影响执行结果）。
		d.appendTurnLog(s, agentName, content, turnLogLenBefore, turnLogStart)
	})
}
