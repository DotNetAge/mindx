package conv

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
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
	DiffText      string
	DiffAdds      int
	DiffDels      int
	DiffFile      string
}

type Action struct {
	Steps         []ActionStep
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
		if m.StartTime.IsZero() {
			m.StartTime = time.Now()
		}
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
		if m.Completed || len(m.Steps) == 0 {
			return m, nil
		}
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
	if len(m.Steps) == 0 {
		return ""
	}

	var b strings.Builder
	blinkOn := m.BlinkOn && !m.Completed

	for i, step := range m.Steps {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ViewActionStep(step, blinkOn, width))
	}

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

		summary := fmt.Sprintf(i18n.T("action.step.complete"), len(lines))
		if step.EstimatedTok > 0 || step.Duration > 0 {
			var parts []string
			if step.EstimatedTok > 0 {
				parts = append(parts, fmt.Sprintf(i18n.T("action.step.tokens"), formatNumber(step.EstimatedTok)))
			}
			if step.Duration > 0 {
				parts = append(parts, fmt.Sprintf(i18n.T("action.step.duration"), formatDuration(step.Duration)))
			}
			summary += " • " + strings.Join(parts, " • ")
		}
		b.WriteString(fmt.Sprintf("  ⎿ %s\n", style.GrayStyle.Render(summary)))
	}

	if step.DiffText != "" && !step.Collapsed {
		diffWidth := width - 4
		b.WriteString(fmt.Sprintf("  ⎿ %s\n", ViewDiffWithFile(step.DiffText, step.DiffFile, step.DiffAdds, step.DiffDels, diffWidth)))
	}

	return b.String()
}

func formatParams(params map[string]any) string {
	if len(params) == 0 {
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
