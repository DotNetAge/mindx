package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/permission"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	bar      permission.PermissionBar
	width    int
	result   string
	scenario int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		return m, nil

	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.scenario = 1
			m.result = ""
			newBar, _ := permission.UpdatePermissionBar(m.bar, clientmsg.PermissionRequestMsg{
				ToolName:      "Bash",
				Reason:        "需要执行 shell 命令来编译项目代码",
				SecurityLevel: 2,
			})
			m.bar = newBar
			return m, nil
		case "2":
			m.scenario = 2
			m.result = ""
			newBar, _ := permission.UpdatePermissionBar(m.bar, clientmsg.PermissionRequestMsg{
				ToolName:      "WriteFile",
				Reason:        "尝试写入系统配置文件 /etc/hosts",
				SecurityLevel: 3,
			})
			m.bar = newBar
			return m, nil
		case "3":
			m.scenario = 3
			m.result = ""
			newBar, _ := permission.UpdatePermissionBar(m.bar, clientmsg.PermissionRequestMsg{
				ToolName:      "WebFetch",
				Reason:        "获取远程 API 数据用于分析",
				SecurityLevel: 1,
			})
			m.bar = newBar
			return m, nil
		default:
			newBar, cmd := permission.UpdatePermissionBar(m.bar, e)
			m.bar = newBar
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		}

	case clientmsg.ChoiceSelectedMsg:
		switch e.Index {
		case permission.PermissionAllow:
			m.result = fmt.Sprintf("✅ 用户已授权 %s 操作", m.bar.ToolName)
		case permission.PermissionDeny:
			m.result = fmt.Sprintf("❌ 用户拒绝了 %s 操作", m.bar.ToolName)
		default:
			m.result = "⏭️ 用户取消了授权请求（Esc）"
		}
		return m, nil
	}

	newBar, cmd := permission.UpdatePermissionBar(m.bar, e)
	m.bar = newBar
	return m, cmd
}

func (m model) View() tea.View {
	var hint string
	switch m.scenario {
	case 1:
		hint = "\n📌 场景1: 中等风险 — Bash 命令执行\n"
	case 2:
		hint = "\n📌 场景2: 高风险 — 系统文件写入\n"
	case 3:
		hint = "\n📌 场景3: 低风险 — 网络请求\n"
	default:
		hint = "\n👆 请选择一个场景查看权限确认栏效果\n"
	}

	hint += "按 1 中等风险(Bash) | 按 2 高风险(WriteFile) | 按 3 低风险(WebFetch)\n"
	hint += "← → 或 Tab 切换按钮 | Enter 确认 | Esc 取消 | q 退出\n"

	view := permission.ViewPermissionBar(m.bar, m.width)
	if view != "" {
		view += "\n"
	}
	if m.result != "" {
		view += fmt.Sprintf("\n%s\n", m.result)
	}

	return tea.NewView(view + hint)
}

func main() {
	p := tea.NewProgram(model{width: 80})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
