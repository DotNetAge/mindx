package statusbar

import (
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

func TestNewStatusBar(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.ConnState != data.Disconnected {
		t.Errorf("expected ConnState=Disconnected, got %v", s.ConnState)
	}
	if s.SessionName != "" {
		t.Errorf("expected empty SessionName, got %q", s.SessionName)
	}
	if s.TokensIn != 0 {
		t.Errorf("expected TokensIn=0, got %d", s.TokensIn)
	}
	if s.TokensOut != 0 {
		t.Errorf("expected TokensOut=0, got %d", s.TokensOut)
	}
	if s.ShowHints {
		t.Error("expected ShowHints=false")
	}
	if len(s.Shortcuts) != 0 {
		t.Errorf("expected empty Shortcuts, got %d", len(s.Shortcuts))
	}
}

func TestStatusBarViewDisconnected(t *testing.T) {
	s := New()
	view := s.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "○") {
		t.Errorf("View() should contain ○, got %q", view)
	}
	if !strings.Contains(view, "Disconnected") {
		t.Errorf("View() should contain Disconnected, got %q", view)
	}
}

func TestStatusBarViewConnecting(t *testing.T) {
	s := New()
	s.ConnState = data.Connecting
	view := s.View()
	if !strings.Contains(view, "●") {
		t.Errorf("View() should contain ●, got %q", view)
	}
	if !strings.Contains(view, "Connecting") {
		t.Errorf("View() should contain Connecting, got %q", view)
	}
}

func TestStatusBarViewConnected(t *testing.T) {
	s := New()
	s.ConnState = data.Connected
	view := s.View()
	if !strings.Contains(view, "●") {
		t.Errorf("View() should contain ●, got %q", view)
	}
	if !strings.Contains(view, "Connected") {
		t.Errorf("View() should contain Connected, got %q", view)
	}
}

func TestStatusBarViewWithSession(t *testing.T) {
	s := New()
	s.SessionName = "my-session"
	view := s.View()
	if !strings.Contains(view, "my-session") {
		t.Errorf("View() should contain session name, got %q", view)
	}
}

func TestStatusBarViewWithTokens(t *testing.T) {
	s := New()
	s.TokensIn = 123
	s.TokensOut = 456
	view := s.View()
	if !strings.Contains(view, "123") {
		t.Errorf("View() should contain tokens in, got %q", view)
	}
	if !strings.Contains(view, "456") {
		t.Errorf("View() should contain tokens out, got %q", view)
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

func TestStatusBarViewWithSessionCost(t *testing.T) {
	s := New()
	s.SessionCost = "$0.05"
	view := s.View()
	if !strings.Contains(view, "$0.05") {
		t.Errorf("View() should contain session cost, got %q", view)
	}
}

func TestStatusBarViewAuthenticated(t *testing.T) {
	s := New()
	s.ConnState = data.Authenticated
	view := s.View()
	if !strings.Contains(view, "●") {
		t.Errorf("View() should contain ●, got %q", view)
	}
	if !strings.Contains(view, "Authenticated") {
		t.Errorf("View() should contain Authenticated, got %q", view)
	}
}

func TestStatusBarViewWithAllFields(t *testing.T) {
	s := New()
	s.ConnState = data.Connected
	s.SessionName = "test-session"
	s.TokensIn = 100
	s.TokensOut = 200
	s.SessionCost = "$0.02"
	s.AgentName = "coder"
	s.ModelName = "gpt-4"
	s.ModeLabel = "chat"
	s.ShowHints = true
	s.Shortcuts = []data.Shortcut{
		{Key: "Ctrl+C", Description: "copy"},
	}
	view := s.View()
	parts := []string{"Connected", "test-session", "100", "200", "$0.02", "coder", "gpt-4", "chat", "Ctrl+C", "copy"}
	for _, p := range parts {
		if !strings.Contains(view, p) {
			t.Errorf("View() should contain %q, got %q", p, view)
		}
	}
}
