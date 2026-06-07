package notify

import (
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/google/uuid"
)

type NotificationBar struct {
	Notifications []data.Notification
	MaxVisible    int
	Width         int
}

func New() *NotificationBar {
	return &NotificationBar{
		Notifications: make([]data.Notification, 0),
		MaxVisible:    5,
		Width:         80,
	}
}

func (n *NotificationBar) Update(msg any) (*NotificationBar, tea.Cmd) {
	switch msg := msg.(type) {
	case clientmsg.NotifTimeoutMsg:
		return n.removeByID(msg.ID), nil
	case clientmsg.WindowResizeMsg:
		n.Width = msg.Width
		return n, nil
	default:
		return n, nil
	}
}

func (n *NotificationBar) Add(notif data.Notification) tea.Cmd {
	if notif.ID == "" {
		notif.ID = uuid.NewString()
	}
	if notif.Duration == 0 {
		notif.Duration = 3 * time.Second
	}
	notif.CreatedAt = time.Now()
	n.Notifications = append(n.Notifications, notif)
	return func() tea.Msg {
		time.Sleep(notif.Duration)
		return clientmsg.NotifTimeoutMsg{ID: notif.ID}
	}
}

func (n *NotificationBar) View() string {
	if len(n.Notifications) == 0 {
		return ""
	}

	var lines []string
	for i := len(n.Notifications) - 1; i >= 0 && len(lines) < n.MaxVisible; i-- {
		notif := n.Notifications[i]

		icon := iconForLevel(notif.Level)
		coloredIcon := colorIcon(notif.Level, icon)

		lineStyle := lipgloss.NewStyle().MaxWidth(n.Width)
		line := lineStyle.Render(coloredIcon + " " + notif.Message)
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (n *NotificationBar) removeByID(id string) *NotificationBar {
	for i, notif := range n.Notifications {
		if notif.ID == id {
			n.Notifications = append(n.Notifications[:i], n.Notifications[i+1:]...)
			break
		}
	}
	return n
}

func iconForLevel(level data.NotificationLevel) string {
	switch level {
	case data.NotifSuccess:
		return "✓"
	case data.NotifError:
		return "✗"
	case data.NotifInfo:
		return "ℹ"
	case data.NotifWarning:
		return "⚠"
	default:
		return "?"
	}
}

func colorIcon(level data.NotificationLevel, icon string) string {
	var s lipgloss.Style
	switch level {
	case data.NotifSuccess:
		s = style.GreenStyle
	case data.NotifError:
		s = style.RedStyle
	case data.NotifInfo:
		s = style.CyanStyle
	case data.NotifWarning:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107"))
	default:
		s = style.GrayStyle
	}
	return s.Render(icon)
}
