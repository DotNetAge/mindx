package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const (
	daemonConnectTimeout = 5 * time.Second
	rpcCallTimeout       = 30 * time.Second
)

type daemonRPCClient struct {
	client    *gateway.Client
	connected bool
}

// connectDaemon establishes a WebSocket connection to the daemon and registers
// all event notification handlers that translate gateway ResponseEnvelope events
// into bubbletea clientmsg.*Msg messages.
func (m *rootModel) connectDaemon() {
	addr := fmt.Sprintf("ws://localhost%s%s", m.daemonAddr, "/ws")
	c := gateway.NewClient(addr)

	c.OnStateChange(func(oldState, newState gateway.ConnectionState) {
		if newState == gateway.StateDisconnected {
			m.rpcConnected = false
			if m.rpc != nil {
				m.rpc.connected = false
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), daemonConnectTimeout)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		m.notifBar.Add(data.Notification{
			Message: fmt.Sprintf(i18n.T("client.notify.rpc.connect.failed"), m.daemonAddr, err),
			Level:   data.NotifWarning,
		})
		m.rpc = &daemonRPCClient{}
		return
	}

	m.rpc = &daemonRPCClient{client: c, connected: true}
	m.rpcConnected = true
	m.registerNotificationHandlers()
	m.registerBroadcastHandlers()
}

func (m *rootModel) rpcIsConnected() bool {
	return m.rpc != nil && m.rpc.connected && m.rpc.client != nil && m.rpc.client.IsConnected()
}

// ---------------------------------------------------------------------------
// 入站事件通知 —— 与 svc 服务端下发事件一一对齐
//
// 覆盖范围：
//   - svc/defaultHandler + ask_handler.go 经 gw.SendResponse 下发的全部 ResponseType
//   - svc/event_dispatcher.go 与 daemon.go 经 BroadcastNotification 广播的通知方法名
// ---------------------------------------------------------------------------

func (m *rootModel) registerNotificationHandlers() {
	c := m.rpc.client

	// ── 流式输出 ──────────────────────────────────────────────

	c.OnResponse(gateway.RespThinkingDelta, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		content, _ := env.Data.(string)
		m.program.Send(clientmsg.ThinkingDeltaMsg{SessionID: env.SessionID, Content: content})
	})

	c.OnResponse(gateway.RespThinkingDone, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		m.program.Send(clientmsg.ThinkingDoneMsg{SessionID: env.SessionID})
	})

	c.OnResponse(gateway.RespMarkdown, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		content, _ := env.Data.(string)
		m.program.Send(clientmsg.ContentDeltaMsg{SessionID: env.SessionID, Content: content})
	})

	c.OnResponse(gateway.RespToolUseDelta, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		index, _ := data["index"].(float64)
		id, _ := data["id"].(string)
		name, _ := data["name"].(string)
		args, _ := data["arguments"].(string)
		m.program.Send(clientmsg.ToolUseDeltaMsg{
			SessionID: env.SessionID,
			Index:     int(index),
			ID:        id,
			Name:      name,
			Arguments: args,
		})
	})

	// ── 工具执行 ──────────────────────────────────────────────

	c.OnResponse(gateway.RespToolExecStart, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		toolName, _ := data["tool_name"].(string)
		params, _ := data["params"].(map[string]any)
		if m.fileTracker != nil {
			m.fileTracker.ToolExecStart(params)
		}
		m.program.Send(clientmsg.ToolExecStartMsg{
			SessionID:    env.SessionID,
			ToolName:     toolName,
			Params:       params,
			EstimatedTok: 0,
		})
	})

	c.OnResponse(gateway.RespToolExecEnd, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		toolName, _ := data["tool_name"].(string)
		toolCallID, _ := data["tool_call_id"].(string)
		success, _ := data["success"].(bool)
		result, _ := data["result"].(string)
		errStr, _ := data["error"].(string)
		durationMS, _ := data["duration_ms"].(float64)

		var diffText string
		var diffAdds, diffDels int
		var diffFile string
		if m.fileTracker != nil {
			m.fileTracker.ToolExecEnd()
			changes := m.fileTracker.Snapshot()
			m.sidebar.SetFileChanges(changes)
			if len(changes) > 0 {
				last := changes[len(changes)-1]
				diffText = last.Diff
				diffAdds = last.Additions
				diffDels = last.Deletions
				diffFile = last.File
			}
		}

		m.program.Send(clientmsg.ToolExecEndMsg{
			SessionID:        env.SessionID,
			ToolName:         toolName,
			ToolCallID:       toolCallID,
			Success:          success,
			Result:           result,
			Error:            errStr,
			Duration:         time.Duration(durationMS) * time.Millisecond,
			PromptTokens:     intFromAny(data["prompt_tokens"]),
			CompletionTokens: intFromAny(data["completion_tokens"]),
			TotalTokens:      intFromAny(data["total_tokens"]),
			CachedTokens:     intFromAny(data["cached_tokens"]),
			DiffText:         diffText,
			DiffAdds:         diffAdds,
			DiffDels:         diffDels,
			DiffFile:         diffFile,
		})
	})

	// RespFileModified 由服务端以两种载荷形态下发：
	//   A. FileModifyHook（tracked）：files 为路径字符串数组 ["path"]
	//   B. tool_exec_end 后的汇总广播：files 为对象数组 [{path,diff,additions,deletions,isNew}]
	// 此处统一兼容两种形态，并与 fileTracker 的本地 diff 合并后刷新侧栏。
	c.OnResponse(gateway.RespFileModified, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		evData, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		filesRaw, _ := evData["files"].([]any)
		if len(filesRaw) == 0 {
			return
		}

		eventFiles := make(map[string]bool, len(filesRaw))
		eventChanges := make([]data.FileChange, 0, len(filesRaw))
		for _, f := range filesRaw {
			switch v := f.(type) {
			case string:
				if v == "" {
					continue
				}
				eventFiles[v] = true
				eventChanges = append(eventChanges, data.FileChange{File: v})
			case map[string]any:
				path, _ := v["path"].(string)
				if path == "" {
					continue
				}
				fc := data.FileChange{
					File:      path,
					Diff:      stringFromAny(v["diff"]),
					Additions: intFromAny(v["additions"]),
					Deletions: intFromAny(v["deletions"]),
				}
				eventFiles[path] = true
				eventChanges = append(eventChanges, fc)
			}
		}

		// 先取 fileTracker 快照中事件涉及文件的本地 diff 数据。
		var merged []data.FileChange
		if m.fileTracker != nil {
			for _, tc := range m.fileTracker.Snapshot() {
				if eventFiles[tc.File] {
					merged = append(merged, tc)
				}
			}
		}
		// 服务端对象形态携带权威 diff，覆盖本地数据；tracker 未覆盖的文件直接采用。
		for _, ec := range eventChanges {
			replaced := false
			for i := range merged {
				if merged[i].File == ec.File {
					if ec.Diff != "" || ec.Additions > 0 || ec.Deletions > 0 {
						merged[i].Diff = ec.Diff
						merged[i].Additions = ec.Additions
						merged[i].Deletions = ec.Deletions
					}
					replaced = true
					break
				}
			}
			if !replaced {
				merged = append(merged, ec)
			}
		}

		if len(merged) > 0 {
			m.sidebar.SetFileChanges(merged)
		}
	})

	// ── 回合产物 ──────────────────────────────────────────────

	c.OnResponse(gateway.RespFinalAnswer, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		content, _ := env.Data.(string)
		m.program.Send(clientmsg.FinalAnswerMsg{SessionID: env.SessionID, Content: content})
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: env.SessionID})
	})

	c.OnResponse(gateway.RespLoopEnd, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		iter := 1
		var reason string
		var dur time.Duration
		if data, ok := env.Data.(map[string]any); ok {
			if v, ok := data["iteration"].(float64); ok {
				iter = int(v)
			}
			reason, _ = data["termination_reason"].(string)
			if d, err := time.ParseDuration(stringFromAny(data["duration"])); err == nil {
				dur = d
			}
		}
		m.program.Send(clientmsg.IterationMsg{
			SessionID:         env.SessionID,
			Iteration:         iter,
			TerminationReason: reason,
			Duration:          dur,
		})
	})

	c.OnResponse(gateway.RespExecutionSummary, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		rawRows, _ := data["rows"].([]any)
		msg := clientmsg.ExecutionSummaryMsg{SessionID: env.SessionID}
		for _, r := range rawRows {
			row, _ := r.(map[string]any)
			if row == nil {
				continue
			}
			metric, _ := row["metric"].(string)
			value, _ := row["value"].(string)
			switch {
			case strings.Contains(metric, "Duration"):
				msg.Duration, _ = time.ParseDuration(value)
			case strings.Contains(metric, "Tool Calls"):
				_, _ = fmt.Sscanf(value, "%d", &msg.ToolCalls)
			case strings.Contains(metric, "Tokens Used"):
				parseTokenUsage(value, &msg.TokensUsed)
			}
		}
		// meta.tokens_used 携带精确数值，优先于表格文本解析。
		if meta, ok := env.Meta["tokens_used"].(map[string]any); ok {
			msg.TokensUsed.PromptTokens = intFromAny(meta["prompt_tokens"])
			msg.TokensUsed.CompletionTokens = intFromAny(meta["completion_tokens"])
			msg.TokensUsed.CachedTokens = intFromAny(meta["cached_tokens"])
			msg.TokensUsed.ReasoningTokens = intFromAny(meta["reasoning_tokens"])
			msg.TokensUsed.TotalTokens = intFromAny(meta["total_tokens"])
		}
		m.program.Send(msg)
	})

	c.OnResponse(gateway.RespTaskSummary, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		content, _ := env.Data.(string)
		msg := clientmsg.TaskSummaryMsg{SessionID: env.SessionID, Content: content}
		if meta := env.Meta; meta != nil {
			msg.TokenUsage.PromptTokens = intFromAny(meta["prompt_tokens"])
			msg.TokenUsage.CompletionTokens = intFromAny(meta["completion_tokens"])
			msg.TokenUsage.CachedTokens = intFromAny(meta["cached_tokens"])
			msg.TokenUsage.ReasoningTokens = intFromAny(meta["reasoning_tokens"])
			msg.TokenUsage.TotalTokens = intFromAny(meta["total_tokens"])
		}
		m.program.Send(msg)
	})

	c.OnResponse(gateway.RespCompaction, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		m.program.Send(clientmsg.CompactionMsg{
			SessionID:      stringFromAny(data["session_id"]),
			MessagesSlid:   intFromAny(data["messages_slid"]),
			RemainingAfter: intFromAny(data["remaining_after"]),
			WindowSize:     intFromAny(data["window_size"]),
		})
	})

	c.OnResponse(gateway.RespMaxTurnsReached, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		turns, _ := data["turns_completed"].(float64)
		maxTurns, _ := data["max_turns"].(float64)
		suggestion, _ := data["suggestion"].(string)
		m.program.Send(clientmsg.MaxTurnsReachedMsg{
			SessionID:      env.SessionID,
			TurnsCompleted: int(turns),
			MaxTurns:       int(maxTurns),
			Suggestion:     suggestion,
		})
	})

	c.OnResponse(gateway.RespLLMCancelled, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		elapsedNS, _ := data["elapsed_ns"].(float64)
		m.program.Send(clientmsg.LLMCancelledMsg{
			SessionID: env.SessionID,
			Elapsed:   time.Duration(elapsedNS),
		})
	})

	c.OnResponse(gateway.RespTokenUsageRecorded, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		raw, err := json.Marshal(env.Data)
		if err != nil {
			return
		}
		var record session.TokenUsageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return
		}
		m.program.Send(clientmsg.TokenUsageRecordedMsg{Record: record})
	})

	// ── 子智能体任务 ──────────────────────────────────────────

	c.OnResponse(gateway.RespSubtaskSpawned, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		timeout, _ := data["timeout"].(float64)
		m.program.Send(clientmsg.SubtaskSpawnedMsg{
			SessionID:    env.SessionID,
			SubSessionID: stringFromAny(data["session_id"]),
			AgentName:    stringFromAny(data["agent_name"]),
			Description:  stringFromAny(data["description"]),
			TimeoutSec:   int(timeout),
		})
	})

	c.OnResponse(gateway.RespSubtaskCompleted, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		m.program.Send(clientmsg.SubtaskCompletedMsg{
			SessionID:    env.SessionID,
			SubSessionID: stringFromAny(data["session_id"]),
			AgentName:    stringFromAny(data["agent_name"]),
			Success:      boolFromAny(data["success"]),
			Answer:       stringFromAny(data["answer"]),
			Error:        stringFromAny(data["error"]),
			Description:  stringFromAny(data["description"]),
		})
	})

	// ── 错误与中断 ────────────────────────────────────────────

	c.OnResponse(gateway.RespError, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		errMsg := toString(env.Data)
		m.program.Send(clientmsg.AgentErrorMsg{
			SessionID: env.SessionID,
			Error:     fmt.Errorf("%s", errMsg),
		})
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: env.SessionID})
	})

	c.OnResponse(gateway.RespPermissionDenied, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		m.program.Send(clientmsg.PermissionDeniedMsg{
			SessionID: env.SessionID,
			Reason:    toString(env.Data),
		})
	})

	// ── 人机交互 ──────────────────────────────────────────────

	c.OnResponse(gateway.RespForm, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}

		m.rpcAskUserQuestions = nil
		if rawQuestions, ok := data["questions"].([]any); ok {
			for _, rq := range rawQuestions {
				if qm, ok := rq.(map[string]any); ok {
					q := struct {
						Question    string
						Options     []string
						MultiSelect bool
					}{
						Question:    fmt.Sprint(qm["question"]),
						MultiSelect: toBool(qm["multi_select"]),
					}
					if opts, ok := qm["options"].([]any); ok {
						for _, o := range opts {
							q.Options = append(q.Options, fmt.Sprint(o))
						}
					}
					m.rpcAskUserQuestions = append(m.rpcAskUserQuestions, q)
				}
			}
		}

		m.program.Send(clientmsg.AskUserEventMsg{})
	})

	c.OnResponse(gateway.RespPermissionRequest, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		toolName, _ := data["tool_name"].(string)
		reason, _ := data["reason"].(string)
		secLevel, _ := data["security_level"].(float64)
		// 子智能体授权冒泡：透传发起请求的会话 ID，用户决策后据此发送
		// 带目标魔法词精确路由，避免多子会话并发挂起时决策错位。
		sessionID, _ := data["session_id"].(string)
		m.program.Send(clientmsg.PermissionRequestMsg{
			ToolName:      toolName,
			Reason:        reason,
			SecurityLevel: int(secLevel),
			SessionID:     sessionID,
		})
	})

	// ── 会话生命周期状态 ──────────────────────────────────────

	c.OnResponse(gateway.RespUserMessageSaved, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, _ := env.Data.(map[string]any)
		m.program.Send(clientmsg.UserMessageSavedMsg{
			SessionID: env.SessionID,
			Timestamp: int64(intFromAny(data["timestamp"])),
		})
	})

	c.OnResponse(gateway.RespMessageQueued, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, _ := env.Data.(map[string]any)
		m.program.Send(clientmsg.MessageQueuedMsg{
			SessionID: env.SessionID,
			Timestamp: int64(intFromAny(data["timestamp"])),
		})
	})

	c.OnResponse(gateway.RespMessageProcessing, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, _ := env.Data.(map[string]any)
		m.program.Send(clientmsg.MessageProcessingMsg{
			SessionID: env.SessionID,
			Timestamp: int64(intFromAny(data["timestamp"])),
		})
	})
}

