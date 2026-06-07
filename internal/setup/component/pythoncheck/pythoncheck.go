package pythoncheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	choice       bool
	pythonInfo   setupdata.PythonInfo
	workspaceDir string
	width        int
	height       int
	renderer     *glamour.TermRenderer
}

func New(pythonInfo setupdata.PythonInfo, workspaceDir string) *Model {
	return &Model{
		choice:       true,
		pythonInfo:   pythonInfo,
		workspaceDir: workspaceDir,
		width:        80,
		height:       24,
		renderer:     initGlamour(minContentWidth),
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

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderer = initGlamour(contentWidth(m.width))

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
		case "esc":
			if m.pythonInfo.Detected {
				return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
			}
			m.choice = false
			return m, func() tea.Msg {
				return setupmsg.PythonDecisionMsg{Setup: false, Version: m.pythonInfo.Version}
			}
		case "left", "right":
			if m.pythonInfo.Detected {
				m.choice = !m.choice
			}
		case "enter":
			if !m.pythonInfo.Detected {
				m.choice = true
				return m, func() tea.Msg {
					return setupmsg.PythonDecisionMsg{Setup: true, Version: m.pythonInfo.Version}
				}
			}
			return m, func() tea.Msg {
				return setupmsg.PythonDecisionMsg{Setup: m.choice, Version: m.pythonInfo.Version}
			}
		case "s", "S":
			if _, err := os.Stat(filepath.Join(m.workspaceDir, ".venv")); err == nil {
				return m, func() tea.Msg {
					return setupmsg.PythonDecisionMsg{Setup: true, Version: m.pythonInfo.Version}
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
	venvPath := filepath.Join(m.workspaceDir, ".venv")
	_, venvExists := os.Stat(venvPath)

	if m.pythonInfo.Detected && venvExists == nil {
		b.WriteString(renderMarkdown(m.renderer, fmt.Sprintf(
			"🐍 Python 环境\n\n✅ **Python %s · 虚拟环境已就绪**\n\n虚拟环境用于隔离 Python 依赖，技能系统可正常使用。\n\n**Enter** 继续  **S** 跳过",
			m.pythonInfo.Version,
		)))
	} else if m.pythonInfo.Detected {
		md := fmt.Sprintf(`🐍 Python 环境

🟢 **Python %s** 已检测
🔴 **虚拟环境未创建**

虚拟环境用于隔离技能所需的 Python 依赖。
创建后将自动安装 skills/ 下所有 requirements.txt。
不创建则 Python 技能不可用，但核心对话功能正常。

是否创建虚拟环境?

%s

← → 切换  **Enter** 确认  **Esc** 退出`,
			m.pythonInfo.Version, yesNoIndicator(m.choice),
		)
		b.WriteString(renderMarkdown(m.renderer, md))
	} else {
		md := `🐍 Python 环境

🔴 **Python 未安装**

Python 是必需组件，技能系统依赖 Python 运行。

配置完成后将自动尝试安装 Python 3.12。

你也可以手动安装：

  1. 访问 python.org 下载 Python 3.10+
  2. 安装时勾选 "Add Python to PATH"
  3. 完成后重新运行配置向导

**Enter** 继续  **Esc** 跳过  **q** 退出`
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
