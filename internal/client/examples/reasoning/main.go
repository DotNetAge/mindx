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
	reasoning conv.Reasoning
	width     int
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
			m.reasoning = conv.NewReasoning()
			return m, tea.Batch(m.tick(), m.reasoningCmd())
		case "2":
			m.reasoning = conv.Reasoning{
				Label:     "深度思考",
				IsActive:  false,
				StartTime: time.Now().Add(-1200 * time.Millisecond),
				Duration:  1200 * time.Millisecond,
			}
			return m, nil
		case "3":
			m.reasoning = conv.NewReasoning().WithLabel("Thinking")
			return m, tea.Batch(m.tick(), m.reasoningCmd())
		case "4":
			m.reasoning = conv.Reasoning{
				Label:     "Thinking",
				IsActive:  false,
				StartTime: time.Now().Add(-800 * time.Millisecond),
				Duration:  800 * time.Millisecond,
			}
			return m, nil
		case "5":
			m.reasoning = conv.NewReasoning()
			go func() {
				time.Sleep(3 * time.Second)
			}()
			return m, tea.Batch(m.tick(), m.reasoningCmd())
		}
		return m, nil
	default:
		newReasoning, cmd := conv.UpdateReasoning(m.reasoning, e)
		m.reasoning = newReasoning

		if _, ok := e.(msg.ThinkingDoneMsg); ok {
			return m, nil
		}

		return m, cmd
	}
}

func (m model) View() tea.View {
	view := conv.ViewReasoning(m.reasoning)
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

func (m model) reasoningCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		return msg.ThinkingDoneMsg{
			SessionID: "demo",
			Reasoning: "经过分析，这是一个使用 bubbletea v2 框架的终端应用项目。",
		}
	}
}

func main() {
	m := model{
		reasoning: conv.NewReasoning(),
		width:     80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
