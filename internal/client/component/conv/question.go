package conv

import (
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

	questionStyle := lipgloss.NewStyle().Foreground(style.ThemePurple).Bold(true)
	return questionStyle.Render("● ") + questionStyle.Render(m.Text)
}
