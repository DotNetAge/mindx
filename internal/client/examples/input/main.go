//go:build example

package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	inputArea *input.InputArea
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.inputArea.Update(clientmsg.WindowResizeMsg{Width: e.Width, Height: e.Height})
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		newInput, cmd := m.inputArea.Update(e)
		m.inputArea = newInput
		return m, cmd
	case clientmsg.UserSendMsg:
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(m.inputArea.View() + "\n\n按 q 退出\n")
}

func main() {
	ia := input.New()
	ia.Width = 80
	ia.Commands = []input.SlashCommand{
		{Name: "help", Description: "显示帮助信息"},
		{Name: "clear", Description: "清除屏幕"},
		{Name: "chat", Description: "切换对话"},
		{Name: "model", Description: "切换模型"},
		{Name: "exit", Description: "退出程序"},
	}
	ia.Agents = []data.AgentInfo{
		{Name: "architect", Description: "架构师"},
		{Name: "debugger", Description: "调试助手"},
	}
	ia.Models = []input.ModelItem{
		{Name: "claude-sonnet-4", Description: "Claude Sonnet 4"},
		{Name: "claude-haiku-4", Description: "Claude Haiku 4"},
	}

	p := tea.NewProgram(model{inputArea: ia})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
