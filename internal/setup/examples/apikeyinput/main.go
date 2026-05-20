package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/apikeyinput"
)

type model struct {
	apiKeyInput *apikeyinput.Model
	submitted   bool
	key         string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.submitted {
				return m, tea.Quit
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)

	if !m.submitted && m.apiKeyInput.Value() != "" {
		fmt.Printf("\n输入中: %s\n", maskKey(m.apiKeyInput.Value()))
	}

	return m, cmd
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:2] + "***" + key[len(key)-2:]
}

func (m model) View() tea.View {
	hint := "\n按 q 退出"
	if m.submitted {
		hint = fmt.Sprintf("\n✅ 已提交 Key: %s  按 q 退出", maskKey(m.key))
	}
	return tea.NewView(m.apiKeyInput.View() + hint)
}

func main() {
	m := model{
		apiKeyInput: apikeyinput.New("GPT-4o", true),
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
