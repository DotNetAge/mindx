package choices

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type choiceItem string

func (i choiceItem) FilterValue() string { return string(i) }

type choiceDelegate struct {
	styles *delegateStyles
}

type delegateStyles struct {
	normal   lipgloss.Style
	selected lipgloss.Style
}

func defaultDelegateStyles() *delegateStyles {
	return &delegateStyles{
		normal:   lipgloss.NewStyle().PaddingLeft(2).Foreground(style.ThemeDim),
		selected: lipgloss.NewStyle().PaddingLeft(2).Foreground(style.ThemeCyan),
	}
}

func (d choiceDelegate) Height() int                             { return 1 }
func (d choiceDelegate) Spacing() int                            { return 0 }
func (d choiceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d choiceDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(choiceItem)
	if !ok {
		return
	}
	if index == m.Index() {
		fmt.Fprint(w, d.styles.selected.Render("> "+string(item)))
	} else {
		fmt.Fprint(w, d.styles.normal.Render(string(item)))
	}
}

type ChoicesPanel struct {
	Visible bool
	Items   []string
	Prompt  string
	Cursor  int
	list    list.Model
	width   int
}

func New() *ChoicesPanel {
	return &ChoicesPanel{}
}

func (p *ChoicesPanel) Update(msg any) (*ChoicesPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case clientmsg.ShowChoicesMsg:
		p.Visible = true
		p.Items = msg.Options
		p.Prompt = msg.Prompt
		p.Cursor = 0

		items := make([]list.Item, len(msg.Options))
		for i, opt := range msg.Options {
			items[i] = choiceItem(opt)
		}

		w := p.width
		if w == 0 {
			w = 80
		}
		height := len(msg.Options) + 2
		if height < 5 {
			height = 5
		}
		if height > 16 {
			height = 16
		}

		d := choiceDelegate{styles: defaultDelegateStyles()}
		l := list.New(items, d, w, height)
		l.Title = msg.Prompt
		l.SetShowStatusBar(false)
		l.SetShowPagination(false)
		l.SetShowTitle(false)
		l.SetShowHelp(false)
		l.SetFilteringEnabled(false)

		p.list = l
		return p, nil

	case tea.WindowSizeMsg:
		p.width = msg.Width
		if p.Visible {
			p.list.SetWidth(msg.Width)
		}
		return p, nil

	case tea.KeyPressMsg:
		if !p.Visible {
			return p, nil
		}

		switch msg.String() {
		case "enter":
			p.Cursor = p.list.Index()
			p.Visible = false
			return p, func() tea.Msg {
				return clientmsg.ChoiceSelectedMsg{Index: p.Cursor}
			}
		case "esc":
			p.Visible = false
			return p, nil
		default:
			var cmd tea.Cmd
			p.list, cmd = p.list.Update(msg)
			p.Cursor = p.list.Index()
			return p, cmd
		}
	}
	return p, nil
}

func (p *ChoicesPanel) View() string {
	if !p.Visible || len(p.Items) == 0 {
		return ""
	}
	return p.list.View()
}
