package gateway

// ResponseType defines the business-level semantic classification of a message
// pushed from server to client. This is an application-layer concept, distinct
// from the protocol-layer Command (BEGN/TEXT/JSON/OK/ERR/CMD) and from
// ContentType (text/plain, application/json, etc. MIME types).
//
// When the server sends a structured response via Server.SendResponse(), the
// Message.Data field contains a JSON-encoded ResponseEnvelope with one of these
// type values. The client uses this type to dispatch to a registered handler.
type ResponseType string

const (
	// --- Built-in response types ---

	// RespText indicates a plain-text notification or message.
	RespText ResponseType = "text"

	// RespTable indicates tabular data that should be rendered as a table.
	// Data should be: {"headers": [...], "rows": [[...], ...]}
	RespTable ResponseType = "table"

	// RespTodo indicates a todo/checklist item list.
	// Data should be: [{"id": ..., "text": ..., "done": ...}, ...]
	RespTodo ResponseType = "todo"

	// RespOptions indicates a selectable options list (single or multi-select).
	// Data should be: [{"value": ..., "label": ...}, ...]
	RespOptions ResponseType = "options"

	// RespForm indicates a form input request asking the user to fill fields.
	// Data should be: {"fields": [{"name", "label", "type", "required", ...}]}
	RespForm ResponseType = "form"

	// RespProgress indicates a progress update for a long-running operation.
	// Data should be: {"total": N, "current": N, "message": "..."}
	RespProgress ResponseType = "progress"

	// RespError indicates a business-level error that should be displayed to the user.
	// Data should be: {"code": "...", "message": "..."} or a plain string
	RespError ResponseType = "error"

	// RespConfirm indicates a confirmation request requiring user yes/no action.
	// Data should be: {"message": "...", "detail": "..."}
	RespConfirm ResponseType = "confirm"

	// RespFile indicates binary or file data being transferred.
	// Data should be: {"filename": "...", "mime": "...", "size": N, "data": "base64..."}
	RespFile ResponseType = "file"

	// RespMarkdown indicates rich text in Markdown format that should be
	// rendered with terminal styling (colored headings, bold, italic, lists,
	// code blocks, etc.). Data should be a raw Markdown string.
	RespMarkdown ResponseType = "markdown"

	// --- Agent (goharness) event response types ---
	// NOTE: These are strictly aligned with goharness/events/types.go ReactEventType.
	// The event_dispatcher performs a transparent 1:1 passthrough — no type mutation,
	// no semantic rewriting. If goharness emits "tool_exec_start", the gateway sends
	// "tool_exec_start". Clients receive the exact same event name and data shape
	// as the goharness runtime produces.

	// RespThinkingDelta indicates a streaming fragment of the agent's thinking process.
	// Data should be a plain text string (the thinking fragment).
	// Aligned with: goharness ReactEventType ThinkingDelta
	RespThinkingDelta ResponseType = "thinking_delta"

	// RespThinkingDone indicates the agent completed a thinking cycle.
	// Aligned with: goharness ReactEventType ThinkingDone
	RespThinkingDone ResponseType = "thinking_done"

	// RespToolUseDelta indicates streaming tool call argument fragments from LLM response.
	// Data should be: {"index": N, "id": "...", "name": "...", "arguments": "..."}
	// Aligned with: goharness ReactEventType ToolUseDelta → ToolUseDeltaData
	RespToolUseDelta ResponseType = "tool_use_delta"

	// RespToolExecStart indicates a tool is about to begin execution.
	// Data should be: {"tool_name": "...", "params": {...}}
	// Aligned with: goharness ReactEventType ToolExecStart → ToolExecStartData
	RespToolExecStart ResponseType = "tool_exec_start"

	// RespToolExecEnd indicates tool execution finished (success or failure).
	// Data should be: {"tool_name": "...", "tool_call_id": "...", "success": bool,
	//                  "result": "...", "error": "...", "duration_ms": N,
	//                  "prompt_tokens": N, "completion_tokens": N, "total_tokens": N,
	//                  "cached_tokens": N}
	// Aligned with: goharness ReactEventType ToolExecEnd → ToolExecEndData
	RespToolExecEnd ResponseType = "tool_exec_end"

	// RespSubtaskSpawned indicates a subagent task has been created.
	// Data should be: {"task_id": "...", "agent_name": "...", "description": "..."}
	RespSubtaskSpawned ResponseType = "subtask_spawned"

	// RespSubtaskCompleted indicates a subagent task has finished.
	// Data should be: {"task_id": "...", "success": true, "answer": "...", "error": "..."}
	RespSubtaskCompleted ResponseType = "subtask_completed"

	// RespFinalAnswer indicates the agent produced its final answer.
	// Data should be a plain text string (the final answer, possibly markdown).
	RespFinalAnswer ResponseType = "final_answer"

	// RespClarifyNeeded indicates the agent needs user clarification.
	// Data should be a plain text string (the question).
	RespClarifyNeeded ResponseType = "clarify_needed"

	// RespPermissionRequest indicates the agent needs user authorization.
	// Data should be: {"tool_name": "...", "reason": "...", "security_level": "..."}
	RespPermissionRequest ResponseType = "permission_request"

	// RespPermissionDenied indicates a tool execution was denied.
	// Data should be a plain text string (denial reason).
	RespPermissionDenied ResponseType = "permission_denied"

	// RespExecutionSummary indicates the agent execution completed with stats.
	// Data should be: {"total_iterations": N, "tool_calls": N, "tools_used": [...], "total_duration_ms": N, "tokens_used": N}
	RespExecutionSummary ResponseType = "execution_summary"

	// RespLoopEnd indicates one Think-Act loop has ended.
	// Data should be: {"iteration": N, "duration_ms": N, "termination_reason": "..."}
	RespLoopEnd ResponseType = "loop_end"

	// RespTaskSummary indicates a natural-language summary of the completed task.
	// Data should be: {"summary": "...", "input_tokens": N, "output_tokens": N}
	RespTaskSummary ResponseType = "task_summary"

	// RespCompaction indicates the session context window was compacted.
	// Data should be: {"session_id": "...", "messages_slid": N, "remaining_after": N, "window_size": N}
	RespCompaction ResponseType = "compaction"

	// RespMaxTurnsReached indicates the Think-Act loop reached its max turn limit.
	// Data should be: {"turns_completed": N, "max_turns": N, "suggestion": "..."}
	RespMaxTurnsReached ResponseType = "max_turns_reached"

	// RespLLMCancelled indicates the LLM call was cancelled by the user (not a timeout).
	// Data should be: {"elapsed_ns": N}
	RespLLMCancelled ResponseType = "llm_cancelled"

	// RespTokenUsageRecorded indicates an LLM call completed and its token usage
	// has been recorded. Data should be a TokenUsageRecord JSON object.
	RespTokenUsageRecorded ResponseType = "token_usage_recorded"

	// RespContextUsage indicates the session's context window usage stats.
	// Data should be: {"window_tokens": N, "max_window_size": N, "usage_ratio": F, ...}
	RespContextUsage ResponseType = "context_usage"

	// RespFileModified indicates files in the session's workspace have been modified
	// by a Write/FileEdit tool execution. This event is broadcast to all clients so
	// they can update their ModifyFiles list and show diff toolbars in real time.
	//
	// Data should be:
	//   {"files": ["path1", "path2", ...], "action": "tracked"}
	// where action is one of: "tracked", "confirmed", "rolled_back".
	//
	// Aligned with: goharness session.FileModifyEvent
	RespFileModified ResponseType = "file_modified"

	// RespUserMessageSaved indicates a real user message has been appended to
	// the session and persisted. Emitted right after the append (magic words
	// are not appended so they don't trigger this). The frontend stores the
	// carried Timestamp as metadata.backendTimestamp to enable real-time
	// "undo this round" via session.delete_round.
	// Data should be: {"timestamp": N}
	// Aligned with: goharness ReactEventType UserMessageSaved → UserMessageSavedData
	RespUserMessageSaved ResponseType = "user_message_saved"

	// RespMessageQueued 表示用户消息已进入同一会话的串行执行队列，
	// 正在等待上一轮执行完成（见 mindx svc.sessionQueue）。
	// Data should be: {"timestamp": N}
	RespMessageQueued ResponseType = "message_queued"

	// RespMessageProcessing 表示排队中的用户消息已开始执行，
	// 前端应把「排队中」状态切换为「处理中」。
	// Data should be: {"timestamp": N}
	RespMessageProcessing ResponseType = "message_processing"

	// --- Session & Memory RPC response types ---

	// RespSessionList indicates the response contains a list of sessions.
	// Data should be: []SessionInfo
	RespSessionList ResponseType = "session_list"

	// RespSessionInfo indicates detailed information about a single session.
	// Data should be: SessionInfo
	RespSessionInfo ResponseType = "session_info"

	// RespMemoryResult indicates memory query/search results.
	// Data should be: []MemoryRecord
	RespMemoryResult ResponseType = "memory_result"

	// RespMemoryStored indicates a memory record was successfully stored.
	// Data should be: {"id": "..."}
	RespMemoryStored ResponseType = "memory_stored"

	// --- Agent/Model/Skill RPC response types ---

	// RespAgentList indicates the response contains a list of agents.
	// Data should be: []AgentConfig
	RespAgentList ResponseType = "agent_list"

	// RespAgentUpdated indicates an agent config was successfully updated.
	// Data should be: {"status": "ok", "agent_name": "..."}
	RespAgentUpdated ResponseType = "agent_updated"

	// RespModelList indicates the response contains a list of models.
	// Data should be: []ModelConfig
	RespModelList ResponseType = "model_list"

	// RespSkillList indicates the response contains a list of skills.
	// Data should be: []Skill
	RespSkillList ResponseType = "skill_list"

	// --- Filesystem RPC response types ---

	// RespFSList indicates the response contains a directory listing.
	// Data should be: []FSEntry
	RespFSList ResponseType = "fs_list"

	// RespFSHome indicates the response contains the user's home directory path.
	// Data should be: {"path": "..."}
	RespFSHome ResponseType = "fs_home"

	// --- Compatibility ---

	// RespUnknown is the zero value used when no type is specified (backward compat).
	RespUnknown ResponseType = ""
)

