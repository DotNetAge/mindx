package client

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---- 图标颜色定义 ----
var (
	dotWhite              = lipgloss.Color("#E0E0E0")
	dotBlue               = lipgloss.Color("#4FC3F7")
	dotGreen              = lipgloss.Color("#4CAF50")
	dotRed                = lipgloss.Color("#CF6679")
	userQuestionDot       = lipgloss.NewStyle().Foreground(dotBlue).SetString("●")
	thinkingDot           = lipgloss.NewStyle().Foreground(dotBlue).SetString("●")
	thinkingDotBlinkWhite = lipgloss.NewStyle().Foreground(dotWhite).SetString("●")
	thinkingDotBlinkBlue  = lipgloss.NewStyle().Foreground(dotBlue).SetString("●")
	toolDotDoing          = lipgloss.NewStyle().Foreground(dotGreen).SetString("●")
	toolDotDone           = lipgloss.NewStyle().Foreground(dotGreen).SetString("●")
	toolDotFailed         = lipgloss.NewStyle().Foreground(dotRed).SetString("●")
	toolDotBlinkWhite     = lipgloss.NewStyle().Foreground(dotWhite).SetString("●")
	toolDotBlinkGreen     = lipgloss.NewStyle().Foreground(dotGreen).SetString("●")
	answerDot             = lipgloss.NewStyle().Foreground(dotWhite).SetString("●")
)

// ---- AgentAnswer ----

// AgentAnswer 对应服务器的一个会话单元。
// 显示顺序: UserQuestion → 思考区 → 工具调用区 → 最终回答区
type AgentAnswer struct {
	SessionID  string
	AgentName  string
	isThinking bool

	userQuestion string

	// 多轮思考支持
	thinkingRounds  []string        // 每轮思考完成后的思想流内容
	currentThink    strings.Builder // 当前轮 streaming 累积（流式直通）
	hasCurrentThink bool            // 当前轮是否已有有效内容

	// 闪烁动画
	blinkOn bool // TickMsg 到达时交替

	results   []answerResult
	actionLog []actionStep

	CreatedAt time.Time
	UpdatedAt time.Time
	Duration  time.Duration

	markdownFn func(string) string
}

type answerResult struct {
	Role    string
	Content string
}

// actionStep 表示一个工具执行步骤。
type actionStep struct {
	ToolName        string
	Status          actionStatus
	EstimatedTokens int
	Params          map[string]any
	ProgressText    string
	ResultText      string
	collapsed       bool // 默认折叠工具输出
}

type actionStatus int

const (
	actionExecuting actionStatus = iota
	actionDone
	actionFailed
)

// NewAgentAnswer 创建一个新的 AgentAnswer。
func NewAgentAnswer(sessionID, agentName string) *AgentAnswer {
	now := time.Now()
	return &AgentAnswer{
		SessionID: sessionID,
		AgentName: agentName,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// StartThinking 激活思考状态。
// 返回初始 Tick 命令以启动闪烁动画。
func (a *AgentAnswer) StartThinking() tea.Cmd {
	a.isThinking = true
	// 新的一轮思考，重置当前缓冲区
	a.currentThink.Reset()
	a.hasCurrentThink = false
	return func() tea.Msg {
		return time.Now()
	}
}

// AppendThinking 追加思想流内容（原始 streaming chunk）。
// 模型输出什么就直接显示什么，不过滤、不解析！
func (a *AgentAnswer) AppendThinking(content string) {
	if content == "" {
		return
	}

	a.currentThink.WriteString(content)
	a.hasCurrentThink = true
}

// Update 更新 AgentAnswer 的内部状态（闪烁动画）。
// TickMsg 驱动 blinkOn 交替。
func (a *AgentAnswer) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case time.Time:
		return a.Tick()
	}
	return nil
}

// Tick 驱动一次闪烁动画状态切换。
// 返回下一个 Tick 命令（如果仍有需要动画的状态）。
func (a *AgentAnswer) Tick() tea.Cmd {
	a.blinkOn = !a.blinkOn
	if a.needsTick() {
		return func() tea.Msg {
			return time.Now()
		}
	}
	return nil
}

