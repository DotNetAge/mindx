package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const daemonConnectTimeout = 5 * time.Second

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

}

func (m *rootModel) rpcIsConnected() bool {
	return m.rpc != nil && m.rpc.connected && m.rpc.client != nil && m.rpc.client.IsConnected()
}

func (m *rootModel) registerNotificationHandlers() {
	c := m.rpc.client

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

	c.OnResponse(gateway.RespToolExecStart, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		data, ok := env.Data.(map[string]any)
		if !ok {
			return
		}
		toolName, _ := data["tool_name"].(string)
		params, _ := data["params"].(map[string]any)
		predicted, _ := data["predicted_tokens"].(float64)
		if m.fileTracker != nil {
			m.fileTracker.ToolExecStart(params)
		}
		m.program.Send(clientmsg.ToolExecStartMsg{
			SessionID:    env.SessionID,
			ToolName:     toolName,
			Params:       params,
			EstimatedTok: int(predicted),
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
			SessionID:  env.SessionID,
			ToolName:   toolName,
			ToolCallID: toolCallID,
			Success:    success,
			Result:     result,
			Error:      errStr,
			DiffText:   diffText,
			DiffAdds:   diffAdds,
			DiffDels:   diffDels,
			DiffFile:   diffFile,
		})
	})

	// RespFileModified is sent by the daemon after each Write/FileEdit tool
	// execution to broadcast the session's current modified files list.
	// Merge with fileTracker's diff data for accurate sidebar display.
	c.OnResponse(gateway.RespFileModified, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		evData, ok := env.Data.(map[string]any)
		if !ok {
			return
		}

		filesRaw, _ := evData["files"].([]any)
		if len(filesRaw) == 0 {
			return
		}

		// Build a set of file paths from the daemon event.
		daemonFiles := make(map[string]bool)
		for _, f := range filesRaw {
			if s, ok := f.(string); ok && s != "" {
				daemonFiles[s] = true
			}
		}

		// Merge with fileTracker's existing changes (which carry diff data).
		var merged []data.FileChange
		if m.fileTracker != nil {
			for _, c := range m.fileTracker.Snapshot() {
				if daemonFiles[c.File] {
					merged = append(merged, c)
					delete(daemonFiles, c.File)
				}
			}
		}
		// Add any new files from daemon that fileTracker doesn't know about.
		for f := range daemonFiles {
			merged = append(merged, data.FileChange{File: f})
		}

		if len(merged) > 0 {
			m.sidebar.SetFileChanges(merged)
		}
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
		m.program.Send(msg)
	})

	c.OnResponse(gateway.RespFinalAnswer, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		content, _ := env.Data.(string)
		m.program.Send(clientmsg.FinalAnswerMsg{SessionID: env.SessionID, Content: content})
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: env.SessionID})
	})

	c.OnResponse(gateway.RespCycleEnd, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		iter := 1
		if data, ok := env.Data.(map[string]any); ok {
			if v, ok := data["iteration"].(float64); ok {
				iter = int(v)
			}
		}
		m.program.Send(clientmsg.IterationMsg{SessionID: env.SessionID, Iteration: iter})
	})

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
		m.program.Send(clientmsg.PermissionRequestMsg{
			ToolName:      toolName,
			Reason:        reason,
			SecurityLevel: int(secLevel),
		})
	})

	c.OnResponse(gateway.RespError, func(env *gateway.ResponseEnvelope, _ *gateway.Message) {
		errMsg := toString(env.Data)
		m.program.Send(clientmsg.AgentErrorMsg{
			SessionID: env.SessionID,
			Error:     fmt.Errorf("%s", errMsg),
		})
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: env.SessionID})
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
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(v)
	}
}

func (m *rootModel) rpcSendMessage(text string) {
	if !m.rpcIsConnected() {
		m.notifBar.Add(data.Notification{
			Message: i18n.T("client.notify.rpc.disconnected"),
			Level:   data.NotifError,
		})
		return
	}

	payload := map[string]string{"text": text}
	if m.currentSessionID != "" {
		payload["session_id"] = m.currentSessionID
	}
	if err := m.rpc.client.Notify("user.message", payload); err != nil {
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

// rpcReplyAskUser is removed — the non-blocking AskUser flow sends answers
// as user messages via rpcSendMessage instead of ask_user.reply RPC.

// rpcReplyPermission is removed — the non-blocking permission flow stores
// grants via execution.resume RPC instead of permission.reply.

func (m *rootModel) rpcCancelExecution() {
	if !m.rpcIsConnected() {
		return
	}
	go func() {
		_, _ = m.rpc.client.Call(context.Background(), "message.cancel", map[string]any{})
	}()
}

func toBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	default:
		return false
	}
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
