package data

import (
	"testing"
	"time"
)

func TestConnectionState(t *testing.T) {
	if Disconnected != 0 {
		t.Errorf("Disconnected = %d, want 0", Disconnected)
	}
	if Connecting != 1 {
		t.Errorf("Connecting = %d, want 1", Connecting)
	}
	if Authenticated != 2 {
		t.Errorf("Authenticated = %d, want 2", Authenticated)
	}
	if Connected != 3 {
		t.Errorf("Connected = %d, want 3", Connected)
	}
}

func TestNotificationLevel(t *testing.T) {
	if NotifInfo != 0 {
		t.Errorf("NotifInfo = %d, want 0", NotifInfo)
	}
	if NotifSuccess != 1 {
		t.Errorf("NotifSuccess = %d, want 1", NotifSuccess)
	}
	if NotifError != 2 {
		t.Errorf("NotifError = %d, want 2", NotifError)
	}
	if NotifWarning != 3 {
		t.Errorf("NotifWarning = %d, want 3", NotifWarning)
	}
}

func TestAnswerStatus(t *testing.T) {
	if StatusThinking != 0 {
		t.Errorf("StatusThinking = %d, want 0", StatusThinking)
	}
	if StatusExecuting != 1 {
		t.Errorf("StatusExecuting = %d, want 1", StatusExecuting)
	}
	if StatusResponding != 2 {
		t.Errorf("StatusResponding = %d, want 2", StatusResponding)
	}
	if StatusDone != 3 {
		t.Errorf("StatusDone = %d, want 3", StatusDone)
	}
	if StatusError != 4 {
		t.Errorf("StatusError = %d, want 4", StatusError)
	}
}

func TestActionStatus(t *testing.T) {
	if ActionExecuting != 0 {
		t.Errorf("ActionExecuting = %d, want 0", ActionExecuting)
	}
	if ActionDone != 1 {
		t.Errorf("ActionDone = %d, want 1", ActionDone)
	}
	if ActionFailed != 2 {
		t.Errorf("ActionFailed = %d, want 2", ActionFailed)
	}
}

func TestAnswerDataInit(t *testing.T) {
	var a AnswerData
	if a.SessionID != "" {
		t.Errorf("AnswerData.SessionID = %q, want empty", a.SessionID)
	}
	if a.AgentName != "" {
		t.Errorf("AnswerData.AgentName = %q, want empty", a.AgentName)
	}
	if a.UserQuestion != "" {
		t.Errorf("AnswerData.UserQuestion = %q, want empty", a.UserQuestion)
	}
	if a.Status != 0 {
		t.Errorf("AnswerData.Status = %d, want 0", a.Status)
	}
	if a.ThinkingLog != nil {
		t.Errorf("AnswerData.ThinkingLog = %v, want nil", a.ThinkingLog)
	}
	if a.PendingThink != "" {
		t.Errorf("AnswerData.PendingThink = %q, want empty", a.PendingThink)
	}
	if a.Actions != nil {
		t.Errorf("AnswerData.Actions = %v, want nil", a.Actions)
	}
	if a.Results != nil {
		t.Errorf("AnswerData.Results = %v, want nil", a.Results)
	}
	if a.IsThinking != false {
		t.Errorf("AnswerData.IsThinking = %v, want false", a.IsThinking)
	}
	if a.ThinkingCollapsed != false {
		t.Errorf("AnswerData.ThinkingCollapsed = %v, want false", a.ThinkingCollapsed)
	}
	if !a.CreatedAt.IsZero() {
		t.Errorf("AnswerData.CreatedAt = %v, want zero", a.CreatedAt)
	}
	if !a.UpdatedAt.IsZero() {
		t.Errorf("AnswerData.UpdatedAt = %v, want zero", a.UpdatedAt)
	}
	_ = time.Time{}
}
