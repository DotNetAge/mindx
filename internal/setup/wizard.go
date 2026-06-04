package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	goreactconfig "github.com/DotNetAge/goreact/config"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/setup/style"

	"github.com/DotNetAge/mindx/internal/core"
)

const minContentWidth = 60

var borderStyle = lipgloss.NewStyle().Padding(1, 2)

type providerItem struct {
	Name        string
	DisplayName string
}

func (i providerItem) Title() string       { return i.DisplayName }
func (i providerItem) Description() string { return i.Name }
func (i providerItem) FilterValue() string { return i.DisplayName }

type modelItem struct {
	Name        string
	desc        string
	BaseURL     string
	CredRef     string
	Provider    string
}

func (i modelItem) Title() string       { return i.Name }
func (i modelItem) Description() string { return i.desc }
func (i modelItem) FilterValue() string { return i.Name }

type daemonInstallMsg struct {
	err error
}

type firstRunModel struct {
	step int

	providerList   list.Model
	providers      []providerItem
	selectedProvider providerItem

	modelList list.Model
	models    []modelItem

	apiKeyInput   textinput.Model
	selectedModel modelItem

	err      error
	done     bool
	quitting bool

	modelsPath  string
	agentsDir   string
	mindxConfig *core.MindxConfig

	daemonChoice    bool
	daemonSubmitted bool
	daemonInstallCh   chan error
	daemonInstallErr  error
	daemonState       int // 0=choice, 1=installing, 2=done

	pythonChoice    bool
	pythonSubmitted bool
	pythonDetected  bool
	pythonVersion   string
	pythonInfo      core.PythonConfig

	memoryState     int
	embedderModel   string
	workspaceDir    string

	pathChoice    bool
	pathSubmitted bool
	installDir    string
	pathInPath    bool

	webUIReady     bool
	webUISubmitted bool

	modelConfigured  bool
	apiKeyConfigured bool

	// preResolvedKeys: 从环境变量预解析的 Provider API Key（provider name -> actual key value）
	preResolvedKeys map[string]string

	width  int
	height int

	renderer *glamour.TermRenderer
}

type firstRunResult struct {
	SelectedProvider string
	SelectedModel    string
	APIKey           string
	ResolvedKeys     map[string]string // 所有从环境变量预解析的非空 Provider Key
	Err              error

	DaemonSetup    bool
	PythonSetup    bool
	PythonInfo     core.PythonConfig
	EmbedderModel  string
	PathSetup      bool
	WebUIReady     bool
}

func (m *firstRunModel) contentWidth() int {
	if m.width > minContentWidth {
		cw := m.width - 4
		if cw > 80 {
			cw = 80
		}
		return cw
	}
	return minContentWidth
}

func (m *firstRunModel) paddedView(content string) string {
	lines := strings.Count(content, "\n") + 1
	if m.height > lines+1 {
		return content + strings.Repeat("\n", m.height-lines)
	}
	return content + "\n"
}

func (m *firstRunModel) renderMarkdown(src string) string {
	if m.renderer == nil {
		return src
	}
	out, err := m.renderer.Render(src)
	if err != nil {
		return src
	}
	return out
}

func (m *firstRunModel) yesNoIndicator(yes bool) string {
	if yes {
		return "**> Yes**  \n  No"
	}
	return "  Yes  \n**> No**"
}

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

// resolveProviderKeysFromEnv 从 providers.yml 读取所有供应商的 api_key 字段（作为环境变量名），
// 尝试从系统环境变量中解析出实际值，返回 map[providerName]actualKeyValue。
func resolveProviderKeysFromEnv(providersPath string) (map[string]string, error) {
	data, err := os.ReadFile(providersPath)
	if err != nil {
		return nil, err
	}

	var provConfig struct {
		Providers []goreactconfig.ProviderConfig `yaml:"providers"`
	}
	if err := yaml.Unmarshal(data, &provConfig); err != nil {
		return nil, err
	}

	keys := make(map[string]string)
	for _, p := range provConfig.Providers {
		if p.APIKey == "" {
			keys[p.Name] = ""
			continue
		}
		// api_key 字段存的是环境变量名，尝试读取实际值
		if v := os.Getenv(p.APIKey); v != "" {
			keys[p.Name] = v
		} else {
			keys[p.Name] = ""
		}
	}
	return keys, nil
}

func runFirstRunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, mindxConfig *core.MindxConfig) firstRunResult {
	providerList, modelList, err := parseProviderAndModels(providersPath, modelsPath)
	if err != nil {
		return firstRunResult{Err: fmt.Errorf("解析配置失败: %w", err)}
	}
	if len(providerList) == 0 {
		return firstRunResult{Err: fmt.Errorf("配置文件中没有可用提供商")}
	}

	ti := textinput.New()
	ti.Placeholder = "请输入 API Key..."
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.Focus()

	pd := list.NewDefaultDelegate()
	pd.ShowDescription = false
	pd.SetSpacing(0)
	pd.SetHeight(1)

	md := list.NewDefaultDelegate()
	md.ShowDescription = true
	md.SetSpacing(0)
	md.SetHeight(2)

	var provItems []list.Item
	for _, p := range providerList {
		provItems = append(provItems, p)
	}
	pl := list.New(provItems, pd, minContentWidth-4, 8)
	pl.SetShowStatusBar(false)
	pl.SetShowPagination(false)
	pl.SetShowTitle(false)
	pl.SetFilteringEnabled(false)

	var modelItems []list.Item
	for _, m := range modelList {
		modelItems = append(modelItems, m)
	}
	ml := list.New(modelItems, md, minContentWidth-4, 8)
	ml.SetShowStatusBar(false)
	ml.SetShowPagination(false)
	ml.SetShowTitle(false)
	ml.SetFilteringEnabled(false)

	pythonInfo := DetectPython()

	m := &firstRunModel{
		step:           0,
		providerList:   pl,
		providers:      providerList,
		modelList:      ml,
		models:         modelList,
		apiKeyInput:    ti,
		modelsPath:     modelsPath,
		agentsDir:      agentsDir,
		mindxConfig:    mindxConfig,
		pythonDetected: pythonInfo.Detected,
		pythonVersion:  pythonInfo.Version,
		pythonInfo:     pythonInfo,
		memoryState:    0,
		workspaceDir:   workspaceDir,
		pathChoice:     true,
		renderer:       initGlamour(minContentWidth),
		width:          80,
		height:         24,
	}

	// 预解析所有 Provider 的 API Key（从环境变量读取）
	if preKeys, err := resolveProviderKeysFromEnv(providersPath); err == nil {
		m.preResolvedKeys = preKeys
	}

	m.modelConfigured = mindxConfig.DefaultModel != ""

	if m.modelConfigured {
		credStore := core.NewCredentialStore(workspaceDir)
		for _, p := range providerList {
			if key, err := credStore.Get(p.Name); err == nil && key != "" {
				m.apiKeyConfigured = true
				break
			}
		}
	}

	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			m.installDir = filepath.Dir(exe)
			m.pathInPath = CheckInPath(m.installDir)
		}
		m.pathChoice = m.pathInPath
	}

	m.daemonChoice = DaemonInstalled(workspaceDir)

	m.pythonChoice = true

	modelPath := filepath.Join(workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		m.memoryState = 2
		m.embedderModel = "model_q4.onnx"
	}

	webDirPath := filepath.Join(workspaceDir, "web")
	if _, err := os.Stat(webDirPath); err == nil {
		m.webUIReady = true
	}

	p := tea.NewProgram(m, tea.WithoutSignals())
	finalModel, err := p.Run()
	if err != nil {
		return firstRunResult{Err: err}
	}

	fm := finalModel.(*firstRunModel)
	if fm.quitting {
		return firstRunResult{Err: fmt.Errorf("用户取消配置")}
	}

	return firstRunResult{
		SelectedProvider: fm.selectedProvider.Name,
		SelectedModel:    fm.selectedModel.Name,
		APIKey:           fm.apiKeyInput.Value(),
		ResolvedKeys:     fm.preResolvedKeys,
		DaemonSetup:      fm.daemonChoice,
		PythonSetup:      fm.pythonChoice,
		PythonInfo:       fm.pythonInfo,
		EmbedderModel:    fm.embedderModel,
		PathSetup:        fm.pathChoice,
		WebUIReady:       fm.webUIReady,
	}
}

