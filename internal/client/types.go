package client

import (
	"charm.land/lipgloss/v2"
	"github.com/DotNetAge/gort/pkg/gateway"
)

const maxConnectRetries = 10

// ---- 网络/UI 消息（通过 Bubble Tea 事件循环传递） ----

type connectedMsg struct{ addr string }
type connectAttempt struct{ count int }
type errMsg error

type agentsFetchedMsg struct {
	agents     []agentInfo
	masterName string
}
type commandsFetchedMsg struct {
	commands []gateway.CommandMeta
}

type agentInfo struct {
	name        string
	role        string
	description string
	model       string
	master      bool
}

// rawEvent 是 client handler 统一推送的结构化事件，通过 eventCh 进入 Root.Update。
// sessionID 为空时路由到当前最新会话。
type rawEvent struct {
	sessionID   string
	contentType string // "thinking" | "result" | "action_start" | "action_progress" | "observation" | "error" | "complete" | "table" | "todo" | "options" | "plain"
	content     string
}

// agentAnswerUpdateMsg 由 sessionLoop goroutine 发出，携带 sessionID 和具体更新。
type agentAnswerUpdateMsg struct {
	sessionID   string
	contentType string
	content     string
}

// agentAnswerDoneMsg 由 sessionLoop goroutine 发出，标记一个会话完成。
type agentAnswerDoneMsg struct {
	sessionID string
}

// agentEventMsg 用于 session 内部通道传输的 Agent 事件，不作为 tea.Msg 流转。
type agentEventMsg struct {
	eventType string
	content   string
}

// sendMsg 由 InputBox 在用户按 Enter 时返回。
type sendMsg struct {
	text string
}

// agentSwitchMsg 由 InputBox 在 @agent 补全时返回。
type agentSwitchMsg struct {
	agentName string
}

// suggestionCompleteMsg 由 Suggestion 列表在选中项时返回，用于补全输入框。
type suggestionCompleteMsg struct {
	text string
}

// clearScreenMsg 由 InputBox 在 Ctrl+L 时返回。
type clearScreenMsg struct{}

// localDisplayMsg 由本地命令（如 /models /skills /agents）返回，在聊天区展示 Markdown 内容，不发送给 LLM。
type localDisplayMsg struct {
	markdown string
}

// sessionLoadedMsg 表示从 chat.json 成功加载了会话。
type sessionLoadedMsg struct {
	agentName string
	sessionID string
}

// sessionInitRequiredMsg 表示需要从服务器初始化新会话。
type sessionInitRequiredMsg struct{}

// exitMsg 表示用户请求退出程序。
type exitMsg struct{}

// ---- Lip Gloss 样式 ----

var (
	thinkingStyle       = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Background(lipgloss.Color("#2d2d2d")).
				Italic(true)
	agentStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9C27B0"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#CF6679"))
	connectedDot        = lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).SetString("●")
	disconnectedDot     = lipgloss.NewStyle().Foreground(lipgloss.Color("#CF6679")).SetString("●")
	actionSpinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD54F"))
	actionDoneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50"))
	actionFailedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#CF6679"))
	actionToolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#81D4FA"))
	actionProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
	actionResultStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)
