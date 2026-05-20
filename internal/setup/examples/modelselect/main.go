package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/modelselect"
	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
)

type model struct {
	modelSelect *modelselect.Model
	selected    *setupdata.ModelItem
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.modelSelect, cmd = m.modelSelect.Update(msg)

	if sel := m.modelSelect.SelectedItem(); sel != nil && (m.selected == nil || sel.Name != m.selected.Name) {
		m.selected = sel
	}
	return m, cmd
}

func (m model) View() tea.View {
	view := m.modelSelect.View()
	if m.selected != nil {
		view += fmt.Sprintf("\n\n选中模型: %s (%s)", m.selected.Name, m.selected.Desc)
	}
	return tea.NewView(view + "\n\n按 q 退出")
}

func main() {
	items := []setupdata.ModelItem{
		{Name: "GPT-4o", Desc: "OpenAI 最新多模态模型", BaseURL: "https://api.openai.com/v1", CredRef: "openai"},
		{Name: "Claude-3.5-Sonnet", Desc: "Anthropic 高性能模型", BaseURL: "https://api.anthropic.com/v1", CredRef: "anthropic"},
		{Name: "Gemini-Pro", Desc: "Google 多模态模型", BaseURL: "https://generativelanguage.googleapis.com/v1beta", CredRef: "google"},
		{Name: "Qwen-Turbo", Desc: "阿里云通义千问高速版", BaseURL: "https://dashscope.aliyuncs.com/api/v1", CredRef: "qwen"},
		{Name: "DeepSeek-V3", Desc: "DeepSeek 高性能推理模型", BaseURL: "https://api.deepseek.com/v1", CredRef: "deepseek"},
	}

	m := model{
		modelSelect: modelselect.New(items, true),
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
