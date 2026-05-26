//go:build example

package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/dialog"
	"github.com/DotNetAge/mindx/internal/client/style"
)

var (
	sideBarStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Left: "│"}).
			BorderForeground(style.ThemeDim).
			PaddingLeft(1)

	lastResult string
)

type model struct {
	width  int
	height int

	leftWidth    int
	rightWidth   int
	mainVp       viewport.Model
	sideVp       viewport.Model

	providerDlg   *dialog.ListDialog
	apiKeyDlg     *dialog.InputDialog
	modelDlg      *dialog.ListDialog
	step         int
	selectedProvider string
	selectedProviderName string
	modelNames    []string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := e.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.leftWidth = m.width*3/4 - 1
		if m.leftWidth < 40 {
			m.leftWidth = m.width - 30
		}
		m.rightWidth = m.width - m.leftWidth - 1
		if m.rightWidth < 20 {
			m.rightWidth = 20
		}

		vpHeight := m.height - 6
		if vpHeight < 3 {
			vpHeight = 3
		}

		m.mainVp.SetWidth(m.leftWidth)
		m.mainVp.SetHeight(vpHeight)
		m.sideVp.SetWidth(m.rightWidth)
		m.sideVp.SetHeight(vpHeight)
		m.mainVp.SetContent(m.buildMainContent())
		m.mainVp.GotoBottom()
		m.sideVp.SetContent(m.buildSideContent())

		m.providerDlg.Update(msg)
		m.apiKeyDlg.Update(msg)
		m.modelDlg.Update(msg)
		return m, nil

	case tea.KeyPressMsg:
		if m.step != 0 {
			return m.handleOverlayKey(msg)
		}
		return m.handleNormalKey(msg)

	case dialog.ListDialogResult:
		switch m.step {
		case stepProvider:
			if !msg.Cancelled {
				providers := mockProviders()
				if msg.Index >= 0 && msg.Index < len(providers) {
					m.selectedProvider = providers[msg.Index].Title
					m.selectedProviderName = providers[msg.Index].Name
					lastResult = fmt.Sprintf("选择提供商: %s (id=%s)", providers[msg.Index].Title, providers[msg.Index].Name)
					m.step = stepAPIKey
					m.apiKeyDlg = dialog.NewInputDialog("API key", "API key")
					m.apiKeyDlg.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				}
			} else {
				m.step = stepNone
				lastResult = "已取消提供商选择"
			}
		case stepModel:
			if !msg.Cancelled && msg.Index >= 0 && msg.Index < len(m.modelNames) {
				realName := m.modelNames[msg.Index]
				lastResult = fmt.Sprintf("完成! 提供商=%s 模型=%s (id=%s)", m.selectedProvider, msg.Value, realName)
			} else {
				lastResult = "已取消模型选择"
			}
			m.step = stepNone
		}
		return m, nil

	case dialog.InputDialogResult:
		if m.step == stepAPIKey {
			if !msg.Cancelled && msg.Value != "" {
				lastResult = fmt.Sprintf("API Key 已输入: %s***", maskKey(msg.Value))
				displayNames, realNames := filterModelsByProvider(m.selectedProviderName)
				m.modelNames = realNames
				m.step = stepModel
				m.modelDlg = dialog.NewListDialog("选择模型")
				m.modelDlg.SetItems(displayNames)
				m.modelDlg.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
			} else {
				lastResult = "已取消 API Key 输入"
				m.step = stepNone
			}
		}
		return m, nil
	}

	newVp, vpCmd := m.mainVp.Update(e)
	m.mainVp = newVp
	if m.width > 0 {
		m.mainVp.SetContent(m.buildMainContent())
	}
	return m, vpCmd
}

const (
	stepNone = iota
	stepProvider
	stepAPIKey
	stepModel
)

func (m model) handleOverlayKey(e tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProvider:
		newDlg, cmd := m.providerDlg.Update(e)
		m.providerDlg = newDlg
		return m, cmd
	case stepAPIKey:
		newDlg, cmd := m.apiKeyDlg.Update(e)
		m.apiKeyDlg = newDlg
		return m, cmd
	case stepModel:
		newDlg, cmd := m.modelDlg.Update(e)
		m.modelDlg = newDlg
		return m, cmd
	}
	return m, nil
}

func (m model) handleNormalKey(e tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch e.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "p":
		m.step = stepProvider
		m.providerDlg = dialog.NewListDialog("连接提供商")
		providers := mockProviders()
		displayNames := make([]string, len(providers))
		for i, p := range providers {
			displayNames[i] = p.Title
		}
		m.providerDlg.SetItems(displayNames)
		m.providerDlg.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		return m, nil
	default:
		var vpCmd tea.Cmd
		newVp, c := m.mainVp.Update(e)
		m.mainVp = newVp
		vpCmd = c
		return m, vpCmd
	}
}

