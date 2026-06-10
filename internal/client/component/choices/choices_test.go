package choices

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/i18n"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

func TestNewChoicesPanel(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.Visible {
		t.Error("expected Visible=false")
	}
}

func TestChoicesPanelHiddenByDefault(t *testing.T) {
	p := New()
	view := p.View()
	if view != "" {
		t.Errorf("expected empty View(), got %q", view)
	}
}

func TestShowChoicesMsg(t *testing.T) {
	p := New()
	options := []string{"alpha", "beta", "gamma"}
	msg := clientmsg.ShowChoicesMsg{
		Options: options,
		Prompt:  "pick one",
	}
	p.Update(msg)
	if !p.Visible {
		t.Error("expected Visible=true after ShowChoicesMsg")
	}
	if len(p.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(p.Items))
	}
	if p.Prompt != "pick one" {
		t.Errorf("expected prompt 'pick one', got %q", p.Prompt)
	}
	if p.Cursor != 0 {
		t.Errorf("expected Cursor=0, got %d", p.Cursor)
	}
}

func TestChoicesPanelView(t *testing.T) {
	p := New()
	options := []string{"alpha", "beta", "gamma"}
	msg := clientmsg.ShowChoicesMsg{
		Options: options,
		Prompt:  "pick one",
	}
	p.Update(msg)
	view := p.View()
	if view == "" {
		t.Fatal("View() returned empty string after ShowChoicesMsg")
	}
	if !strings.Contains(view, "alpha") {
		t.Errorf("View() should contain 'alpha' option, got %q", view)
	}
	if !strings.Contains(view, "beta") {
		t.Errorf("View() should contain 'beta' option, got %q", view)
	}
	if !strings.Contains(view, "gamma") {
		t.Errorf("View() should contain 'gamma' option, got %q", view)
	}
}

func TestCursorNavigationDown(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "choose",
	})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.Cursor != 1 {
		t.Errorf("expected Cursor=1 after down, got %d", p.Cursor)
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.Cursor != 2 {
		t.Errorf("expected Cursor=2 after second down, got %d", p.Cursor)
	}
}

func TestCursorNavigationUp(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "choose",
	})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.Cursor != 2 {
		t.Fatalf("expected Cursor=2 after two downs, got %d", p.Cursor)
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.Cursor != 1 {
		t.Errorf("expected Cursor=1 after up, got %d", p.Cursor)
	}
}

func TestCursorBoundaryUp(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "choose",
	})
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.Cursor != 0 {
		t.Errorf("expected Cursor=0 when pressing up at first item, got %d", p.Cursor)
	}
}

func TestCursorBoundaryDown(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "choose",
	})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.Cursor != 2 {
		t.Errorf("expected Cursor=2 when pressing down past end, got %d", p.Cursor)
	}
}

func TestChoicesPanelEnterSelect(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "choose",
	})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	updated, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on enter")
	}
	msg := cmd()
	selected, ok := msg.(clientmsg.ChoiceSelectedMsg)
	if !ok {
		t.Fatalf("expected ChoiceSelectedMsg, got %T", msg)
	}
	if selected.Index != 2 {
		t.Errorf("expected Index=2, got %d", selected.Index)
	}
	if updated.Visible {
		t.Error("expected Visible=false after enter selection")
	}
}

func TestHiddenChoicesIgnoreKeys(t *testing.T) {
	p := New()
	p.Items = []string{"a", "b", "c"}
	p.Visible = false
	beforeCursor := p.Cursor
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.Cursor != beforeCursor {
		t.Error("key events should be ignored when panel is hidden")
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.Cursor != beforeCursor {
		t.Error("key events should be ignored when panel is hidden")
	}
}

func TestChoicesPanelEmptyItems(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{},
		Prompt:  "empty",
	})
	view := p.View()
	if view != "" {
		t.Errorf("expected empty View() for empty items, got %q", view)
	}
}

func TestMultiSelectMode(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"a", "b", "c"},
		Prompt:      "multi pick",
		MultiSelect: true,
	})

	if !p.MultiSelect {
		t.Error("expected MultiSelect=true")
	}
	if p.Selected == nil {
		t.Error("expected Selected map to be initialized")
	}

	view := p.View()
	if !strings.Contains(view, "[ ] ") {
		t.Errorf("multi-select view should show unchecked boxes, got %q", view)
	}
}

func TestMultiSelectToggle(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"a", "b", "c"},
		Prompt:      "multi pick",
		MultiSelect: true,
	})

	p.Update(tea.KeyPressMsg{Code: ' '})
	if !p.Selected[0] {
		t.Error("expected item 0 to be selected after Space")
	}

	view := p.View()
	if !strings.Contains(view, "[✓]") {
		t.Errorf("view should show checked box after selection, got %q", view)
	}

	p.Update(tea.KeyPressMsg{Code: ' '})
	if p.Selected[0] {
		t.Error("expected item 0 to be deselected after second Space")
	}
}

func TestMultiSelectMultiple(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"a", "b", "c"},
		Prompt:      "multi pick",
		MultiSelect: true,
	})

	p.Update(tea.KeyPressMsg{Code: ' '})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: ' '})

	count := 0
	for _, v := range p.Selected {
		if v {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 selected items, got %d", count)
	}
}

