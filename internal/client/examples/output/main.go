package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
)

type model struct {
	output conv.Output
	width  int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		return m, nil
	case tea.KeyPressMsg:
		if e.String() == "q" || e.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		conv.ViewOutput(m.output, m.width) +
			"\n\n按 q 退出\n",
	)
}

func main() {
	m := model{
		output: conv.Output{
			Entries: []conv.OutputEntry{
				{
					Role: "assistant",
					Content: "## 项目依赖分析结果\n\n" +
						"项目 **MindX CLI** 是一个基于 `bubbletea` 的终端 AI 客户端。\n\n" +
						"### 核心依赖\n\n" +
						"| 依赖 | 版本 | 用途 |\n" +
						"|------|------|------|\n" +
						"| bubbletea | v2 | 终端 UI 框架 |\n" +
						"| goreact | v0.5 | AI Agent 编排 |\n" +
						"| chromadb | v0.3 | 向量存储 |\n\n" +
						"### 建议\n\n" +
						"- 所有依赖版本兼容\n" +
						"- 无需额外更新",
				},
			},
		},
		width: 80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