// registerBroadcastHandlers registers handlers for daemon-broadcast notifications
// that are not tied to a single client request (BroadcastNotification 方法名，
// 非 ResponseEnvelope 信封）。
func (m *rootModel) registerBroadcastHandlers() {
	c := m.rpc.client

	// ── 调度任务生命周期：schedule.job_started/completed/failed/missed ──
	for _, method := range []string{"schedule.job_started", "schedule.job_completed", "schedule.job_failed", "schedule.job_missed"} {
		c.On(method, func(_ context.Context, raw json.RawMessage) {
			var payload struct {
				Data schedulerJobLifecyclePayload `json:"data"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return
			}
			info := payload.Data
			if info.SessionID == "" && len(raw) > 0 {
				// 兜底读取顶层 session_id（broadcastJobLifecycle 同时写在顶层）。
				var top struct {
					SessionID string `json:"session_id"`
				}
				if err := json.Unmarshal(raw, &top); err == nil {
					info.SessionID = top.SessionID
				}
			}
			m.program.Send(clientmsg.ScheduleJobMsg{
				EntryID:    info.EntryID,
				RunID:      info.RunID,
				Agent:      info.Agent,
				SessionID:  info.SessionID,
				Status:     info.Status,
				Content:    info.Content,
				ProjectDir: info.ProjectDir,
				Error:      info.Error,
			})
		})
	}

	// ── 自动升级：update_started / update_installed ──
	for _, phase := range []string{"started", "installed"} {
		method := "update_" + phase
		c.On(method, func(_ context.Context, raw json.RawMessage) {
			var payload struct {
				Data struct {
					Version string `json:"version"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return
			}
			m.program.Send(clientmsg.DaemonUpdateMsg{Phase: phase, Version: payload.Data.Version})
		})
	}

	// ── 上下文压缩广播：compact_* / micro_compact_* ──
	for _, variant := range []struct {
		method string
		micro  bool
		done   bool
	}{
		{"compact_start", false, false},
		{"compact_done", false, true},
		{"micro_compact_start", true, false},
		{"micro_compact_done", true, true},
	} {
		variant := variant
		c.On(variant.method, func(_ context.Context, raw json.RawMessage) {
			var payload struct {
				SessionID string         `json:"session_id"`
				Data      map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return
			}
			phase := "start"
			if variant.done {
				phase = "done"
			}
			m.program.Send(clientmsg.ContextCompactionMsg{
				SessionID:    payload.SessionID,
				Micro:        variant.micro,
				Phase:        phase,
				WindowTokens: intFromAny(payload.Data["window_tokens"]),
				MessagesSlid: intFromAny(payload.Data["messages_slid"]),
				Compressed:   intFromAny(payload.Data["compressed"]),
				Deduped:      intFromAny(payload.Data["deduped"]),
				Ratio:        floatFromAny(payload.Data["ratio"]),
			})
		})
	}

	// ── 上下文窗口占用：context_usage ──
	c.On("context_usage", func(_ context.Context, raw json.RawMessage) {
		var payload struct {
			SessionID string         `json:"session_id"`
			Data      map[string]any `json:"data"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return
		}
		d := payload.Data
		m.program.Send(clientmsg.ContextUsageMsg{
			SessionID:          payload.SessionID,
			WindowTokens:       floatFromAny(d["window_tokens"]),
			MaxWindowSize:      floatFromAny(d["max_window_size"]),
			UsageRatio:         floatFromAny(d["usage_ratio"]),
			MessageCount:       intFromAny(d["message_count"]),
			Cursor:             intFromAny(d["cursor"]),
			ActiveMessageCount: intFromAny(d["active_message_count"]),
			TotalActualTokens:  floatFromAny(d["total_actual_tokens"]),
			TotalCost:          floatFromAny(d["total_cost"]),
		})
	})

	// ── 终端会话推送：terminal.output / terminal.exit（ResponseEnvelope 形态）──
	c.On("terminal.output", func(_ context.Context, raw json.RawMessage) {
		env, ok := decodeEnvelope(raw)
		if !ok {
			return
		}
		m.program.Send(clientmsg.TerminalOutputMsg{
			SessionID: env.SessionID,
			Data:      toString(env.Data),
		})
	})

	c.On("terminal.exit", func(_ context.Context, raw json.RawMessage) {
		env, ok := decodeEnvelope(raw)
		if !ok {
			return
		}
		m.program.Send(clientmsg.TerminalExitMsg{
			SessionID: env.SessionID,
			ExitCode:  intFromAny(env.Meta["exit_code"]),
		})
	})
}

// schedulerJobLifecyclePayload 对应 pkg/scheduler.JobLifecycleInfo 的 JSON 形态。
type schedulerJobLifecyclePayload struct {
	EntryID    string `json:"entry_id"`
	RunID      string `json:"run_id"`
	Agent      string `json:"agent"`
	SessionID  string `json:"session_id"`
	Content    string `json:"content"`
	ProjectDir string `json:"project_dir"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

// decodeEnvelope 解析 SendResponse 下发的 ResponseEnvelope 通知。
func decodeEnvelope(raw json.RawMessage) (gateway.ResponseEnvelope, bool) {
	var env gateway.ResponseEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return env, false
	}
	return env, true
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(v)
	}
}

func toBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	default:
		return false
	}
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func boolFromAny(v any) bool {
	b, _ := v.(bool)
	return b
}

// intFromAny 把 JSON 数字（解码为 float64）安全转为 int。
func intFromAny(v any) int {
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int(f)
}

func floatFromAny(v any) float64 {
	f, _ := v.(float64)
	return f
}

// ---------------------------------------------------------------------------
// 出站消息（user.message 通知）
// ---------------------------------------------------------------------------

func (m *rootModel) rpcSendMessage(text string) {
	if !m.rpcIsConnected() {
		m.notifBar.Add(data.Notification{
			Message: i18n.T("client.notify.rpc.disconnected"),
			Level:   data.NotifError,
		})
		return
	}

	if err := m.sendUserMessage(text, "", ""); err != nil {
		m.notifBar.Add(data.Notification{
			Message: fmt.Sprintf(i18n.T("client.notify.rpc.send.failed"), err),
			Level:   data.NotifError,
		})
		return
	}

	m.executing = true
	m.statusBar.CurrentState = i18n.T("client.status.thinking")
	m.statusBar.SessionStart = time.Now()
	m.statusBar.SessionDuration = 0
}

// sendUserMessage 通过 user.message 通知发送用户消息。
// jobEntryID/jobRunID 非空时随消息透传调度任务标识（前台执行链路：
// 本轮对话结束后服务端据此 ReportResult 并广播 completed/failed）。
func (m *rootModel) sendUserMessage(text, jobEntryID, jobRunID string) error {
	payload := map[string]string{"text": text}
	if m.currentSessionID != "" {
		payload["session_id"] = m.currentSessionID
	}
	if jobEntryID != "" {
		payload["job_entry_id"] = jobEntryID
	}
	if jobRunID != "" {
		payload["job_run_id"] = jobRunID
	}
	return m.rpc.client.Notify("user.message", payload)
}

// rpcSendMagicWord 将授权魔法词送达会话：RPC 模式经 user.message 通知，
// 本地模式经 UserSendMsg 进入内嵌 runtime。用于授权允许/允许会话/拒绝决策。
func (m *rootModel) rpcSendMagicWord(magicWord string) {
	if m.rpcConnected {
		m.rpcSendMessage(magicWord)
		return
	}
	m.program.Send(clientmsg.UserSendMsg{Text: magicWord})
}

func (m *rootModel) rpcCancelExecution() {
	if !m.rpcIsConnected() {
		return
	}
	go func() {
		_, _ = m.rpc.client.Call(context.Background(), "message.cancel", map[string]any{})
	}()
}

func parseTokenUsage(value string, tu *session.TokenUsage) {
	// format: "1234 (in:100 out:200 cached:50 reasoning:50)"
	// The leading number is the billable/effective total (prompt + completion - cached);
	// TotalTokens must be the raw API total (prompt + completion).
	if idx := strings.Index(value, "("); idx >= 0 {
		inner := value[idx+1 : len(value)-1]
		var in, out, cached, reasoning int
		_, _ = fmt.Sscanf(inner, "in:%d out:%d cached:%d reasoning:%d", &in, &out, &cached, &reasoning)
		tu.PromptTokens = in
		tu.CompletionTokens = out
		tu.CachedTokens = cached
		tu.ReasoningTokens = reasoning
		tu.TotalTokens = in + out
	}
}

// ---------------------------------------------------------------------------
// 出站 JSON-RPC 指令层
//
// 与 svc/handler_registry.go 注册的方法一一对应。参数键名以各 handler 的
// 解析逻辑为准；响应形态简单的解码为基础类型，复杂的返回原始 map 由调用方取舍。
// ---------------------------------------------------------------------------

// call 发送 JSON-RPC 请求并等待响应（默认超时），返回原始结果。
func (r *daemonRPCClient) call(method string, params any) (json.RawMessage, error) {
	if !r.connected || r.client == nil || !r.client.IsConnected() {
		return nil, errors.New("daemon not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcCallTimeout)
	defer cancel()
	return r.client.Call(ctx, method, params)
}

// callInto 发送请求并把结果解码到 out（out 为 nil 时忽略结果）。
func (r *daemonRPCClient) callInto(method string, params any, out any) error {
	raw, err := r.call(method, params)
	if err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// ── 会话：session.* ──────────────────────────────────────────

// SessionList 列出会话；agent 非空时按智能体过滤。
func (r *daemonRPCClient) SessionList(agent string) ([]session.SessionInfo, error) {
	var out []session.SessionInfo
	err := r.callInto("session.list", map[string]any{"agent": agent}, &out)
	return out, err
}

// SessionLatestByDir 返回指定项目目录最近活动的会话；无会话时返回 nil。
func (r *daemonRPCClient) SessionLatestByDir(projectDir string) (*session.SessionInfo, error) {
	var out *session.SessionInfo
	err := r.callInto("session.latest_by_dir", map[string]any{"project_dir": projectDir}, &out)
	return out, err
}

// SessionGet 读取会话详情；includeSlid 时附带已压缩滑出的消息。
func (r *daemonRPCClient) SessionGet(sessionID string, includeSlid bool) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("session.get", map[string]any{"session_id": sessionID, "include_slid": includeSlid}, &out)
	return out, err
}

func (r *daemonRPCClient) SessionMeta(sessionID string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("session.meta", map[string]any{"session_id": sessionID}, &out)
	return out, err
}

// SessionCreate 为指定智能体在 projectDir 创建新会话，返回 {session_id,...}。
func (r *daemonRPCClient) SessionCreate(agent, projectDir string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("session.create", map[string]any{"agent": agent, "project_dir": projectDir}, &out)
	return out, err
}

func (r *daemonRPCClient) SessionDelete(sessionID string) error {
	return r.callInto("session.delete", map[string]any{"session_id": sessionID}, nil)
}

// SessionConfirmFiles 确认（保留）会话追踪的修改文件；files 为空时确认全部。
func (r *daemonRPCClient) SessionConfirmFiles(sessionID string, files []string) error {
	return r.callInto("session.confirm_files", map[string]any{"session_id": sessionID, "files": files}, nil)
}

// SessionRollbackFiles 从备份回滚会话追踪的修改文件；files 为空时回滚全部。
func (r *daemonRPCClient) SessionRollbackFiles(sessionID string, files []string) error {
	return r.callInto("session.rollback_files", map[string]any{"session_id": sessionID, "files": files}, nil)
}

// SessionContextStats 会话上下文窗口占用统计（session.context / session.compact 共用）。
type SessionContextStats struct {
	SessionID          string  `json:"session_id"`
	WindowTokens       float64 `json:"window_tokens"`
	MaxWindowSize      float64 `json:"max_window_size"`
	UsageRatio         float64 `json:"usage_ratio"`
	MessageCount       int     `json:"message_count"`
	Cursor             int     `json:"cursor"`
	ActiveMessageCount int     `json:"active_message_count"`
	TotalActualTokens  float64 `json:"total_actual_tokens"`
	TotalCost          float64 `json:"total_cost"`
}

func (r *daemonRPCClient) SessionContext(sessionID string) (*SessionContextStats, error) {
	var out *SessionContextStats
	err := r.callInto("session.context", map[string]any{"session_id": sessionID}, &out)
	return out, err
}

func (r *daemonRPCClient) SessionTruncate(sessionID string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("session.truncate", map[string]any{"session_id": sessionID}, &out)
	return out, err
}

// SessionDeleteRound 按 backendTimestamp 回收一轮对话（user+assistant）。
func (r *daemonRPCClient) SessionDeleteRound(sessionID string, timestamp int64) error {
	return r.callInto("session.delete_round", map[string]any{"session_id": sessionID, "id": timestamp}, nil)
}

// SessionCompact 手动触发上下文压缩；mode 为空或 "full"，或 "micro"。
func (r *daemonRPCClient) SessionCompact(sessionID, mode string) (*SessionContextStats, error) {
	params := map[string]any{"session_id": sessionID}
	if mode != "" {
		params["mode"] = mode
	}
	var out *SessionContextStats
	err := r.callInto("session.compact", params, &out)
	return out, err
}

// ── 记忆：memory.* ───────────────────────────────────────────

// MemoryQuery 检索共享记忆；limit/minScore 非正时使用服务端默认值。
func (r *daemonRPCClient) MemoryQuery(query string, limit int, minScore float64) ([]map[string]any, error) {
	params := map[string]any{"query": query}
	if limit > 0 {
		params["limit"] = limit
	}
	if minScore > 0 {
		params["min_score"] = minScore
	}
	var out []map[string]any
	err := r.callInto("memory.query", params, &out)
	return out, err
}

// MemoryStore 存储记忆片段，返回记录 ID。
func (r *daemonRPCClient) MemoryStore(content, title string) (string, error) {
	params := map[string]any{"content": content}
	if title != "" {
		params["title"] = title
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.callInto("memory.store", params, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (r *daemonRPCClient) MemoryDelete(id string) error {
	return r.callInto("memory.delete", map[string]any{"id": id}, nil)
}

// MemoryUpdate 局部更新记忆；空字符串字段跳过，tags 为 nil 时跳过。
func (r *daemonRPCClient) MemoryUpdate(id, title, summary, content string, tags []string) error {
	params := map[string]any{"id": id}
	if title != "" {
		params["title"] = title
	}
	if summary != "" {
		params["summary"] = summary
	}
	if content != "" {
		params["content"] = content
	}
	if tags != nil {
		params["tags"] = tags
	}
	return r.callInto("memory.update", params, nil)
}

// MemoryListBySession 返回指定会话的记忆条目（时间倒序）。
func (r *daemonRPCClient) MemoryListBySession(sessionID string) ([]map[string]any, error) {
	var out struct {
		Chunks []map[string]any `json:"chunks"`
	}
	err := r.callInto("memory.list_by_session", map[string]any{"session_id": sessionID}, &out)
	return out.Chunks, err
}

// MemoryChunks 分页浏览记忆块；page/pageSize 非正时使用服务端默认值。
func (r *daemonRPCClient) MemoryChunks(page, pageSize int) (map[string]any, error) {
	params := map[string]any{}
	if page > 0 {
		params["page"] = page
	}
	if pageSize > 0 {
		params["page_size"] = pageSize
	}
	var out map[string]any
	err := r.callInto("memory.chunks", params, &out)
	return out, err
}

func (r *daemonRPCClient) MemoryGetChunks(docID string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("memory.get_chunks", map[string]any{"doc_id": docID}, &out)
	return out, err
}

func (r *daemonRPCClient) MemoryCount() (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	err := r.callInto("memory.count", nil, &out)
	return out.Count, err
}

// ── 智能体：agent.* ──────────────────────────────────────────

// AgentUpsertParams 用于 agent.create / agent.update；
// 空字段在 update 中表示不修改（create 时 name/role/description/model 必填）。
type AgentUpsertParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role,omitempty"`
	Description  string         `json:"description,omitempty"`
	Model        string         `json:"model,omitempty"`
	Body         string         `json:"body,omitempty"` // 仅 create 生效（introduction 为空时的正文）
	Introduction string         `json:"introduction,omitempty"`
	Skills       []string       `json:"skills,omitempty"`
	ExcludeTools []string       `json:"exclude_tools,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (r *daemonRPCClient) AgentList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("agent.list", nil, &out)
	return out, err
}

func (r *daemonRPCClient) AgentGet(name string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("agent.get", map[string]any{"name": name}, &out)
	return out, err
}

func (r *daemonRPCClient) AgentCreate(p AgentUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("agent.create", p, &out)
	return out, err
}

func (r *daemonRPCClient) AgentUpdate(p AgentUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("agent.update", p, &out)
	return out, err
}

// AgentScore 为智能体在指定任务上的表现打分（1-10）；notes 可为空。
func (r *daemonRPCClient) AgentScore(agentName, task string, score int, notes string) (map[string]any, error) {
	params := map[string]any{"agent_name": agentName, "task": task, "score": score}
	if notes != "" {
		params["notes"] = notes
	}
	var out map[string]any
	err := r.callInto("agent.score", params, &out)
	return out, err
}

func (r *daemonRPCClient) AgentReload() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("agent.reload", nil, &out)
	return out, err
}

// ── 模型与提供商：model.* / provider.* ───────────────────────

// ModelUpsertParams 用于 model.create / model.update。
// 注意：布尔字段因 omitempty 无法显式置 false（update 中省略即保持不变）。
type ModelUpsertParams struct {
	Name          string  `json:"name"`
	Title         string  `json:"title,omitempty"`
	Description   string  `json:"description,omitempty"`
	Provider      string  `json:"provider,omitempty"`
	BaseURL       string  `json:"base_url,omitempty"`
	APIKey        string  `json:"api_key,omitempty"`
	AuthToken     string  `json:"auth_token,omitempty"`
	ContextLength int64   `json:"context_length,omitempty"`
	IsLocal       bool    `json:"is_local,omitempty"`
	FuncCalling   bool    `json:"func_calling,omitempty"`
	Structuring   bool    `json:"structuring,omitempty"`
	WebSearching  bool    `json:"web_searching,omitempty"`
	Visioning     bool    `json:"visioning,omitempty"`
	PrefixCon     bool    `json:"prefix_con,omitempty"`
	ContextCache  bool    `json:"context_cache,omitempty"`
	Enabled       bool    `json:"enabled,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	MaxTurns      int     `json:"max_turns,omitempty"`
	CostPer1MIn   float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut  float64 `json:"cost_per_1m_out,omitempty"`
}

func (r *daemonRPCClient) ModelList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("model.list", nil, &out)
	return out, err
}

func (r *daemonRPCClient) ModelGet(name string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("model.get", map[string]any{"name": name}, &out)
	return out, err
}

// ModelSwitch 切换当前模型；provider 非空时同时设置默认提供商。
func (r *daemonRPCClient) ModelSwitch(name, provider string) (map[string]any, error) {
	params := map[string]any{"name": name}
	if provider != "" {
		params["provider"] = provider
	}
	var out map[string]any
	err := r.callInto("model.switch", params, &out)
	return out, err
}

func (r *daemonRPCClient) ModelCreate(p ModelUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("model.create", p, &out)
	return out, err
}

func (r *daemonRPCClient) ModelUpdate(p ModelUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("model.update", p, &out)
	return out, err
}

func (r *daemonRPCClient) ModelDelete(name string) error {
	return r.callInto("model.delete", map[string]any{"name": name}, nil)
}

// ProviderUpsertParams 用于 provider.create / provider.update；
// api_key/auth_token 实际存入凭据存储，YAML 仅保存引用名。
type ProviderUpsertParams struct {
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	AuthToken string `json:"auth_token,omitempty"`
	IsLocal   bool   `json:"is_local,omitempty"`
}

func (r *daemonRPCClient) ProviderList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("provider.list", nil, &out)
	return out, err
}

func (r *daemonRPCClient) ProviderCreate(p ProviderUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("provider.create", p, &out)
	return out, err
}

func (r *daemonRPCClient) ProviderUpdate(p ProviderUpsertParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("provider.update", p, &out)
	return out, err
}

func (r *daemonRPCClient) ProviderDelete(name string) error {
	return r.callInto("provider.delete", map[string]any{"name": name}, nil)
}

// ProviderFetchOllamaModels 探测 Ollama/OpenAI 兼容端点的可用模型列表。
func (r *daemonRPCClient) ProviderFetchOllamaModels(baseURL string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("provider.fetch_ollama_models", map[string]any{"base_url": baseURL}, &out)
	return out, err
}

func (r *daemonRPCClient) ProviderFetchOllamaModelDetail(baseURL, modelName string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("provider.fetch_ollama_model_detail",
		map[string]any{"base_url": baseURL, "model_name": modelName}, &out)
	return out, err
}

// ── 技能：skill.* ────────────────────────────────────────────

func (r *daemonRPCClient) SkillList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("skill.list", nil, &out)
	return out, err
}

func (r *daemonRPCClient) SkillGet(name string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("skill.get", map[string]any{"name": name}, &out)
	return out, err
}

func (r *daemonRPCClient) SkillReload() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("skill.reload", nil, &out)
	return out, err
}

// ── 文件系统：fs.* ───────────────────────────────────────────

// FSList 列出目录内容；path 为空时默认用户主目录。
func (r *daemonRPCClient) FSList(path string) ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("fs.list", map[string]any{"path": path}, &out)
	return out, err
}

