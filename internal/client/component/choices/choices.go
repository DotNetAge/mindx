package choices

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type ChoicesPanel struct {
	Visible bool
	Items   []string
	Prompt  string
	Cursor  int
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
		return p, nil
	case tea.KeyPressMsg:
		if !p.Visible {
			return p, nil
		}
		switch msg.String() {
		case "up":
			if p.Cursor > 0 {
				p.Cursor--
			}
		case "down":
			if p.Cursor < len(p.Items)-1 {
				p.Cursor++
			}
		case "enter":
			p.Visible = false
			return p, func() tea.Msg {
				return clientmsg.ChoiceSelectedMsg{Index: p.Cursor}
			}
		}
	}
	return p, nil
}

func (p *ChoicesPanel) View() string {
	if !p.Visible || len(p.Items) == 0 {
		return ""
	}

	maxItemLen := 0
	for _, item := range p.Items {
		if len(item) > maxItemLen {
			maxItemLen = len(item)
		}
	}

	boxWidth := max(5+maxItemLen, 4+len(p.Prompt))

	var b strings.Builder

	header := fmt.Sprintf("┌ %s ", p.Prompt)
	headerDashes := boxWidth - 4 - len(p.Prompt)
	b.WriteString(header)
	b.WriteString(strings.Repeat("─", headerDashes))
	b.WriteString("┐\n")

	for _, item := range p.Items {
		b.WriteString("│ ")
		itemPad := boxWidth - 5 - len(item)
		if itemPad < 0 {
			itemPad = 0
		}
		if item == p.Items[p.Cursor] {
			b.WriteString(style.CyanStyle.Render("> "))
			b.WriteString(item)
		} else {
			b.WriteString("  ")
			b.WriteString(style.DimStyle.Render(item))
		}
		b.WriteString(strings.Repeat(" ", itemPad))
		b.WriteString(" │\n")
	}

	bottomDashes := boxWidth - 2
	b.WriteString("└")
	b.WriteString(strings.Repeat("─", bottomDashes))
	b.WriteString("┘")

	return b.String()
}
