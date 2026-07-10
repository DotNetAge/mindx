package svc

import (
	"fmt"

	"github.com/DotNetAge/goharness/agents"
	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// askEventHandlers groups the callback functions for common AskBuilder event
// types that are shared between defaultHandler and executeScheduleCommand.
// A zero-value (nil) field means the event is omitted from the builder.
type askEventHandlers struct {
	Thinking           func(chunk string)
	Content            func(chunk string)
	ToolUseDelta       func(data goharnessevents.ToolUseDeltaData)
	ThinkingDone       func()
	ToolStart          func(data goharnessevents.ToolExecStartData)
	ToolEnd            func(data goharnessevents.ToolExecEndData)
	Answer             func(answer string)
	ExecutionSummary   func(data goharnessevents.ExecutionSummaryData)
	CycleEnd           func(data goharnessevents.CycleInfo)
	Compaction         func(data goharnessevents.CompactionData)
	MaxTurnsReached    func(data goharnessevents.MaxTurnsReachedData)
	Error              func(errMsg string)
	SubtaskSpawned     func(data goharnessevents.SubtaskInfo)
	SubtaskCompleted   func(data goharnessevents.SubtaskResult)
	TaskSummary        func(data goharnessevents.TaskSummaryData)
	LLMTimeout         func(data goharnessevents.LLMTimeoutData)
	TokenUsageRecorded func(record goharnesssession.TokenUsageRecord)
	AskUserPending     func(data goharnessevents.AskUserPendingData)
	PermissionPending  func(data goharnessevents.PermissionPendingData)
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
	if h.CycleEnd != nil {
		b = b.OnCycleEnd(h.CycleEnd)
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
	if h.TokenUsageRecorded != nil {
		b = b.OnTokenUsageRecorded(h.TokenUsageRecorded)
	}
	if h.AskUserPending != nil {
		b = b.OnAskUserPending(h.AskUserPending)
	}
	if h.PermissionPending != nil {
		b = b.OnPermissionPending(h.PermissionPending)
	}
	return b
}

// ── Factory: per-client event handlers (used by defaultHandler) ──────────

// newClientAskHandlers creates event handlers that route AskBuilder events to
// a specific WebSocket client via gw.SendResponse / d.sendEvent.
// getAgentName is called lazily so that it reflects the live agent name
// (updated by OnEvent during execution).
func newClientAskHandlers(
	d *Daemon,
	gw *gateway.Server,
	clientID, sid string,
	withAgent func() gateway.ResponseOption,
	s *goharnesssession.Session,
	getAgentName func() string,
) askEventHandlers {
	// Snapshot of files already tracked before the current tool executes.
	// Used in ToolEnd to broadcast only files newly modified by this tool,
	// preventing the same file from being re-emitted on every subsequent tool.
	toolStartModFiles := make(map[string]struct{})

	return askEventHandlers{
		Thinking: func(chunk string) {
			_ = gw.SendResponse(clientID, gateway.RespThinkingDelta, i18n.T("svc.event.thinking"), chunk, gateway.WithSessionID(sid), withAgent())
		},
		Content: func(chunk string) {
			d.sendEvent(clientID, sid, gateway.RespMarkdown, i18n.T("svc.event.outputting"), chunk, withAgent())
		},
		ToolUseDelta: func(data goharnessevents.ToolUseDeltaData) {
			_ = gw.SendResponse(clientID, gateway.RespToolUseDelta, i18n.T("svc.event.tool.use.delta"), map[string]any{
				"index": data.Index, "id": data.ID, "name": data.Name, "arguments": data.Arguments,
			}, gateway.WithSessionID(sid), withAgent())
		},
		ThinkingDone: func() {
			d.sendEvent(clientID, sid, gateway.RespThinkingDone, i18n.T("svc.event.thinking.done"), i18n.T("svc.event.thinking.done.detail"), withAgent())
		},
		ToolStart: func(data goharnessevents.ToolExecStartData) {
			_ = gw.SendResponse(clientID, gateway.RespToolExecStart, i18n.T("svc.event.tool.start"), map[string]any{
				"tool_name": data.ToolName, "params": data.Params, "predicted_tokens": data.PredictedTokens,
			}, gateway.WithSessionID(sid), withAgent())
			// Snapshot the current tracked files so ToolEnd can detect only
			// files newly added during this tool execution.
			toolStartModFiles = make(map[string]struct{})
			for _, fp := range s.GetModifyFiles() {
				toolStartModFiles[fp] = struct{}{}
			}
		},
		ToolEnd: func(data goharnessevents.ToolExecEndData) {
			_ = gw.SendResponse(clientID, gateway.RespToolExecEnd, i18n.T("svc.event.tool.end"), map[string]any{
				"tool_name": data.ToolName, "tool_call_id": data.ToolCallID,
				"success": data.Success, "result": data.Result, "error": data.Error,
				"duration_ms": int(data.Duration.Milliseconds()),
			}, gateway.WithSessionID(sid), withAgent())
			// Broadcast only files that were newly tracked during this tool execution.
			// Previous implementation broadcast the full cumulative list on every ToolEnd,
			// causing the same file to be emitted repeatedly after unrelated tools.
			modFiles := s.GetModifyFiles()
			var newlyTracked []string
			for _, fp := range modFiles {
				if _, already := toolStartModFiles[fp]; !already {
					newlyTracked = append(newlyTracked, fp)
				}
			}
			if len(newlyTracked) > 0 {
				fileInfos := make([]fileDiffInfo, 0, len(newlyTracked))
				for _, fp := range newlyTracked {
					fileInfos = append(fileInfos, computeFileDiff(s, fp))
				}
				_ = gw.SendResponse(clientID, gateway.RespFileModified, i18n.T("svc.event.file.modified"), map[string]any{
					"files":  fileInfos,
					"action": "tracked",
				}, gateway.WithSessionID(sid), withAgent())
			}
		},
		Answer: func(answer string) {
			d.sendEvent(clientID, sid, gateway.RespFinalAnswer, i18n.T("svc.event.final.answer"), answer, withAgent())
		},
		ExecutionSummary: func(data goharnessevents.ExecutionSummaryData) {
			d.sendExecutionSummary(clientID, sid, data, getAgentName())
		},
		CycleEnd: func(data goharnessevents.CycleInfo) {
			_ = gw.SendResponse(clientID, gateway.RespCycleEnd, i18n.T("svc.event.cycle.end"), map[string]any{
				"iteration": data.Iteration, "termination_reason": data.TerminationReason, "duration": data.Duration.String(),
			}, gateway.WithSessionID(sid), withAgent())
		},
		Compaction: func(data goharnessevents.CompactionData) {
			_ = gw.SendResponse(clientID, gateway.RespCompaction, i18n.T("svc.event.compaction"), map[string]any{
				"session_id": data.SessionID, "messages_slid": data.MessagesSlid, "remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
			}, gateway.WithSessionID(sid), withAgent())
		},
		MaxTurnsReached: func(data goharnessevents.MaxTurnsReachedData) {
			_ = gw.SendResponse(clientID, gateway.RespMaxTurnsReached, i18n.T("svc.event.max.turns.reached"), map[string]any{
				"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
			}, gateway.WithSessionID(sid), withAgent())
		},
		Error: func(errMsg string) {
			d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.error"), errMsg, withAgent())
		},
		SubtaskSpawned: func(data goharnessevents.SubtaskInfo) {
			_ = gw.SendResponse(clientID, gateway.RespSubtaskSpawned, i18n.T("svc.event.subtask.spawned"), map[string]any{
				"task_id":     data.TaskID,
				"agent_name":  data.AgentName,
				"description": data.Description,
				"timeout":     data.Timeout,
			}, gateway.WithSessionID(sid), withAgent())
		},
		SubtaskCompleted: func(data goharnessevents.SubtaskResult) {
			_ = gw.SendResponse(clientID, gateway.RespSubtaskCompleted, i18n.T("svc.event.subtask.completed"), map[string]any{
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
			_ = gw.SendResponse(clientID, gateway.RespTaskSummary, i18n.T("svc.event.task.summary"), md,
				gateway.WithSessionID(sid),
				gateway.WithResponseMeta(map[string]any{
					"input_tokens":  data.TokenUsage.InputTokens,
					"output_tokens": data.TokenUsage.OutputTokens,
					"agent_name":    getAgentName(),
				}))
		},
		LLMTimeout: func(data goharnessevents.LLMTimeoutData) {
			msg := fmt.Sprintf(i18n.T("svc.event.llm.timeout"), data.Elapsed, data.Error)
			d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.timeout"), msg, withAgent())
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
			}, gateway.WithSessionID(sid), withAgent())
		},
		AskUserPending: func(data goharnessevents.AskUserPendingData) {
			_ = gw.SendResponse(clientID, gateway.RespForm, i18n.T("svc.event.ask.user"), map[string]any{
				"questions": data.Questions,
			}, gateway.WithSessionID(sid), withAgent())
		},
		PermissionPending: func(data goharnessevents.PermissionPendingData) {
			_ = gw.SendResponse(clientID, gateway.RespPermissionRequest, i18n.T("svc.event.permission.request"), map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
			}, gateway.WithSessionID(sid), withAgent())
		},
	}
}

// ── Factory: broadcast event handlers (used by executeScheduleCommand) ──

// newBroadcastAskHandlers creates event handlers that broadcast AskBuilder
// events to all connected clients via d.broadcastScheduleEvent.
func newBroadcastAskHandlers(
	d *Daemon,
	sessionID, agent string,
) askEventHandlers {
	return askEventHandlers{
		Thinking: func(chunk string) {
			d.broadcastScheduleEvent(sessionID, agent, "thinking_delta", chunk)
		},
		Content: func(chunk string) {
			d.broadcastScheduleEvent(sessionID, agent, "markdown", chunk)
		},
		ToolUseDelta: func(data goharnessevents.ToolUseDeltaData) {
			d.broadcastScheduleEvent(sessionID, agent, "tool_use_delta", map[string]any{
				"index": data.Index, "id": data.ID, "name": data.Name, "arguments": data.Arguments,
			})
		},
		ThinkingDone: func() {
			d.broadcastScheduleEvent(sessionID, agent, "thinking_done", nil)
		},
		ToolStart: func(data goharnessevents.ToolExecStartData) {
			d.broadcastScheduleEvent(sessionID, agent, "tool_exec_start", map[string]any{
				"tool_name": data.ToolName, "params": data.Params, "predicted_tokens": data.PredictedTokens,
			})
		},
		ToolEnd: func(data goharnessevents.ToolExecEndData) {
			d.broadcastScheduleEvent(sessionID, agent, "tool_exec_end", map[string]any{
				"tool_name": data.ToolName, "tool_call_id": data.ToolCallID,
				"success": data.Success, "result": data.Result, "error": data.Error,
				"duration_ms": int(data.Duration.Milliseconds()),
			})
		},
		Answer: func(answer string) {
			d.broadcastScheduleEvent(sessionID, agent, "final_answer", answer)
		},
		ExecutionSummary: func(data goharnessevents.ExecutionSummaryData) {
			d.broadcastScheduleEvent(sessionID, agent, "execution_summary", map[string]any{
				"total_iterations": data.TotalIterations, "tool_calls": data.ToolCalls,
				"tools_used": data.ToolsUsed, "total_duration": data.TotalDuration.String(),
				"tokens_used": map[string]any{
					"total_tokens":     data.TokensUsed.TotalTokens,
					"input_tokens":     data.TokensUsed.InputTokens,
					"output_tokens":    data.TokensUsed.OutputTokens,
					"cached_tokens":    data.TokensUsed.CachedTokens,
					"reasoning_tokens": data.TokensUsed.ReasoningTokens,
				},
				"termination_reason": data.TerminationReason,
			})
		},
		CycleEnd: func(data goharnessevents.CycleInfo) {
			d.broadcastScheduleEvent(sessionID, agent, "cycle_end", map[string]any{
				"iteration": data.Iteration, "termination_reason": data.TerminationReason, "duration": data.Duration.String(),
			})
		},
		Compaction: func(data goharnessevents.CompactionData) {
			d.broadcastScheduleEvent(sessionID, agent, "compaction", map[string]any{
				"session_id": data.SessionID, "messages_slid": data.MessagesSlid,
				"remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
			})
		},
		MaxTurnsReached: func(data goharnessevents.MaxTurnsReachedData) {
			d.broadcastScheduleEvent(sessionID, agent, "max_turns_reached", map[string]any{
				"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
			})
		},
		Error: func(errMsg string) {
			d.broadcastScheduleEvent(sessionID, agent, "error", errMsg)
		},
		SubtaskSpawned: func(data goharnessevents.SubtaskInfo) {
			d.broadcastScheduleEvent(sessionID, agent, "subtask_spawned", map[string]any{
				"task_id": data.TaskID, "agent_name": data.AgentName,
				"description": data.Description, "timeout": data.Timeout,
			})
		},
		SubtaskCompleted: func(data goharnessevents.SubtaskResult) {
			d.broadcastScheduleEvent(sessionID, agent, "subtask_completed", map[string]any{
				"task_id": data.TaskID, "agent_name": data.AgentName,
				"success": data.Success, "answer": data.Answer, "error": data.Error,
				"description": data.Description,
			})
		},
		TaskSummary: func(data goharnessevents.TaskSummaryData) {
			d.broadcastScheduleEvent(sessionID, agent, "task_summary", map[string]any{
				"summary": data.Summary,
				"token_usage": map[string]any{
					"input_tokens":  data.TokenUsage.InputTokens,
					"output_tokens": data.TokenUsage.OutputTokens,
					"total_tokens":  data.TokenUsage.TotalTokens,
				},
			})
		},
		LLMTimeout: func(data goharnessevents.LLMTimeoutData) {
			d.broadcastScheduleEvent(sessionID, agent, "llm_timeout", map[string]any{
				"elapsed": data.Elapsed, "error": data.Error,
			})
		},
		TokenUsageRecorded: func(record goharnesssession.TokenUsageRecord) {
			d.broadcastScheduleEvent(sessionID, agent, "token_usage_recorded", map[string]any{
				"id": record.ID, "session_id": record.SessionID,
				"model_name": record.ModelName, "provider_name": record.ProviderName,
				"agent_name": record.AgentName, "total_tokens": record.TotalTokens,
				"prompt_tokens": record.PromptTokens, "completion_tokens": record.CompletionTokens,
				"cached_tokens": record.CachedTokens, "reasoning_tokens": record.ReasoningTokens,
				"timestamp": record.Timestamp,
			})
		},
		AskUserPending: func(data goharnessevents.AskUserPendingData) {
			d.broadcastScheduleEvent(sessionID, agent, "form", map[string]any{
				"questions": data.Questions,
			})
		},
		PermissionPending: func(data goharnessevents.PermissionPendingData) {
			d.broadcastScheduleEvent(sessionID, agent, "permission_request", map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
			})
		},
	}
}
