package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	thinking conv.Thinking
	width    int
}

func (m model) Init() tea.Cmd {
	return m.tick()
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.thinking = conv.NewThinking()
			return m, tea.Batch(m.tick(), m.simulateThinkingDelta())
		case "2":
			m.thinking = conv.Thinking{
				IsActive:  false,
				Duration:  1200 * time.Millisecond,
			}
			return m, nil
		case "3":
			m.thinking = conv.NewThinking()
			return m, tea.Batch(m.tick(), m.simulateThinkingDelta())
		case "4":
			m.thinking = conv.Thinking{
				IsActive:  false,
				Duration:  800 * time.Millisecond,
			}
			return m, nil
		case "5":
			m.thinking = conv.NewThinking()
			return m, tea.Batch(m.tick(), m.simulateThinkingDelta())
		}
		return m, nil
	default:
		newThinking, cmd := conv.UpdateThinking(m.thinking, e)
		m.thinking = newThinking

		if _, ok := e.(msg.ThinkingDoneMsg); ok {
			return m, nil
		}

		return m, cmd
	}
}

func (m model) View() tea.View {
	view := conv.ViewThinking(m.thinking)
	if view == "" {
		view = "(空状态 - 请按按键查看示例)"
	}

	hint := "\n按 1 中文思考中(动画) | 按 2 中文结果 | 按 3 英文思考中(动画) | 按 4 英文结果 | 按 5 模拟完整流程 | 按 q 退出\n"
	return tea.NewView(view + hint)
}

func (m model) tick() tea.Cmd {
	return tea.Every(conv.StandardTickInterval, func(t time.Time) tea.Msg {
		return msg.TickMsg{Time: t}
	})
}

func (m model) simulateThinkingDelta() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		return msg.ThinkingDeltaMsg{
			SessionID: "demo",
			Content:   "",
		}
	}
}

func main() {
	m := model{
		thinking: conv.NewThinking(),
		width:    80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
