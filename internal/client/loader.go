package client

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Loader struct {
	spinner spinner.Model
	loading bool
	message string
}

func NewLoader(message string) *Loader {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#6fb809ff"))

	return &Loader{
		spinner: s,
		loading: false,
		message: message,
	}
}

func (l *Loader) SetLoading(v bool) { l.loading = v }

func (l *Loader) Init() tea.Cmd {
	return l.spinner.Tick
}
func (l *Loader) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if l.loading {
		s, cmd := l.spinner.Update(msg)
		l.spinner = s
		return l, cmd
	}
	return l, nil
}
func (l *Loader) View() tea.View {
	str := l.message + l.spinner.View()
	return tea.NewView(str)
}
