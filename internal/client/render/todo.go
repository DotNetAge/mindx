package render

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/style"
	lipgloss "charm.land/lipgloss/v2"
)

type TodoStatus int

const (
	TodoPending TodoStatus = iota
	TodoInProgress
	TodoCompleted
	TodoCancelled
)

type TodoItem struct {
	ID          string
	Text        string
	Status      TodoStatus
	Priority    int
	Assignee    string
	DueDate     string
	Labels      []string
}

type TodoList struct {
	Title string
	Items []TodoItem
	Width int
}

func NewTodoList(title string, width int) *TodoList {
	return &TodoList{
		Title: title,
		Width: width,
	}
}

func (tl *TodoList) Add(item TodoItem) {
	tl.Items = append(tl.Items, item)
}

func (tl *TodoList) Render() string {
	if len(tl.Items) == 0 {
		return ""
	}

	var b strings.Builder

	if tl.Title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(style.ThemeCyan).
			Bold(true).
			MarginBottom(1)
		b.WriteString(titleStyle.Render(tl.Title))
		b.WriteByte('\n')
	}

	for _, item := range tl.Items {
		b.WriteString(tl.renderItem(item))
	}

	return b.String()
}

func (tl *TodoList) renderItem(item TodoItem) string {
	var b strings.Builder

	icon := tl.statusIcon(item.Status)
	iconStyle := tl.statusIconStyle(item.Status)

	textStyle := style.WhiteStyle
	if item.Status == TodoCompleted {
		textStyle = style.GrayStyle
	} else if item.Status == TodoCancelled {
		textStyle = style.DimStyle
	}

	line1 := fmt.Sprintf("%s %s", iconStyle.Render(icon), textStyle.Render(item.Text))
	b.WriteString(line1)
	b.WriteByte('\n')

	metaParts := []string{}
	if item.Assignee != "" {
		metaParts = append(metaParts, fmt.Sprintf("@%s", item.Assignee))
	}
	if item.DueDate != "" {
		metaParts = append(metaParts, fmt.Sprintf("📅 %s", item.DueDate))
	}
	if len(metaParts) > 0 {
		meta := strings.Join(metaParts, "  ")
		b.WriteString(fmt.Sprintf("   %s\n", style.GrayStyle.Render(meta)))
	}

	if len(item.Labels) > 0 {
		labels := make([]string, len(item.Labels))
		for i, label := range item.Labels {
			labels[i] = style.DimStyle.Render(fmt.Sprintf("[%s]", label))
		}
		b.WriteString(fmt.Sprintf("   %s\n", strings.Join(labels, " ")))
	}

	return b.String()
}

func (tl *TodoList) statusIcon(status TodoStatus) string {
	switch status {
	case TodoPending:
		return "○"
	case TodoInProgress:
		return "◐"
	case TodoCompleted:
		return "●"
	case TodoCancelled:
		return "✗"
	default:
		return "?"
	}
}

func (tl *TodoList) statusIconStyle(status TodoStatus) lipgloss.Style {
	switch status {
	case TodoPending:
		return style.GrayStyle
	case TodoInProgress:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107"))
	case TodoCompleted:
		return style.GreenStyle
	case TodoCancelled:
		return style.RedStyle
	default:
		return style.GrayStyle
	}
}

func RenderTodoList(title string, items []TodoItem, width int) string {
	tl := NewTodoList(title, width)
	for _, item := range items {
		tl.Add(item)
	}
	return tl.Render()
}

func RenderProgressBar(current, total int, width int) string {
	if total <= 0 {
		return ""
	}

	ratio := float64(current) / float64(total)
	filled := int(ratio * float64(width))

	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	percentage := int(ratio * 100)

	var barStyle lipgloss.Style
	if percentage >= 80 {
		barStyle = lipgloss.NewStyle().Foreground(style.ThemeGreen)
	} else if percentage >= 50 {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107"))
	} else {
		barStyle = lipgloss.NewStyle().Foreground(style.ThemeRed)
	}

	return barStyle.Render(fmt.Sprintf("%s %d%%", bar, percentage))
}
