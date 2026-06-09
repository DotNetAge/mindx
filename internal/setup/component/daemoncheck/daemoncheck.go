package daemoncheck

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/i18n"
	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	choice    bool
	installed bool
	width     int
	height    int
	renderer  *glamour.TermRenderer
}

func New(installed bool) *Model {
	return &Model{
		choice:    installed,
		installed: installed,
		width:     80,
		height:    24,
		renderer:  initGlamour(minContentWidth),
	}
}

func (m *Model) Choice() bool { return m.choice }

func initGlamour(width int) *glamour.TermRenderer {
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return r
}

func renderMarkdown(r *glamour.TermRenderer, src string) string {
	if r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

func yesNoIndicator(yes bool) string {
	if yes {
		return "**> Yes**  \n  No"
	}
	return "  Yes  \n**> No**"
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderer = initGlamour(contentWidth(m.width))

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
		case "left", "right":
			m.choice = !m.choice
		case "enter":
			return m, func() tea.Msg {
				return setupmsg.DaemonDecisionMsg{Install: m.choice}
			}
		case "s", "S":
			if m.installed {
				return m, func() tea.Msg {
					return setupmsg.DaemonDecisionMsg{Install: true}
				}
			}
		}
	}
	return m, nil
}

func contentWidth(w int) int {
	if w > minContentWidth {
		cw := w - 4
		return cw
	}
	return minContentWidth
}

func (m *Model) View() string {
	var b strings.Builder
	if m.installed {
		b.WriteString(renderMarkdown(m.renderer,
			i18n.T("setup.daemon.check.title")+"\n\n"+
				i18n.T("setup.daemon.check.installed")+"\n\n"+
				i18n.T("setup.daemon.check.installed.desc")+"\n\n"+
				"**Enter** "+i18n.T("setup.daemon.check.continue")+"  **S** "+i18n.T("setup.daemon.check.skip"),
		))
	} else {
		md := i18n.T("setup.daemon.check.title") + `

` + i18n.T("setup.daemon.check.not_installed") + `

` + i18n.T("setup.daemon.check.not_installed.desc") + `

` + i18n.T("setup.daemon.check.not_installed.warning") + `:
  - ` + i18n.T("setup.daemon.check.feature.scheduled") + `
  - ` + i18n.T("setup.daemon.check.feature.websocket") + `
  - ` + i18n.T("setup.daemon.check.feature.tray") + `

` + i18n.T("setup.daemon.check.not_installed.question") + `?

` + yesNoIndicator(m.choice) + `

← → ` + i18n.T("setup.daemon.check.toggle") + `  **Enter** ` + i18n.T("setup.daemon.check.confirm") + `  **Esc** ` + i18n.T("setup.daemon.check.quit")
		b.WriteString(renderMarkdown(m.renderer, md))
	}
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle(""),
		"",
		content,
	) + "\n"
}
