package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
)

type model struct {
	question conv.Question
	width    int
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
		conv.ViewQuestion(m.question, m.width) +
			"\n\n按 q 退出\n",
	)
}

func main() {
	m := model{
		question: conv.Question{Text: "分析项目的依赖关系并生成报告"},
		width:    80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
