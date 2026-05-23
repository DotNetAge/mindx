package input

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/style"
	lipgloss "charm.land/lipgloss/v2"
)

// Suggestion holds common suggestion state and is embedded by concrete suggestion types.
type Suggestion[T any] struct {
	Items  []T
	Filter string
	SelIdx int
}

func (s *Suggestion[T]) Reset() {
	s.Filter = ""
	s.SelIdx = 0
}

func filterItems[T any](items []T, filter string, match func(T, string) bool) []T {
	if filter == "" {
		return items
	}
	var out []T
	for _, item := range items {
		if match(item, filter) {
			out = append(out, item)
		}
	}
	return out
}

func selectIndex[T any](items []T, idx int) (T, bool) {
	if len(items) == 0 || idx >= len(items) {
		var zero T
		return zero, false
	}
	return items[idx], true
}

// ---------- SlashCommand ----------

type SlashCommand struct {
	Name        string
	Description string
}

type CommandSuggestion struct {
	Suggestion[SlashCommand]
}

func (s *CommandSuggestion) filtered() []SlashCommand {
	return filterItems(s.Items, s.Filter, func(c SlashCommand, f string) bool {
		return strings.Contains(c.Name, f)
	})
}

func (s *CommandSuggestion) Select() (SlashCommand, bool) {
	return selectIndex(s.filtered(), s.SelIdx)
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

// ---------- Model ----------

type ModelItem struct {
	Name        string
	Description string
}

type ModelSuggestion struct {
	Suggestion[ModelItem]
}

func (s *ModelSuggestion) filtered() []ModelItem {
	return filterItems(s.Items, s.Filter, func(m ModelItem, f string) bool {
		return strings.Contains(strings.ToLower(m.Name), strings.ToLower(f))
	})
}

func (s *ModelSuggestion) Select() (ModelItem, bool) {
	return selectIndex(s.filtered(), s.SelIdx)
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

// ---------- Session ----------

type SessionItem struct {
	ID          string
	AgentName   string
	Preview     string
	IsSpecial   bool
	SpecialType string
}

type SessionSuggestion struct {
	Suggestion[SessionItem]
}

func (s *SessionSuggestion) filtered() []SessionItem {
	return filterItems(s.Items, s.Filter, func(sess SessionItem, f string) bool {
		return strings.Contains(sess.ID, f)
	})
}

func (s *SessionSuggestion) Select() (SessionItem, bool) {
	return selectIndex(s.filtered(), s.SelIdx)
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

// ---------- Agent ----------

type AgentSuggestion struct {
	Suggestion[data.AgentInfo]
}

func (s *AgentSuggestion) filtered() []data.AgentInfo {
	return filterItems(s.Items, s.Filter, func(a data.AgentInfo, f string) bool {
		return strings.Contains(strings.ToLower(a.Name), strings.ToLower(f))
	})
}

func (s *AgentSuggestion) Select() (data.AgentInfo, bool) {
	return selectIndex(s.filtered(), s.SelIdx)
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