// FSRead 读取文本文件内容（上限 100MB）。
func (r *daemonRPCClient) FSRead(path string) (string, error) {
	var out struct {
		Content string `json:"content"`
	}
	err := r.callInto("fs.read", map[string]any{"path": path}, &out)
	return out.Content, err
}

// FSReadBase64 以 base64 读取二进制文件（上限 50MB），同时返回 MIME 类型。
func (r *daemonRPCClient) FSReadBase64(path string) (content, mime string, err error) {
	var out struct {
		Content string `json:"content"`
		Mime    string `json:"mime"`
	}
	err = r.callInto("fs.read_base64", map[string]any{"path": path}, &out)
	return out.Content, out.Mime, err
}

func (r *daemonRPCClient) FSWrite(path, content string) error {
	return r.callInto("fs.write", map[string]any{"path": path, "content": content}, nil)
}

// FSHome 返回用户主目录路径。
func (r *daemonRPCClient) FSHome() (string, error) {
	var out struct {
		Path string `json:"path"`
	}
	err := r.callInto("fs.home", nil, &out)
	return out.Path, err
}

func (r *daemonRPCClient) FSMkdir(path string) error {
	return r.callInto("fs.mkdir", map[string]any{"path": path}, nil)
}

// FSRm 删除文件或目录；recurse 递归删除，force 强制删除非空目录。
func (r *daemonRPCClient) FSRm(path string, recurse, force bool) error {
	return r.callInto("fs.rm", map[string]any{"path": path, "recurse": recurse, "force": force}, nil)
}

