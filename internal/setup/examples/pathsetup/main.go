package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/pathsetup"
)

type model struct {
	pathSetup *pathsetup.Model
	scene     int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.scene = 1
			m.pathSetup = pathsetup.New("C:\\Users\\AppData\\Local\\MindX", false)
			return m, nil
		case "2":
			m.scene = 2
			m.pathSetup = pathsetup.New("C:\\Program Files\\MindX", true)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.pathSetup, cmd = m.pathSetup.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	hint := "\n按 1 未在PATH中 | 按 2 已在PATH中 | 按 q 退出"
	switch m.scene {
	case 1:
		hint = "\n📌 场景: mindx 不在系统 PATH 中" + hint
	case 2:
		hint = "\n📌 场景: mindx 已在系统 PATH 中" + hint
	default:
		hint = "\n👆 选择一个场景查看 PATH 配置页面" + hint
	}
	return tea.NewView(m.pathSetup.View() + hint)
}

func main() {
	m := model{
		pathSetup: pathsetup.New("C:\\Users\\AppData\\Local\\MindX", false),
		scene:     0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
