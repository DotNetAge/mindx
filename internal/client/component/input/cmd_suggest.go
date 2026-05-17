package input

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/style"
	lipgloss "charm.land/lipgloss/v2"
)

type CommandSuggestion struct {
	Commands []SlashCommand
	Filter   string
	SelIdx   int
}

type SlashCommand struct {
	Name        string
	Description string
}

func (s *CommandSuggestion) filtered() []SlashCommand {
	if s.Filter == "" {
		return s.Commands
	}
	var out []SlashCommand
	for _, c := range s.Commands {
		if strings.Contains(c.Name, s.Filter) {
			out = append(out, c)
		}
	}
	return out
}

func (s *CommandSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range list {
		line := fmt.Sprintf("/%s  %s", c.Name, c.Description)
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (s *CommandSuggestion) Reset() {
	s.Filter = ""
	s.SelIdx = 0
}

func (s *CommandSuggestion) Select() (selected SlashCommand, ok bool) {
	list := s.filtered()
	if len(list) == 0 || s.SelIdx >= len(list) {
		return SlashCommand{}, false
	}
	return list[s.SelIdx], true
}
