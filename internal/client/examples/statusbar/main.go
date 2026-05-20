package main

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/statusbar"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

const tickInterval = 800 * time.Millisecond

type model struct {
	bar   *statusbar.StatusBar
	timer timer.Model
}

func (m model) Init() tea.Cmd {
	return m.timer.Init()
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.bar.Update(clientmsg.WindowResizeMsg{Width: e.Width, Height: e.Height})
		return m, nil
	case timer.TickMsg:
		newTimer, timerCmd := m.timer.Update(e)
		m.timer = newTimer
		m.bar.Tick()
		return m, timerCmd
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.bar = statusbar.New()
			m.bar.Width = 80
			m.bar.CurrentState = "思考中"
			m.bar.TokensTotal = 1234
			m.bar.AgentName = "architect"
			m.bar.ModelName = "claude-sonnet-4-20250514"
			m.bar.SessionName = "sess-001"
			m.bar.SessionStart = time.Now()
			return m, nil
		case "2":
			m.bar = statusbar.New()
			m.bar.Width = 80
			m.bar.CurrentState = "执行中"
			m.bar.TokensTotal = 3456
			m.bar.AgentName = "debugger"
			m.bar.ModelName = "claude-sonnet-4-20250514"
			m.bar.SessionName = "sess-abc123"
			m.bar.SessionStart = time.Now()
			return m, nil
		case "3":
			m.bar = statusbar.New()
			m.bar.Width = 80
			m.bar.CurrentState = "完成"
			m.bar.TokensTotal = 1234567
			m.bar.AgentName = "architect"
			m.bar.ModelName = "claude-sonnet-4-20250514"
			m.bar.SessionName = "sess-abc123"
			m.bar.SessionStart = time.Now()
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		m.bar.View() + "\n\n按 1 思考中 | 按 2 执行中 | 按 3 完成 | 按 q 退出\n",
	)
}

func main() {
	b := statusbar.New()
	b.Width = 80
	b.CurrentState = "空闲"
	b.TokensTotal = 0
	b.AgentName = "architect"
	b.ModelName = "claude-sonnet-4-20250514"
	b.SessionName = "sess-abc123"

	p := tea.NewProgram(model{
		bar:   b,
		timer: timer.New(100*365*24*time.Hour, timer.WithInterval(tickInterval)),
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
