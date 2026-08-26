package input

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const (
	// suggestionIndicator 选中行光标，固定占 2 列（含尾随空格），未选中行以等宽空格补位。
	suggestionIndicator = "▸"
	// maxNameColumn 名称列对齐宽度上限，防止超长 ID 把描述列推出屏幕。
	maxNameColumn = 32
)

// renderSuggestionList 是所有补全面板的唯一渲染路径：统一指示符、缩进与名称列对齐。
// 列宽基于 lipgloss.Width（CJK 按 2 列计），中英文混排不散架。
// line 返回单行的 (名称, 描述)；描述为空时整行只渲染名称。
func renderSuggestionList[T any](items []T, selected int, width int, title string, line func(T) (name, desc string)) string {
	if len(items) == 0 {
		return ""
	}

	type row struct{ name, desc string }
	rows := make([]row, len(items))
	nameW := 0
	for i, item := range items {
		rows[i].name, rows[i].desc = line(item)
		if w := lipgloss.Width(rows[i].name); w > nameW {
			nameW = w
		}
	}
	if nameW > maxNameColumn {
		nameW = maxNameColumn
	}

	var b strings.Builder
	if title != "" {
		b.WriteString(style.BoldWhite.Render(title))
		b.WriteByte('\n')
	}
	for i, r := range rows {
		text := r.name
		if r.desc != "" {
			pad := nameW - lipgloss.Width(r.name) + 2
			if pad < 2 {
				pad = 2
			}
			text += strings.Repeat(" ", pad) + r.desc
		}
		if i == selected {
			b.WriteString(style.CyanStyle.Render(suggestionIndicator + " " + text))
		} else {
			b.WriteString("  " + text)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

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
	return renderSuggestionList(list, s.SelIdx, width, "", func(c SlashCommand) (string, string) {
		return "/" + c.Name, c.Description
	})
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
	return renderSuggestionList(list, s.SelIdx, width, i18n.T("client.ui.suggest.model.title"), func(m ModelItem) (string, string) {
		return m.Name, m.Description
	})
}

// ---------- Session ----------

type SessionItem struct {
	ID          string
	AgentName   string
	Preview     string
	IsSpecial   bool
	SpecialType string
}

const (
	sessionSpecialNew   = "new"
	sessionSpecialClear = "clear"
)

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

func sessionLine(sess SessionItem) (string, string) {
	if !sess.IsSpecial {
		return sess.ID, sess.AgentName + " · " + sess.Preview
	}
	switch sess.SpecialType {
	case sessionSpecialNew:
		return "[" + style.GreenStyle.Render(sessionSpecialNew) + "]", i18n.T("client.ui.suggest.session.new")
	case sessionSpecialClear:
		return "[" + style.RedStyle.Render(sessionSpecialClear) + "]", i18n.T("client.ui.suggest.session.clear")
	default:
		return "[" + sess.SpecialType + "]", ""
	}
}

func (s *SessionSuggestion) View(width int) string {
	list := s.filtered()
	return renderSuggestionList(list, s.SelIdx, width, i18n.T("client.ui.suggest.session.title"), sessionLine)
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
	return renderSuggestionList(list, s.SelIdx, width, "", func(a data.AgentInfo) (string, string) {
		return "@" + a.Name, a.Description
	})
}
