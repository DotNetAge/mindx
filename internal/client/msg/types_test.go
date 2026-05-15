package msg

import (
	"errors"
	"testing"
)

func TestWindowResizeMsg(t *testing.T) {
	m := WindowResizeMsg{Width: 120, Height: 40}
	if m.Width != 120 {
		t.Errorf("Width = %d, want 120", m.Width)
	}
	if m.Height != 40 {
		t.Errorf("Height = %d, want 40", m.Height)
	}
}

func TestShowChoicesMsg(t *testing.T) {
	m := ShowChoicesMsg{Options: []string{"a", "b"}, Prompt: "choose"}
	if len(m.Options) != 2 || m.Options[0] != "a" || m.Options[1] != "b" {
		t.Errorf("Options = %v, want [a b]", m.Options)
	}
	if m.Prompt != "choose" {
		t.Errorf("Prompt = %q, want %q", m.Prompt, "choose")
	}
}

func TestActionStartMsg(t *testing.T) {
	m := ActionStartMsg{SessionID: "s1", ToolName: "read", EstimatedTok: 100}
	if m.SessionID != "s1" {
		t.Errorf("SessionID = %q, want %q", m.SessionID, "s1")
	}
	if m.ToolName != "read" {
		t.Errorf("ToolName = %q, want %q", m.ToolName, "read")
	}
	if m.EstimatedTok != 100 {
		t.Errorf("EstimatedTok = %d, want 100", m.EstimatedTok)
	}
}

func TestAgentErrorMsg(t *testing.T) {
	m := AgentErrorMsg{Error: errors.New("err")}
	if m.Error.Error() != "err" {
		t.Errorf("Error = %q, want %q", m.Error.Error(), "err")
	}
}

func TestTickMsg(t *testing.T) {
	var m TickMsg
	if m != (TickMsg{}) {
		t.Errorf("TickMsg zero value mismatch")
	}
}

func TestExitMsg(t *testing.T) {
	var m ExitMsg
	if m != (ExitMsg{}) {
		t.Errorf("ExitMsg zero value mismatch")
	}
}
