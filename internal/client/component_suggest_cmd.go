package client

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// CommandSuggestions 在用户输入 / 时显示可用的 slash 命令，选中后直接执行。
type CommandSuggestions struct {
	visible  bool
	list     list.Model
	registry *SlashCommandRegistry
	width    int
}

func NewCommandSuggestions(registry *SlashCommandRegistry) CommandSuggestions {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	d.SetHeight(1)

	l := list.New(nil, d, 0, 10)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)

	return CommandSuggestions{
		list:     l,
		registry: registry,
		visible:  false,
	}
}

func (s *CommandSuggestions) SetWidth(w int) { s.width = w; s.list.SetWidth(w) }

func (s *CommandSuggestions) Trigger(value string) bool {
	if !strings.HasPrefix(value, "/") {
		s.visible = false
		return false
	}
	prefix := strings.TrimPrefix(value, "/")
	if strings.Contains(prefix, " ") {
		s.visible = false
		return false
	}
	var items []list.Item
	for _, c := range s.registry.Visible() {
		if prefix == "" || strings.HasPrefix(c.Name, prefix) {
			items = append(items, cmdItem{name: c.Name, desc: c.Description})
		}
	}
	if len(items) == 0 {
		s.visible = false
		return false
	}
	s.list.SetItems(items)
	s.visible = true
	return true
}

func (s *CommandSuggestions) Dismiss() { s.visible = false }

func (s *CommandSuggestions) Update(msg tea.Msg) (CommandSuggestions, tea.Cmd) {
	if !s.visible {
		return *s, nil
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.String() == "esc" {
		s.visible = false
		return *s, nil
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item := s.list.SelectedItem(); item != nil {
				if ci, ok := item.(cmdItem); ok {
					s.visible = false
					return *s, func() tea.Msg {
						return suggestionCompleteMsg{text: "/" + ci.name + " "}
					}
				}
			}
		}
	}
	return *s, cmd
}

func (s *CommandSuggestions) View() string {
	if !s.visible {
		return ""
	}
	return s.list.View()
}

type cmdItem struct {
	name, desc string
}

func (i cmdItem) Title() string       { return "/" + i.name + "  " + i.desc }
func (i cmdItem) Description() string { return "" }
func (i cmdItem) FilterValue() string { return i.name }
