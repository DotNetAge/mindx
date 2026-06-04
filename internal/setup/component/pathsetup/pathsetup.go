package pathsetup

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
	choice     bool
	pathInPath bool
	installDir string
	width      int
	height     int
	renderer   *glamour.TermRenderer
}

func New(installDir string, pathInPath bool) *Model {
	return &Model{
		choice:     pathInPath,
		pathInPath: pathInPath,
		installDir: installDir,
		width:      80,
		height:     24,
		renderer:   initGlamour(minContentWidth),
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

func contentWidth(w int) int {
	if w > minContentWidth {
		cw := w - 4
		return cw
	}
	return minContentWidth
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
			if !m.pathInPath {
				m.choice = !m.choice
			}
		case "enter":
			return m, func() tea.Msg {
				return setupmsg.PathDecisionMsg{AddToPath: m.choice}
			}
		case "s", "S":
			if m.pathInPath {
				return m, func() tea.Msg {
					return setupmsg.PathDecisionMsg{AddToPath: true}
				}
			}
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var b strings.Builder
	if m.pathInPath {
		b.WriteString(renderMarkdown(m.renderer, fmt.Sprintf(
			"📌 系统 PATH 配置\n\n✅ **mindx 已在系统 PATH 中**\n\n当前安装路径: `%s`\n\n**Enter** 继续  **S** 跳过",
			m.installDir,
		)))
	} else {
		md := fmt.Sprintf(`📌 系统 PATH 配置

安装路径: %s

将 mindx 所在目录添加到系统 PATH 环境变量后，你可以在任意终端窗口中直接运行 mindx 命令。
（修改用户级 PATH，无需管理员权限）

是否添加到 PATH?

%s

← → 切换  **Enter** 确认  **Esc** 退出`,
			m.installDir, yesNoIndicator(m.choice),
		)
		b.WriteString(renderMarkdown(m.renderer, md))
	}
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle(""),
		"",
		content,
	) + "\n"
}