func (r *daemonRPCClient) FSMv(src, dst string) error {
	return r.callInto("fs.mv", map[string]any{"src": src, "dst": dst}, nil)
}

// FSReveal 在系统文件管理器中显示路径。
func (r *daemonRPCClient) FSReveal(path string) error {
	return r.callInto("fs.reveal", map[string]any{"path": path}, nil)
}

// FSStat 返回文件元信息 {name,path,size,is_dir,mode,mod_time}。
func (r *daemonRPCClient) FSStat(path string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("fs.stat", map[string]any{"path": path}, &out)
	return out, err
}

// ── Git：git.clone ───────────────────────────────────────────

// GitClone 克隆仓库到 dir/<仓库名>，返回完整克隆目录。
func (r *daemonRPCClient) GitClone(url, dir string) (string, error) {
	var out struct {
		Dir string `json:"dir"`
	}
	err := r.callInto("git.clone", map[string]any{"url": url, "dir": dir}, &out)
	return out.Dir, err
}

// ── 终端：terminal.* ─────────────────────────────────────────

// TerminalStart 在 cwd 启动持久终端会话（空则用后台服务工作目录），
// 返回终端会话 ID；输出经 terminal.output 推送，退出经 terminal.exit 推送。
func (r *daemonRPCClient) TerminalStart(cwd string) (string, error) {
	params := map[string]any{}
	if cwd != "" {
		params["cwd"] = cwd
	}
	var out struct {
		SessionID string `json:"session_id"`
	}
	if err := r.callInto("terminal.start", params, &out); err != nil {
		return "", err
	}
	return out.SessionID, nil
}