// needsTick 检查是否需要继续接收 TickMsg（有正在闪烁的状态）。
func (a *AgentAnswer) needsTick() bool {
	if a.isThinking {
		return true
	}
	for _, s := range a.actionLog {
		if s.Status == actionExecuting {
			return true
		}
	}
	return false
}

// SetThinkingDone 标记当前轮思考完成。
// 将当前累积的流式内容保存到历史轮次，重置状态。
// 注意：不从 content 参数提取任何内容！
// 思想流 100% 来自 ThinkingDelta 的实时流式输出。
func (a *AgentAnswer) SetThinkingDone(content string) {
	if a.currentThink.Len() > 0 {
		a.thinkingRounds = append(a.thinkingRounds, a.currentThink.String())
	}
	a.currentThink.Reset()
	a.hasCurrentThink = false
	a.isThinking = false
}

// MarkUpdated 更新时间戳和耗时。
func (a *AgentAnswer) MarkUpdated() {
	a.UpdatedAt = time.Now()
	if !a.CreatedAt.IsZero() {
		a.Duration = a.UpdatedAt.Sub(a.CreatedAt)
	}
}

// AppendResult 追加一条结果。第一个 result 被视为用户问题。
func (a *AgentAnswer) AppendResult(content string) {
	if a.userQuestion == "" {
		a.userQuestion = content
		return
	}
	a.results = append(a.results, answerResult{
		Role:    "result",
		Content: content,
	})
}

// AppendError 追加一条错误。
func (a *AgentAnswer) AppendError(content string) {
	a.results = append(a.results, answerResult{
		Role:    "error",
		Content: content,
	})
}

// AppendTyped 追加一条结构化内容（表格/待办/选项等）。
func (a *AgentAnswer) AppendTyped(content string) {
	a.results = append(a.results, answerResult{
		Role:    "typed",
		Content: content,
	})
}

// AppendAction 追加一个工具执行步骤（状态=执行中）。
func (a *AgentAnswer) AppendAction(toolName string, estimatedTokens int, params map[string]any) {
	a.actionLog = append(a.actionLog, actionStep{
		ToolName:        toolName,
		Status:          actionExecuting,
		EstimatedTokens: estimatedTokens,
		Params:          params,
		collapsed:       true, // 默认折叠
	})
}

// MarkActionDone 将最近一个执行中的步骤标记为完成。
func (a *AgentAnswer) MarkActionDone(resultText string) {
	for i := len(a.actionLog) - 1; i >= 0; i-- {
		if a.actionLog[i].Status == actionExecuting {
			a.actionLog[i].Status = actionDone
			a.actionLog[i].ResultText = resultText
			break
		}
	}
}

// MarkActionFailed 将最近一个执行中的步骤标记为失败。
func (a *AgentAnswer) MarkActionFailed(errorText string) {
	for i := len(a.actionLog) - 1; i >= 0; i-- {
		if a.actionLog[i].Status == actionExecuting {
			a.actionLog[i].Status = actionFailed
			a.actionLog[i].ResultText = errorText
			break
		}
	}
}

// ToggleActionCollapse 切换指定动作步骤的折叠状态。
func (a *AgentAnswer) ToggleActionCollapse(index int) bool {
	if index < 0 || index >= len(a.actionLog) {
		return false
	}
	a.actionLog[index].collapsed = !a.actionLog[index].collapsed
	return a.actionLog[index].collapsed
}

// SetActionProgress 更新当前执行中步骤的进度文本。
func (a *AgentAnswer) SetActionProgress(text string) {
	for i := len(a.actionLog) - 1; i >= 0; i-- {
		if a.actionLog[i].Status == actionExecuting {
			a.actionLog[i].ProgressText = text
			break
		}
	}
}