func parseProviderAndModels(providersPath, modelsPath string) ([]providerItem, []modelItem, error) {
	var providerItems []providerItem

	// Load providers from providers.yml (if exists)
	if data, err := os.ReadFile(providersPath); err == nil {
		var provConfig struct {
			Providers []goreactconfig.ProviderConfig `yaml:"providers"`
		}
		if err := yaml.Unmarshal(data, &provConfig); err == nil {
			for _, p := range provConfig.Providers {
				title := p.Title
				if title == "" {
					title = p.Name
				}
				providerItems = append(providerItems, providerItem{Name: p.Name, DisplayName: title})
			}
		}
	}

	// Fallback: if no providers found in providers.yml, try to extract from models.yml
	if len(providerItems) == 0 {
		data, err := os.ReadFile(modelsPath)
		if err != nil {
			return nil, nil, err
		}

		var config struct {
			Providers []goreactconfig.ProviderConfig `yaml:"providers"`
			Models    []goreactconfig.ModelConfig    `yaml:"models"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, nil, err
		}

		for _, p := range config.Providers {
			title := p.Title
			if title == "" {
				title = p.Name
			}
			providerItems = append(providerItems, providerItem{Name: p.Name, DisplayName: title})
		}
	}

	// Load models from models.yml
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		return nil, nil, err
	}

	var modelConfig struct {
		Models []goreactconfig.ModelConfig `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &modelConfig); err != nil {
		return nil, nil, err
	}

	var items []modelItem
	for _, m := range modelConfig.Models {
		desc := strings.TrimSpace(m.Description)
		items = append(items, modelItem{
			Name:     m.Name,
			desc:     desc,
			BaseURL:  m.BaseURL,
			CredRef:  m.APIKey,
			Provider: m.Provider,
		})
	}
	return providerItems, items, nil
}

func (m *firstRunModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *firstRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := m.contentWidth()
		m.apiKeyInput.SetWidth(cw - 8)
		m.renderer = initGlamour(cw)
		m.modelList.SetWidth(cw - 4)
	}

	switch m.step {
	case 0:
		return m.updateProviderSelect(msg)
	case 1:
		return m.updateAPIKeyInput(msg)
	case 2:
		return m.updateModelSelect(msg)
	case 3:
		return m.updateDaemonCheck(msg)
	case 4:
		return m.updatePythonCheck(msg)
	case 5:
		return m.updateMemoryConfig(msg)
	case 6:
		return m.updatePathSetup(msg)
	case 7:
		return m.updateWebUIComplete(msg)
	}
	return m, nil
}

