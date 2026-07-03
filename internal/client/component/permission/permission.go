package permission

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

type PermissionBar struct {
	ToolName      string
	Reason        string
	SecurityLevel int
	SelectedIndex int
	Visible       bool
	Remember      bool
}

func NewPermissionBar(toolName, reason string, securityLevel int) PermissionBar {
	return PermissionBar{
		ToolName:      toolName,
		Reason:        reason,
		SecurityLevel: securityLevel,
		SelectedIndex: 0,
		Visible:       true,
		Remember:      false,
	}
}

const (
	PermissionAllow        = 0
	PermissionDeny         = 1
	PermissionAllowSession = 2
)

func permissionLabel(idx int) string {
	if idx == 0 {
		return " " + i18n.T("client.ui.permission.allow") + " "
	}
	return " " + i18n.T("client.ui.permission.deny") + " "
}

func UpdatePermissionBar(m PermissionBar, e tea.Msg) (PermissionBar, tea.Cmd) {
	switch e := e.(type) {
	case tea.KeyPressMsg:
		if !m.Visible {
			return m, nil
		}
		key := tea.Key(e)
		switch key.Code {
		case tea.KeyLeft:
			m.SelectedIndex = (m.SelectedIndex + 1) % 2
			return m, nil
		case tea.KeyRight, tea.KeyTab:
			m.SelectedIndex = (m.SelectedIndex + 1) % 2
			return m, nil
		case tea.KeyEnter, ' ':
			m.Visible = false
			return m, func() tea.Msg {
				idx := m.SelectedIndex
				if idx == PermissionAllow && m.Remember {
					idx = PermissionAllowSession
				}
				return msg.ChoiceSelectedMsg{Index: idx}
			}
		case tea.KeyEsc:
			m.Visible = false
			return m, func() tea.Msg {
				return msg.ChoiceSelectedMsg{Index: -1}
			}
		default:
			// Toggle remember mode with 'r' key when Allow is focused.
			if key.String() == "r" {
				m.Remember = !m.Remember
				return m, nil
			}
		}
	case msg.PermissionRequestMsg:
		m = NewPermissionBar(e.ToolName, e.Reason, e.SecurityLevel)
		return m, nil
	}
	return m, nil
}

func ViewPermissionBar(m PermissionBar, width int) string {
	if !m.Visible || m.ToolName == "" {
		return ""
	}

	icon := style.YellowStyle.Render("🔒 ")
	promptLine := " " + icon + style.BoldWhite.Render(m.ToolName)
	if m.Reason != "" {
		promptLine += style.GrayStyle.Render(" — " + m.Reason)
	}

	allowBtn := renderButton(permissionLabel(0), m.SelectedIndex == PermissionAllow)
	denyBtn := renderButton(permissionLabel(1), m.SelectedIndex == PermissionDeny)

	// Remember indicator
	var rememberTag string
	if m.Remember {
		rememberTag = " " + style.GreenStyle.Render("📌 [会话白名单]")
	} else {
		rememberTag = " " + style.GrayStyle.Render("[r: 记住授权]")
	}

	gap := "  "
	buttonsRow := " " + allowBtn + gap + denyBtn + rememberTag

	div := style.Divider(strings.Repeat("─", maxI(width, 4)))

	var b strings.Builder
	b.WriteString(div)
	b.WriteByte('\n')
	b.WriteString(promptLine)
	b.WriteByte('\n')
	b.WriteString(buttonsRow)
	b.WriteByte('\n')
	b.WriteString(div)

	return b.String()
}

func renderButton(label string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().
			Foreground(style.ThemeBg).
			Background(style.ThemeGreen).
			Bold(true).
			Render(label)
	}
	return lipgloss.NewStyle().
		Foreground(style.ThemeWhite).
		Background(style.ThemeDarkGray).
		Render(label)
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