func (r *daemonRPCClient) TerminalInput(sessionID, data string) error {
	return r.callInto("terminal.input", map[string]any{"session_id": sessionID, "data": data}, nil)
}

func (r *daemonRPCClient) TerminalResize(sessionID string, rows, cols int) error {
	return r.callInto("terminal.resize",
		map[string]any{"session_id": sessionID, "rows": rows, "cols": cols}, nil)
}

func (r *daemonRPCClient) TerminalKill(sessionID string) error {
	return r.callInto("terminal.kill", map[string]any{"session_id": sessionID}, nil)
}

// TerminalList 返回当前客户端的终端会话 ID 列表。
func (r *daemonRPCClient) TerminalList() ([]string, error) {
	var out struct {
		Sessions []struct {
			SessionID string `json:"session_id"`
		} `json:"sessions"`
	}
	if err := r.callInto("terminal.list", nil, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Sessions))
	for _, s := range out.Sessions {
		ids = append(ids, s.SessionID)
	}
	return ids, nil
}

// ── 日志：log.* ──────────────────────────────────────────────

// LogRead 逆向分页读取后台日志；stream 为 "main" 或 "error"（空则 main）。
func (r *daemonRPCClient) LogRead(limit, offset int, stream string) (map[string]any, error) {
	params := map[string]any{}
	if limit > 0 {
		params["limit"] = limit
	}
	if offset > 0 {
		params["offset"] = offset
	}
	if stream != "" {
		params["stream"] = stream
	}
	var out map[string]any
	err := r.callInto("log.read", params, &out)
	return out, err
}

