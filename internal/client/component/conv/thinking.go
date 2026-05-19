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
		isLastLine = m.Pending == "" && !hasReasoning
		collapseText := fmt.Sprintf("… +%d lines (ctrl+o to expand)", len(lines)-3)
		if isLastLine {
			collapseText += tokensSuffix
		}
		b.WriteString(style.DimStyle.Render(indent + "│ " + collapseText))
		b.WriteByte('\n')
	}

	if m.Pending != "" {
		pendingLines := strings.Split(m.Pending, "\n")
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