func (m *firstRunModel) updateProviderSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item := m.providerList.SelectedItem(); item != nil {
				if pi, ok := item.(providerItem); ok {
					m.selectedProvider = pi
					// 检查是否已有预解析的环境变量 Key 或 CredentialStore 中的 Key
					if preKey, hasPre := m.preResolvedKeys[pi.Name]; hasPre && preKey != "" {
						m.apiKeyInput.SetValue(preKey)
						m.step = 2
						return m, nil
					}
					if m.apiKeyConfigured {
						m.step = 2
						return m, nil
					}
					m.apiKeyInput.Placeholder = fmt.Sprintf("请输入 %s 的 API Key...", pi.Title())
					m.apiKeyInput.Focus()
					m.step = 1
					return m, textinput.Blink
				}
			}
		case "s", "S":
			if m.apiKeyConfigured {
				// Skip to model selection if API key already configured
				if item := m.providerList.SelectedItem(); item != nil {
					if pi, ok := item.(providerItem); ok {
						m.selectedProvider = pi
						m.step = 2
						return m, nil
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.providerList, cmd = m.providerList.Update(msg)
	return m, cmd
}

func (m *firstRunModel) updateModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Build filtered model list for the selected provider
	var filtered []list.Item
	for _, mi := range m.models {
		if mi.Provider == m.selectedProvider.Name {
			filtered = append(filtered, mi)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.step = 1
			return m, nil
		case "enter":
			if item := m.modelList.SelectedItem(); item != nil {
				if mi, ok := item.(modelItem); ok {
					m.selectedModel = mi
					m.step = 3
					return m, nil
				}
			}
		case "s", "S":
			if m.modelConfigured {
				if item := m.modelList.SelectedItem(); item != nil {
					if mi, ok := item.(modelItem); ok {
						m.selectedModel = mi
						m.step = 3
						return m, nil
					}
				}
			}
		}
	}

	m.modelList.SetItems(filtered)
	var cmd tea.Cmd
	m.modelList, cmd = m.modelList.Update(msg)
	return m, cmd
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
			m.step = 2
			return m, nil
		case "s", "S":
			if m.apiKeyConfigured {
				m.step = 2
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	return m, cmd
}

func (m *firstRunModel) updateDaemonCheck(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "left", "right":
			if m.daemonState == 0 {
				m.daemonChoice = !m.daemonChoice
			}
		case "enter":
			if m.daemonState == 0 {
				if DaemonInstalled(m.workspaceDir) {
					m.step = 4
					return m, nil
				}
				if m.daemonChoice {
					m.daemonState = 1
					m.daemonInstallCh = make(chan error, 1)
					ch := m.daemonInstallCh
					wd := m.workspaceDir
					go func() {
						ch <- SetupDaemon(wd)
					}()
					return m, m.listenDaemonInstallCmd()
				}
				m.step = 4
				return m, nil
			}
			if m.daemonState == 2 {
				m.step = 4
				return m, nil
			}
		case "s", "S":
			if DaemonInstalled(m.workspaceDir) {
				m.step = 4
				return m, nil
			}
		}
	case daemonInstallMsg:
		m.daemonInstallErr = msg.err
		m.daemonState = 2
		return m, nil
	}
	return m, nil
}

func (m *firstRunModel) listenDaemonInstallCmd() tea.Cmd {
	return func() tea.Msg {
		err := <-m.daemonInstallCh
		return daemonInstallMsg{err: err}
	}
}