// LogClear 清空后台日志文件。
func (r *daemonRPCClient) LogClear() error {
	return r.callInto("log.clear", map[string]any{"confirmed": true}, nil)
}

// LogCount 返回各日志文件的字节/行数统计。
func (r *daemonRPCClient) LogCount() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("log.count", nil, &out)
	return out, err
}

// ── 多语言：i18n.* ───────────────────────────────────────────

func (r *daemonRPCClient) I18nGet() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("i18n.get", nil, &out)
	return out, err
}

// I18nSwitch 切换界面语言（BCP47 标签，如 zh-CN / en）。
func (r *daemonRPCClient) I18nSwitch(lang string) error {
	return r.callInto("i18n.switch", map[string]any{"lang": lang}, nil)
}

func (r *daemonRPCClient) I18nList() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("i18n.list", nil, &out)
	return out, err
}

// ── 知识图谱：graph.* ────────────────────────────────────────

// GraphNodeParam 图节点 upsert 参数；ID 必填。
type GraphNodeParam struct {
	ID         string         `json:"id"`
	Labels     []string       `json:"labels,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// GraphEdgeParam 图边 upsert 参数；三个 ID/类型字段必填。
type GraphEdgeParam struct {
	FromNodeID string         `json:"from_node_id"`
	ToNodeID   string         `json:"to_node_id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
}

