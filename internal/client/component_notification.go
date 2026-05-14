package client

import (
	"fmt"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var notifIDCounter atomic.Int64

// notification 表示一条通知消息。
type notification struct {
	id        string
	level     string // "info", "success", "error", "warning"
	message   string
	createdAt time.Time
	duration  time.Duration // 0 = 需手动关闭
}

// NotificationBar 是浮动通知栏组件，支持自动消失的 toast 通知。
type NotificationBar struct {
	notifications []notification
	maxVisible    int // 同时最多显示的通知数
	width         int
}

// NewNotificationBar 创建一个新的通知栏。
func NewNotificationBar() *NotificationBar {
	return &NotificationBar{
		maxVisible: 1,
	}
}

// notificationTimeoutMsgInternal 由 tea.Tick 触发，用于自动关闭通知
type notificationTimeoutMsgInternal struct{}

// Push 添加一条通知。duration=0 表示需手动关闭，否则 auto-dismiss。
// 返回一个 tea.Cmd 用于触发超时消息。
func (n *NotificationBar) Push(level, message string, duration time.Duration) tea.Cmd {
	id := fmt.Sprintf("notif-%d", notifIDCounter.Add(1))
	notif := notification{
		id:        id,
		level:     level,
		message:   message,
		createdAt: time.Now(),
		duration:  duration,
	}

	n.notifications = append(n.notifications, notif)
	if len(n.notifications) > n.maxVisible {
		n.notifications = n.notifications[len(n.notifications)-n.maxVisible:]
	}

	if duration > 0 {
		return tea.Tick(duration, func(t time.Time) tea.Msg {
			return notificationTimeoutMsgInternal{}
		})
	}
	return nil
}

// HandleTick 处理 timeout tick，返回被关闭通知的 ID（空字符串表示无关闭）。
func (n *NotificationBar) HandleTick() string {
	if len(n.notifications) == 0 {
		return ""
	}
	// 关闭最旧的可超时通知
	for i, notif := range n.notifications {
		if notif.duration > 0 {
			n.notifications = append(n.notifications[:i], n.notifications[i+1:]...)
			return notif.id
		}
	}
	return ""
}

// Dismiss 按 ID 移除通知。
func (n *NotificationBar) Dismiss(id string) {
	for i, notif := range n.notifications {
		if notif.id == id {
			n.notifications = append(n.notifications[:i], n.notifications[i+1:]...)
			break
		}
	}
}

// SetWidth 设置宽度。
func (n *NotificationBar) SetWidth(w int) { n.width = w }

// HasNotifications 是否有关联通知。
func (n *NotificationBar) HasNotifications() bool {
	return len(n.notifications) > 0
}

// Height 返回通知栏高度（无通知时返回 0）。
func (n *NotificationBar) Height() int {
	if len(n.notifications) == 0 {
		return 0
	}
	// RoundedBorder 占 3 行：上边框 + 内容行 + 下边框
	return 3
}

// View 渲染通知栏。返回空字符串表示没有活跃通知。
func (n *NotificationBar) View() string {
	if len(n.notifications) == 0 || n.width == 0 {
		return ""
	}

	notif := n.notifications[len(n.notifications)-1]

	var baseStyle lipgloss.Style
	switch notif.level {
	case "success":
		baseStyle = NotificationSuccessStyle
	case "error":
		baseStyle = NotificationErrorStyle
	case "warning":
		baseStyle = NotificationWarningStyle
	default:
		baseStyle = NotificationInfoStyle
	}

	// 可用内容宽度（减去边框和 padding）
	contentWidth := n.width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	msg := notif.message
	if approximateWidth(msg) > contentWidth {
		msg = truncateLine(msg, contentWidth-3) + "..."
	}

	// 用 lipgloss 的 Border 组件渲染通知框
	notifStyle := baseStyle.Padding(0, 1).Width(n.width - 2)
	return notifStyle.Render(msg)
}
