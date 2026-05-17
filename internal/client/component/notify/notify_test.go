package notify

import (
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

func TestNewNotificationBar(t *testing.T) {
	n := New()
	if n == nil {
		t.Fatal("New() returned nil")
	}
	if n.MaxVisible != 5 {
		t.Errorf("expected MaxVisible=5, got %d", n.MaxVisible)
	}
	if n.Width != 80 {
		t.Errorf("expected Width=80, got %d", n.Width)
	}
	if len(n.Notifications) != 0 {
		t.Errorf("expected empty notifications, got %d", len(n.Notifications))
	}
}

func TestNotificationBarEmptyView(t *testing.T) {
	n := New()
	view := n.View()
	if view != "" {
		t.Errorf("expected empty View(), got %q", view)
	}
}

func TestNotificationAdd(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		Message: "hello world",
	})
	if len(n.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(n.Notifications))
	}
	if n.Notifications[0].Message != "hello world" {
		t.Errorf("expected message 'hello world', got %q", n.Notifications[0].Message)
	}
}

func TestNotificationIconInfo(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		Level:   data.NotifInfo,
		Message: "info test",
	})
	view := n.View()
	if !strings.Contains(view, "ℹ") {
		t.Errorf("View() should contain ℹ for info, got %q", view)
	}
	if !strings.Contains(view, "info test") {
		t.Errorf("View() should contain message, got %q", view)
	}
}

func TestNotificationIconSuccess(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		Level:   data.NotifSuccess,
		Message: "success test",
	})
	view := n.View()
	if !strings.Contains(view, "✓") {
		t.Errorf("View() should contain ✓ for success, got %q", view)
	}
}

func TestNotificationIconError(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		Level:   data.NotifError,
		Message: "error test",
	})
	view := n.View()
	if !strings.Contains(view, "✗") {
		t.Errorf("View() should contain ✗ for error, got %q", view)
	}
}

func TestNotificationIconWarning(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		Level:   data.NotifWarning,
		Message: "warning test",
	})
	view := n.View()
	if !strings.Contains(view, "⚠") {
		t.Errorf("View() should contain ⚠ for warning, got %q", view)
	}
}

func TestNotificationMaxVisible(t *testing.T) {
	n := New()
	for range 6 {
		n.Add(data.Notification{
			Level:   data.NotifInfo,
			Message: "notif",
		})
	}
	if len(n.Notifications) != 6 {
		t.Fatalf("expected 6 notifications stored, got %d", len(n.Notifications))
	}
	view := n.View()
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) > n.MaxVisible {
		t.Errorf("expected at most %d lines, got %d", n.MaxVisible, len(lines))
	}
}

func TestNotifTimeoutRemove(t *testing.T) {
	n := New()
	notif := data.Notification{
		ID:      "test-id-1",
		Level:   data.NotifInfo,
		Message: "to remove",
	}
	n.Add(notif)
	if len(n.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(n.Notifications))
	}
	n.Update(clientmsg.NotifTimeoutMsg{ID: "test-id-1"})
	if len(n.Notifications) != 0 {
		t.Errorf("expected 0 notifications after timeout, got %d", len(n.Notifications))
	}
}

func TestWindowResize(t *testing.T) {
	n := New()
	if n.Width != 80 {
		t.Fatalf("expected initial Width=80, got %d", n.Width)
	}
	n.Update(clientmsg.WindowResizeMsg{Width: 100, Height: 50})
	if n.Width != 100 {
		t.Errorf("expected Width=100 after resize, got %d", n.Width)
	}
}

func TestNotificationOrder(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		ID:      "first",
		Level:   data.NotifInfo,
		Message: "first notif",
	})
	n.Add(data.Notification{
		ID:      "second",
		Level:   data.NotifInfo,
		Message: "second notif",
	})
	n.Add(data.Notification{
		ID:      "third",
		Level:   data.NotifInfo,
		Message: "third notif",
	})
	view := n.View()
	firstIdx := strings.Index(view, "first notif")
	secondIdx := strings.Index(view, "second notif")
	thirdIdx := strings.Index(view, "third notif")
	if thirdIdx > secondIdx {
		t.Error("expected newest (third) to appear before second")
	}
	if secondIdx > firstIdx {
		t.Error("expected second to appear before first")
	}
}

func TestNotificationWithCustomID(t *testing.T) {
	n := New()
	n.Add(data.Notification{
		ID:      "my-custom-id",
		Level:   data.NotifInfo,
		Message: "custom id",
	})
	if n.Notifications[0].ID != "my-custom-id" {
		t.Errorf("expected ID 'my-custom-id', got %q", n.Notifications[0].ID)
	}
}

func TestNotificationIconDefault(t *testing.T) {
	n := New()
	var unknownLevel data.NotificationLevel = 99
	n.Add(data.Notification{
		Level:   unknownLevel,
		Message: "unknown level",
	})
	view := n.View()
	if !strings.Contains(view, "?") {
		t.Errorf("View() should contain ? for unknown level, got %q", view)
	}
}