// GraphQuery 执行只读 Cypher 查询，返回 {columns, rows}。
func (r *daemonRPCClient) GraphQuery(query string, params map[string]any) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("graph.query", map[string]any{"query": query, "params": params}, &out)
	return out, err
}

// GraphExec 执行写操作 Cypher 语句。
func (r *daemonRPCClient) GraphExec(query string, params map[string]any) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("graph.exec", map[string]any{"query": query, "params": params}, &out)
	return out, err
}

// GraphUpsertNodes 批量 upsert 节点，返回写入数量。
func (r *daemonRPCClient) GraphUpsertNodes(nodes []GraphNodeParam) (int, error) {
	var out struct {
		Upserted int `json:"upserted"`
	}
	err := r.callInto("graph.upsert_nodes", map[string]any{"nodes": nodes}, &out)
	return out.Upserted, err
}

// GraphUpsertEdges 批量 upsert 边，返回写入数量。
func (r *daemonRPCClient) GraphUpsertEdges(edges []GraphEdgeParam) (int, error) {
	var out struct {
		Upserted int `json:"upserted"`
	}
	err := r.callInto("graph.upsert_edges", map[string]any{"edges": edges}, &out)
	return out.Upserted, err
}

func (r *daemonRPCClient) GraphGetNode(id string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("graph.get_node", map[string]any{"id": id}, &out)
	return out, err
}

// GraphGetNeighbors 返回邻居节点及连接边；depth/limit 非正时用服务端默认值，
// types 为空时不过滤边类型。
func (r *daemonRPCClient) GraphGetNeighbors(id string, depth, limit int, types []string) ([]map[string]any, error) {
	params := map[string]any{"id": id}
	if depth > 0 {
		params["depth"] = depth
	}
	if limit > 0 {
		params["limit"] = limit
	}
	if len(types) > 0 {
		params["types"] = types
	}
	var out []map[string]any
	err := r.callInto("graph.get_neighbors", params, &out)
	return out, err
}

func (r *daemonRPCClient) GraphListNodes() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("graph.list_nodes", nil, &out)
	return out, err
}

func (r *daemonRPCClient) GraphListEdges() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("graph.list_edges", nil, &out)
	return out, err
}

// ── KV 存储：kvstore.* ───────────────────────────────────────

// KVGet 返回 {found, item:{key,value,created_at,expires_at}}。
func (r *daemonRPCClient) KVGet(key string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("kvstore.get", map[string]any{"key": key}, &out)
	return out, err
}

// KVSet 写入键值；ttlSeconds 大于 0 时设置过期时间。
func (r *daemonRPCClient) KVSet(key string, value any, ttlSeconds int) error {
	params := map[string]any{"key": key, "value": value}
	if ttlSeconds > 0 {
		params["ttl"] = ttlSeconds
	}
	return r.callInto("kvstore.set", params, nil)
}

func (r *daemonRPCClient) KVDelete(key string) error {
	return r.callInto("kvstore.delete", map[string]any{"key": key}, nil)
}

// KVList 按前缀列出键；withValues 为 true 时附带值，limit 非正时默认 100。
func (r *daemonRPCClient) KVList(prefix string, limit int, withValues bool) (map[string]any, error) {
	params := map[string]any{}
	if prefix != "" {
		params["prefix"] = prefix
	}
	if limit > 0 {
		params["limit"] = limit
	}
	if withValues {
		params["with_values"] = true
	}
	var out map[string]any
	err := r.callInto("kvstore.list", params, &out)
	return out, err
}

// KVBatchEntry 批量写入条目。
type KVBatchEntry struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

// KVBatchSet 批量写入，返回实际写入的键列表。
func (r *daemonRPCClient) KVBatchSet(entries []KVBatchEntry) ([]string, error) {
	var out struct {
		WroteKeys []string `json:"wrote_keys"`
	}
	err := r.callInto("kvstore.batch_set", map[string]any{"entries": entries}, &out)
	return out.WroteKeys, err
}

// KVClear 清空前缀匹配的键（空前缀清空全部），返回删除数量。
func (r *daemonRPCClient) KVClear(prefix string) (int, error) {
	var out struct {
		Deleted int `json:"deleted"`
	}
	err := r.callInto("kvstore.clear", map[string]any{"prefix": prefix}, &out)
	return out.Deleted, err
}

// ── 规则：rule.* ─────────────────────────────────────────────

// RuleList 返回全部规则（{count, rules:[...]} 的 rules 部分）。
func (r *daemonRPCClient) RuleList() ([]map[string]any, error) {
	var out struct {
		Rules []map[string]any `json:"rules"`
	}
	err := r.callInto("rule.list", nil, &out)
	return out.Rules, err
}

// RuleGet 返回 {found, rule?}。
func (r *daemonRPCClient) RuleGet(id string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("rule.get", map[string]any{"id": id}, &out)
	return out, err
}

// RuleCreate 创建规则；scope 为空时服务端默认 global。
func (r *daemonRPCClient) RuleCreate(id, intro, scope string, priority int, enabled bool) error {
	params := map[string]any{"id": id, "intro": intro, "priority": priority, "enabled": enabled}
	if scope != "" {
		params["scope"] = scope
	}
	return r.callInto("rule.create", params, nil)
}

