package input

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/style"
	lipgloss "charm.land/lipgloss/v2"
)

type AgentSuggestion struct {
	Agents []data.AgentInfo
	Filter string
	SelIdx int
}

func (s *AgentSuggestion) filtered() []data.AgentInfo {
	if s.Filter == "" {
		return s.Agents
	}
	var out []data.AgentInfo
	for _, a := range s.Agents {
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(s.Filter)) {
			out = append(out, a)
		}
	}
	return out
}

func (s *AgentSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range list {
		line := fmt.Sprintf("@%s  %s", a.Name, a.Description)
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (s *AgentSuggestion) Reset() {
	s.Filter = ""
	s.SelIdx = 0
}

func (s *AgentSuggestion) Select() (selected data.AgentInfo, ok bool) {
	list := s.filtered()
	if len(list) == 0 || s.SelIdx >= len(list) {
		return data.AgentInfo{}, false
	}
	return list[s.SelIdx], true
}
