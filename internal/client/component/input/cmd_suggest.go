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

type ModelSuggestion struct {
	Models []ModelItem
	Filter string
	SelIdx int
}

type ModelItem struct {
	Name        string
	Description string
}

func (s *ModelSuggestion) filtered() []ModelItem {
	if s.Filter == "" {
		return s.Models
	}
	var out []ModelItem
	for _, m := range s.Models {
		if strings.Contains(strings.ToLower(m.Name), strings.ToLower(s.Filter)) {
			out = append(out, m)
		}
	}
	return out
}

func (s *ModelSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(style.BoldWhite.Render("📦 可用模型\n"))
	for i, m := range list {
		line := fmt.Sprintf("  %s  %s", m.Name, m.Description)
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("▸" + line))
		} else {
			b.WriteString(" " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (s *ModelSuggestion) Reset() {
	s.Filter = ""
	s.SelIdx = 0
}

func (s *ModelSuggestion) Select() (selected ModelItem, ok bool) {
	list := s.filtered()
	if len(list) == 0 || s.SelIdx >= len(list) {
		return ModelItem{}, false
	}
	return list[s.SelIdx], true
}

type SessionSuggestion struct {
	Sessions   []SessionItem
	Filter     string
	SelIdx     int
}

type SessionItem struct {
	ID          string
	AgentName   string
	Preview     string
	IsSpecial   bool
	SpecialType string
}

func (s *SessionSuggestion) filtered() []SessionItem {
	if s.Filter == "" {
		return s.Sessions
	}
	var out []SessionItem
	for _, sess := range s.Sessions {
		if strings.Contains(sess.ID, s.Filter) {
			out = append(out, sess)
		}
	}
	return out
}

func (s *SessionSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(style.BoldWhite.Render("💬 会话管理\n"))
	for i, sess := range list {
		var line string
		if sess.IsSpecial {
			switch sess.SpecialType {
			case "new":
				line = fmt.Sprintf("  [%s]  新建会话", style.GreenStyle.Render("new"))
			case "clear":
				line = fmt.Sprintf("  [%s]  清除当前会话", style.RedStyle.Render("clear"))
			default:
				line = fmt.Sprintf("  [%s]", sess.SpecialType)
			}
		} else {
			line = fmt.Sprintf("  %s  %s · %s", sess.ID, sess.AgentName, sess.Preview)
		}
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("▸" + line))
		} else {
			b.WriteString(" " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (s *SessionSuggestion) Reset() {
	s.Filter = ""
	s.SelIdx = 0
}

func (s *SessionSuggestion) Select() (selected SessionItem, ok bool) {
	list := s.filtered()
	if len(list) == 0 || s.SelIdx >= len(list) {
		return SessionItem{}, false
	}
	return list[s.SelIdx], true
}
