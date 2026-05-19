package choices

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
