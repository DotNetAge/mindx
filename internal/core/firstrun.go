package core

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/goreact/core"
	"gopkg.in/yaml.v3"
)

const wizardWidth = 72

var (
	wizardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				MarginBottom(1)

	wizardLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94A3B8")).
				MarginBottom(1)

	wizardHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#64748B")).
				MarginTop(1)

	wizardBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7C3AED")).
				Padding(1, 2).
				Width(wizardWidth)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")).
			PaddingLeft(4).
			Width(wizardWidth - 10)

	infoKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B"))

	infoValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))
)

type modelItem struct {
	Name        string
	Description string
	BaseURL     string
}

type firstRunModel struct {
	step int

	cursor   int
	models   []modelItem
	offset   int
	maxVisible int

	apiKeyInput textinput.Model
	selectedModel modelItem

	err      error
	done     bool
	quitting bool

	modelsPath string
	agentsDir  string
	mindxConfig *MindxConfig
}

type FirstRunResult struct {
	SelectedModel string
	APIKey        string
	Err           error
}

func runFirstRunWizard(modelsPath, agentsDir string, mindxConfig *MindxConfig) FirstRunResult {
	modelList, err := parseModelsForWizard(modelsPath)
	if err != nil {
		return FirstRunResult{Err: fmt.Errorf("解析模型配置失败: %w", err)}
	}
	if len(modelList) == 0 {
		return FirstRunResult{Err: fmt.Errorf("模型配置文件中没有可用模型")}
	}

	ti := textinput.New()
	ti.Placeholder = "请输入 API Key..."
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.SetWidth(wizardWidth - 8)
	ti.Focus()

	m := &firstRunModel{
		step:        0,
		models:      modelList,
		maxVisible:  8,
		apiKeyInput: ti,
		modelsPath:  modelsPath,
		agentsDir:   agentsDir,
		mindxConfig: mindxConfig,
	}

	p := tea.NewProgram(m, tea.WithoutSignals())
	finalModel, err := p.Run()
	if err != nil {
		return FirstRunResult{Err: err}
	}

	fm := finalModel.(*firstRunModel)
	if fm.quitting {
		return FirstRunResult{Err: fmt.Errorf("用户取消配置")}
	}

	return FirstRunResult{
		SelectedModel: fm.selectedModel.Name,
		APIKey:        fm.apiKeyInput.Value(),
	}
}

