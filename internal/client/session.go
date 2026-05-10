package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/gort/pkg/gateway"
)

// currentRenderWidth 当前可用于渲染的宽度，由 Root 在 WindowSizeMsg 时更新。
var currentRenderWidth int = 0

// sessionRegistry 管理活跃会话的答案引用。
// 极简实现：无需 goroutine、无需 channel、无需 mutex。
type sessionRegistry struct {
	answers map[string]*AgentAnswer
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		answers: make(map[string]*AgentAnswer),
	}
}

func (r *sessionRegistry) add(sessionID string, answer *AgentAnswer) {
	r.answers[sessionID] = answer
}

func (r *sessionRegistry) remove(sessionID string) {
	delete(r.answers, sessionID)
}

func (r *sessionRegistry) get(sessionID string) *AgentAnswer {
	return r.answers[sessionID]
}

func (r *sessionRegistry) count() int {
	return len(r.answers)
}

func (r *sessionRegistry) clear() {
	r.answers = make(map[string]*AgentAnswer)
}


// RegisterHandlers 注册所有事件处理器。
// 所有事件统一通过 outputCh 进入 Bubble Tea 事件循环，由 Root.Update 集中分发。
// 不在 WebSocket 回调中直接修改 AgentAnswer，避免数据竞争和 UI 刷新遗漏。
func RegisterHandlers(client *gateway.Client, reg *sessionRegistry, outputCh chan<- tea.Msg) {
	// 未分类的原始消息 → rawEvent fallback
	client.OnReceived(func(message string) {
		sessionID := extractSession(message)
		trySend(outputCh, rawEvent{sessionID: sessionID, contentType: "plain", content: message})
	})

	registerTypedHandler(client, outputCh, gateway.RespTable, "table", renderTableEnvelope)
	registerTypedHandler(client, outputCh, gateway.RespTodo, "todo", renderTodoEnvelope)
	registerTypedHandler(client, outputCh, gateway.RespOptions, "options", renderOptionsEnvelope)

	registerEventUpdate(client, outputCh, string(gateway.RespThinkingDelta), "thinking")
	registerEventUpdate(client, outputCh, string(gateway.RespThinkingDone), "thinking_done")
	registerTypedHandler(client, outputCh, gateway.RespActionStart, "action_start", renderActionStart)
	registerEventUpdate(client, outputCh, string(gateway.RespActionProgress), "action_progress")
	registerTypedHandler(client, outputCh, gateway.RespActionResult, "action_result", renderActionResult)
	registerEventUpdate(client, outputCh, string(gateway.RespFinalAnswer), "result")
	registerEventUpdate(client, outputCh, string(gateway.RespClarifyNeeded), "thinking")
	registerEventUpdate(client, outputCh, string(gateway.RespPermissionRequest), "thinking")
	registerEventUpdate(client, outputCh, string(gateway.RespError), "error")

	// 会话完成事件 → agentAnswerDoneMsg
	registerEventDone(client, outputCh, string(gateway.RespExecutionSummary))
}

func registerTypedHandler(client *gateway.Client, outputCh chan<- tea.Msg, eventType gateway.ResponseType, contentType string, renderer func(*gateway.ResponseEnvelope, int) string) {
	client.OnResponse(eventType, func(env *gateway.ResponseEnvelope, orig *gateway.Message) {
		content := renderer(env, currentRenderWidth)
		trySend(outputCh, agentAnswerUpdateMsg{
			sessionID:   env.SessionID,
			contentType: contentType,
			content:     content,
		})
	})
}

func registerEventUpdate(client *gateway.Client, outputCh chan<- tea.Msg, eventType, contentType string) {
	client.On(eventType, func(ctx context.Context, params json.RawMessage) {
		env := parseEventParams(params)
		if env == nil {
			return
		}
		trySend(outputCh, agentAnswerUpdateMsg{
			sessionID:   env.SessionID,
			contentType: contentType,
			content:     env.Data,
		})
	})
}

func registerEventDone(client *gateway.Client, outputCh chan<- tea.Msg, eventType string) {
	client.On(eventType, func(ctx context.Context, params json.RawMessage) {
		env := parseEventParams(params)
		if env == nil {
			return
		}
		trySend(outputCh, agentAnswerDoneMsg{
			sessionID: env.SessionID,
		})
	})
}

type eventParams struct {
	SessionID string
	Data      string
}

func parseEventParams(params json.RawMessage) *eventParams {
	var env struct {
		SessionID string          `json:"session_id"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(params, &env); err != nil {
		return nil
	}

	result := &eventParams{SessionID: env.SessionID}
	if len(env.Data) > 0 {
		// Data 可能是 JSON 字符串（大多数事件）或 JSON 对象（结构化事件如 ActionStart）
		// 优先作为 JSON 字符串解码以正确处理 \n 等转义字符
		var strVal string
		if err := json.Unmarshal(env.Data, &strVal); err == nil {
			result.Data = strVal
		} else {
			result.Data = string(env.Data)
		}
	}
	return result
}

func trySend(ch chan<- tea.Msg, msg tea.Msg) bool {
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}

// waitEvent 返回一个读取 outputCh 的 tea.Cmd，每次收到事件后需重新排队。
func waitEvent(outputCh <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-outputCh
	}
}

// extractSession 从原始消息中尝试提取 sessionID。
func extractSession(msg string) string {
	var env struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(msg), &env); err == nil && env.SessionID != "" {
		return env.SessionID
	}
	return ""
}

// extractSessionFromEnv 从 ResponseEnvelope 中提取 sessionID。
func extractSessionFromEnv(env *gateway.ResponseEnvelope) string {
	return env.SessionID
}

// connectWithRetry 建立连接，使用指数退避重试。单次连接有 10 秒超时。
func connectWithRetry(client *gateway.Client, attempt connectAttempt) tea.Cmd {
	return func() tea.Msg {
		if attempt.count >= maxConnectRetries-1 {
			return errMsg(fmt.Errorf("连接失败（已重试 %d 次）", attempt.count+1))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			delay := time.Duration(1<<uint(attempt.count)) * 500 * time.Millisecond
			if delay > 8*time.Second {
				delay = 8*time.Second
			}
			return tea.Tick(delay, func(time.Time) tea.Msg {
				return connectWithRetry(client, connectAttempt{count: attempt.count + 1})
			})
		}
		return connectedMsg{addr: fmt.Sprintf("%d", client.Port())}
	}
}

// sendToServer 发送用户消息到服务器。
func sendToServer(client *gateway.Client, text string) tea.Cmd {
	return func() tea.Msg {
		if err := client.Notify("user.message", map[string]string{"text": text}); err != nil {
			return errMsg(fmt.Errorf("发送失败: %w", err))
		}
		return nil
	}
}

// sendToServerWithSession 发送用户消息到服务器，并携带 sessionID 以确保会话一致。
func sendToServerWithSession(client *gateway.Client, text, sessionID string) tea.Cmd {
	return func() tea.Msg {
		payload := map[string]string{
			"text":       text,
			"session_id": sessionID,
		}
		if err := client.Notify("user.message", payload); err != nil {
			return errMsg(fmt.Errorf("发送失败: %w", err))
		}
		return nil
	}
}
