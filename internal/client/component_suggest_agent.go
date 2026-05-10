package client

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// AgentSuggestions 在用户输入 @ 时显示可用 Agent 列表。
type AgentSuggestions struct {
	visible bool
	list    list.Model
	agents  []agentInfo
	width   int
}

func NewAgentSuggestions() *AgentSuggestions {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	d.SetHeight(1)

	l := list.New(nil, d, 0, 10)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)

	return &AgentSuggestions{
		list:    l,
		visible: false,
	}
}

func (s *AgentSuggestions) SetWidth(w int) { s.width = w; s.list.SetWidth(w) }

func (s *AgentSuggestions) SetAgents(agents []agentInfo) {
	s.agents = agents
}

// Trigger 检查输入是否在 @ 模式，是则展示建议。
func (s *AgentSuggestions) Trigger(value string) bool {
	if !strings.HasPrefix(value, "@") {
		s.visible = false
		return false
	}
	prefix := strings.TrimPrefix(value, "@")
	if strings.Contains(prefix, " ") {
		s.visible = false
		return false
	}
	var items []list.Item
	for _, a := range s.agents {
		if prefix == "" || strings.HasPrefix(strings.ToLower(a.name), strings.ToLower(prefix)) {
			items = append(items, agentItem{name: a.name, desc: a.description})
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

func (s *AgentSuggestions) Dismiss() { s.visible = false }

func (s *AgentSuggestions) Update(msg tea.Msg) (*AgentSuggestions, tea.Cmd) {
	if !s.visible {
		return s, nil
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.String() == "esc" {
		s.visible = false
		return s, nil
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item := s.list.SelectedItem(); item != nil {
				if ai, ok := item.(agentItem); ok {
					s.visible = false
					return s, func() tea.Msg {
						return suggestionCompleteMsg{text: "@" + ai.name + " "}
					}
				}
			}
		}
	}
	return s, cmd
}

func (s *AgentSuggestions) View() string {
	if !s.visible {
		return ""
	}
	return s.list.View()
}

type agentItem struct {
	name, desc string
}

func (i agentItem) Title() string       { return "@" + i.name + "  " + i.desc }
func (i agentItem) Description() string { return "" }
func (i agentItem) FilterValue() string { return i.name }
