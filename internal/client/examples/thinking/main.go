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
	thought conv.Thought
	width   int
}

func (m model) Init() tea.Cmd {
	return nil
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
			now := time.Now()
			m.thought = conv.Thought{
				Content:   "项目使用 Go 语言开发，核心框架为 bubbletea v2。\n架构分为三层：client（终端 UI）、core（业务逻辑）、pkg（工具库）。\n这是第一轮思考，使用 ● 前缀。",
				TokensIn:  420,
				TokensOut: 95,
				Timestamp: now.Add(-8 * time.Minute),
			}
			return m, nil
		case "2":
			now := time.Now()
			m.thought = conv.Thought{
				Content:   "诊断服务启动失败问题。查看系统日志发现端口绑定异常。\n通过 journalctl 发现关键错误信息。\n错误指向 8080 端口已被占用。",
				TokensIn:  230,
				TokensOut: 60,
				Timestamp: now.Add(-3 * time.Minute),
			}
			return m, nil
		case "3":
			m.thought = conv.Thought{
					IsActive: true,
			}
			return m, tickCmd()
		}
		return m, nil
	case msg.TickMsg:
		newThought, _ := conv.UpdateThought(m.thought, e)
		m.thought = newThought
		return m, tickCmd()
	case msg.ThinkingDeltaMsg, msg.ThinkingDoneMsg:
		newThought, _ := conv.UpdateThought(m.thought, e)
		m.thought = newThought
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		conv.ViewThought(m.thought)+
			"\n\n按 1 单轮思考 | 按 2 单轮+数据 | 按 3 纯Pending+blink | 按 q 退出\n",
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
		return msg.TickMsg{Time: t}
	})
}

func main() {
	now := time.Now()
	m := model{
		thought: conv.Thought{
			Content:   "项目使用 Go 语言开发，核心框架为 bubbletea v2。\n架构分为三层：client（终端 UI）、core（业务逻辑）、pkg（工具库）。\n这是第一轮思考，使用 ● 前缀。",
			TokensIn:  420,
			TokensOut: 95,
			Timestamp: now.Add(-8 * time.Minute),
		},
		width: 80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
