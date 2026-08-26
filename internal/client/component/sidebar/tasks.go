package sidebar

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// TaskStatus 与 goharness/tools.TaskStatus 对齐（跨仓库不引依赖，按字符串同步）。
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskCancelled  TaskStatus = "cancelled"
)

// Task 是侧栏任务面板的展示条目，由 TUI 从工具执行结果中提取。
type Task struct {
	ID       string
	Subject  string
	Status   TaskStatus
	Progress string // 可选的进度说明（如 "3/5"）
}

// taskGlyph 状态图标与对应配色：○ 待办 / ● 进行中(青) / ✓ 完成(绿) / ✕ 取消(灰)。
func taskGlyph(st TaskStatus) (string, lipgloss.Style) {
	switch st {
	case TaskInProgress:
		return "●", style.CyanStyle
	case TaskCompleted:
		return "✓", style.GreenStyle
	case TaskCancelled:
		return "✕", style.GrayStyle
	default:
		return "○", style.DimStyle
	}
}

// renderTasks 渲染任务区块；无任务时返回空串。
// 进行中的任务显示 active_form 语义的 Subject，完成项置灰。
func renderTasks(tasks []Task, doneCount int, totalCount int, width int) string {
	if len(tasks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(boldLabel.Render(fmt.Sprintf(i18n.T("client.ui.sidebar.tasks.title"), doneCount, totalCount)))
	b.WriteByte('\n')

	for _, t := range tasks {
		glyph, color := taskGlyph(t.Status)
		label := t.Subject
		switch t.Status {
		case TaskInProgress:
			label = style.CyanStyle.Render(label)
		case TaskCompleted, TaskCancelled:
			label = style.GrayStyle.Render(label)
		}
		line := fmt.Sprintf("  %s %s", color.Render(glyph), label)
		if t.Progress != "" {
			line += style.GrayStyle.Render(" (" + t.Progress + ")")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}
