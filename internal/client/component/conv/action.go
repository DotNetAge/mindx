package conv

import (
	"fmt"
	"sort"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// ActionStep 是对话流中一次工具调用的展示单元，由 JSON-RPC 工具事件驱动：
//
//   - tool_use_delta：流式拼接参数预览；
//   - tool_exec_start：显示「⏺ 工具名(参数…) | 计时」，计时随 Tick 实时跳动；
//   - tool_exec_end：补齐「耗时 | Token」（Token 为服务器计算的真实消耗 =
//     输入 + 输出 - 缓存），并在下方渲染输出结果；执行失败时结果区切换为
//     错误组件（红框）呈现。
type ActionStep struct {
	ToolCallID    string
	ArgIndex      int // tool_use_delta 流式序号（与服务端 index 对应）
	ToolName      string
	Status        ActionStepStatus
	Params        map[string]any
	StreamingArgs string // 参数流式预览（start 后被 Params 取代）

	StartTime time.Time // 执行开始时刻（计时来源）
	Duration  time.Duration
	Tokens    int // 真实 token 消耗（in + out - cached）

	Result    string // 成功为输出结果；失败为错误信息
	DiffText  string
	DiffAdds  int
	DiffDels  int
	DiffFile  string
	Collapsed bool
}

// ResultFormatter 按工具名定制结果区的渲染。
// 不同工具返回的内容格式各异（JSON / diff / 表格 / 长文本…），
// 通过 RegisterResultFormatter 注册即可替换默认渲染，无需改动组件本身。
type ResultFormatter func(step ActionStep, result string, width int) string

var resultFormatters = map[string]ResultFormatter{}

// RegisterResultFormatter 注册某工具的结果渲染器（建议在包 init 中调用）。
func RegisterResultFormatter(toolName string, f ResultFormatter) {
	resultFormatters[toolName] = f
}

// stepElapsed 返回步骤当前耗时：执行中按实时计算，结束后取固定值。
func stepElapsed(step ActionStep) time.Duration {
	if step.Status == ActionStepExecuting && !step.StartTime.IsZero() {
		return time.Since(step.StartTime).Truncate(100 * time.Millisecond)
	}
	return step.Duration
}

// ViewActionStep 渲染单个工具调用条目。s 提供闪烁相位等流级状态。
func ViewActionStep(s Stream, step ActionStep, width int) string {
	if step.ToolName == "" && step.StreamingArgs == "" {
		return ""
	}

	var b strings.Builder
	blink := s.BlinkOn && s.Status != StatusDone && s.Status != StatusError

	var icon string
	switch step.Status {
	case ActionStepExecuting:
		icon = ViewBlink(Blink{Symbol: "⏺", BlinkOn: blink}, style.GreenStyle)
	case ActionStepDone:
		icon = style.WhiteStyle.Render("⏺")
	case ActionStepFailed:
		icon = style.RedStyle.Render("⏺")
	}
	b.WriteString(icon)
	b.WriteByte(' ')

	nameStyle := style.BoldWhite
	if step.Status == ActionStepFailed {
		nameStyle = style.RedStyle.Bold(true)
	}
	b.WriteString(nameStyle.Render(step.ToolName))

	// (参数…)
	if paramStr := formatParams(step.Params); paramStr != "" {
		b.WriteString(fmt.Sprintf("(%s)", paramStr))
	} else if preview := previewStreamingArgs(step); preview != "" {
		b.WriteString(style.DimStyle.Render("(" + preview + "▌)"))
	}

	// | 计时 | Token
	var meta []string
	if d := stepElapsed(step); d > 0 {
		meta = append(meta, formatDuration(d))
	}
	if step.Status != ActionStepExecuting && step.Tokens > 0 {
		meta = append(meta, fmt.Sprintf(i18n.T("action.step.tokens"), formatNumber(step.Tokens)))
	}
	if len(meta) > 0 {
		b.WriteString(style.GrayStyle.Render(" | " + strings.Join(meta, " | ")))
	}
	b.WriteByte('\n')

	// 结果区：失败 → 错误组件；成功 → 结果渲染（支持格式化扩展）。
	if step.Status == ActionStepFailed {
		if step.Result != "" {
			b.WriteString(viewStepError(step.Result, width))
		}
	} else if step.Result != "" {
		b.WriteString(viewStepResult(step, width))
	}

	if step.DiffText != "" && !step.Collapsed {
		diffWidth := width - 6
		if diffWidth < 20 {
			diffWidth = 20
		}
		b.WriteString("  ⎿ ")
		b.WriteString(ViewDiffWithFile(step.DiffText, step.DiffFile, step.DiffAdds, step.DiffDels, diffWidth))
		b.WriteByte('\n')
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// viewStepResult 渲染成功结果：优先使用注册的 ResultFormatter。
// formatter 未注册或返回空（解析失败、格式不识别）时兜底渲染原始文本——
// 结果内容不允许被静默吞掉，保持原始 JSON 可见是排查问题的最后手段。
func viewStepResult(step ActionStep, width int) string {
	var rendered string
	if f, ok := resultFormatters[step.ToolName]; ok {
		rendered = f(step, step.Result, width)
	}
	if rendered == "" {
		rendered = fallbackResultPreview(step.Result)
	}
	return prefixLines(rendered, "  ⎿ ", true)
}

// fallbackResultPreview 原样预览未经格式化的结果，超长截断。
func fallbackResultPreview(result string) string {
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	const maxLines = 20
	shown := lines
	if len(shown) > maxLines {
		shown = shown[:maxLines]
	}
	var b strings.Builder
	for _, line := range shown {
		b.WriteString(style.DimStyle.Render(line))
		b.WriteByte('\n')
	}
	if len(lines) > maxLines {
		fmt.Fprintf(&b, "… +%d lines", len(lines)-maxLines)
		return strings.TrimSuffix(b.String(), "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// viewStepError 以错误组件（红框）渲染失败的执行结果。
func viewStepError(errMsg string, width int) string {
	boxWidth := width - 8
	if boxWidth < 20 {
		boxWidth = 20
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemeRed).
		Padding(0, 1).
		Width(boxWidth)

	indented := prefixLines(errMsg, "  ", false)
	return "  ⎿ " + border.Render(style.RedStyle.Render(indented)) + "\n"
}

// prefixLines 为多行文本添加前缀；keepEmpty 控制是否保留空行。
func prefixLines(text, prefix string, keepEmpty bool) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line == "" && !keepEmpty {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// previewStreamingArgs 截断显示流式参数片段。
func previewStreamingArgs(step ActionStep) string {
	args := step.StreamingArgs
	if args == "" || step.Status != ActionStepExecuting {
		return ""
	}
	args = strings.TrimSpace(args)
	if len(args) > 60 {
		args = args[:57] + "..."
	}
	return args
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
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}