func parseModelsForWizard(path string) ([]modelItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Models []core.ModelConfig `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var items []modelItem
	for _, m := range config.Models {
		desc := m.Description
		desc = strings.ReplaceAll(desc, "\n", " ")
		desc = strings.TrimSpace(desc)
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}
		items = append(items, modelItem{
			Name:        m.Name,
			Description: desc,
			BaseURL:     m.BaseURL,
		})
	}
	return items, nil
}

func (m *firstRunModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *firstRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		return m, tea.Quit
	}

	switch m.step {
	case 0:
		return m.updateModelSelect(msg)
	case 1:
		return m.updateAPIKeyInput(msg)
	}
	return m, nil
}

func (m *firstRunModel) updateModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxVisible {
					m.offset = m.cursor - m.maxVisible + 1
				}
			}
		case "enter":
			m.selectedModel = m.models[m.cursor]
			m.apiKeyInput.Placeholder = fmt.Sprintf("请输入 %s 的 API Key...", m.selectedModel.Name)
			m.apiKeyInput.Focus()
			m.step = 1
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *firstRunModel) updateAPIKeyInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.apiKeyInput.SetValue("")
			m.apiKeyInput.Blur()
			m.step = 0
			return m, nil
		case "enter":
			if m.apiKeyInput.Value() == "" {
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	return m, cmd
}

func (m *firstRunModel) renderModelSelect() string {
	var b strings.Builder

	b.WriteString(wizardTitleStyle.Render("🧠 MindX 首次配置"))
	b.WriteString("\n")
	b.WriteString(wizardLabelStyle.Render("选择默认模型:"))
	b.WriteString("\n\n")

	end := m.offset + m.maxVisible
	if end > len(m.models) {
		end = len(m.models)
	}

	for i := m.offset; i < end; i++ {
		model := m.models[i]
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▶ ")
			b.WriteString(selectedStyle.Render(prefix + model.Name))
		} else {
			b.WriteString(normalStyle.Render(prefix + model.Name))
		}
		b.WriteString("\n")
		b.WriteString(descStyle.Render(model.Description))
		b.WriteString("\n")
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(wizardHelpStyle.Render("↑↓ 选择  Enter 确认  Esc 退出"))

	return wizardBorderStyle.Render(b.String())
}

func (m *firstRunModel) renderAPIKeyInput() string {
	var b strings.Builder

	b.WriteString(wizardTitleStyle.Render("🧠 MindX 首次配置"))
	b.WriteString("\n\n")

	b.WriteString(infoKeyStyle.Render("模型:     "))
	b.WriteString(infoValStyle.Render(m.selectedModel.Name))
	b.WriteString("\n")

	baseURL := m.selectedModel.BaseURL
	if len(baseURL) > 50 {
		baseURL = baseURL[:50] + "..."
	}
	b.WriteString(infoKeyStyle.Render("Base URL: "))
	b.WriteString(infoValStyle.Render(baseURL))
	b.WriteString("\n\n")

	b.WriteString(wizardLabelStyle.Render("API Key:"))
	b.WriteString("\n")
	b.WriteString(m.apiKeyInput.View())
	b.WriteString("\n\n")
	b.WriteString(wizardHelpStyle.Render("Enter 确认  Esc 返回上一步"))

	return wizardBorderStyle.Render(b.String())
}

func (m *firstRunModel) View() tea.View {
	switch m.step {
	case 0:
		return tea.NewView(m.renderModelSelect())
	case 1:
		return tea.NewView(m.renderAPIKeyInput())
	}
	return tea.NewView("")
}

func ApplyFirstRunResult(result FirstRunResult, modelsPath, agentsDir string, mindxConfig *MindxConfig) error {
	if err := updateModelAPIKey(modelsPath, result.SelectedModel, result.APIKey); err != nil {
		return fmt.Errorf("更新模型 API Key 失败: %w", err)
	}

	if err := updateAllAgentsModel(agentsDir, result.SelectedModel); err != nil {
		return fmt.Errorf("更新 Agent 模型配置失败: %w", err)
	}

	mindxConfig.DefaultModel = result.SelectedModel
	mindxConfig.Initialized = true
	if err := mindxConfig.Save(); err != nil {
		return fmt.Errorf("保存 mindx.json 失败: %w", err)
	}

	return nil
}

func updateModelAPIKey(modelsPath, modelName, apiKey string) error {
	registry, err := goreact.LoadModels(modelsPath)
	if err != nil {
		return err
	}

	cfg := registry.Get(modelName)
	if cfg == nil {
		return fmt.Errorf("模型 %q 未在配置中找到", modelName)
	}

	cfg.APIKey = apiKey

	type modelsWrapper struct {
		Models []core.ModelConfig `yaml:"models"`
	}

	wrapper := modelsWrapper{}
	for _, m := range registry.List() {
		wrapper.Models = append(wrapper.Models, *m)
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("序列化模型配置失败: %w", err)
	}

	return os.WriteFile(modelsPath, data, 0644)
}

func updateAllAgentsModel(agentsDir, modelName string) error {
	registry, err := goreact.LoadAgentsFrom(agentsDir)
	if err != nil {
		return err
	}

	for _, agent := range registry.List() {
		agent.Model = modelName
		if err := registry.SaveTo(agent); err != nil {
			return fmt.Errorf("保存 Agent %q 模型配置失败: %w", agent.Name, err)
		}
	}

	return nil
}
