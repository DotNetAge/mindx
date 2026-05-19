package data

import "testing"

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
