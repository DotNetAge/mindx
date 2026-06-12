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
	goharnessconfig "github.com/DotNetAge/goharness/config"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/setup/style"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
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
	Name     string
	desc     string
	BaseURL  string
	CredRef  string
	Provider string
	Enabled  bool
}

func (i modelItem) Title() string       { return i.Name }
func (i modelItem) Description() string { return i.desc }
func (i modelItem) FilterValue() string { return i.Name }

type daemonInstallMsg struct {
	err error
}

type firstRunModel struct {
	step int

	providerList     list.Model
	providers        []providerItem
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

	daemonChoice     bool
	daemonInstallCh  chan error
	daemonInstallErr error
	daemonState      int // 0=choice, 1=installing, 2=done

	pythonChoice    bool
	pythonSubmitted bool
	pythonDetected  bool
	pythonVersion   string
	pythonInfo      core.PythonConfig

	memoryState   int
	embedderModel string
	workspaceDir  string

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

	DaemonSetup   bool
	PythonSetup   bool
	PythonInfo    core.PythonConfig
	EmbedderModel string
	PathSetup     bool
	WebUIReady    bool
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
	// Reserve 3 lines for gradient title (1) + two newlines (2) added in View().
	// Without this, the title gets truncated by Bubble Tea's renderer when
	// the frame taller than the terminal height.
	availableHeight := m.height - 3
	if availableHeight > lines+1 {
		return content + strings.Repeat("\n", availableHeight-lines)
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
		return i18n.T("wizard.yesno.yes")
	}
	return i18n.T("wizard.yesno.no")
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

// resolveAllProviderKeys 从 providers.yml 读取所有供应商的 api_key 字段（作为环境变量名），
// 依次尝试从系统环境变量和 CredentialStore 中解析实际值，返回 map[providerName]actualKeyValue。
func resolveAllProviderKeys(providersPath, workspaceDir string) (map[string]string, error) {
	data, err := os.ReadFile(providersPath)
	if err != nil {
		return nil, err
	}

	var provConfig struct {
		Providers []goharnessconfig.ProviderConfig `yaml:"providers"`
	}
	if err := yaml.Unmarshal(data, &provConfig); err != nil {
		return nil, err
	}

	credStore := core.NewCredentialStore(workspaceDir)
	keys := make(map[string]string)
	for _, p := range provConfig.Providers {
		if p.APIKey == "" {
			keys[p.Name] = ""
			continue
		}
		// api_key 字段存的是环境变量名，尝试读取实际值
		if v := os.Getenv(p.APIKey); v != "" {
			keys[p.Name] = v
		} else if credStore != nil {
			// 尝试从 CredentialStore 读取
			if v, err := credStore.Get(p.Name); err == nil && v != "" {
				keys[p.Name] = v
			} else {
				keys[p.Name] = ""
			}
		} else {
			keys[p.Name] = ""
		}
	}
	return keys, nil
}

func runFirstRunWizard(modelsPath, providersPath, agentsDir, workspaceDir string, mindxConfig *core.MindxConfig) firstRunResult {
	// 提前解析所有 Provider 的 API Key（环境变量 + CredentialStore）
	providerKeys, _ := resolveAllProviderKeys(providersPath, workspaceDir)

	providerList, modelList, err := parseProviderAndModels(providersPath, modelsPath, providerKeys)
	if err != nil {
		return firstRunResult{Err: fmt.Errorf("%s: %w", i18n.T("wizard.error.parse.config"), err)}
	}
	if len(providerList) == 0 {
		return firstRunResult{Err: fmt.Errorf("%s", i18n.T("wizard.error.no.provider"))}
	}

	ti := textinput.New()
	ti.Placeholder = i18n.T("wizard.step.apikey.placeholder")
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

	// 预解析所有 Provider 的 API Key（从环境变量和 CredentialStore 读取）
	if preKeys, err := resolveAllProviderKeys(providersPath, workspaceDir); err == nil {
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
		return firstRunResult{Err: fmt.Errorf("%s", i18n.T("wizard.error.user.cancelled"))}
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

func parseProviderAndModels(providersPath, modelsPath string, providerKeys map[string]string) ([]providerItem, []modelItem, error) {
	var providerItems []providerItem

	// Load providers from providers.yml (if exists)
	if data, err := os.ReadFile(providersPath); err == nil {
		var provConfig struct {
			Providers []goharnessconfig.ProviderConfig `yaml:"providers"`
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
			Providers []goharnessconfig.ProviderConfig `yaml:"providers"`
			Models    []goharnessconfig.ModelConfig    `yaml:"models"`
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
		Models []goharnessconfig.ModelConfig `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &modelConfig); err != nil {
		return nil, nil, err
	}

	var items []modelItem
	for _, m := range modelConfig.Models {
		desc := strings.TrimSpace(m.Description)
		enabled := false
		if providerKeys != nil && providerKeys[m.Provider] != "" {
			enabled = true
		}
		items = append(items, modelItem{
			Name:     m.Name,
			desc:     desc,
			BaseURL:  m.BaseURL,
			CredRef:  m.APIKey,
			Provider: m.Provider,
			Enabled:  enabled,
		})
	}
	return providerItems, items, nil
}

// enableModelsForProvider 将 models.yml 中指定提供商下的所有模型的 Enabled 设置为 true。
func enableModelsForProvider(modelsPath, providerName string) error {
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		return err
	}

	var cfg goharnessconfig.ModelsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	changed := false
	for i := range cfg.Models {
		if cfg.Models[i].Provider == providerName {
			if !cfg.Models[i].Enabled {
				cfg.Models[i].Enabled = true
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(modelsPath, out, 0644)
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
					m.apiKeyInput.Placeholder = fmt.Sprintf(i18n.T("wizard.step.apikey.placeholder.provider"), pi.Title())
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
			// 将新输入的 Key 写入 preResolvedKeys，并启用该提供商下所有模型
			if m.preResolvedKeys == nil {
				m.preResolvedKeys = make(map[string]string)
			}
			m.preResolvedKeys[m.selectedProvider.Name] = m.apiKeyInput.Value()
			go func() {
				_ = enableModelsForProvider(m.modelsPath, m.selectedProvider.Name)
			}()
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
	b.WriteString(m.renderMarkdown(fmt.Sprintf("%s\n\n%s\n\n",
		i18n.T("wizard.step.provider.title"),
		i18n.T("wizard.step.provider.desc"))))
	b.WriteString(m.providerList.View())
	b.WriteString("\n")
	help := i18n.T("wizard.help.provider.select")
	if m.apiKeyConfigured {
		help = i18n.T("wizard.help.provider.select.skip")
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderAPIKeyInput() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown(fmt.Sprintf("%s\n\n", i18n.T("wizard.step.apikey.title"))))
	b.WriteString(fmt.Sprintf("%s: **%s**\n\n", i18n.T("wizard.step.apikey.provider.label"), m.selectedProvider.DisplayName))
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.step.apikey.prompt")))
	b.WriteString(m.apiKeyInput.View())
	b.WriteString("\n\n")
	help := i18n.T("wizard.help.apikey.confirm")
	if m.apiKeyConfigured {
		help = i18n.T("wizard.help.apikey.confirm.skip")
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderModelSelect() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown(fmt.Sprintf("%s\n\n%s: **%s**\n\n%s\n\n",
		i18n.T("wizard.step.model.title"),
		i18n.T("wizard.step.apikey.provider.label"),
		m.selectedProvider.DisplayName,
		i18n.T("wizard.step.model.desc"))))
	b.WriteString(m.modelList.View())
	b.WriteString("\n")
	help := i18n.T("wizard.help.model.select")
	if m.modelConfigured {
		help = i18n.T("wizard.help.model.select.skip")
	}
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderDaemonCheck() string {
	var b strings.Builder
	if m.daemonState == 1 {
		b.WriteString(m.renderMarkdown(i18n.T("wizard.daemon.installing.wait")))
		return m.paddedView(borderStyle.Render(b.String()))
	}
	if m.daemonState == 2 {
		if m.daemonInstallErr != nil {
			b.WriteString(m.renderMarkdown(fmt.Sprintf(
				i18n.T("wizard.daemon.install.error.format"),
				m.daemonInstallErr.Error(),
			)))
		} else {
			restartHint := ""
			if runtime.GOOS == "windows" {
				restartHint = i18n.T("wizard.daemon.restart.hint.installed")
			}
			b.WriteString(m.renderMarkdown(fmt.Sprintf(
				i18n.T("wizard.daemon.installed.success.format"), restartHint,
			)))
		}
		return m.paddedView(borderStyle.Render(b.String()))
	}
	installed := DaemonInstalled(m.workspaceDir)
	if installed {
		restartHint := ""
		if runtime.GOOS == "windows" {
			restartHint = i18n.T("wizard.daemon.restart.hint.installed")
		}
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
			i18n.T("wizard.daemon.already.installed.format"), restartHint,
		)))
	} else {
		md := i18n.T("wizard.daemon.notinstalled.desc") + "\n\n" + m.yesNoIndicator(m.daemonChoice) + "\n\n" + i18n.T("wizard.daemon.toggle.nav")
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
			i18n.T("wizard.python.ready.format"),
			m.pythonVersion,
		)))
	} else if m.pythonDetected {
		md := fmt.Sprintf(
			i18n.T("wizard.python.detected.no.venv.format"),
			m.pythonVersion, m.yesNoIndicator(m.pythonChoice),
		)
		b.WriteString(m.renderMarkdown(md))
	} else {
		md := i18n.T("wizard.python.notinstalled.desc")
		b.WriteString(m.renderMarkdown(md))
	}
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderMemoryConfig() string {
	md := i18n.T("wizard.memory.embedder.ready.detail")
	return m.paddedView(borderStyle.Render(m.renderMarkdown(md)))
}

func (m *firstRunModel) renderPathSetup() string {
	var b strings.Builder
	if m.pathInPath {
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
			i18n.T("wizard.path.already.in.format"),
			m.installDir,
		)))
	} else {
		md := fmt.Sprintf(
			i18n.T("wizard.path.add.desc.format"),
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
		items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.provider"), m.selectedProvider.DisplayName))
	}

	// Model
	if m.selectedModel.Name != "" {
		items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.model"), m.selectedModel.Name))
	}

	// Daemon - re-check
	if DaemonInstalled(m.workspaceDir) {
		items = append(items, i18n.T("wizard.complete.item.daemon.installed"))
	} else if m.daemonState == 2 && m.daemonInstallErr != nil {
		items = append(items, i18n.T("wizard.complete.item.daemon.failed"))
	} else {
		items = append(items, i18n.T("wizard.complete.item.daemon.skipped"))
	}

	// Python + venv - re-check
	venvPath := filepath.Join(m.workspaceDir, ".venv")
	_, venvExists := os.Stat(venvPath)
	if m.pythonDetected {
		if venvExists == nil {
			items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.python.ready"), m.pythonVersion))
		} else {
			items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.python.no.venv"), m.pythonVersion))
		}
	} else {
		items = append(items, i18n.T("wizard.complete.item.python.missing"))
	}

	// Embedder model - re-check
	modelPath := filepath.Join(m.workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		items = append(items, i18n.T("wizard.complete.item.embedder.ready"))
	} else {
		items = append(items, i18n.T("wizard.complete.item.embedder.pending"))
	}

	// PATH - re-check (Windows only)
	if runtime.GOOS == "windows" && m.installDir != "" {
		if CheckInPath(m.installDir) {
			items = append(items, i18n.T("wizard.complete.item.path.added"))
		} else if m.pathChoice {
			items = append(items, i18n.T("wizard.complete.item.path.failed"))
		} else {
			items = append(items, i18n.T("wizard.complete.item.path.skipped"))
		}
	}

	// WebUI
	webDir := filepath.Join(m.workspaceDir, "web")
	if _, err := os.Stat(webDir); err == nil {
		items = append(items, i18n.T("wizard.complete.item.webui.ready"))
	} else {
		items = append(items, i18n.T("wizard.complete.item.webui.missing"))
	}

	// Build markdown output
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.title")))
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.status.header")))
	for _, item := range items {
		b.WriteString(item + "\n\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.header")))
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.tui.detail")))
	b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.webui.detail")))
	if runtime.GOOS == "windows" {
		b.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.windows.restart")))
	}
	b.WriteString(i18n.T("wizard.complete.finish"))

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
	return tea.View{
		Content:   style.GradientTitle("") + "\n\n" + content,
		AltScreen: true,
	}
}
