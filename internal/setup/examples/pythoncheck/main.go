package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/pythoncheck"
	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
)

type model struct {
	pythonCheck *pythoncheck.Model
	scene       int
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
			m.pythonCheck = pythoncheck.New(setupdata.PythonInfo{
				Detected: false,
				Version:  "",
			}, "/tmp/workspace")
			return m, nil
		case "2":
			m.scene = 2
			m.pythonCheck = pythoncheck.New(setupdata.PythonInfo{
				Detected: true,
				Version:  "3.12.0",
			}, "/tmp/workspace")
			return m, nil
		case "3":
			m.scene = 3
			m.pythonCheck = pythoncheck.New(setupdata.PythonInfo{
				Detected: true,
				Version:  "3.11.5",
			}, "/tmp/workspace-with-venv")
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.pythonCheck, cmd = m.pythonCheck.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	hint := "\n按 1 Python未安装 | 按 2 Python已检测 | 按 3 虚拟环境已就绪 | 按 q 退出"
	switch m.scene {
	case 1:
		hint = "\n📌 场景: Python 未安装" + hint
	case 2:
		hint = "\n📌 场景: Python 已检测，无虚拟环境" + hint
	case 3:
		hint = "\n📌 场景: Python + 虚拟环境已就绪" + hint
	default:
		hint = "\n👆 选择一个场景查看不同状态下的 Python 检查页面" + hint
	}
	return tea.NewView(m.pythonCheck.View() + hint)
}

func main() {
	m := model{
		pythonCheck: pythoncheck.New(setupdata.PythonInfo{Detected: true, Version: "3.12.0"}, "/tmp/workspace"),
		scene:       0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
