package client

import (
	"encoding/json"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// AgentAnswer 对应服务器的一个会话单元。
// 内部结构（从上到下）: UserQuestion → Thinks → Results → ActionLog
// 所有内容均为 append-only。
type AgentAnswer struct {
	SessionID   string
	AgentName   string
	isThinking  bool

	userQuestion    string
	thinking        strings.Builder
	thinkingDone    string
	thinkingSpinner spinner.Model
	results         []answerResult
	actionLog       []actionStep

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
	ProgressText    string
	ResultText      string
	spinner         spinner.Model
}

type actionStatus int

const (
	actionExecuting actionStatus = iota
	actionDone
	actionFailed
)

// NewAgentAnswer 创建一个新的 AgentAnswer。
func NewAgentAnswer(sessionID, agentName string) *AgentAnswer {
	sp := spinner.New()
	return &AgentAnswer{
		SessionID:       sessionID,
		AgentName:       agentName,
		thinkingSpinner: sp,
	}
}

// StartThinking 激活思考状态，立即显示 Loading 动画。
// 应在用户发送问题后立即调用，无需等待服务器返回 thinking 事件。
func (a *AgentAnswer) StartThinking() {
	a.isThinking = true
}

// AppendThinking 追加思考内容。同一 event 流中的多个 delta 会自动合并到同一块。
// 会自动过滤 JSON 内容，只保留可读文本。
func (a *AgentAnswer) AppendThinking(content string) {
	cleaned := cleanThinkingContent(content)
	if cleaned != "" {
		a.thinking.WriteString(cleaned)
	}
}

// Update 更新 AgentAnswer 的内部状态（如 spinner 动画）。
// 需要在每次收到消息时调用，以驱动 spinner 动画。
func (a *AgentAnswer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	a.thinkingSpinner, cmd = a.thinkingSpinner.Update(msg)
	return cmd
}

// cleanThinkingContent 清理思考内容，只保留reasoning字段
func cleanThinkingContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	var jsonObj map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonObj); err == nil {
		if reasoning, ok := jsonObj["reasoning"].(string); ok && reasoning != "" {
			return cleanWhitespace(reasoning)
		}
		if reasoning, ok := jsonObj["Reasoning"].(string); ok && reasoning != "" {
			return cleanWhitespace(reasoning)
		}
		return ""
	}

	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return ""
	}

	return cleanWhitespace(content)
}

// cleanWhitespace 清理多余空白字符
func cleanWhitespace(s string) string {
	// 将连续的空白字符替换为单个空格
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

// SetThinkingDone 设置思考完成的格式化内容（替换原始流式内容）。
func (a *AgentAnswer) SetThinkingDone(content string) {
	a.thinkingDone = content
	a.isThinking = false
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
func (a *AgentAnswer) AppendAction(toolName string, estimatedTokens int) {
	sp := spinner.New()
	a.actionLog = append(a.actionLog, actionStep{
		ToolName:        toolName,
		Status:          actionExecuting,
		EstimatedTokens: estimatedTokens,
		spinner:         sp,
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

// SetActionProgress 更新当前执行中步骤的进度文本。
func (a *AgentAnswer) SetActionProgress(text string) {
	for i := len(a.actionLog) - 1; i >= 0; i-- {
		if a.actionLog[i].Status == actionExecuting {
			a.actionLog[i].ProgressText = text
			break
		}
	}
}

// View 渲染整个 AgentAnswer 的字符串。
func (a *AgentAnswer) View() string {
	var b strings.Builder

	// UserQuestion - 用户问题显示在最上方
	if a.userQuestion != "" {
		content := a.userQuestion
		if a.markdownFn != nil {
			content = a.markdownFn(content)
		}
		b.WriteString(userQuestionStyle.Render(content))
		b.WriteString("\n")
	}

	// Thinks - 思考内容显示在用户问题之后
	if a.thinkingDone != "" {
		formatted := a.thinkingDone
		if a.markdownFn != nil {
			formatted = a.markdownFn(formatted)
		}
		b.WriteString(thinkingStyle.Render(formatted))
		b.WriteString("\n")
	} else if a.isThinking {
		var thinkBuilder strings.Builder
		thinkBuilder.WriteString(a.thinkingSpinner.View())
		thinkBuilder.WriteString(" ")
		thinkBuilder.WriteString(thinkingStyle.Render("深度思考中"))
		thinking := a.thinking.String()
		if thinking != "" {
			thinkBuilder.WriteString("\n")
			if a.markdownFn != nil {
				thinking = a.markdownFn(thinking)
			}
			thinkBuilder.WriteString(thinkingStyle.Render(thinking))
		}
		b.WriteString(thinkBuilder.String())
		b.WriteString("\n")
	}

	// Results - 模型答案
	for _, r := range a.results {
		content := r.Content
		switch r.Role {
		case "error":
			b.WriteString(errorStyle.Render(content))
		case "result":
			if a.markdownFn != nil {
				content = a.markdownFn(content)
			}
			b.WriteString(content)
		default: // "typed" 等已预渲染的字符串
			b.WriteString(content)
		}
		b.WriteString("\n")
	}

	// ActionLog
	for _, s := range a.actionLog {
		b.WriteString(s.View(a.AgentName))
		b.WriteString("\n")
	}

	return b.String()
}

// ---- actionStep rendering ----

func (s actionStep) View(agentName string) string {
	var icon string
	switch s.Status {
	case actionExecuting:
		s.spinner.View()
		icon = s.spinner.View()
	case actionDone:
		icon = actionDoneStyle.Render("✓")
	case actionFailed:
		icon = actionFailedStyle.Render("✗")
	}

	var b strings.Builder

	b.WriteString(icon)
	b.WriteString(" ")
	b.WriteString(agentStyle.Render(agentName))
	b.WriteString(" ")

	switch s.Status {
	case actionExecuting:
		b.WriteString(actionToolStyle.Render(s.ToolName))
		if s.ProgressText != "" {
			b.WriteString("(")
			b.WriteString(actionProgressStyle.Render(s.ProgressText))
			b.WriteString(")")
		}
		if s.EstimatedTokens > 0 {
			b.WriteString(" | 预")
			b.WriteString(formatTokens(s.EstimatedTokens))
			b.WriteString("Token")
		}
	case actionDone:
		b.WriteString(actionToolStyle.Render(s.ToolName))
		b.WriteString(" 已完成")
		if s.ResultText != "" {
			b.WriteString(" | 消耗 ")
			b.WriteString(actionResultStyle.Render(s.ResultText))
		}
	case actionFailed:
		b.WriteString(actionToolStyle.Render(s.ToolName))
		b.WriteString(" 失败")
		if s.ResultText != "" {
			b.WriteString(" | ")
			b.WriteString(actionResultStyle.Render(s.ResultText))
		}
	}

	return b.String()
}