// HasToolResult 检查内容是否与已有工具步骤的执行结果重复。
func (a *AgentAnswer) HasToolResult(content string) bool {
	for _, s := range a.actionLog {
		if s.ResultText != "" && s.ResultText == content {
			return true
		}
	}
	return false
}

// ---- View 渲染 ----

// View 渲染整个 AgentAnswer。
// 显示顺序: UserQuestion → 思考区 → 工具调用区 → 最终回答区
func (a *AgentAnswer) View() string {
	var b strings.Builder

	// 1. UserQuestion
	if a.userQuestion != "" {
		content := a.userQuestion
		if a.markdownFn != nil {
			content = strings.TrimSpace(a.markdownFn(content))
		}
		b.WriteString(userQuestionDot.String())
		b.WriteString(" ")
		b.WriteString(UserQuestionStyle.Render(content))
		b.WriteString("\n")
	}

	// 2. 思考区
	hasThinking := a.isThinking || len(a.thinkingRounds) > 0 || a.currentThink.Len() > 0
	if hasThinking {
		// b.WriteString("---\u601d\u8003\u533a---\n")
		// 已完成的思考轮次
		for _, round := range a.thinkingRounds {
			if round == "" {
				continue
			}
			if a.markdownFn != nil {
				round = strings.TrimSpace(a.markdownFn(round))
			}
			b.WriteString(thinkingDot.String())
			b.WriteString(" ")
			b.WriteString(ThinkingStyle.Render(round))
			b.WriteString("\n")
		}
		// 当前轮 streaming（思考中）
		if a.isThinking {
			currThink := a.currentThink.String()
			if currThink == "" {
				// 等待中：闪烁 ● 深度思考中
				if a.blinkOn {
					b.WriteString(thinkingDotBlinkWhite.String())
				} else {
					b.WriteString(thinkingDotBlinkBlue.String())
				}
				b.WriteString(" ")
				b.WriteString(ThinkingStyle.Render("\u6df1\u5ea6\u601d\u8003\u4e2d"))
			} else {
				// 有正在累积的思想流（完整显示，不截断）— 图标仍闪烁表示思考进行中
				if a.markdownFn != nil {
					currThink = strings.TrimSpace(a.markdownFn(currThink))
				}
				if a.blinkOn {
					b.WriteString(thinkingDotBlinkWhite.String())
				} else {
					b.WriteString(thinkingDotBlinkBlue.String())
				}
				b.WriteString(" ")
				b.WriteString(ThinkingStyle.Render(currThink))
			}
			b.WriteString("\n")
		}
	}

	// 3. 工具调用区
	if len(a.actionLog) > 0 {
		// b.WriteString("---\u5de5\u5177\u8c03\u7528---\n")
		for _, s := range a.actionLog {
			b.WriteString(s.View(a.blinkOn))
			b.WriteString("\n")
			// 工具结果（默认折叠，显示缩进格式）
			if s.ResultText != "" && !s.collapsed {
				b.WriteString(formatActionResult(s.ResultText, s.Status, s.collapsed))
			} else if s.ResultText != "" && s.collapsed {
				b.WriteString(formatActionCollapsedHint(s))
			}
		}
	}

	// 4. 最终回答区
	if len(a.results) > 0 {
		for _, r := range a.results {
			content := r.Content
			switch r.Role {
			case "error":
				b.WriteString(toolDotFailed.String())
				b.WriteString(" ")
				b.WriteString(ErrorStyle.Render(content))
			case "result":
				b.WriteString(answerDot.String())
				b.WriteString(" ")
				if a.markdownFn != nil {
					content = strings.TrimSpace(a.markdownFn(content))
				}
				b.WriteString(content)
			default:
				b.WriteString(answerDot.String())
				b.WriteString(" ")
				b.WriteString(content)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ---- 工具步骤渲染 ----

// formatParams 将工具参数格式化为可读的短字符串。
func formatParams(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	var parts []string
	for k, v := range params {
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > 60 {
			valStr = valStr[:60] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, valStr))
	}
	return strings.Join(parts, ", ")
}

func (s actionStep) View(blinkOn bool) string {
	// ● 图标
	var dot string
	switch s.Status {
	case actionExecuting:
		if blinkOn {
			dot = toolDotBlinkWhite.String()
		} else {
			dot = toolDotBlinkGreen.String()
		}
	case actionDone:
		dot = toolDotDone.String()
	case actionFailed:
		dot = toolDotFailed.String()
	}

	var b strings.Builder
	b.WriteString(dot)
	b.WriteString(" ")

	if s.ToolName != "" {
		b.WriteString(ActionToolStyle.Render(s.ToolName))
	} else {
		b.WriteString(ActionToolStyle.Render("Unknown"))
	}

	switch s.Status {
	case actionExecuting:
		if paramStr := formatParams(s.Params); paramStr != "" {
			b.WriteString("(")
			b.WriteString(ActionProgressStyle.Render(paramStr))
			b.WriteString(")")
		}
		if s.ProgressText != "" {
			b.WriteString(" | ")
			b.WriteString(ActionProgressStyle.Render(s.ProgressText))
		}
		if s.EstimatedTokens > 0 {
			b.WriteString(" | \u9884\u8ba1\u6d88\u8017 ")
			b.WriteString(formatTokens(s.EstimatedTokens))
			b.WriteString(" Tokens")
		}
	case actionDone:
		if s.EstimatedTokens > 0 {
			b.WriteString(" | \u9884\u8ba1\u6d88\u8017 ")
			b.WriteString(formatTokens(s.EstimatedTokens))
			b.WriteString(" Tokens")
		}
		if s.ResultText != "" {
			b.WriteString(" | ")
			if s.collapsed {
				b.WriteString(ActionResultStyle.Render("[+] Show output (ctrl+o to expand)"))
			} else {
				preview := firstLine(s.ResultText, 100)
				b.WriteString(ActionResultStyle.Render(preview))
				b.WriteString("  ")
				b.WriteString(ActionResultStyle.Render("[\u2212] Hide output"))
			}
		}
	case actionFailed:
		b.WriteString(" | ")
		b.WriteString(ActionResultStyle.Render(s.ResultText))
	}

	return b.String()
}

// formatActionResult 格式化工具执行结果（⎿ 缩进，最多三行，灰色小字）。
func formatActionResult(text string, status actionStatus, collapsed bool) string {
	if text == "" {
		return ""
	}
	if collapsed {
		return ""
	}

	lines := strings.Split(text, "\n")
	const maxDisplayLines = 3

	var b strings.Builder
	for i, line := range lines {
		if i >= maxDisplayLines {
			remaining := len(lines) - i
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("  \u2026 +%d lines (ctrl+o to expand)\n", remaining))
			}
			break
		}
		if i == 0 {
			b.WriteString("  \u23bf ")
		} else {
			b.WriteString("    ")
		}
		b.WriteString(ActionResultStyle.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

// formatActionCollapsedHint 格式化折叠状态的工具结果提示。
func formatActionCollapsedHint(s actionStep) string {
	var b strings.Builder
	b.WriteString("  \u23bf ")
	switch s.Status {
	case actionDone:
		b.WriteString(ActionResultStyle.Render(fmt.Sprintf("\u5b8c\u6210 (%d lines)", countLines(s.ResultText))))
	case actionFailed:
		b.WriteString(ActionResultStyle.Render(fmt.Sprintf("\u5931\u8d25: %s", firstLine(s.ResultText, 50))))
	default:
		b.WriteString(ActionResultStyle.Render("\u6267\u884c\u4e2d..."))
	}
	b.WriteString("\n")
	return b.String()
}

// countLines 计算字符串的行数。
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// firstLine 返回字符串的第一行。
func firstLine(s string, maxLen int) string {
	idx := strings.Index(s, "\n")
	if idx >= 0 {
		s = s[:idx]
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// ---- 旧接口兼容（供其他文件导入） ----

// cleanWhitespace 清理多余空白字符
func cleanWhitespace(s string) string {
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}
