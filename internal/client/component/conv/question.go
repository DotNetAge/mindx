package conv

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type Question struct {
	Text string
}

func ViewQuestion(m Question, width int) string {
	if m.Text == "" {
		return ""
	}

	var b strings.Builder
	questionStyle := lipgloss.NewStyle().Foreground(style.ThemePurple).Bold(true)
	b.WriteString(questionStyle.Render("● "))
	b.WriteString(questionStyle.Render(m.Text))

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemePurple).
		Padding(0, 1).
		Width(width - 4)

	return border.Render(b.String())
}
