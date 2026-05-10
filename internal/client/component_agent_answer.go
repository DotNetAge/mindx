package client

import (
	"fmt"
	"strings"
)

// AgentAnswer 对应服务器的一个会话单元。
// 内部结构（从上到下）: Thinks → Results → ActionLog
// 所有内容均为 append-only。
type AgentAnswer struct {
	SessionID  string
	AgentName  string

	thinking       strings.Builder
	thinkingDone   string
	results        []answerResult
	actionLog      []actionStep

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
	ResultText      string // 完成/失败后的结果摘要
}

type actionStatus int

const (
	actionExecuting actionStatus = iota
	actionDone
	actionFailed
)

// NewAgentAnswer 创建一个新的 AgentAnswer。
func NewAgentAnswer(sessionID, agentName string) *AgentAnswer {
	return &AgentAnswer{
		SessionID: sessionID,
		AgentName: agentName,
	}
}

// AppendThinking 追加思考内容。同一 event 流中的多个 delta 会自动合并到同一块。
func (a *AgentAnswer) AppendThinking(content string) {
	a.thinking.WriteString(content)
}

// SetThinkingDone 设置思考完成的格式化内容（替换原始流式内容）。
func (a *AgentAnswer) SetThinkingDone(content string) {
	a.thinkingDone = content
}

// AppendResult 追加一条结果。
func (a *AgentAnswer) AppendResult(content string) {
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
	a.actionLog = append(a.actionLog, actionStep{
		ToolName:        toolName,
		Status:          actionExecuting,
		EstimatedTokens: estimatedTokens,
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

	// Thinks - 优先显示格式化的 thinking_done 内容，否则显示原始流式内容
	if a.thinkingDone != "" {
		formatted := a.thinkingDone
		if a.markdownFn != nil {
			formatted = a.markdownFn(formatted)
		}
		b.WriteString(thinkingStyle.Render(formatted))
		b.WriteString("\n")
	} else if a.thinking.Len() > 0 {
		thinking := a.thinking.String()
		if a.markdownFn != nil {
			thinking = a.markdownFn(thinking)
		}
		b.WriteString(thinkingStyle.Render(thinking))
		b.WriteString("\n")
	}

	// Results
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
		icon = actionSpinnerStyle.Render("⏳")
	case actionDone:
		icon = actionDoneStyle.Render("✓")
	case actionFailed:
		icon = actionFailedStyle.Render("✗")
	}

	parts := []string{
		icon,
		agentStyle.Render(agentName),
		actionToolStyle.Render(s.ToolName),
	}

	if s.Status == actionExecuting && s.ProgressText != "" {
		parts = append(parts, actionProgressStyle.Render(s.ProgressText))
	}

	if s.EstimatedTokens > 0 {
		parts = append(parts, fmt.Sprintf("[预计 %s tokens]", formatTokens(s.EstimatedTokens)))
	}

	if s.ResultText != "" {
		parts = append(parts, actionResultStyle.Render(s.ResultText))
	}

	return strings.Join(parts, " ")
}