// RuleUpdateParams 局部更新规则；指针为 nil 的字段不修改。
type RuleUpdateParams struct {
	ID       string  `json:"id"`
	Intro    *string `json:"intro,omitempty"`
	Scope    *string `json:"scope,omitempty"`
	Priority *int    `json:"priority,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
}

func (r *daemonRPCClient) RuleUpdate(p RuleUpdateParams) error {
	return r.callInto("rule.update", p, nil)
}

func (r *daemonRPCClient) RuleDelete(id string) error {
	return r.callInto("rule.delete", map[string]any{"id": id}, nil)
}

// ── 调度任务：schedule.* ─────────────────────────────────────

func (r *daemonRPCClient) ScheduleList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("schedule.list", nil, &out)
	return out, err
}

// ScheduleAddParams 创建调度任务：CronExpr 与 ScheduledAt 二选一必填
// （ScheduledAt 格式 "YYYY-MM-DDTHH:mm:ss" 或 RFC3339）。
type ScheduleAddParams struct {
	Agent       string `json:"agent"`
	Content     string `json:"content"`
	CronExpr    string `json:"cron_expr,omitempty"`
	ScheduledAt string `json:"scheduled_at,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	ProjectDir  string `json:"project_dir,omitempty"`
}

// ScheduleAdd 创建任务并返回完整 ScheduleEntry（schedule.create 为其服务端别名）。
func (r *daemonRPCClient) ScheduleAdd(p ScheduleAddParams) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("schedule.add", p, &out)
	return out, err
}

func (r *daemonRPCClient) ScheduleDelete(id string) error {
	return r.callInto("schedule.del", map[string]any{"id": id}, nil)
}

// ScheduleJobCancel 取消一次在途运行（幂等：已结束返回 already_finished）。
func (r *daemonRPCClient) ScheduleJobCancel(entryID, runID string) error {
	return r.callInto("schedule.job_cancel", map[string]any{"entry_id": entryID, "run_id": runID}, nil)
}

// ── 服务端管理：server.* / user.config ───────────────────────

func (r *daemonRPCClient) ServerVersion() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("server.version", nil, &out)
	return out, err
}

// ServerCheckUpdate 检查新版本，返回 UpdateInfo + install_source。
func (r *daemonRPCClient) ServerCheckUpdate() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("server.check_update", nil, &out)
	return out, err
}

// ServerApplyUpdate 下载安装新版本并异步重启后台进程。
func (r *daemonRPCClient) ServerApplyUpdate() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("server.apply_update", nil, &out)
	return out, err
}

func (r *daemonRPCClient) ServerRestartDaemon() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("server.restart_daemon", nil, &out)
	return out, err
}

// UserConfig 读取或局部更新用户配置；changes 为 nil 或空时纯读。
// 已知键：last_agent/last_session_id/last_model/default_model/default_provider/
// language/kb_addr（string）、auto_indexing（bool）。
func (r *daemonRPCClient) UserConfig(changes map[string]any) (map[string]any, error) {
	var params any
	if len(changes) > 0 {
		params = changes
	}
	var out map[string]any
	err := r.callInto("user.config", params, &out)
	return out, err
}

// ── Token 用量：token.usage.* ────────────────────────────────

func (r *daemonRPCClient) TokenUsageOverview() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("token.usage.overview", nil, &out)
	return out, err
}

func (r *daemonRPCClient) TokenUsageMonthly(year, month int) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("token.usage.monthly", map[string]any{"year": year, "month": month}, &out)
	return out, err
}

// TokenUsageByModel 按模型统计用量；year/month 非正时默认当前月。
func (r *daemonRPCClient) TokenUsageByModel(model string, year, month int) ([]map[string]any, error) {
	params := map[string]any{"model": model}
	if year > 0 {
		params["year"] = year
	}
	if month > 0 {
		params["month"] = month
	}
	var out []map[string]any
	err := r.callInto("token.usage.by_model", params, &out)
	return out, err
}

func (r *daemonRPCClient) TokenUsageTotal() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("token.usage.total", nil, &out)
	return out, err
}

func (r *daemonRPCClient) TokenUsageSession(sessionID string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("token.usage.session", map[string]any{"session_id": sessionID}, &out)
	return out, err
}

// TokenUsageSessionDetail 返回会话内逐次 LLM 调用的用量记录。
func (r *daemonRPCClient) TokenUsageSessionDetail(sessionID string) (map[string]any, error) {
	var out map[string]any
	err := r.callInto("token.usage.session.detail", map[string]any{"session_id": sessionID}, &out)
	return out, err
}

// ── 文本处理：translate.rpc / optimize.rpc ───────────────────

// Translate 翻译文本到目标语言，返回译文（服务端带 KV 缓存）。
func (r *daemonRPCClient) Translate(text, lang string) (string, error) {
	var out struct {
		Text string `json:"text"`
	}
	err := r.callInto("translate.rpc", map[string]any{"text": text, "lang": lang}, &out)
	return out.Text, err
}

// Optimize 优化提示词文本，返回优化结果。
func (r *daemonRPCClient) Optimize(text string) (string, error) {
	var out struct {
		Text string `json:"text"`
	}
	err := r.callInto("optimize.rpc", map[string]any{"text": text}, &out)
	return out.Text, err
}

// ── MCP：mcp.server.* / mcp.manifest.* ───────────────────────

// MCPServerAddParams 注册 MCP 服务器；stdio 型需 command，sse/http 型需 url。
type MCPServerAddParams struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // stdio | sse | http
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Credential  map[string]string `json:"credential,omitempty"` // 只存引用名，实际值入凭据存储
	IdleTTLSecs int               `json:"idle_ttl_secs,omitempty"`
}

func (r *daemonRPCClient) MCPServerAdd(p MCPServerAddParams) error {
	return r.callInto("mcp.server.add", p, nil)
}

func (r *daemonRPCClient) MCPServerRemove(name string) error {
	return r.callInto("mcp.server.remove", map[string]any{"name": name}, nil)
}

func (r *daemonRPCClient) MCPServerList() ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("mcp.server.list", nil, &out)
	return out, err
}

// MCPServerTest 测试连通性；ok 为 false 时携带业务失败原因。
func (r *daemonRPCClient) MCPServerTest(name string) (bool, error) {
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := r.callInto("mcp.server.test", map[string]any{"name": name}, &out); err != nil {
		return false, err
	}
	return out.OK, nil
}

// MCPServerDiscover 发现服务器暴露的工具清单。
func (r *daemonRPCClient) MCPServerDiscover(name string) ([]map[string]any, error) {
	var out []map[string]any
	err := r.callInto("mcp.server.discover", map[string]any{"name": name}, &out)
	return out, err
}

// MCPManifestEntry 工具清单条目；name 由服务端生成为 mcp:<server>:<mcp_name>。
type MCPManifestEntry struct {
	Description string `json:"description"`
	Server      string `json:"server"`
	MCPName     string `json:"mcp_name"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// MCPManifestSave 保存工具清单，返回工具数量。
func (r *daemonRPCClient) MCPManifestSave(tools []MCPManifestEntry) (int, error) {
	var out struct {
		ToolCount int `json:"tool_count"`
	}
	err := r.callInto("mcp.manifest.save", map[string]any{"tools": tools}, &out)
	return out.ToolCount, err
}

// MCPManifestGet 返回 {version, updated_at, tools}。
func (r *daemonRPCClient) MCPManifestGet() (map[string]any, error) {
	var out map[string]any
	err := r.callInto("mcp.manifest.get", nil, &out)
	return out, err
}
