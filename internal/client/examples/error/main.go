//go:build example

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
	err   conv.ErrorMsg
	width int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "t":
			// 模拟思考阶段错误
			m.err, _ = conv.UpdateErrorMsg(m.err, msg.AgentErrorMsg{
				Error: fmt.Errorf("think error: API 返回异常，请稍后重试"),
			})
		case "a":
			// 模拟执行阶段错误
			m.err, _ = conv.UpdateErrorMsg(m.err, msg.AgentErrorMsg{
				Error: fmt.Errorf("act error: 文件写入权限不足"),
			})
		case "o":
			// 模拟反思阶段错误
			m.err, _ = conv.UpdateErrorMsg(m.err, msg.AgentErrorMsg{
				Error: fmt.Errorf("observe error: 输出格式解析失败"),
			})
		case "c":
			// 清空错误
			m.err = conv.ErrorMsg{}
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		conv.ViewErrorMsg(m.err, m.width) +
			"\n\n按 t/a/o 触发不同阶段错误 | 按 c 清空 | 按 q 退出\n",
	)
}

func main() {
	now := time.Now()
	m := model{
		err: conv.ErrorMsg{
			Error: "think error: API 返回异常，请稍后重试",
			Phase: "思考阶段",
			Time:  now,
		},
		width: 80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
