package statusbar

import (
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

func TestNewStatusBar(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.CurrentState != i18n.T("client.status.idle") {
		t.Errorf("expected CurrentState %q, got %q", i18n.T("client.status.idle"), s.CurrentState)
	}
	if s.SessionName != "" {
		t.Errorf("expected empty SessionName, got %q", s.SessionName)
	}
	if s.TokensTotal != 0 {
		t.Errorf("expected TokensTotal=0, got %d", s.TokensTotal)
	}
	if s.ShowHints {
		t.Error("expected ShowHints=false")
	}
	if len(s.Shortcuts) != 0 {
		t.Errorf("expected empty Shortcuts, got %d", len(s.Shortcuts))
	}
}

func TestStatusBarViewIdle(t *testing.T) {
	s := New()
	s.CurrentState = i18n.T("client.status.idle")
	view := s.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "●") {
		t.Errorf("View() should contain ●, got %q", view)
	}
	if !strings.Contains(view, i18n.T("client.status.idle")) {
		t.Errorf("View() should contain %s, got %q", i18n.T("client.status.idle"), view)
	}
}

func TestStatusBarViewProcessing(t *testing.T) {
	s := New()
	s.CurrentState = "处理中"
	s.BlinkOn = true
	view := s.View()
	if !strings.Contains(view, "●") {
		t.Errorf("View() should contain ●, got %q", view)
	}
	if !strings.Contains(view, "处理中") {
		t.Errorf("View() should contain 处理中, got %q", view)
	}
}

func TestStatusBarViewWithSession(t *testing.T) {
	s := New()
	s.SessionName = "my-session"
	view := s.View()
	if s.SessionName != "my-session" {
		t.Errorf("SessionName should be stored, got %q", s.SessionName)
	}
	if view == "" {
		t.Fatal("View() returned empty string")
	}
}

func TestStatusBarViewWithTokens(t *testing.T) {
	s := New()
	s.TokensTotal = 12345
	view := s.View()
	if !strings.Contains(view, "12.3k") {
		t.Errorf("View() should contain formatted tokens, got %q", view)
	}
}

func TestStatusBarViewWithAgentModel(t *testing.T) {
	s := New()
	s.AgentName = "architect"
	s.ModelName = "claude-sonnet"
	view := s.View()
	if !strings.Contains(view, "architect") {
		t.Errorf("View() should contain agent name, got %q", view)
	}
	if !strings.Contains(view, "claude-sonnet") {
		t.Errorf("View() should contain model name, got %q", view)
	}
}

func TestStatusBarViewWithMode(t *testing.T) {
	s := New()
	s.ModeLabel = "transcript"
	view := s.View()
	if !strings.Contains(view, "transcript") {
		t.Errorf("View() should contain mode label, got %q", view)
	}
}

func TestStatusBarViewWithShortcuts(t *testing.T) {
	s := New()
	s.ShowHints = true
	s.Shortcuts = []data.Shortcut{
		{Key: "Ctrl+O", Description: "test"},
	}
	view := s.View()
	if !strings.Contains(view, "Ctrl+O") {
		t.Errorf("View() should contain shortcut key, got %q", view)
	}
	if !strings.Contains(view, "test") {
		t.Errorf("View() should contain shortcut description, got %q", view)
	}
}

func TestStatusBarUpdate(t *testing.T) {
	s := New()
	updated, cmd := s.Update(clientmsg.TickMsg{})
	if updated != s {
		t.Error("Update() should return the same pointer")
	}
	if cmd != nil {
		t.Error("Update() should return nil cmd")
	}
}

func TestStatusBarViewWithAllFields(t *testing.T) {
	s := New()
	s.CurrentState = "空闲"
	s.SessionName = "test-session"
	s.TokensTotal = 1500
	s.AgentName = "coder"
	s.ModelName = "gpt-4"
	s.ModeLabel = "chat"
	s.ShowHints = true
	s.Shortcuts = []data.Shortcut{
		{Key: "Ctrl+C", Description: "copy"},
	}
	view := s.View()
	parts := []string{"空闲", "1.5k", "coder", "gpt-4", "chat", "Ctrl+C", "copy"}
	for _, p := range parts {
		if !strings.Contains(view, p) {
			t.Errorf("View() should contain %q, got %q", p, view)
		}
	}
}