func TestMultiSelectEnterResult(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"x", "y", "z"},
		Prompt:      "multi",
		MultiSelect: true,
	})

	p.Update(tea.KeyPressMsg{Code: ' '})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: ' '})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on enter in multi-select")
	}
	msg := cmd()
	selected, ok := msg.(clientmsg.ChoiceSelectedMsg)
	if !ok {
		t.Fatalf("expected ChoiceSelectedMsg, got %T", msg)
	}
	if len(selected.Indices) != 2 {
		t.Errorf("expected 2 indices in result, got %d", len(selected.Indices))
	}
}

func TestMultiSelectWithTextInput(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"file_a", "file_b"},
		Prompt:         "files with other option",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	if !p.AllowTextInput {
		t.Error("expected AllowTextInput=true")
	}
	if p.inputActive {
		t.Error("expected inputActive=false by default")
	}

	view := p.View()
	if !strings.Contains(view, i18n.T("choices.input.other")) {
		t.Errorf("view should show custom input field, got %q", view)
	}
}

func TestCustomInputTabToggle(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !p.inputActive {
		t.Error("expected inputActive=true after Tab")
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if p.inputActive {
		t.Error("expected inputActive=false after second Tab")
	}
}

func TestCustomInputTyping(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	p.Update(tea.KeyPressMsg{Code: 'h'})
	p.Update(tea.KeyPressMsg{Code: 'i'})

	if p.CustomText != "hi" {
		t.Errorf("expected CustomText='hi', got %q", p.CustomText)
	}
}

func TestCustomInputBackspace(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	p.Update(tea.KeyPressMsg{Code: 'a'})
	p.Update(tea.KeyPressMsg{Code: 'b'})
	p.Update(tea.KeyPressMsg{Code: 'c'})
	p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if p.CustomText != "ab" {
		t.Errorf("expected CustomText='ab' after backspace, got %q", p.CustomText)
	}
}

func TestCustomInputEscExits(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !p.inputActive {
		t.Error("expected inputActive=true after Tab")
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if p.inputActive {
		t.Error("expected inputActive=false after Esc")
	}
	if !p.Visible {
		t.Error("expected panel still visible after Esc from input mode")
	}
}

func TestCustomInputUpReturnsToList(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.inputActive {
		t.Error("expected inputActive=false after Up arrow")
	}
}

func TestCustomInputEnterSubmitsWithText(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"a", "b"},
		Prompt:         "test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	p.Update(tea.KeyPressMsg{Code: 'o'})
	p.Update(tea.KeyPressMsg{Code: 't'})
	p.Update(tea.KeyPressMsg{Code: 'h'})
	p.Update(tea.KeyPressMsg{Code: 'e'})
	p.Update(tea.KeyPressMsg{Code: 'r'})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on Enter in input mode")
	}
	msg := cmd()
	selected, ok := msg.(clientmsg.ChoiceSelectedMsg)
	if !ok {
		t.Fatalf("expected ChoiceSelectedMsg, got %T", msg)
	}
	if selected.CustomText != "other" {
		t.Errorf("expected CustomText='other', got %q", selected.CustomText)
	}
}

func TestMultiSelectEscCancel(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"a", "b"},
		Prompt:      "test",
		MultiSelect: true,
	})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on esc")
	}
	msg := cmd()
	selected, ok := msg.(clientmsg.ChoiceSelectedMsg)
	if !ok {
		t.Fatalf("expected ChoiceSelectedMsg, got %T", msg)
	}
	if selected.Index != -1 {
		t.Errorf("expected Index=-1 for cancel, got %d", selected.Index)
	}
}

func TestSingleSelectSpaceIgnored(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options: []string{"a", "b", "c"},
		Prompt:  "single",
	})

	beforeVisible := p.Visible
	p.Update(tea.KeyPressMsg{Code: ' '})
	if p.Visible != beforeVisible {
		t.Error("Space should be ignored in single-select mode")
	}
}

func TestMultiSelectCursorVisibleInRender(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:     []string{"first", "second", "third"},
		Prompt:      "cursor test",
		MultiSelect: true,
	})

	view := p.View()
	if !strings.Contains(view, "● [ ] first") && !strings.Contains(view, "● [ ]first") {
		t.Errorf("multi-select view should show cursor ● on first item, got %q", view)
	}

	p.Update(tea.KeyPressMsg{Code: ' '})
	view = p.View()
	if !strings.Contains(view, "● [✓] first") && !strings.Contains(view, "● [✓]first") {
		t.Errorf("multi-select view should show cursor + checked on first item, got %q", view)
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	view = p.View()
	if strings.Contains(view, "● [✓] second") || strings.Contains(view, "● [✓]second") {
		t.Error("second item should not be checked yet")
	}
	if !strings.Contains(view, "○ [✓] first") {
		t.Errorf("first item should still show as checked (non-cursor), got %q", view)
	}
}

func TestMultiSelectInputNotLinkedToItems(t *testing.T) {
	p := New()
	p.Update(clientmsg.ShowChoicesMsg{
		Options:        []string{"opt1", "opt2"},
		Prompt:         "independent input test",
		MultiSelect:    true,
		AllowTextInput: true,
	})

	p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	p.Update(tea.KeyPressMsg{Code: 'f'})
	p.Update(tea.KeyPressMsg{Code: 'r'})
	p.Update(tea.KeyPressMsg{Code: 'e'})
	p.Update(tea.KeyPressMsg{Code: 'e'})

	if p.CustomText != "free" {
		t.Errorf("expected CustomText='free', got %q", p.CustomText)
	}

	for i := 1; i < len(p.Items); i++ {
		if p.Selected[i] {
			t.Errorf("item %d should not be affected by custom text input", i)
		}
	}
}
