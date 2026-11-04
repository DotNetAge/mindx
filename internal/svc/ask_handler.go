package svc

import (
	"fmt"
	"strings"

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
	LLMRetry           func(data goharnessevents.LLMRetryData)
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
	if h.LLMRetry != nil {
		b = b.OnLLMRetry(h.LLMRetry)
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

// ── Factory: broadcast event handlers (used by defaultHandler & scheduled background execution) ──

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

// ── Factory: broadcast event handlers (used by scheduled background execution) ──

// broadcastResp 以 SendResponse 同构的 ResponseEnvelope 信封把事件广播到所有
// 在线客户端（无人在线时消息自然丢弃，执行本身不依赖客户端）。
func broadcastResp(gw *gateway.Server, respType gateway.ResponseType, title string, data any, opts ...gateway.ResponseOption) {
	if gw == nil {
		return
	}
	env := gateway.ResponseEnvelope{Type: respType, Title: title, Data: data}
	for _, opt := range opts {
		opt(&env)
	}
	gw.BroadcastNotification(string(respType), env)
}

// broadcastEvent 是 d.sendEvent 的广播等价物（自动附加会话 ID）。
func broadcastEvent(gw *gateway.Server, sessionID string, respType gateway.ResponseType, title string, data string, opts ...gateway.ResponseOption) {
	broadcastResp(gw, respType, title, data, append([]gateway.ResponseOption{gateway.WithSessionID(sessionID)}, opts...)...)
}

// newBroadcastAskHandlers 构造广播版事件回调集合：AskBuilder 事件以
// ResponseEnvelope 信封广播到所有在线客户端（交互对话与调度任务后台执行
// 共用本工厂——事件以会话 ID 为路由键，不单播绑定发起请求时的 clientID，
// 发起客户端断连后事件不丢失，任一在线客户端均可继续接收；前端与 TUI
// 按同一套信封解析。getAgentName/getSubSessionID 的懒取语义保持不变。
// d 供挂起登记（蜂群侧栏气球）使用，与 gw 同为必传。
func newBroadcastAskHandlers(
	d *Daemon,
	gw *gateway.Server,
	sid string,
	withAgent func() gateway.ResponseOption,
	s *goharnesssession.Session,
	getAgentName func() string,
	getSubSessionID func() string,
) askEventHandlers {
	return askEventHandlers{
		Thinking: func(chunk string) {
			broadcastResp(gw, gateway.RespThinkingDelta, i18n.T("svc.event.thinking"), chunk, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Content: func(chunk string) {
			broadcastEvent(gw, effectiveEventSession(sid, getSubSessionID), gateway.RespMarkdown, i18n.T("svc.event.outputting"), chunk, withAgent())
		},
		ToolUseDelta: func(data goharnessevents.ToolUseDeltaData) {
			broadcastResp(gw, gateway.RespToolUseDelta, i18n.T("svc.event.tool.use.delta"), map[string]any{
				"index": data.Index, "id": data.ID, "name": data.Name, "arguments": data.Arguments,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		ThinkingDone: func() {
			// data 必须为空：与 client 版同约束，思考正文只经 thinking_delta 流承载。
			broadcastEvent(gw, effectiveEventSession(sid, getSubSessionID), gateway.RespThinkingDone, i18n.T("svc.event.thinking.done"), "", withAgent())
		},
		ToolStart: func(data goharnessevents.ToolExecStartData) {
			broadcastResp(gw, gateway.RespToolExecStart, i18n.T("svc.event.tool.start"), map[string]any{
				"tool_name": data.ToolName, "params": data.Params,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		ToolEnd: func(data goharnessevents.ToolExecEndData) {
			evSid := effectiveEventSession(sid, getSubSessionID)
			broadcastResp(gw, gateway.RespToolExecEnd, i18n.T("svc.event.tool.end"), map[string]any{
				"tool_name": data.ToolName, "tool_call_id": data.ToolCallID,
				"success": data.Success, "result": data.Result, "error": data.Error,
				"duration_ms":       int(data.Duration.Milliseconds()),
				"prompt_tokens":     data.PromptTokens,
				"completion_tokens": data.CompletionTokens,
				"total_tokens":      data.TotalTokens,
				"cached_tokens":     data.CachedTokens,
				"result_meta":       data.ResultMeta,
			}, gateway.WithSessionID(evSid), withAgent())
			// 文件 diff 广播仅限主会话事件（与 client 版同约束：子会话的修改
			// 文件未被 activeSessions 登记追踪，不串流主会话累积变更）。
			if evSid != sid {
				return
			}
			modFiles := s.GetModifyFiles()
			if len(modFiles) > 0 {
				fileInfos := make([]fileDiffInfo, 0, len(modFiles))
				for _, fp := range modFiles {
					fileInfos = append(fileInfos, computeFileDiff(s, fp))
				}
				broadcastResp(gw, gateway.RespFileModified, i18n.T("svc.event.file.modified"), map[string]any{
					"files":  fileInfos,
					"action": "tracked",
				}, gateway.WithSessionID(sid), withAgent())
			}
		},
		Answer: func(answer string) {
			broadcastEvent(gw, effectiveEventSession(sid, getSubSessionID), gateway.RespFinalAnswer, i18n.T("svc.event.final.answer"), answer, withAgent())
		},
		ExecutionSummary: func(data goharnessevents.ExecutionSummaryData) {
			// 子会话 execution_summary 路由到子会话 ID（与 client 版单一数据源同语义）。
			broadcastExecutionSummary(gw, effectiveEventSession(sid, getSubSessionID), data, getAgentName())
		},
		LoopEnd: func(data goharnessevents.CycleInfo) {
			broadcastResp(gw, gateway.RespLoopEnd, i18n.T("svc.event.cycle.end"), map[string]any{
				"iteration": data.Iteration, "termination_reason": data.TerminationReason, "duration": data.Duration.String(),
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Compaction: func(data goharnessevents.CompactionData) {
			broadcastResp(gw, gateway.RespCompaction, i18n.T("svc.event.compaction"), map[string]any{
				"session_id": data.SessionID, "messages_slid": data.MessagesSlid, "remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		MaxTurnsReached: func(data goharnessevents.MaxTurnsReachedData) {
			broadcastResp(gw, gateway.RespMaxTurnsReached, i18n.T("svc.event.max.turns.reached"), map[string]any{
				"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		Error: func(errMsg string) {
			broadcastEvent(gw, effectiveEventSession(sid, getSubSessionID), gateway.RespError, i18n.T("svc.event.error"), errMsg, withAgent())
		},
		SubtaskSpawned: func(data goharnessevents.SubtaskInfo) {
			broadcastResp(gw, gateway.RespSubtaskSpawned, i18n.T("svc.event.subtask.spawned"), map[string]any{
				"session_id":  data.SessionID,
				"task_id":     data.TaskID,
				"agent_name":  data.AgentName,
				"description": data.Description,
				"timeout":     data.Timeout,
			}, gateway.WithSessionID(sid), withAgent())
		},
		SubtaskCompleted: func(data goharnessevents.SubtaskResult) {
			broadcastResp(gw, gateway.RespSubtaskCompleted, i18n.T("svc.event.subtask.completed"), map[string]any{
				"session_id":  data.SessionID,
				"task_id":     data.TaskID,
				"agent_name":  data.AgentName,
				"success":     data.Success,
				"answer":      data.Answer,
				"error":       data.Error,
				"description": data.Description,
			}, gateway.WithSessionID(sid), withAgent())
		},
		TaskSummary: func(data goharnessevents.TaskSummaryData) {
			md := buildTaskSummaryMarkdown(data)
			broadcastResp(gw, gateway.RespTaskSummary, i18n.T("svc.event.task.summary"), md,
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
			broadcastEvent(gw, effectiveEventSession(sid, getSubSessionID), gateway.RespError, i18n.T("svc.event.timeout"), msg, withAgent())
		},
		LLMCancelled: func(data goharnessevents.LLMCancelledData) {
			broadcastResp(gw, gateway.RespLLMCancelled, i18n.T("svc.event.llm.cancelled"), map[string]any{
				"elapsed_ns": data.Elapsed.Nanoseconds(),
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		LLMRetry: func(data goharnessevents.LLMRetryData) {
			broadcastResp(gw, gateway.ResponseType("llm_retry"), i18n.T("svc.event.llm.retry"), map[string]any{
				"session_id":     data.SessionID,
				"provider":       data.Provider,
				"model":          data.Model,
				"status_code":    data.StatusCode,
				"attempt":        data.Attempt,
				"max_attempts":   data.MaxAttempts,
				"retry_after_ns": data.RetryAfter.Nanoseconds(),
				"error":          data.Error,
				"phase":          data.Phase,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		TokenUsageRecorded: func(record goharnesssession.TokenUsageRecord) {
			broadcastResp(gw, gateway.RespTokenUsageRecorded, i18n.T("svc.event.token.usage"), map[string]any{
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
			// 挂起登记（蜂群侧栏气球）：与 client 版同语义（子会话提问统一登记到父会话）。
			d.markSessionBlock(sid, "ask")
			if data.SessionID != "" && data.SessionID != sid {
				d.markSubBlockParent(data.SessionID, sid)
			}
			broadcastResp(gw, gateway.RespForm, i18n.T("svc.event.ask.user"), map[string]any{
				"questions": data.Questions,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		PermissionPending: func(data goharnessevents.PermissionPendingData) {
			// 挂起登记（蜂群侧栏气球）：与 client 版同语义。
			d.markSessionBlock(sid, "permission")
			if data.SessionID != "" && data.SessionID != sid {
				d.markSubBlockParent(data.SessionID, sid)
			}
			broadcastResp(gw, gateway.RespPermissionRequest, i18n.T("svc.event.permission.request"), map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
				// 子智能体授权冒泡：透传发起授权请求的会话 ID（与 client 版同语义）。
				"session_id": data.SessionID,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
		UserMessageSaved: func(timestamp int64) {
			broadcastResp(gw, gateway.RespUserMessageSaved, "用户消息已保存", map[string]any{
				"timestamp": timestamp,
			}, gateway.WithSessionID(effectiveEventSession(sid, getSubSessionID)), withAgent())
		},
	}
}

// broadcastExecutionSummary 是 d.sendExecutionSummary 的广播等价物：
// payload 构造与定向版逐字段一致，仅发送通道改为全客户端广播。
func broadcastExecutionSummary(gw *gateway.Server, sessionID string, summary goharnessevents.ExecutionSummaryData, agentName string) {
	if gw == nil {
		return
	}
	tokensUsed := summary.TokensUsed
	effectiveTotal := tokensUsed.ActualTokens()
	rawTotal := tokensUsed.PromptTokens + tokensUsed.CompletionTokens
	tokenValue := fmt.Sprintf("%d (in:%d out:%d cached:%d reasoning:%d)",
		effectiveTotal, tokensUsed.PromptTokens, tokensUsed.CompletionTokens,
		tokensUsed.CachedTokens, tokensUsed.ReasoningTokens)
	tableData := map[string]any{
		"headers": []string{"Metric", "Value"},
		"rows": []map[string]string{
			{"metric": "Iterations", "value": fmt.Sprintf("%d", summary.TotalIterations)},
			{"metric": "Tool Calls", "value": fmt.Sprintf("%d", summary.ToolCalls)},
			{"metric": "Tools Used", "value": strings.Join(summary.ToolsUsed, ", ")},
			{"metric": "Duration", "value": formatDuration(summary.TotalDuration)},
			{"metric": "Tokens Used", "value": tokenValue},
			{"metric": "Termination", "value": summary.TerminationReason},
		},
	}
	broadcastResp(gw, gateway.RespExecutionSummary, i18n.T("svc.event.execution.summary"), tableData,
		gateway.WithSessionID(sessionID),
		gateway.WithResponseMeta(map[string]any{
			"tokens_used": map[string]any{
				"total_tokens":      rawTotal,
				"actual_tokens":     effectiveTotal,
				"prompt_tokens":     tokensUsed.PromptTokens,
				"completion_tokens": tokensUsed.CompletionTokens,
				"cached_tokens":     tokensUsed.CachedTokens,
				"reasoning_tokens":  tokensUsed.ReasoningTokens,
			},
			"iterations": summary.TotalIterations,
			"tool_calls": summary.ToolCalls,
			"duration":   summary.TotalDuration.String(),
			"agent_name": agentName,
		}))
}
