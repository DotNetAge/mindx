package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/choices"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	panel    *choices.ChoicesPanel
	selected string
	result   string
	width    int
	scene    int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		newPanel, _ := m.panel.Update(e)
		m.panel = newPanel
		return m, nil

	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.scene = 1
			m.result = ""
			m.selected = ""
			newPanel, _ := m.panel.Update(clientmsg.ShowChoicesMsg{
				Prompt:  "请选择一个操作：",
				Options: []string{
					"分析代码结构",
					"生成测试用例",
					"重构模块",
					"更新文档",
					"部署到生产环境",
				},
			})
			m.panel = newPanel
			return m, nil
		case "2":
			m.scene = 2
			m.result = ""
			m.selected = ""
			newPanel, _ := m.panel.Update(clientmsg.ShowChoicesMsg{
				Prompt:         "请选择要执行的任务（可多选）：",
				Options:        []string{"代码审查", "运行测试", "构建项目", "性能分析", "安全扫描"},
				MultiSelect:    true,
				AllowTextInput: false,
			})
			m.panel = newPanel
			return m, nil
		case "3":
			m.scene = 3
			m.result = ""
			m.selected = ""
			newPanel, _ := m.panel.Update(clientmsg.ShowChoicesMsg{
				Prompt:         "请选择需要修改的文件（多选，或输入其他文件名）：",
				Options:        []string{"client.go", "app.go", "config.yaml", "main.go", "utils.go"},
				MultiSelect:    true,
				AllowTextInput: true,
			})
			m.panel = newPanel
			return m, nil
		default:
			newPanel, cmd := m.panel.Update(e)
			m.panel = newPanel
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		}

	case clientmsg.ChoiceSelectedMsg:
		switch m.scene {
		case 1:
			if e.Index >= 0 && e.Index < len(m.panel.Items) {
				m.selected = m.panel.Items[e.Index]
				m.result = fmt.Sprintf("✅ 单选结果: %s (索引 %d)", m.selected, e.Index)
			} else if e.Index == -1 {
				m.result = "⏭️ 用户取消了选择（Esc）"
			}
		case 2, 3:
			if len(e.Indices) > 0 {
				var names []string
				sort.Ints(e.Indices)
				for _, idx := range e.Indices {
					if idx >= 0 && idx < len(m.panel.Items) {
						names = append(names, m.panel.Items[idx])
					}
				}
				m.result = fmt.Sprintf("✅ 多选结果 (%d 项): %s", len(e.Indices), strings.Join(names, ", "))
				if e.CustomText != "" {
					m.result += fmt.Sprintf("\n📝 其他输入: %s", e.CustomText)
				}
			} else if e.CustomText != "" {
				m.result = fmt.Sprintf("✅ 自定义输入: %s", e.CustomText)
			} else if e.Index == -1 {
				m.result = "⏭️ 用户取消了选择（Esc）"
			} else {
				m.result = "❌ 未选择任何项"
			}
		}
		return m, nil
	}

	newPanel, cmd := m.panel.Update(e)
	m.panel = newPanel
	return m, cmd
}

func (m model) View() tea.View {
	hint := "\n按 1 单选模式 | 按 2 多选模式 | 按 3 多选+自定义输入 | q 退出\n"
	switch m.scene {
	case 1:
		hint = "\n📌 场景1: 单选模式 — 从列表中选择一个选项\n" + hint
	case 2:
		hint = "\n📌 场景2: 多选模式 — Space 选择/取消，Enter 确认\n" + hint
	case 3:
		hint = "\n📌 场景3: 多选+自定义 — Space 选择，Tab 切换到底部输入自由文本\n" + hint
	default:
		hint = "\n👆 请选择一个场景查看 ChoicesPanel 不同模式\n" + hint
	}

	view := m.panel.View()
	if view != "" {
		view += "\n"
	}
	if m.result != "" {
		view += fmt.Sprintf("\n%s\n", m.result)
	}

	return tea.NewView(view + hint)
}

func main() {
	p := tea.NewProgram(model{panel: choices.New()})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
