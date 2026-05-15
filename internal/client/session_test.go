package client

import (
	"testing"

	"github.com/DotNetAge/mindx/internal/client/data"
)

func TestChatSession(t *testing.T) {
	s := data.ChatSession{AgentName: "test", SessionID: "sid1"}
	if s.AgentName != "test" {
		t.Errorf("AgentName = %q, want %q", s.AgentName, "test")
	}
	if s.SessionID != "sid1" {
		t.Errorf("SessionID = %q, want %q", s.SessionID, "sid1")
	}
}