// ResponseEnvelope is the standard envelope format for all structured responses
// sent from server to client via Server.SendResponse() / BroadcastResponse().
//
// When a Message's Data field is a JSON object starting with {"type":", it is
// interpreted as a ResponseEnvelope. The client dispatches Envelope.Type to the
// corresponding registered TypedResponseHandler.
type ResponseEnvelope struct {
	// Type is the response type classifier (required). Determines which handler
	// is invoked on the client side.
	Type ResponseType `json:"type"`

	// ID optionally associates this response with a prior request, enabling
	// request-response correlation in push-based scenarios.
	ID string `json:"id,omitempty"`

	// SessionID identifies the agent session that produced this response.
	// The TUI uses this field to route events to the correct AgentAnswer.
	SessionID string `json:"session_id,omitempty"`

	// Title is an optional human-readable title shown alongside the response.
	Title string `json:"title,omitempty"`

	// Data carries the type-specific payload. Its structure depends on Type:
	//   - RespTable:  {"headers":[string],"rows":[][]string}
	//   - RespTodo:   []{"id":string,"text":string,"done":bool}
	//   - RespOptions:[]{"value":string,"label":string}
	//   - and so on for each type.
	Data interface{} `json:"data,omitempty"`

	// Meta holds arbitrary extension metadata for advanced use cases.
	Meta map[string]interface{} `json:"meta,omitempty"`
}
