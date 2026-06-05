package welcome

import (
	"strings"

	"image/color"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type WelcomePanel struct {
	Data  data.WelcomeData
	Width int
}

func New() *WelcomePanel {
	return &WelcomePanel{
		Width: 80,
	}
}

func (w *WelcomePanel) Update(msg any) (*WelcomePanel, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		w.Width = m.Width
	}
	return w, nil
}

func (w *WelcomePanel) View() string {
	var b strings.Builder

	b.WriteString(w.renderGradientTitle())
	b.WriteByte('\n')
	b.WriteByte('\n')

	if w.Data.Workspace != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Project: "))
		b.WriteString(style.WhiteStyle.Render(w.Data.Workspace))
		b.WriteByte('\n')
	}

	if w.Data.SessionID != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white")).Render("Session: "))
		b.WriteString(style.WhiteStyle.Render(w.Data.SessionID))
		b.WriteByte('\n')
	}

	return lipgloss.NewStyle().
			Padding(0, 1).
			Width(w.Width).
			Render(strings.TrimRight(b.String(), "\n"))
}

func (w *WelcomePanel) renderGradientTitle() string {
	titleText := w.Data.Version
	if titleText == "" {
		titleText = "MindX CLI"
	} else {
		titleText = "MindX CLI " + titleText
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
