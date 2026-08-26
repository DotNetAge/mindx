package input

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
)

func TestRenderSuggestionListEmpty(t *testing.T) {
	if got := renderSuggestionList([]ModelItem{}, 0, 40, "title", func(m ModelItem) (string, string) { return m.Name, m.Description }); got != "" {
		t.Errorf("empty items should render empty string, got %q", got)
	}
}

// CJK 名称按 2 列计宽，描述列必须对齐：各行描述起始列差应为 0。
// 统一使用相同描述词作为锚点，排除锚点自身偏移干扰。
func TestRenderSuggestionListCJKAlignment(t *testing.T) {
	items := []ModelItem{
		{Name: "gpt-4o", Description: "MARK"},
		{Name: "通义千问-max", Description: "MARK"},
		{Name: "claude-sonnet-4", Description: "MARK"},
	}
	out := renderSuggestionList(items, -1, 80, "", func(m ModelItem) (string, string) { return m.Name, m.Description })

	lines := strings.Split(out, "\n")
	if len(lines) != len(items) {
		t.Fatalf("expected %d lines, got %d", len(items), len(lines))
	}
	start := descDisplayColumn(lines[0])
	for i, l := range lines {
		if col := descDisplayColumn(l); col != start {
			t.Errorf("desc column misaligned: line0 at %d, line%d at %d\n%s", start, i, col, out)
		}
	}
}

// descDisplayColumn 用 lipgloss.Width（CJK 按 2 列计）计算描述词的终端显示列，
// strings.Index 的字节偏移无法反映 CJK 宽字符的真实占位。
func descDisplayColumn(line string) int {
	return lipgloss.Width(strings.SplitN(line, "MARK", 2)[0])
}

func TestRenderSuggestionListNameColumnCapped(t *testing.T) {
	items := []ModelItem{
		{Name: strings.Repeat("x", 60), Description: "LONGROW"},
		{Name: "short", Description: "SHORTROW"},
	}
	out := renderSuggestionList(items, -1, 200, "", func(m ModelItem) (string, string) { return m.Name, m.Description })

	lines := strings.Split(out, "\n")
	// 短名称行的描述列被超长名称撑到上限后封顶：前缀(2) + maxNameColumn + 间隔(2)。
	shortRowDescStart := strings.Index(lines[1], "SHORTROW")
	if want := 2 + maxNameColumn + 2; shortRowDescStart != want {
		t.Errorf("capped desc column = %d, want %d\n%s", shortRowDescStart, want, out)
	}
}

func TestRenderSuggestionListSelectionIndicator(t *testing.T) {
	items := []ModelItem{{Name: "a"}, {Name: "b"}}
	out := renderSuggestionList(items, 1, 40, "", func(m ModelItem) (string, string) { return m.Name, "" })
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], "a") || strings.Contains(lines[0], suggestionIndicator) {
		t.Errorf("unselected row should not carry indicator: %q", lines[0])
	}
	if !strings.Contains(lines[1], suggestionIndicator+" b") {
		t.Errorf("selected row should carry indicator: %q", lines[1])
	}
}

func TestUpdateSuggestionBareModelKeepsCommandPanel(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{{Name: "model"}}
	i.Models = []ModelItem{{Name: "gpt-4o"}}

	for _, ch := range "/model" {
		i.insertAtCursor(ch)
	}
	if len(i.modelSuggest.Items) != 0 {
		t.Error("bare /model should NOT activate inline model panel (reserved for command panel)")
	}
	if len(i.cmdSuggest.Items) == 0 {
		t.Error("bare /model should keep command suggestions active")
	}

	i.insertAtCursor(' ')
	if len(i.modelSuggest.Items) == 0 {
		t.Error("/model<space> should activate inline model panel")
	}
	if len(i.cmdSuggest.Items) != 0 {
		t.Error("command panel should be reset once model panel activates")
	}
}

func TestUpdateSuggestionModelfooNoTrigger(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{{Name: "modelfoo"}}
	i.Models = []ModelItem{{Name: "gpt-4o"}}

	for _, ch := range "/modelfoo" {
		i.insertAtCursor(ch)
	}
	if len(i.modelSuggest.Items) != 0 {
		t.Error("/modelfoo must not trigger the model panel")
	}
	if len(i.cmdSuggest.Items) == 0 {
		t.Error("/modelfoo should match as command suggestion instead")
	}
}