func (m *firstRunModel) updatePythonCheck(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if m.pythonDetected {
				m.quitting = true
				return m, tea.Quit
			}
			m.pythonChoice = false
			m.pythonSubmitted = true
			m.step = 5
			return m, nil
		case "left", "right":
			if m.pythonDetected {
				m.pythonChoice = !m.pythonChoice
			}
		case "enter":
			if !m.pythonDetected {
				m.pythonChoice = true
				m.pythonSubmitted = true
				m.step = 5
				return m, nil
			}
			m.pythonSubmitted = true
			m.step = 5
			return m, nil
		case "s", "S":
			if _, err := os.Stat(filepath.Join(m.workspaceDir, ".venv")); err == nil {
				m.pythonSubmitted = true
				m.step = 5
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *firstRunModel) updateMemoryConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ", "s", "S":
			if runtime.GOOS == "windows" {
				m.step = 6
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *firstRunModel) updatePathSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "left", "right":
			if !m.pathInPath {
				m.pathChoice = !m.pathChoice
			}
		case "enter":
			m.pathSubmitted = true
			m.step = 7
			return m, nil
		case "s", "S":
			if m.pathInPath {
				m.pathSubmitted = true
				m.step = 7
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *firstRunModel) updateWebUIComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ", "s", "S":
			m.webUISubmitted = true
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *firstRunModel) renderProviderSelect() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown("选择提供商\n\n请选择一个 AI 提供商。\n\n"))
	b.WriteString(m.providerList.View())
	b.WriteString("\n")
	help := "↑↓ 选择  **Enter** 确认  **Esc** 退出"
	if m.apiKeyConfigured {
		help = "↑↓ 选择  **Enter** 确认  **S** 跳过 (使用已有 Key)  **Esc** 退出"
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderAPIKeyInput() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown("API Key 配置\n\n"))
	b.WriteString(fmt.Sprintf("提供商: **%s**\n\n", m.selectedProvider.DisplayName))
	b.WriteString("输入你的 API Key：\n\n")
	b.WriteString(m.apiKeyInput.View())
	b.WriteString("\n\n")
	help := "**Enter** 确认  **Esc** 返回上一步"
	if m.apiKeyConfigured {
		help = "**Enter** 确认  **S** 跳过 (使用已有 Key)  **Esc** 返回上一步"
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderModelSelect() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown(fmt.Sprintf("选择模型\n\n提供商: **%s**\n\n请选择一个 AI 模型作为默认对话模型。\n\n", m.selectedProvider.DisplayName)))
	b.WriteString(m.modelList.View())
	b.WriteString("\n")
	help := "↑↓ 选择  **Enter** 确认  **Esc** 返回上一步"
	if m.modelConfigured {
		help = "↑↓ 选择  **Enter** 确认  **S** 跳过 (使用已有配置)  **Esc** 返回上一步"
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderDaemonCheck() string {
	var b strings.Builder
	if m.daemonState == 1 {
		b.WriteString(m.renderMarkdown("⚙️ **正在注册 Daemon 自启动服务...**\n\n请稍候..."))
		return m.paddedView(borderStyle.Render(b.String()))
	}
	if m.daemonState == 2 {
		if m.daemonInstallErr != nil {
			b.WriteString(m.renderMarkdown(fmt.Sprintf(
				"⚙️ Daemon 后台服务\n\n❌ **安装失败**\n\n错误: %s\n\n你可以稍后运行 `mindx doctor` 重新配置。\n\n**Enter** 继续",
				m.daemonInstallErr.Error(),
			)))
		} else {
			restartHint := ""
			if runtime.GOOS == "windows" {
				restartHint = "\n\nDaemon 已成功安装到系统，请**重新启动电脑**使其生效。"
			}
			b.WriteString(m.renderMarkdown(fmt.Sprintf(
				"⚙️ Daemon 后台服务\n\n✅ **安装完成**\n\nDaemon 已注册为开机自启动服务。%s\n\n**Enter** 继续", restartHint,
			)))
		}
		return m.paddedView(borderStyle.Render(b.String()))
	}
	installed := DaemonInstalled(m.workspaceDir)
	if installed {
		restartHint := ""
		if runtime.GOOS == "windows" {
			restartHint = "\n\nDaemon 已成功安装到系统，请**重新启动电脑**使其生效。"
		}
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
			"⚙️ Daemon 后台服务\n\n✅ **已安装**\n\nDaemon 已注册为开机自启动服务。%s\n\n**Enter** 继续  **S** 跳过", restartHint,
		)))
	} else {
		md := `⚙️ Daemon 后台服务

🔴 **未安装**

Daemon 是 MindX 的核心服务，提供多 Agent 协作与友好的 Web 界面，
如跳过则只能使用基于命令行的 TUI。

未安装不影响本地对话，但以下功能不可用：
  - 多 Agent 协作调度
  - WebSocket 远程连接
  - 系统托盘常驻

是否注册为开机自启动服务?

` + m.yesNoIndicator(m.daemonChoice) + `

← → 切换  **Enter** 确认  **Esc** 退出`
		b.WriteString(m.renderMarkdown(md))
	}
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderPythonCheck() string {
	var b strings.Builder
	venvPath := filepath.Join(m.workspaceDir, ".venv")
	_, venvExists := os.Stat(venvPath)

	if m.pythonDetected && venvExists == nil {
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
			"🐍 Python 环境\n\n✅ **Python %s · 虚拟环境已就绪**\n\n虚拟环境用于隔离 Python 依赖，技能系统可正常使用。\n\n**Enter** 继续  **S** 跳过",
			m.pythonVersion,
		)))
	} else if m.pythonDetected {
		md := fmt.Sprintf(`🐍 Python 环境

🟢 **Python %s** 已检测
🔴 **虚拟环境未创建**

虚拟环境用于隔离技能所需的 Python 依赖。
创建后将自动安装 skills/ 下所有 requirements.txt。
不创建则 Python 技能不可用，但核心对话功能正常。

是否创建虚拟环境?

%s

← → 切换  **Enter** 确认  **Esc** 退出`,
			m.pythonVersion, m.yesNoIndicator(m.pythonChoice),
		)
		b.WriteString(m.renderMarkdown(md))
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
		b.WriteString(m.renderMarkdown(md))
	}
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderMemoryConfig() string {
	md := `💾 记忆体配置

✅ **Embedder 模型已内嵌**

Chinese-CLIP (model_q4.onnx) 已打包在程序内部，
启动时自动释放到工作目录，无需单独下载。

记忆体功能默认启用，支持语义搜索和 RAG 跨会话检索。

**Enter** 继续  **S** 跳过`
	return m.paddedView(borderStyle.Render(m.renderMarkdown(md)))
}

