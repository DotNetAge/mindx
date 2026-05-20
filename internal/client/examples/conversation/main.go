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
	conv  conv.Conversation
	width int
}

func (m model) Init() tea.Cmd {
	return tickCmd()
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
			m.conv = popDoneConv()
			return m, nil
		case "2":
			m.conv = popRunningConv()
			return m, tickCmd()
		case "3":
			m.conv = popThinkingConv()
			return m, tickCmd()
		}
		return m, nil
	case msg.TickMsg:
		newConv, _ := conv.UpdateConversation(m.conv, e)
		m.conv = newConv
		return m, tickCmd()
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		conv.ViewConversation(m.conv, m.width) +
			"\n\n按 1 已完成 | 按 2 执行中 | 按 3 思考中 | 按 q 退出\n",
	)
}

func popDoneConv() conv.Conversation {
	c := conv.NewConversation("s1", "architect", "分析项目依赖关系")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content:   "首先分析项目的整体架构。这是一个基于 Go 的 CLI 工具，使用 bubbletea 框架构建终端 UI。\n项目采用模块化设计，主要分为 client、core 和 pkg 三层。",
			TokensIn:  342,
			TokensOut: 89,
			Timestamp: time.Now().Add(-5 * time.Minute),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{ToolCount: 1, ToolNames: []string{"read_file"}},
			Steps: []conv.ActionStep{
				{
					ToolName:   "read_file",
					Status:     conv.ActionStepDone,
					ResultText: "module github.com/DotNetAge/mindx\ngo 1.22",
					Collapsed:  true,
				},
			},
			Completed:     true,
			SuccessCount:  1,
			FailedCount:   0,
			TotalTokens:   500,
			TotalDuration: 5 * time.Second,
		},
	})

	c.Output = conv.Output{
		Entries: []conv.OutputEntry{
			{
				Role:    "assistant",
				Content: "## 分析结果\n\n项目依赖分析完成，所有依赖兼容。",
			},
		},
	}

	return c
}

func popRunningConv() conv.Conversation {
	c := conv.NewConversation("s2", "debugger", "排查服务启动失败")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Pending:  "正在分析错误日志...",
			IsActive: true,
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{ToolCount: 1, ToolNames: []string{"exec_command"}},
			Steps: []conv.ActionStep{
				{
					ToolName:  "exec_command",
					Status:    conv.ActionStepExecuting,
					Params:    map[string]any{"cmd": "journalctl -xe"},
					Collapsed: true,
				},
			},
			StartTime: time.Now().Add(-3 * time.Second),
			Elapsed:   3 * time.Second,
		},
	})

	return c
}

func popThinkingConv() conv.Conversation {
	c := conv.NewConversation("s3", "architect", "设计系统架构方案")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Pending:  "正在检索项目结构，分析模块依赖关系，评估技术选型方案...\n考虑使用 clean architecture 分层设计，确保各模块职责清晰。",
			IsActive: true,
		},
	})

	return c
}

func tickCmd() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
		return msg.TickMsg{Time: t}
	})
}

func main() {
	p := tea.NewProgram(model{
		conv:  popDoneConv(),
		width: 80,
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
