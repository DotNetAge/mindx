package conv

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type ActionStep struct {
	ToolName      string
	Status        ActionStepStatus
	EstimatedTok  int
	Duration      time.Duration
	Params        map[string]any
	ProgressText  string
	ResultText    string
	StreamingArgs string
	Collapsed     bool
	DiffText      string // unified diff output for file-modifying tools
	DiffAdds      int    // lines added
	DiffDels      int    // lines removed
	DiffFile      string // file path that was changed
}

type ActionInfo struct {
	ToolCount            int
	ToolNames            []string
	TotalPredictedTokens int
}

type Action struct {
	Steps         []ActionStep
	CurrentInfo   *ActionInfo
	Completed     bool
	SuccessCount  int
	FailedCount   int
	TotalTokens   int
	TotalDuration time.Duration
	Elapsed       time.Duration
	StartTime     time.Time
	BlinkOn       bool
}

func UpdateAction(m Action, e tea.Msg) (Action, tea.Cmd) {
	switch e := e.(type) {
	case msg.ActionStartMsg:
		if m.Completed {
			return m, nil
		}
		m.CurrentInfo = &ActionInfo{
			ToolCount:            e.ToolCount,
			ToolNames:            e.ToolNames,
			TotalPredictedTokens: e.EstimatedTok,
		}
		m.StartTime = time.Now()
		m.Elapsed = 0
		return m, nil

	case msg.ToolExecStartMsg:
		if m.Completed {
			return m, nil
		}
		step := ActionStep{
			ToolName:     e.ToolName,
			Status:       ActionStepExecuting,
			EstimatedTok: e.EstimatedTok,
			Params:       e.Params,
			Collapsed:    true,
		}
		m.Steps = append(m.Steps, step)
		return m, nil

	case msg.ToolUseDeltaMsg:
		if m.Completed {
			return m, nil
		}
		idx := e.Index
		if idx < 0 {
			idx = len(m.Steps)
		}
		for idx < len(m.Steps) {
			step := &m.Steps[idx]
			if step.ToolName == "" || step.ToolName == e.Name {
				step.ToolName = e.Name
				step.StreamingArgs += e.Arguments
				if step.Status != ActionStepExecuting {
					step.Status = ActionStepExecuting
				}
				break
			}
			idx++
		}
		if idx >= len(m.Steps) {
			m.Steps = append(m.Steps, ActionStep{
				ToolName:      e.Name,
				Status:        ActionStepExecuting,
				StreamingArgs: e.Arguments,
				Collapsed:     true,
			})
		}
		return m, nil

	case msg.ToolExecEndMsg:
		if m.Completed || len(m.Steps) == 0 { return m, nil }
		for i := len(m.Steps) - 1; i >= 0; i-- {
			step := &m.Steps[i]
			if step.ToolName == e.ToolName && step.Status == ActionStepExecuting {
				if e.Success {
					step.Status = ActionStepDone
					step.ResultText = e.Result
					step.Duration = e.Duration
				} else {
					step.Status = ActionStepFailed
					step.ResultText = e.Error
					step.Duration = e.Duration
				}
				step.Collapsed = false
				// Attach diff if the tool produced file changes
				if e.DiffText != "" {
					step.DiffText = e.DiffText
					step.DiffAdds = e.DiffAdds
					step.DiffDels = e.DiffDels
					step.DiffFile = e.DiffFile
				}
				break
			}
		}
		return m, nil

	case msg.ActionEndMsg:
		if m.Completed {
			return m, nil
		}
		m.Completed = true
		m.SuccessCount = e.SuccessCount
		m.FailedCount = e.FailedCount
		return m, nil

	case msg.ExecutionSummaryMsg:
		m.TotalTokens = e.TokensUsed.TotalTokens
		m.TotalDuration = e.Duration
		return m, nil

	case msg.CollapseToggleMsg:
		if e.ActionIndex < 0 {
			for i := range m.Steps {
				m.Steps[i].Collapsed = !m.Steps[i].Collapsed
			}
		} else if e.ActionIndex < len(m.Steps) {
			m.Steps[e.ActionIndex].Collapsed = !m.Steps[e.ActionIndex].Collapsed
		}
		return m, nil

	case msg.ActionProgressMsg:
		return m, nil

	case msg.TickMsg:
		m.BlinkOn = !m.BlinkOn
		if !m.Completed && !m.StartTime.IsZero() {
			m.Elapsed = time.Since(m.StartTime).Truncate(100 * time.Millisecond)
		}
		return m, nil
	}

	return m, nil
}

func ViewAction(m Action, width int) string {
	if m.CurrentInfo == nil && len(m.Steps) == 0 {
		return ""
	}

	var b strings.Builder
	blinkOn := m.BlinkOn && !m.Completed

	if m.CurrentInfo != nil {
		b.WriteString(ViewActionHeader(*m.CurrentInfo, blinkOn, m.Elapsed, m.Completed, m.SuccessCount, m.FailedCount, m.TotalDuration))
	}

	for i, step := range m.Steps {
		if i > 0 || m.CurrentInfo != nil {
			b.WriteByte('\n')
		}
		b.WriteString(ViewActionStep(step, blinkOn, width))
	}

	return b.String()
}

