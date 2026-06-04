package apikeyinput

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	input     textinput.Model
	modelName string
	skipMode  bool
	width     int
	height    int
	renderer  *glamour.TermRenderer
}

func New(modelName string, skipMode bool) *Model {
	ti := textinput.New()
	ti.Placeholder = fmt.Sprintf("请输入 %s 的 API Key...", modelName)
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.Focus()

	return &Model{
		input:     ti,
		modelName: modelName,
		skipMode:  skipMode,
		width:     80,
		height:    24,
		renderer:  initGlamour(60),
	}
}

func (m *Model) SetModelName(name string) {
	m.modelName = name
	m.input.Placeholder = fmt.Sprintf("请输入 %s 的 API Key...", name)
}

func (m *Model) Value() string { return m.input.Value() }

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
		cw := contentWidth(m.width)
		m.input.SetWidth(cw - 8)
		m.renderer = initGlamour(cw)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.input.SetValue("")
			m.input.Blur()
			return m, func() tea.Msg { return setupmsg.StepPrevMsg{} }
		case "enter":
			if m.input.Value() == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return setupmsg.APIKeySubmittedMsg{Key: m.input.Value()}
			}
		case "s", "S":
			if m.skipMode {
				return m, func() tea.Msg {
					return setupmsg.APIKeySubmittedMsg{Key: ""}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(renderMarkdown(m.renderer, "API Key 配置\n\n"))
	b.WriteString(fmt.Sprintf("模型: **%s**\n\n", m.modelName))
	b.WriteString("输入你的 API Key：\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	help := "**Enter** 确认  **Esc** 返回上一步"
	if m.skipMode {
		help = "**Enter** 确认  **S** 跳过 (使用已有 Key)  **Esc** 返回上一步"
	}
	b.WriteString(renderMarkdown(m.renderer, help))
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle(""),
		"",
		content,
	) + "\n"
}
