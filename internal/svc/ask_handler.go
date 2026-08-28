package svc

import (
	"fmt"

	"github.com/DotNetAge/goharness/agents"
	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// askEventHandlers 聚合 defaultHandler 使用的通用 AskBuilder 事件回调集合
// （调度任务已改为前台执行，与主动对话走同一条事件路由）。
// 字段为 nil 表示对应事件不注册到 builder。
type askEventHandlers struct {
	Thinking           func(chunk string)
	Content            func(chunk string)
	ToolUseDelta       func(data goharnessevents.ToolUseDeltaData)
	ThinkingDone       func()
	ToolStart          func(data goharnessevents.ToolExecStartData)
	ToolEnd            func(data goharnessevents.ToolExecEndData)
	Answer             func(answer string)
	ExecutionSummary   func(data goharnessevents.ExecutionSummaryData)
	LoopEnd            func(data goharnessevents.CycleInfo)
	Compaction         func(data goharnessevents.CompactionData)
	MaxTurnsReached    func(data goharnessevents.MaxTurnsReachedData)
	Error              func(errMsg string)
	SubtaskSpawned     func(data goharnessevents.SubtaskInfo)
	SubtaskCompleted   func(data goharnessevents.SubtaskResult)
	TaskSummary        func(data goharnessevents.TaskSummaryData)
	LLMTimeout         func(data goharnessevents.LLMTimeoutData)
	LLMCancelled       func(data goharnessevents.LLMCancelledData)
	TokenUsageRecorded func(record goharnesssession.TokenUsageRecord)
	AskUserPending     func(data goharnessevents.AskUserPendingData)
	PermissionPending  func(data goharnessevents.PermissionPendingData)
	UserMessageSaved   func(timestamp int64)
}

// wireAskEvents attaches the common set of event handlers onto an AskBuilder.
// Only non-nil handlers in h are wired. The caller is then responsible for
// adding any extra handlers (OnEvent, OnAskUser, OnPermissionRequest, etc.)
// and calling Run().
func wireAskEvents(b *agents.AskBuilder, h askEventHandlers) *agents.AskBuilder {
	if h.Thinking != nil {
		b = b.OnThinking(h.Thinking)
	}
	if h.Content != nil {
		b = b.OnContent(h.Content)
	}
	if h.ToolUseDelta != nil {
		b = b.OnToolUseDelta(h.ToolUseDelta)
	}
	if h.ThinkingDone != nil {
		b = b.OnThinkingDone(h.ThinkingDone)
	}
	if h.ToolStart != nil {
		b = b.OnToolStart(h.ToolStart)
	}
	if h.ToolEnd != nil {
		b = b.OnToolEnd(h.ToolEnd)
	}
	if h.Answer != nil {
		b = b.OnAnswer(h.Answer)
	}
	if h.ExecutionSummary != nil {
		b = b.OnExecutionSummary(h.ExecutionSummary)
	}
	if h.LoopEnd != nil {
		b = b.OnLoopEnd(h.LoopEnd)
	}
	if h.Compaction != nil {
		b = b.OnCompaction(h.Compaction)
	}
	if h.MaxTurnsReached != nil {
		b = b.OnMaxTurnsReached(h.MaxTurnsReached)
	}
	if h.Error != nil {
		b = b.OnError(h.Error)
	}
	if h.SubtaskSpawned != nil {
		b = b.OnSubtaskSpawned(h.SubtaskSpawned)
	}
	if h.SubtaskCompleted != nil {
		b = b.OnSubtaskCompleted(h.SubtaskCompleted)
	}
	if h.TaskSummary != nil {
		b = b.OnTaskSummary(h.TaskSummary)
	}
	if h.LLMTimeout != nil {
		b = b.OnLLMTimeout(h.LLMTimeout)
	}
	if h.LLMCancelled != nil {
		b = b.OnLLMCancelled(h.LLMCancelled)
	}
	if h.TokenUsageRecorded != nil {
		b = b.OnTokenUsageRecorded(h.TokenUsageRecorded)
	}
	if h.AskUserPending != nil {
		b = b.OnAskUserPending(h.AskUserPending)
	}
	if h.PermissionPending != nil {
		b = b.OnPermissionPending(h.PermissionPending)
	}
	if h.UserMessageSaved != nil {
		b = b.OnUserMessageSaved(func(d goharnessevents.UserMessageSavedData) {
			h.UserMessageSaved(d.Timestamp)
		})
	}
	return b
}

// ── Factory: per-client event handlers (used by defaultHandler) ──────────

// effectiveEventSession 返回当前事件应归属的会话 ID：
// 子会话转发事件（getSubSessionID 返回子会话 ID）时用子会话 ID，
// 使前端能精确反查到对应的子任务卡片；主会话事件保持父会话 ID。
func effectiveEventSession(sid string, getSubSessionID func() string) string {
	if getSubSessionID != nil {
		if sub := getSubSessionID(); sub != "" && sub != sid {
			return sub
		}
	}
	return sid
}

// newClientAskHandlers creates event handlers that route AskBuilder events to
// a specific WebSocket client via gw.SendResponse / d.sendEvent.
// getAgentName is called lazily so that it reflects the live agent name
// (updated by OnEvent during execution).
// getSubSessionID is called lazily to fetch the current event's originating
// session ID (sub-session ID for forwarded sub-agent events, empty otherwise).
func newClientAskHandlers(
	d *Daemon,
	gw *gateway.Server,
	clientID, sid string,
	withAgent func() gateway.ResponseOption,
	s *goharnesssession.Session,
	getAgentName func() string,
	getSubSessionID func() string,
) askEventHandlers {
	return askEventHandlers{
		Thinking: func(chunk string) {
			_ = gw.SendResponse(clientID, gateway.RespThinkingDelta, i18n.T("svc.event.thinking"), chunk, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Content: func(chunk string) {
			d.sendEvent(clientID, effectiveEventSession(sid, getSubSessionID), gateway.RespMarkdown, i18n.T("svc.event.outputting"), chunk, withAgent())
		},
		ToolUseDelta: func(data goharnessevents.ToolUseDeltaData) {
			_ = gw.SendResponse(clientID, gateway.RespToolUseDelta, i18n.T("svc.event.tool.use.delta"), map[string]any{
				"index": data.Index, "id": data.ID, "name": data.Name, "arguments": data.Arguments,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		ThinkingDone: func() {
			// data 必须为空：思考正文已由 thinking_delta 流承载，客户端据此累积。
			// 严禁把「模型已完成思考链」这类提示文本放进 data —— 它会被前端/TUI
			// 当作思考正文显示（思考区出现荒谬固定文字，见 ISSUE）。标题 title 已表达「思考完成」。
			d.sendEvent(clientID, effectiveEventSession(sid, getSubSessionID), gateway.RespThinkingDone, i18n.T("svc.event.thinking.done"), "", withAgent())
		},
		ToolStart: func(data goharnessevents.ToolExecStartData) {
			_ = gw.SendResponse(clientID, gateway.RespToolExecStart, i18n.T("svc.event.tool.start"), map[string]any{
				"tool_name": data.ToolName, "params": data.Params,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		ToolEnd: func(data goharnessevents.ToolExecEndData) {
			evSid := effectiveEventSession(sid, getSubSessionID)
			// 实际 token 用量：exec 循环将工具所在轮次 LLM 调用的 usage 回填到事件，
			// 随 tool_exec_end 下发，前端「查看结果」对话框展示真实消耗。
			_ = gw.SendResponse(clientID, gateway.RespToolExecEnd, i18n.T("svc.event.tool.end"), map[string]any{
				"tool_name": data.ToolName, "tool_call_id": data.ToolCallID,
				"success": data.Success, "result": data.Result, "error": data.Error,
				"duration_ms":       int(data.Duration.Milliseconds()),
				"prompt_tokens":     data.PromptTokens,
				"completion_tokens": data.CompletionTokens,
				"total_tokens":      data.TotalTokens,
				"cached_tokens":     data.CachedTokens,
			}, gateway.WithSessionID(evSid), withAgent())
			// 文件 diff 广播仅限主会话事件：子会话的修改文件未被 activeSessions
			// 登记追踪（FileModifyHook 按 sessionID 查不到 tracker），此处 s（主会话）
			// 的修改列表不应串流到子会话流，否则每次子会话工具结束都会重复广播
			// 主会话累积的文件变更。
			if evSid != sid {
				return
			}
			// 广播本轮所有变更的文件的 diff。
			// 前端 chatStore.handleFileModified 已按文件路径去重，
			// 同一路径多次修改只在消息列表中保留一个 DiffView（更新而非追加），
			// pendingFileModifications 也会被每次新事件完全覆盖。
			modFiles := s.GetModifyFiles()
			if len(modFiles) > 0 {
				fileInfos := make([]fileDiffInfo, 0, len(modFiles))
				for _, fp := range modFiles {
					fileInfos = append(fileInfos, computeFileDiff(s, fp))
				}
				_ = gw.SendResponse(clientID, gateway.RespFileModified, i18n.T("svc.event.file.modified"), map[string]any{
					"files":  fileInfos,
					"action": "tracked",
				}, gateway.WithSessionID(sid), withAgent())
			}
		},
		Answer: func(answer string) {
			d.sendEvent(clientID, effectiveEventSession(sid, getSubSessionID), gateway.RespFinalAnswer, i18n.T("svc.event.final.answer"), answer, withAgent())
		},
		ExecutionSummary: func(data goharnessevents.ExecutionSummaryData) {
			// 子会话 execution_summary 路由到子会话 ID（单一数据源）：
			// 前端把执行摘要写入子会话自己的消息流，主会话 Tab 不被污染。
			d.sendExecutionSummary(clientID, effectiveEventSession(sid, getSubSessionID), data, getAgentName())
		},
		LoopEnd: func(data goharnessevents.CycleInfo) {
			_ = gw.SendResponse(clientID, gateway.RespLoopEnd, i18n.T("svc.event.cycle.end"), map[string]any{
				"iteration": data.Iteration, "termination_reason": data.TerminationReason, "duration": data.Duration.String(),
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Compaction: func(data goharnessevents.CompactionData) {
			_ = gw.SendResponse(clientID, gateway.RespCompaction, i18n.T("svc.event.compaction"), map[string]any{
				"session_id": data.SessionID, "messages_slid": data.MessagesSlid, "remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		MaxTurnsReached: func(data goharnessevents.MaxTurnsReachedData) {
			_ = gw.SendResponse(clientID, gateway.RespMaxTurnsReached, i18n.T("svc.event.max.turns.reached"), map[string]any{
				"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Error: func(errMsg string) {
			// 子会话执行错误同样路由到子会话 ID，前端写入子会话自己的消息流。
			d.sendEvent(clientID, effectiveEventSession(sid, getSubSessionID), gateway.RespError, i18n.T("svc.event.error"), errMsg, withAgent())
		},
		SubtaskSpawned: func(data goharnessevents.SubtaskInfo) {
			_ = gw.SendResponse(clientID, gateway.RespSubtaskSpawned, i18n.T("svc.event.subtask.spawned"), map[string]any{
				"session_id":  data.SessionID,
				"agent_name":  data.AgentName,
				"description": data.Description,
				"timeout":     data.Timeout,
			}, gateway.WithSessionID(sid), withAgent())
		},
		SubtaskCompleted: func(data goharnessevents.SubtaskResult) {
			_ = gw.SendResponse(clientID, gateway.RespSubtaskCompleted, i18n.T("svc.event.subtask.completed"), map[string]any{
				"session_id":  data.SessionID,
				"agent_name":  data.AgentName,
				"success":     data.Success,
				"answer":      data.Answer,
				"error":       data.Error,
				"description": data.Description,
			}, gateway.WithSessionID(sid), withAgent())
		},
		TaskSummary: func(data goharnessevents.TaskSummaryData) {
			md := buildTaskSummaryMarkdown(data)
			// 子会话的 TaskSummary 用 effectiveEventSession 路由到子会话 ID，
			// 前端据此把子任务总结写入对应子卡片，避免与主消息流重复显示
			_ = gw.SendResponse(clientID, gateway.RespTaskSummary, i18n.T("svc.event.task.summary"), md,
				gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)),
				gateway.WithResponseMeta(map[string]any{
					"prompt_tokens":     data.TokenUsage.PromptTokens,
					"completion_tokens": data.TokenUsage.CompletionTokens,
					"cached_tokens":     data.TokenUsage.CachedTokens,
					"total_tokens":      data.TokenUsage.TotalTokens,
					"reasoning_tokens":  data.TokenUsage.ReasoningTokens,
					"agent_name":        getAgentName(),
				}))
		},
		LLMTimeout: func(data goharnessevents.LLMTimeoutData) {
			msg := fmt.Sprintf(i18n.T("svc.event.llm.timeout"), data.Elapsed, data.Error)
			d.sendEvent(clientID, effectiveEventSession(sid, getSubSessionID), gateway.RespError, i18n.T("svc.event.timeout"), msg, withAgent())
		},
		LLMCancelled: func(data goharnessevents.LLMCancelledData) {
			_ = gw.SendResponse(clientID, gateway.RespLLMCancelled, i18n.T("svc.event.llm.cancelled"), map[string]any{
				"elapsed_ns": data.Elapsed.Nanoseconds(),
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		TokenUsageRecorded: func(record goharnesssession.TokenUsageRecord) {
			_ = gw.SendResponse(clientID, gateway.RespTokenUsageRecorded, i18n.T("svc.event.token.usage"), map[string]any{
				"id": record.ID, "session_id": record.SessionID,
				"conversation_id": record.ConversationID,
				"model_name":      record.ModelName, "provider_name": record.ProviderName,
				"agent_name":    record.AgentName,
				"prompt_tokens": record.PromptTokens, "completion_tokens": record.CompletionTokens,
				"cached_tokens": record.CachedTokens, "reasoning_tokens": record.ReasoningTokens,
				"total_tokens": record.TotalTokens,
				"timestamp":    record.Timestamp,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		AskUserPending: func(data goharnessevents.AskUserPendingData) {
			// 子会话 AskUser 阻塞同样经 effectiveEventSession 路由到子会话 ID：
			// 前端把 form 事件写入子会话自己的消息流（单一数据源），
			// SubAgentView 观察窗据此闪动提示，用户在子会话 Tab 以普通消息回答。
			_ = gw.SendResponse(clientID, gateway.RespForm, i18n.T("svc.event.ask.user"), map[string]any{
				"questions": data.Questions,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		PermissionPending: func(data goharnessevents.PermissionPendingData) {
			_ = gw.SendResponse(clientID, gateway.RespPermissionRequest, i18n.T("svc.event.permission.request"), map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
				// 子智能体授权冒泡：透传发起授权请求的会话 ID（子会话），
				// 前端据此发送带目标魔法词精确路由授权决策。
				"session_id": data.SessionID,
				// envelope.session_id 同样按子会话 ID 下发（effectiveEventSession），
				// 前端据此把阻塞事件写入子会话自己的消息流（单一数据源）。
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		UserMessageSaved: func(timestamp int64) {
			// 回传后端 user 消息 Timestamp，前端回填 metadata.backendTimestamp，
			// 使实时对话的"回收本轮"按钮在刷新前即可用。
			// 子会话的用户消息（用户在子会话 Tab 回答 AskUser / 追问）同样
			// 路由到子会话 ID，避免污染主会话的时间戳回填。
			_ = gw.SendResponse(clientID, gateway.RespUserMessageSaved, "用户消息已保存", map[string]any{
				"timestamp": timestamp,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
	}
}

// ── Factory: broadcast event handlers ──
// 说明：调度任务已改为前台执行（OnJobStart → 客户端对话流），不再需要
// newBroadcastAskHandlers 的旁路广播链；Ask 事件统一走 newClientAskHandlers。