func (m *firstRunModel) renderPathSetup() string {
	var b strings.Builder
	if m.pathInPath {
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
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
			m.installDir, m.yesNoIndicator(m.pathChoice),
		)
		b.WriteString(m.renderMarkdown(md))
	}
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderWebUIComplete() string {
	var items []string

	// Provider
	if m.selectedProvider.Name != "" {
		items = append(items, fmt.Sprintf("✅ 提供商 · **%s**", m.selectedProvider.DisplayName))
	}

	// Model
	if m.selectedModel.Name != "" {
		items = append(items, fmt.Sprintf("✅ 模型 · **%s**", m.selectedModel.Name))
	}

	// Daemon - re-check
	if DaemonInstalled(m.workspaceDir) {
		items = append(items, "✅ Daemon · 已注册为开机自启动服务")
	} else if m.daemonState == 2 && m.daemonInstallErr != nil {
		items = append(items, "❌ Daemon · 安装失败")
	} else {
		items = append(items, "⏭️ Daemon · 未安装")
	}

	// Python + venv - re-check
	venvPath := filepath.Join(m.workspaceDir, ".venv")
	_, venvExists := os.Stat(venvPath)
	if m.pythonDetected {
		if venvExists == nil {
			items = append(items, fmt.Sprintf("✅ Python · %s · 虚拟环境已创建", m.pythonVersion))
		} else {
			items = append(items, fmt.Sprintf("✅ Python · %s (未创建虚拟环境)", m.pythonVersion))
		}
	} else {
		items = append(items, "❌ Python · 未检测到")
	}

	// Embedder model - re-check
	modelPath := filepath.Join(m.workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		items = append(items, "✅ Embedder · Chinese-CLIP 模型已就绪")
	} else {
		items = append(items, "⏭️ Embedder · 待启动时自动释放")
	}

	// PATH - re-check (Windows only)
	if runtime.GOOS == "windows" && m.installDir != "" {
		if CheckInPath(m.installDir) {
			items = append(items, "✅ PATH · 已添加到系统 PATH")
		} else if m.pathChoice {
			items = append(items, "❌ PATH · 添加失败")
		} else {
			items = append(items, "⏭️ PATH · 未配置")
		}
	}

	// WebUI
	webDir := filepath.Join(m.workspaceDir, "web")
	if _, err := os.Stat(webDir); err == nil {
		items = append(items, "✅ WebUI · 资源文件已就绪")
	} else {
		items = append(items, "❌ WebUI · 资源文件未检测到")
	}

	// Build markdown output
	var b strings.Builder
	b.WriteString("🎉 **配置完成！**\n\n")
	b.WriteString("安装状态：\n\n")
	for _, item := range items {
		b.WriteString(item + "\n\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("**使用方式：**\n\n")
	b.WriteString("1. **终端 TUI**：直接运行 `mindx` 进入命令行界面\n\n")
	b.WriteString("2. **浏览器 WebUI**：访问 http://localhost:1313\n\n")
	if runtime.GOOS == "windows" {
		b.WriteString("💡 Windows 用户请**重新启动终端**后使用 `mindx` 命令\n\n")
	}
	b.WriteString("**Enter** 完成 退出向导")

	return m.paddedView(borderStyle.Render(m.renderMarkdown(b.String())))
}

func (m *firstRunModel) View() tea.View {
	content := ""
	switch m.step {
	case 0:
		content = m.renderProviderSelect()
	case 1:
		content = m.renderAPIKeyInput()
	case 2:
		content = m.renderModelSelect()
	case 3:
		content = m.renderDaemonCheck()
	case 4:
		content = m.renderPythonCheck()
	case 5:
		content = m.renderMemoryConfig()
	case 6:
		content = m.renderPathSetup()
	case 7:
		content = m.renderWebUIComplete()
	}
	return tea.NewView(style.GradientTitle("") + "\n\n" + content)
}
