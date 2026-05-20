package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	welcome *welcome.WelcomePanel
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.welcome.Update(msg.WindowResizeMsg{Width: e.Width, Height: e.Height})
		return m, nil
	case tea.KeyPressMsg:
		if e.String() == "q" || e.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(m.welcome.View() + "\n\n按 q 退出\n")
}

func main() {
	w := welcome.New()
	w.Data = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0 Beta",
		AgentName: "architect",
		Workspace: "/home/user/project",
		SessionID: "sess-abc123",
		ModelName: "claude-sonnet-4-20250514",
	}

	p := tea.NewProgram(model{welcome: w})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
