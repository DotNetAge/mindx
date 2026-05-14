package client

import "charm.land/lipgloss/v2"

const (
	MindxVersion = "2.0.0"
)

// MindxLogo 返回 MindX² ASCII 艺术字 logo
func MindxLogo() string {
	return `  ███╗   ███╗██╗███╗   ██╗██████╗ ██╗  ██╗
  ████╗ ████║██║████╗  ██║██╔══██╗╚██╗██╔╝
  ██╔████╔██║██║██╔██╗ ██║██║  ██║ ╚███╔╝
  ██║╚██╔╝██║██║██║╚██╗██║██║  ██║ ██╔██╗
  ██║ ╚═╝ ██║██║██║ ╚████║██████╔╝██╔╝ ██╗
  ╚═╝     ╚═╝╚═╝╚═╝  ╚═══╝╚═════╝ ╚═╝  ╚═╝`
}

// ── Brand Colors ──
var (
	mindxPrimary      = lipgloss.Color("#9C27B0") // 紫色 MindX 主色
	mindxPrimaryDark  = lipgloss.Color("#7B1FA2") // 深紫色
	mindxAccent       = lipgloss.Color("#BB86FC") // 浅紫色强调色
	mindxSurface      = lipgloss.Color("#1E1E2E") // 深色背景色
	mindxTextPrimary  = lipgloss.Color("#E0E0E0") // 主要文字色
	mindxTextSecondary = lipgloss.Color("#A0A0A0") // 次要文字色
)

// ── Functional Colors (existing Material palette, preserved) ──
var (
	colorUserQuestion = lipgloss.Color("#4FC3F7") // 青色
	colorThinking     = lipgloss.Color("#888888") // 灰色
	colorError        = lipgloss.Color("#CF6679") // 红色
	colorConnected    = lipgloss.Color("#4CAF50") // 绿色
	colorDisconnected = lipgloss.Color("#CF6679") // 红色
	colorActionDoing  = lipgloss.Color("#FFD54F") // 黄色
	colorActionDone   = lipgloss.Color("#4CAF50") // 绿色
	colorActionFailed = lipgloss.Color("#CF6679") // 红色
	colorToolName     = lipgloss.Color("#81D4FA") // 浅蓝
	colorProgress     = lipgloss.Color("#888888") // 灰色
	colorActionResult = lipgloss.Color("#888888") // 灰色
)

// ── Message / Input Styles (backward-compatible aliases for types.go) ──
var (
	UserQuestionStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorUserQuestion)
	ThinkingStyle       = lipgloss.NewStyle().Foreground(colorThinking).Italic(true)
	AgentStyle          = lipgloss.NewStyle().Bold(true).Foreground(mindxPrimary)
	ErrorStyle          = lipgloss.NewStyle().Foreground(colorError)
	ConnectedDot        = lipgloss.NewStyle().Foreground(colorConnected).SetString("●")
	DisconnectedDot     = lipgloss.NewStyle().Foreground(colorDisconnected).SetString("●")
	ActionSpinnerStyle  = lipgloss.NewStyle().Foreground(colorActionDoing)
	ActionDoneStyle     = lipgloss.NewStyle().Foreground(colorActionDone)
	ActionFailedStyle   = lipgloss.NewStyle().Foreground(colorActionFailed)
	ActionToolStyle     = lipgloss.NewStyle().Foreground(colorToolName)
	ActionProgressStyle = lipgloss.NewStyle().Foreground(colorProgress).Italic(true)
	ActionResultStyle   = lipgloss.NewStyle().Foreground(colorActionResult)
)

// ── Header Styles ──
var (
	HeaderLogoStyle   = lipgloss.NewStyle().Bold(true).Foreground(mindxAccent)
	HeaderStatusStyle = lipgloss.NewStyle().Foreground(mindxTextSecondary)
)

// ── StatusBar Styles ──
var (
	StatusBarStyle    = lipgloss.NewStyle().Foreground(mindxTextSecondary)
	ShortcutHintStyle = lipgloss.NewStyle().Foreground(mindxTextSecondary).Italic(true)
)

// ── Search Styles ──
var (
	SearchInputStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(mindxAccent)
	SearchMatchStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#FFD54F")).Foreground(lipgloss.Color("#000000"))
)

// ── Notification Styles ──
var (
	NotificationInfoStyle    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4FC3F7"))
	NotificationSuccessStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorConnected)
	NotificationErrorStyle   = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorError)
	NotificationWarningStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorActionDoing)
)

// ── Timeline / Misc Styles ──
var (
	TimestampStyle = lipgloss.NewStyle().Foreground(mindxTextSecondary).Italic(true)
	DividerStyle   = lipgloss.NewStyle().Foreground(mindxTextSecondary)
)

// ThemeTitleStyle 渲染表格等结构化内容的标题（迁移自 render.go 中的 styleTableTitle）
var ThemeTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(mindxAccent)
