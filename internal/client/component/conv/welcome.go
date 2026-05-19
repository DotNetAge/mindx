package conv

import (
	"strings"

	"image/color"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

func (p *ConversationPanel) renderWelcome() string {
	var b strings.Builder

	b.WriteString(p.renderGradientTitle())
	b.WriteByte('\n')
	b.WriteByte('\n')

	if p.WelcomeData.Workspace != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Workspace: "))
		b.WriteString(style.WhiteStyle.Render(p.WelcomeData.Workspace))
		b.WriteByte('\n')
	}

	if p.WelcomeData.SessionID != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Session: "))
		b.WriteString(style.WhiteStyle.Render(p.WelcomeData.SessionID))
		b.WriteByte('\n')
	}

	if p.WelcomeData.AgentName != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Agent: "))
		b.WriteString(style.WhiteStyle.Render(p.WelcomeData.AgentName))
		b.WriteByte('\n')
	}

	if p.WelcomeData.ModelName != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Model: "))
		b.WriteString(style.WhiteStyle.Render(p.WelcomeData.ModelName))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(style.Divider(strings.Repeat("─", p.width)))
	b.WriteByte('\n')
	b.WriteString(style.GrayStyle.Render(" ℹ Type a message to start chatting"))
	b.WriteByte('\n')

	return b.String()
}

func (p *ConversationPanel) renderGradientTitle() string {
	titleText := p.WelcomeData.AppTitle
	if titleText == "" {
		titleText = "MindX CLI v2.0.0 Beta"
	}

	gradientColors := []color.Color{
		lipgloss.Color("#1799EA"),
		lipgloss.Color("#548BE1"),
		lipgloss.Color("#6985DC"),
		lipgloss.Color("#8D78CD"),
		lipgloss.Color("#9774C1"),
		lipgloss.Color("#A371B6"),
		lipgloss.Color("#AD6EAA"),
		lipgloss.Color("#B26CA4"),
		lipgloss.Color("#BC6899"),
		lipgloss.Color("#D0617F"),
	}

	blendedColors := lipgloss.Blend1D(len(titleText), gradientColors...)
	var b strings.Builder
	for i, ch := range titleText {
		if i < len(blendedColors) {
			b.WriteString(
				lipgloss.NewStyle().
					Foreground(blendedColors[i]).
					Bold(true).
					Render(string(ch)),
			)
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
