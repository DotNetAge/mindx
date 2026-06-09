package conv

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

type ErrorMsg struct {
	Error string
	Phase string
	Time  time.Time
}

func UpdateErrorMsg(m ErrorMsg, e tea.Msg) (ErrorMsg, tea.Cmd) {
	switch e := e.(type) {
	case msg.AgentErrorMsg:
		return ErrorMsg{
			Error: e.Error.Error(),
			Phase: extractPhase(e.Error.Error()),
			Time:  time.Now(),
		}, nil
	}
	return m, nil
}

func ViewErrorMsg(m ErrorMsg, width int) string {
	if m.Error == "" {
		return ""
	}

	red := lipgloss.NewStyle().Foreground(style.ThemeRed).Bold(true)
	timeStr := m.Time.Format("15:04:05")

	var b strings.Builder
	b.WriteString(red.Render("⏺"))
	if m.Phase != "" {
		b.WriteString(red.Render(" [" + m.Phase + "]"))
	}
	b.WriteString(red.Render(" " + m.Error))
	b.WriteString(lipgloss.NewStyle().Foreground(style.ThemeRed).Faint(true).Render(" (" + timeStr + ")"))

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemeRed).
		Padding(0, 1).
		Width(width - 4)

	return border.Render(b.String())
}

func extractPhase(errMsg string) string {
	if strings.HasPrefix(errMsg, "think error:") {
		return i18n.T("error.phase.thinking")
	}
	if strings.HasPrefix(errMsg, "act error:") {
		return i18n.T("error.phase.executing")
	}
	if strings.HasPrefix(errMsg, "observe error:") {
		return i18n.T("error.phase.reflecting")
	}
	return ""
}
