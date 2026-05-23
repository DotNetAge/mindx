package conv

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type Thought struct {
	Content     string
	Pending     string
	TokensIn    int
	TokensOut   int
	Timestamp   time.Time
	ThoughtData map[string]any
	IsActive    bool
	Collapsed   bool
	BlinkOn     bool
}

type ThoughtActionRound struct {
	Thought Thought
	Action  Action
}

type Thinking struct {
	IsActive  bool
	BlinkOn   bool
	StartTime time.Time
	Duration  time.Duration
}

const StandardTickInterval = 100 * time.Millisecond

func NewThinking() Thinking {
	return Thinking{}
}

func UpdateThinking(m Thinking, e tea.Msg) (Thinking, tea.Cmd) {
	switch e.(type) {
	case msg.ThinkingDeltaMsg:
		if !m.IsActive {
			m.IsActive = true
			m.StartTime = time.Now()
		}
		return m, nil

	case msg.ThinkingDoneMsg:
		if !m.IsActive {
			return m, nil
		}
		m.IsActive = false
		m.Duration = time.Since(m.StartTime).Round(time.Millisecond)
		return m, nil

	case msg.TickMsg:
		if m.IsActive {
			m.BlinkOn = !m.BlinkOn
		}
		return m, nil
	}

	return m, nil
}

func ViewThinking(m Thinking) string {
	if !m.IsActive && m.Duration == 0 {
		return ""
	}

	if m.IsActive {
		return ViewBlink(Blink{Symbol: " ● 思考中", BlinkOn: m.BlinkOn}, style.GrayStyle)
	}

	d := m.Duration
	if d < time.Millisecond {
		d = 0
	}
	return style.GrayStyle.Render(fmt.Sprintf(" ● 思考完成 %s", d))
}

func UpdateThought(m Thought, e tea.Msg) (Thought, tea.Cmd) {
	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		if m.IsActive {
			m.Pending += e.Content
		}
		return m, nil

	case msg.ThinkingDoneMsg:
		if !m.IsActive {
			return m, nil
		}
		if m.Pending != "" {
			m.Content = m.Pending
			m.Pending = ""
		}
		m.Timestamp = time.Now()
		if e.ThoughtData != nil {
			m.ThoughtData = e.ThoughtData

			if tokensIn, ok := e.ThoughtData["tokens_in"].(int); ok {
				m.TokensIn = tokensIn
			} else if tokensIn64, ok := e.ThoughtData["tokens_in"].(float64); ok {
				m.TokensIn = int(tokensIn64)
			}

			if tokensOut, ok := e.ThoughtData["tokens_out"].(int); ok {
				m.TokensOut = tokensOut
			} else if tokensOut64, ok := e.ThoughtData["tokens_out"].(float64); ok {
				m.TokensOut = int(tokensOut64)
			}
		}
		m.IsActive = false
		return m, nil

	case msg.TickMsg:
		m.BlinkOn = !m.BlinkOn
		return m, nil

	case msg.ThinkCollapseMsg:
		m.Collapsed = !m.Collapsed
		return m, nil
	}

	return m, nil
}

const (
	maxContentLines  = 50  // max lines of thought content to render
	maxPendingLines  = 20  // max lines of streaming pending content to render
	maxReasoningLines = 30 // max lines of reasoning data to render
)

func ViewThought(m Thought) string {
	if m.Content == "" && m.Pending == "" && !m.IsActive {
		return ""
	}

	var b strings.Builder
	indent := " "

	if m.IsActive {
		b.WriteString(ViewBlink(Blink{Symbol: " ● 深度思考", BlinkOn: m.BlinkOn}, style.GrayStyle))
	} else {
		b.WriteString(style.GrayStyle.Render(" ● 深度思考"))
	}
	b.WriteByte('\n')

	var tokensSuffix string
	if m.TokensIn > 0 || m.TokensOut > 0 {
		tokensSuffix = fmt.Sprintf("  [Tokens: %s in / %s out]", formatNumber(m.TokensIn), formatNumber(m.TokensOut))
	}

	hasReasoning := false
	if m.ThoughtData != nil {
		if reasoning, ok := m.ThoughtData["reasoning"].(string); ok && reasoning != "" {
			hasReasoning = true
		}
	}

	lines := strings.Split(m.Content, "\n")
	shouldCollapse := m.Collapsed && len(lines) > 3
	shouldTruncate := len(lines) > maxContentLines
	var hiddenLines int
	if shouldTruncate {
		hiddenLines = len(lines) - maxContentLines
		lines = lines[len(lines)-maxContentLines:]
		shouldCollapse = true
	}
	displayLines := lines
	if shouldCollapse {
		displayLines = lines[len(lines)-3:]
	}

	isLastLine := m.Pending == "" && !hasReasoning
	for i, line := range displayLines {
		prefix := indent + "│ "
		if i == 0 && m.Pending == "" {
			prefix = indent + "├─ "
		}
		if i == len(displayLines)-1 && isLastLine {
			prefix = indent + "└─ "
			line += tokensSuffix
		}
		b.WriteString(style.DimStyle.Render(prefix + line))
		b.WriteByte('\n')
	}
	if shouldCollapse {
		// Show both truncation and collapse notices when applicable
		totalHidden := hiddenLines
		if shouldTruncate && len(lines) > 3 {
			totalHidden = len(lines) - 3 + hiddenLines
		} else if len(lines) > 3 {
			totalHidden = len(lines) - 3
		}
		if totalHidden > 0 {
			collapseText := fmt.Sprintf("… +%d lines (ctrl+o to expand)", totalHidden)
			isLastLine = m.Pending == "" && !hasReasoning
			if isLastLine {
				collapseText += tokensSuffix
			}
			b.WriteString(style.DimStyle.Render(indent + "│ " + collapseText))
			b.WriteByte('\n')
		}
	}

	if m.Pending != "" {
		pendingLines := strings.Split(m.Pending, "\n")
		if len(pendingLines) > maxPendingLines {
			hidden := len(pendingLines) - maxPendingLines
			pendingLines = append(
				[]string{fmt.Sprintf("… +%d lines streaming", hidden)},
				pendingLines[len(pendingLines)-maxPendingLines:]...,
			)
		}
		isLastLine = !hasReasoning
		for i, line := range pendingLines {
			if m.Content == "" && i == 0 {
				b.WriteString(style.DimStyle.Render(indent + "├─ " + line))
			} else if i == len(pendingLines)-1 && isLastLine {
				b.WriteString(style.DimStyle.Render(indent + "└─ " + line + tokensSuffix))
			} else {
				b.WriteString(style.DimStyle.Render(indent + "│ " + line))
			}
			b.WriteByte('\n')
		}
	}

	if hasReasoning {
		reasoning := m.ThoughtData["reasoning"].(string)
		reasoningLines := strings.Split(reasoning, "\n")
		if len(reasoningLines) > maxReasoningLines {
			hidden := len(reasoningLines) - maxReasoningLines
			reasoningLines = append(
				[]string{fmt.Sprintf("… +%d lines reasoning truncated", hidden)},
				reasoningLines[len(reasoningLines)-maxReasoningLines:]...,
			)
		}
		for i, line := range reasoningLines {
			prefix := indent + "└─ "
			if i < len(reasoningLines)-1 {
				prefix = indent + "├─ "
			}
			if i == len(reasoningLines)-1 {
				line += tokensSuffix
			}
			b.WriteString(style.DimStyle.Render(prefix + line))
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func NewThoughtActionRound() ThoughtActionRound {
	return ThoughtActionRound{
		Thought: Thought{IsActive: true, Collapsed: true},
		Action:  Action{},
	}
}
