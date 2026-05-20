package daemoncheck

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	choice    bool
	installed bool
	width     int
	height    int
	renderer  *glamour.TermRenderer
}

func New(installed bool) *Model {
	return &Model{
		choice:    installed,
		installed: installed,
		width:     80,
		height:    24,
		renderer:  initGlamour(minContentWidth),
	}
}

func (m *Model) Choice() bool { return m.choice }

func initGlamour(width int) *glamour.TermRenderer {
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return r
}

func renderMarkdown(r *glamour.TermRenderer, src string) string {
	if r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

func yesNoIndicator(yes bool) string {
	if yes {
		return "**> Yes**  \n  No"
	}
	return "  Yes  \n**> No**"
}

func paddedView(content string, height int) string {
	lines := strings.Count(content, "\n") + 1
	if height > lines+1 {
		return content + strings.Repeat("\n", height-lines)
	}
	return content + "\n"
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderer = initGlamour(contentWidth(m.width))

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
		case "left", "right":
			m.choice = !m.choice
		case "enter":
			return m, func() tea.Msg {
				return setupmsg.DaemonDecisionMsg{Install: m.choice}
			}
		case "s", "S":
			if m.installed {
				return m, func() tea.Msg {
					return setupmsg.DaemonDecisionMsg{Install: true}
				}
			}
		}
	}
	return m, nil
}

func contentWidth(w int) int {
	if w > minContentWidth {
		cw := w - 4
		return cw
	}
	return minContentWidth
}

func (m *Model) View() string {
	var b strings.Builder
	if m.installed {
		b.WriteString(renderMarkdown(m.renderer, fmt.Sprintf(
			"⚙️ Daemon 后台服务\n\n✅ **已安装**\n\nDaemon 已注册为开机自启动服务。\n\n**Enter** 继续  **S** 跳过",
		)))
	} else {
		md := `⚙️ Daemon 后台服务

🔴 **未安装**

Daemon 是后台常驻服务，用于接收定时任务和 WebSocket 连接。

未安装不影响本地对话，但以下功能不可用：
  - 定时任务自动触发
  - WebSocket 远程连接
  - 系统托盘常驻

是否注册为开机自启动服务?

` + yesNoIndicator(m.choice) + `

← → 切换  **Enter** 确认  **Esc** 退出`
		b.WriteString(renderMarkdown(m.renderer, md))
	}
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle("MindX2 beta Setup"),
		"",
		content,
	) + "\n"
}
