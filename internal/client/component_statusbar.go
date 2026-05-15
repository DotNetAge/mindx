package client

import (
	"fmt"
	"strings"
)

// shortcut 描述一个键盘快捷键提示。
type shortcut struct {
	key  string
	desc string
}

// StatusBar 状态栏组件，支持两行显示：
// 行1: 连接状态 + token用量 + 费用 + agent/model
// 行2: 键盘快捷键提示（可选）
type StatusBar struct {
	width        int
	connected    bool
	tokensTotal  int
	tokensIn     int
	tokensOut    int
	currentAgent string
	currentModel string

	// 新增字段
	sessionCost string // 当前会话费用 "$0.42"
	mcpCount    int    // MCP 连接数
	fastMode    bool   // 快速模式标记
	modeLabel   string // 模式标签 "Prompt" / "Transcript"
	showHints   bool   // 是否显示第二行的快捷键提示
	shortcuts   []shortcut
	loader      *Loader
}

func NewStatusBar() StatusBar {
	return StatusBar{
		showHints: true,
		shortcuts: defaultShortcuts(),
		loader:    NewLoader("Loading..."),
	}
}

// defaultShortcuts 返回默认的键盘快捷键列表。
func defaultShortcuts() []shortcut {
	return []shortcut{
		{key: "Ctrl+O", desc: "Transcript"},
		{key: "Esc", desc: "Cancel"},
		{key: "Ctrl+C", desc: "Exit"},
		{key: "PgUp/Dn", desc: "Scroll"},
		{key: "@", desc: "Agent"},
		{key: "/", desc: "Cmd"},
	}
}

// ── Setter 方法 ──

func (s *StatusBar) SetConnected(v bool)   { s.connected = v }
func (s *StatusBar) SetTokens(in, out int) { s.tokensIn = in; s.tokensOut = out }
func (s *StatusBar) AddTokens(in, out int) {
	s.tokensIn += in
	s.tokensOut += out
	s.tokensTotal += in + out
}
func (s *StatusBar) SetAgent(name, model string) { s.currentAgent = name; s.currentModel = model }
func (s *StatusBar) SetWidth(w int)              { s.width = w }

// SetSessionCost 设置当前会话费用。
func (s *StatusBar) SetSessionCost(cost string) { s.sessionCost = cost }

// SetMCPCount 设置 MCP 连接数。
func (s *StatusBar) SetMCPCount(count int) { s.mcpCount = count }

// SetFastMode 设置快速模式标记。
func (s *StatusBar) SetFastMode(v bool) { s.fastMode = v }

// SetModeLabel 设置当前模式标签。
func (s *StatusBar) SetModeLabel(label string) { s.modeLabel = label }

// ShowShortcutHints 控制是否显示快捷键提示行。
func (s *StatusBar) ShowShortcutHints(v bool) { s.showHints = v }

// SetShortcuts 自定义快捷键列表。
func (s *StatusBar) SetShortcuts(shortcuts []shortcut) { s.shortcuts = shortcuts }

// Height 返回状态栏占用的行数。
func (s *StatusBar) Height() int {
	if s.showHints && len(s.shortcuts) > 0 && s.width > 40 {
		return 2
	}
	return 1
}

// View 渲染状态栏。
func (s *StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	var b strings.Builder

	// ── 行1: 状态信息 ──
	line1 := s.renderLine1()
	b.WriteString(StatusBarStyle.Render(line1))
	b.WriteString("\n")

	// ── 行2: 快捷键提示 ──
	if s.showHints && s.width > 60 {
		line2 := s.renderLine2()
		if line2 != "" {
			b.WriteString(ShortcutHintStyle.Render(line2))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (s *StatusBar) renderLine1() string {
	// Left: 连接状态 + token + 费用
	var leftParts []string
	if s.connected {
		leftParts = append(leftParts, ConnectedDot.Render()+" Connected")
	} else {
		leftParts = append(leftParts, DisconnectedDot.Render()+" Connecting...")
	}
	if s.tokensTotal > 0 {
		leftParts = append(leftParts, formatTokens(s.tokensTotal)+" tok")
	}
	if s.sessionCost != "" {
		leftParts = append(leftParts, s.sessionCost)
	}
	left := strings.Join(leftParts, "  ")

	// Right: mode | agent | model | tokens in/out
	var rightParts []string
	if s.modeLabel != "" {
		rightParts = append(rightParts, s.modeLabel)
	}
	if s.currentAgent != "" {
		rightParts = append(rightParts, s.currentAgent)
	}
	if s.currentModel != "" {
		rightParts = append(rightParts, s.currentModel)
	}
	if s.tokensIn > 0 || s.tokensOut > 0 {
		rightParts = append(rightParts, fmt.Sprintf("↓%s ↑%s", formatTokens(s.tokensIn), formatTokens(s.tokensOut)))
	}
	right := strings.Join(rightParts, " | ")

	// Padding
	leftW := approximateWidth(left)
	rightW := approximateWidth(right)
	pad := s.width - leftW - rightW
	if pad < 4 {
		pad = 4
	}
	return left + strings.Repeat(" ", pad) + right
}

func (s *StatusBar) renderLine2() string {
	// Build shortcut hints line
	var parts []string
	for _, sc := range s.shortcuts {
		parts = append(parts, fmt.Sprintf("[%s] %s", sc.key, sc.desc))
	}
	hint := strings.Join(parts, "  ")

	// Truncate if too long
	if approximateWidth(hint) > s.width {
		// Try to fit by removing items from the end
		for len(parts) > 2 {
			parts = parts[:len(parts)-1]
			hint = strings.Join(parts, "  ")
			if approximateWidth(hint) <= s.width {
				break
			}
		}
		// Final truncate if still too long
		if approximateWidth(hint) > s.width {
			hint = truncateLine(hint, s.width-3) + "..."
		}
	}

	return hint
}