func (m model) View() tea.View {
	if m.width == 0 || m.rightWidth < 4 {
		return tea.NewView("初始化中...")
	}

	mainArea := m.mainVp.View()
	sideArea := sideBarStyle.Render(m.sideVp.View())

	layout := lipgloss.JoinHorizontal(lipgloss.Top, mainArea, sideArea)

	full := layout

	switch m.step {
	case stepProvider:
		modal := m.providerDlg.View()
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	case stepAPIKey:
		modal := m.apiKeyDlg.View()
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	case stepModel:
		modal := m.modelDlg.View()
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) buildMainContent() string {
	lines := []string{
		"  Provider Dialog 示例（Title 字段演示）",
		"",
		"  按 p 打开「连接提供商」对话框",
		"  流程: 选择提供商 → 输入 API Key → 选择模型",
		"",
		"  对话框显示 Title，内部使用 Name 作为标识符",
		"",
	}
	if lastResult != "" {
		lines = append(lines, "  "+style.GreenStyle.Render("✓ "+lastResult))
	}
	return strings.Join(lines, "\n")
}

func (m model) buildSideContent() string {
	parts := []string{
		style.DimStyle.Render("  当前步骤: ") + style.WhiteStyle.Render(stepLabel(m.step)),
		"",
		style.DimStyle.Render(strings.Repeat("─", max(m.rightWidth-4, 4))),
		"",
		style.BoldWhite.Render("  可用提供商 (Title / Name)"),
	}
	for _, p := range mockProviders() {
		label := p.Title
		if label != p.Name {
			label += style.DimStyle.Render(" (" + p.Name + ")")
		}
		parts = append(parts, style.GrayStyle.Render("  · "+label))
	}
	return strings.Join(parts, "\n")
}

func stepLabel(step int) string {
	switch step {
	case stepNone:
		return "空闲"
	case stepProvider:
		return "选择提供商"
	case stepAPIKey:
		return "输入 API Key"
	case stepModel:
		return "选择模型"
	default:
		return "?"
	}
}

type mockItem struct {
	Name  string
	Title string
}

type mockModelItem struct {
	Name  string
	Title string
}

func mockProviders() []mockItem {
	return []mockItem{
		{Name: "openai", Title: "OpenAI"},
		{Name: "anthropic", Title: "Anthropic"},
		{Name: "google", Title: "Google"},
		{Name: "deepseek", Title: "DeepSeek"},
		{Name: "opencode-zen", Title: "OpenCode Zen"},
		{Name: "github-copilot", Title: "GitHub Copilot"},
		{Name: "auriko", Title: "Auriko"},
		{Name: "fireworks", Title: "Fireworks"},
		{Name: "groq", Title: "Groq"},
		{Name: "mistral", Title: "Mistral"},
		{Name: "together-ai", Title: "Together AI"},
		{Name: "z-ai", Title: "Z.AI"},
	}
}

func mockModels() map[string][]mockModelItem {
	return map[string][]mockModelItem{
		"openai": {
			{Name: "gpt-4o", Title: "GPT-4o"},
			{Name: "gpt-4o-mini", Title: "GPT-4o Mini"},
			{Name: "gpt-4-turbo", Title: "GPT-4 Turbo"},
			{Name: "o1", Title: "o1"},
			{Name: "o3-mini", Title: "o3-mini"},
		},
		"anthropic": {
			{Name: "claude-opus-4", Title: "Claude Opus 4"},
			{Name: "claude-sonnet-4", Title: "Claude Sonnet 4"},
			{Name: "claude-haiku-3.5", Title: "Claude Haiku 3.5"},
		},
		"google": {
			{Name: "gemini-2.5-pro", Title: "Gemini 2.5 Pro"},
			{Name: "gemini-2.5-flash", Title: "Gemini 2.5 Flash"},
			{Name: "gemini-2.0-flash", Title: "Gemini 2.0 Flash"},
		},
		"deepseek": {
			{Name: "deepseek-v3", Title: "DeepSeek V3"},
			{Name: "deepseek-r1", Title: "DeepSeek R1"},
			{Name: "deepseek-r1-distill", Title: "DeepSeek R1 Distill"},
		},
		"opencode-zen": {{Name: "opencode-default", Title: "OpenCode Default"}},
		"github-copilot": {
			{Name: "copilot-gpt-4", Title: "Copilot GPT-4"},
			{Name: "copilot-gpt-4-mini", Title: "Copilot GPT-4 Mini"},
		},
		"auriko": {
			{Name: "auriko-chat", Title: "Auriko Chat"},
			{Name: "auriko-code", Title: "Auriko Code"},
		},
		"fireworks": {
			{Name: "fireworks-llama-4", Title: "Fireworks Llama 4"},
			{Name: "fireworks-qwen-2.5", Title: "Fireworks Qwen 2.5"},
		},
		"groq": {
			{Name: "llama-3.3-70b", Title: "Llama 3.3 70B"},
			{Name: "mixtral-8x7b", Title: "Mixtral 8x7B"},
			{Name: "gemma2-9b", Title: "Gemma 2 9B"},
		},
		"mistral": {
			{Name: "mistral-large", Title: "Mistral Large"},
			{Name: "mistral-medium", Title: "Mistral Medium"},
			{Name: "mistral-small", Title: "Mistral Small"},
		},
		"together-ai": {
			{Name: "meta-llama-4", Title: "Meta Llama 4"},
			{Name: "qwen-qwq-32b", Title: "Qwen QwQ 32B"},
		},
		"z-ai": {
			{Name: "zai-coder", Title: "Z.AI Coder"},
			{Name: "zai-chat", Title: "Z.AI Chat"},
		},
	}
}

func filterModelsByProvider(providerName string) ([]string, []string) {
	all := mockModels()
	if models, ok := all[providerName]; ok {
		var display, names []string
		for _, m := range models {
			display = append(display, m.Title)
			names = append(names, m.Name)
		}
		return display, names
	}
	return nil, nil
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:2] + strings.Repeat("*", min(len(key)-4, 6)) + key[len(key)-2:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	p := tea.NewProgram(model{
		providerDlg: dialog.NewListDialog("连接提供商"),
		apiKeyDlg:   dialog.NewInputDialog("API key", "API key"),
		modelDlg:    dialog.NewListDialog("选择模型"),
		mainVp:      viewport.New(),
		sideVp:      viewport.New(),
	})

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
