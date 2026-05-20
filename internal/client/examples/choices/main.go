package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/choices"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	choices   *choices.ChoicesPanel
	selected  string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			return m, func() tea.Msg {
				return clientmsg.ShowChoicesMsg{
					Prompt: "请选择一个操作：",
					Options: []string{
						"分析代码结构",
						"生成测试用例",
						"重构模块",
						"更新文档",
						"部署到生产环境",
						"性能优化",
						"安全审计",
					},
				}
			}
		case "up", "down", "k", "j", "enter", "esc":
			newChoices, cmd := m.choices.Update(e)
			m.choices = newChoices
			return m, cmd
		}
		return m, nil
	case clientmsg.ShowChoicesMsg:
		newChoices, cmd := m.choices.Update(e)
		m.choices = newChoices
		return m, cmd
	case clientmsg.ChoiceSelectedMsg:
		if e.Index >= 0 && e.Index < len(m.choices.Items) {
			m.selected = m.choices.Items[e.Index]
		}
		m.choices = choices.New()
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	view := m.choices.View()
	if view == "" && m.selected != "" {
		view = fmt.Sprintf("已选择: %s\n（按 s 重新选择）", m.selected)
	} else if view == "" {
		view = "（按 s 显示选择面板）"
	}
	return tea.NewView(view + "\n\n按 s 显示选项 | 方向键选择 | Enter 确认 | Esc 取消 | q 退出\n")
}

func main() {
	p := tea.NewProgram(model{choices: choices.New()})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
