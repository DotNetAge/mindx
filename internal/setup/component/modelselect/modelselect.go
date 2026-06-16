package modelselect

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/i18n"
	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	list     list.Model
	items    []setupdata.ModelItem
	width    int
	height   int
	skipMode bool
	renderer *glamour.TermRenderer
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(setupdata.ModelItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	nameStyle := lipgloss.NewStyle().Bold(true)
	descStyle := lipgloss.NewStyle()

	var line string
	if item.Desc != "" {
		line = fmt.Sprintf(" %s — %s", nameStyle.Render(item.Name), descStyle.Render(item.Desc))
	} else {
		line = fmt.Sprintf(" %s", nameStyle.Render(item.Name))
	}

	if isSelected {
		_, _ = fmt.Fprint(w, lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Render("❯ "+line))
	} else {
		_, _ = fmt.Fprint(w, lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("  "+line))
	}
}

func New(items []setupdata.ModelItem, skipMode bool) *Model {
	d := &itemDelegate{}

	var listItems []list.Item
	for _, item := range items {
		listItems = append(listItems, item)
	}

	l := list.New(listItems, d, minContentWidth-4, 10)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)

	return &Model{
		list:     l,
		items:    items,
		width:    80,
		height:   24,
		skipMode: skipMode,
		renderer: initGlamour(minContentWidth),
	}
}

func (m *Model) SelectedItem() *setupdata.ModelItem {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	if mi, ok := item.(setupdata.ModelItem); ok {
		return &mi
	}
	return nil
}

func (m *Model) SelectByName(name string) {
	for i, item := range m.items {
		if item.Name == name {
			m.list.Select(i)
			break
		}
	}
}

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

func contentWidth(w int) int {
	if w > minContentWidth {
		cw := w - 4
		return cw
	}
	return minContentWidth
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := contentWidth(m.width)
		m.list.SetWidth(cw - 4)
		m.renderer = initGlamour(cw)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
		case "enter":
			if sel := m.SelectedItem(); sel != nil {
				return m, func() tea.Msg {
					return setupmsg.ModelSelectedMsg{
						Name:    sel.Name,
						BaseURL: sel.BaseURL,
						CredRef: sel.CredRef,
						Desc:    sel.Desc,
					}
				}
			}
		case "s", "S":
			if m.skipMode {
				if sel := m.SelectedItem(); sel != nil {
					return m, func() tea.Msg {
						return setupmsg.ModelSelectedMsg{
							Name:    sel.Name,
							BaseURL: sel.BaseURL,
							CredRef: sel.CredRef,
							Desc:    sel.Desc,
						}
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	var b strings.Builder
	md := i18n.T("setup.model.select.title") + "\n\n"
	md += i18n.T("setup.model.select.desc") + "\n\n"
	help := i18n.T("setup.model.select.help")
	if m.skipMode {
		help = i18n.T("setup.model.select.help.skip")
	}
	b.WriteString(renderMarkdown(m.renderer, md))
	b.WriteString(m.list.View())
	b.WriteString("\n")
	b.WriteString(renderMarkdown(m.renderer, help))
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle(""),
		"",
		content,
	) + "\n"
}
