package conv

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// Thinking tracks the thinking phase blink state.
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
		return ViewBlink(Blink{Symbol: i18n.T("client.ui.thinking.active"), BlinkOn: m.BlinkOn}, style.GrayStyle)
	}

	d := m.Duration
	if d < time.Millisecond {
		d = 0
	}
	return style.GrayStyle.Render(fmt.Sprintf(i18n.T("client.ui.thinking.done"), d))
}

const (
	maxThoughtContentLines = 50
)

// ViewThought renders the thinking content in a tree-style format.
func ViewThought(content string, tokensIn, tokensOut int, collapsed bool, tokensSuffix string) string {
	if content == "" {
		return ""
	}

	var b strings.Builder
	indent := " "

	b.WriteString(style.GrayStyle.Render(" ⏺ " + i18n.T("client.ui.thinking.label")))
	b.WriteByte('\n')

	if content != "" {
		lines := strings.Split(content, "\n")
		shouldCollapse := collapsed && len(lines) > 3
		shouldTruncate := len(lines) > maxThoughtContentLines
		var hiddenLines int
		if shouldTruncate {
			hiddenLines = len(lines) - maxThoughtContentLines
			lines = lines[len(lines)-maxThoughtContentLines:]
			shouldCollapse = true
		}
		displayLines := lines
		if shouldCollapse {
			displayLines = lines[len(lines)-3:]
		}

		for i, line := range displayLines {
			prefix := indent + "│ "
			if i == 0 {
				prefix = indent + "├─ "
			}
			if i == len(displayLines)-1 {
				prefix = indent + "└─ "
				line += tokensSuffix
			}
			b.WriteString(style.DimStyle.Render(prefix + line))
			b.WriteByte('\n')
		}
		if shouldCollapse {
			totalHidden := hiddenLines
			if shouldTruncate && len(lines) > 3 {
				totalHidden = len(lines) - 3 + hiddenLines
			} else if len(lines) > 3 {
				totalHidden = len(lines) - 3
			}
			if totalHidden > 0 {
				collapseText := fmt.Sprintf("… +%d lines (ctrl+o to expand)", totalHidden)
				collapseText += tokensSuffix
				b.WriteString(style.DimStyle.Render(indent + "│ " + collapseText))
				b.WriteByte('\n')
			}
		}
	}

	return b.String()
}
