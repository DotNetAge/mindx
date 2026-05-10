package client

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ChoicesPanel 是可交互的选择器，用于服务器需要用户选择时。
type ChoicesPanel struct {
	visible  bool
	list     list.Model
	prompt   string
	onSubmit func(selected int) tea.Cmd
}

func NewChoicesPanel() ChoicesPanel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 10)
	return ChoicesPanel{list: l}
}

func (p *ChoicesPanel) Show(prompt string, options []string, onSubmit func(selected int) tea.Cmd) {
	var items []list.Item
	for i, opt := range options {
		items = append(items, choiceItem{idx: i, label: opt})
	}
	p.list.SetItems(items)
	p.prompt = prompt
	p.onSubmit = onSubmit
	p.visible = true
}

func (p *ChoicesPanel) Dismiss() {
	p.visible = false
	p.prompt = ""
	p.onSubmit = nil
}

func (p *ChoicesPanel) IsVisible() bool {
	return p.visible
}

// approxHeight 返回 ChoicesPanel 的近似占用行数。
func (p *ChoicesPanel) approxHeight() int {
	if !p.visible {
		return 0
	}
	return 2 + p.list.Height() // prompt + 空行 + 列表高度
}

func (p *ChoicesPanel) Update(msg tea.Msg) (ChoicesPanel, tea.Cmd) {
	if !p.visible {
		return *p, nil
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.String() == "esc" {
		p.Dismiss()
		return *p, nil
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item := p.list.SelectedItem(); item != nil {
				if ci, ok := item.(choiceItem); ok && p.onSubmit != nil {
					return *p, p.onSubmit(ci.idx)
				}
			}
		}
	}
	return *p, cmd
}

func (p *ChoicesPanel) View() string {
	if !p.visible {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(p.prompt))
	b.WriteString("\n")
	b.WriteString(p.list.View())
	return b.String()
}

type choiceItem struct {
	idx   int
	label string
}

func (i choiceItem) Title() string       { return fmt.Sprintf("%d. %s", i.idx+1, i.label) }
func (i choiceItem) Description() string { return "" }
func (i choiceItem) FilterValue() string { return i.label }
