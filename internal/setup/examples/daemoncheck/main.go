package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/daemoncheck"
)

type model struct {
	daemonCheck *daemoncheck.Model
	decision    *bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.decision != nil {
				return m, tea.Quit
			}
			return m, tea.Quit
		case "1":
			m.daemonCheck = daemoncheck.New(false)
			m.decision = nil
			return m, nil
		case "2":
			m.daemonCheck = daemoncheck.New(true)
			m.decision = nil
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.daemonCheck, cmd = m.daemonCheck.Update(msg)

	if m.decision == nil {
		choice := m.daemonCheck.Choice()
		m.decision = &choice
	}

	return m, cmd
}

func (m model) View() tea.View {
	status := ""
	if m.decision != nil {
		if *m.decision {
			status = "\n当前选择: ✅ 安装 Daemon"
		} else {
			status = "\n当前选择: ❌ 跳过安装"
		}
	}
	hint := `
按 1 模拟未安装状态 | 按 2 模拟已安装状态 | 按 q 退出`
	return tea.NewView(m.daemonCheck.View() + status + hint)
}

func main() {
	m := model{
		daemonCheck: daemoncheck.New(false),
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