func ViewActionHeader(info ActionInfo, blinkOn bool, elapsed time.Duration, completed bool, successCount, failedCount int, totalDuration time.Duration) string {
	var b strings.Builder
	icon := ViewBlink(Blink{Symbol: "⏺", BlinkOn: blinkOn}, style.GrayStyle)
	if completed {
		icon = style.WhiteStyle.Render("⏺")
	}
	b.WriteString(icon)
	b.WriteString(" ")
	b.WriteString(style.WhiteStyle.Render(fmt.Sprintf("执行操作: %d 个工具", info.ToolCount)))
	if info.TotalPredictedTokens > 0 {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(fmt.Sprintf("预计消耗 %s Tokens", formatNumber(info.TotalPredictedTokens)))))
	}
	if completed {
		b.WriteString(fmt.Sprintf(" | %s", style.GreenStyle.Render(fmt.Sprintf("%d 成功, %d 失败", successCount, failedCount))))
		if totalDuration > 0 {
			b.WriteString(fmt.Sprintf(" | %s", style.WhiteStyle.Render(formatDuration(totalDuration))))
		}
	} else if elapsed > 0 {
		b.WriteString(fmt.Sprintf(" | %s", style.WhiteStyle.Render(formatDuration(elapsed))))
	}
	b.WriteByte('\n')
	return b.String()
}

func ViewActionStep(step ActionStep, blinkOn bool, width int) string {
	var b strings.Builder
	var icon string
	switch step.Status {
	case ActionStepExecuting:
		icon = ViewBlink(Blink{Symbol: "⏺", BlinkOn: blinkOn}, style.GreenStyle)
	case ActionStepDone:
		icon = style.WhiteStyle.Render("⏺")
	case ActionStepFailed:
		icon = ViewBlink(Blink{Symbol: "⏺", BlinkOn: blinkOn}, style.RedStyle)
	}

	b.WriteString(icon)
	b.WriteString(" ")

	if step.Status == ActionStepFailed {
		b.WriteString(style.RedStyle.Bold(true).Render(step.ToolName))
	} else {
		b.WriteString(style.BoldWhite.Render(step.ToolName))
	}

	paramStr := formatParams(step.Params)
	if paramStr != "" {
		b.WriteString(fmt.Sprintf("(%s)", paramStr))
	}
	if step.StreamingArgs != "" && step.Status == ActionStepExecuting {
		argsPreview := step.StreamingArgs
		if len(argsPreview) > 80 {
			argsPreview = argsPreview[:77] + "..."
		}
		b.WriteString(fmt.Sprintf(" | %s", style.DimStyle.Render(argsPreview+style.DimStyle.Render("▌"))))
	} else if step.ProgressText != "" {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(step.ProgressText)))
	}
	b.WriteByte('\n')

	if step.ResultText != "" {
		lines := strings.Split(step.ResultText, "\n")

		// Show result content (max 3 lines) when meaningful and not collapsed
		if !step.Collapsed && len(step.ResultText) > 10 {
			lineStyle := style.DimStyle
			if step.Status == ActionStepFailed {
				lineStyle = style.RedStyle
			}
			for i, line := range lines {
				if i >= 3 {
					b.WriteString(fmt.Sprintf("  ⎿ … +%d lines (ctrl+o toggle)\n", len(lines)-i))
					break
				}
				b.WriteString(fmt.Sprintf("  ⎿ %s\n", lineStyle.Render(line)))
			}
		}

		// Completion summary (always shown below result content)
		summary := fmt.Sprintf("完成 (%d lines)", len(lines))
		if step.EstimatedTok > 0 || step.Duration > 0 {
			var parts []string
			if step.EstimatedTok > 0 {
				parts = append(parts, fmt.Sprintf("Token 消耗 %s", formatNumber(step.EstimatedTok)))
			}
			if step.Duration > 0 {
				parts = append(parts, fmt.Sprintf("用时 %s", formatDuration(step.Duration)))
			}
			summary += " • " + strings.Join(parts, " • ")
		}
		b.WriteString(fmt.Sprintf("  ⎿ %s\n", style.GrayStyle.Render(summary)))
	}

	// Diff display for file-modifying tools
	if step.DiffText != "" && !step.Collapsed {
		diffWidth := width - 4 // account for indentation
		b.WriteString(fmt.Sprintf("  ⎿ %s\n", ViewDiffWithFile(step.DiffText, step.DiffFile, step.DiffAdds, step.DiffDels, diffWidth)))
	}

	return b.String()
}

func formatParams(params map[string]any) string {
	if params == nil || len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%v", params[k]))
	}
	result := strings.Join(parts, " ")
	if len(result) > 60 {
		return result[:57] + "..."
	}
	return result
}

func formatNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		if n%1_000 == 0 {
			return fmt.Sprintf("%dK", n/1_000)
		}
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}
