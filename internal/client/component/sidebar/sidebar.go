package sidebar

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

var (
	borderStyle = lipgloss.NewStyle().
			Foreground(style.ThemeDim).
			Inline(true)

	sideBarStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Left: "│"}).
			BorderForeground(style.ThemeDim).
			PaddingLeft(1)
)

type Sidebar struct {
	width   int
	height  int
	vp      viewport.Model
	welcome *welcome.WelcomePanel
}

func New() *Sidebar {
	return &Sidebar{
		welcome: welcome.New(),
		vp:     viewport.New(),
	}
}

func (s *Sidebar) Update(msg any) (*Sidebar, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		s.width = m.Width
		s.height = m.Height
		s.welcome.Update(m)
		s.vp.SetWidth(s.width)
		s.vp.SetHeight(s.height)
		s.vp.SetContent(s.buildContent())
		return s, nil
	}

	newVp, cmd := s.vp.Update(msg)
	s.vp = newVp
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
	return s, cmd
}

func (s *Sidebar) View() string {
	content := s.vp.View()
	return sideBarStyle.Render(content)
}

func (s *Sidebar) SetWelcomeData(d data.WelcomeData) {
	s.welcome.Data = d
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
}

func (s *Sidebar) buildContent() string {
	view := s.welcome.View()
	var parts []string

	if view != "" {
		parts = append(parts, view)
	} else {
		parts = append(parts, style.DimStyle.Render("  Welcome Panel"))
	}

	sep := borderStyle.Render(strings.Repeat("─", max(s.width-4, 4)))
	parts = append(parts, sep)

	return strings.Join(parts, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
