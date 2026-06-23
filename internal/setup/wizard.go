package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	goharnessconfig "github.com/DotNetAge/goharness/config"
	goragutils "github.com/DotNetAge/gorag/v2/utils"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/setup/style"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const minContentWidth = 60

var borderStyle = lipgloss.NewStyle().Padding(1, 2)

// ── Provider / Model list items ───────────────────────────────────────────

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

// ── Auto-setup task ──────────────────────────────────────────────────────

// taskStatus 表示一个自动安装任务的状态。
type taskStatus int

const (
	taskPending taskStatus = iota // 待执行
	taskRunning                   // 执行中
	taskDone                      // 已完成
	taskFailed                    // 失败
	taskSkipped                   // 已跳过
)

// setupTask 代表向导中自动执行的一个安装步骤。
type setupTask struct {
	Name    string        // 显示名称
	Status  taskStatus    // 当前状态
	Err     error         // 错误信息（仅 taskFailed）
	Detail  string        // 补充信息（如版本号、路径等）
	spinner spinner.Model // 运行中动画
}

func (t *setupTask) statusIcon() string {
	switch t.Status {
	case taskDone:
		return i18n.T("wizard.autosetup.icon.done")
	case taskFailed:
		return i18n.T("wizard.autosetup.icon.failed")
	case taskSkipped:
		return i18n.T("wizard.autosetup.icon.skipped")
	case taskRunning:
		return t.spinner.View()
	default:
		return i18n.T("wizard.autosetup.icon.pending")
	}
}

// autoSetupDoneMsg 在所有自动安装任务完成后发送。
type autoSetupDoneMsg struct{}

// downloadProgressMsg 用于 embedder 模型下载进度。
type downloadProgressMsg struct {
	Current int64
	Total   int64
	File    string
	Done    bool
	Err     error
	Status  string
}

// downloadObserver 将 goragutils 的下载事件转换为 Bubble Tea 消息。
type downloadObserver struct {
	ch      chan<- downloadProgressMsg
	taskIdx int
}

func (o *downloadObserver) OnEvent(event goragutils.DownloadEvent) {
	msg := downloadProgressMsg{
		Current: event.Current,
		Total:   event.Total,
		File:    event.File,
	}
	switch event.Type {
	case goragutils.EventStart:
		msg.Status = fmt.Sprintf(i18n.T("setup.memory.downloading"), event.File)
	case goragutils.EventProgress:
		downloadedMB := float64(event.Current) / (1024 * 1024)
		if event.Total > 0 {
			totalMB := float64(event.Total) / (1024 * 1024)
			msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.progress"), downloadedMB, totalMB)
		} else {
			msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.progress.mb"), downloadedMB)
		}
	case goragutils.EventComplete:
		msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.complete"), event.File)
	}
	o.ch <- msg
}

// ── Wizard model ─────────────────────────────────────────────────────────

type firstRunModel struct {
	step int

	// Step 0-2: Provider / API Key / Model 选择
	providerList     list.Model
	providers        []providerItem
	selectedProvider providerItem

	modelList list.Model
	models    []modelItem

	apiKeyInput   textinput.Model
	selectedModel modelItem

	preResolvedKeys map[string]string

	modelConfigured  bool
	apiKeyConfigured bool

	// Step 3: 自动安装进度
	autoTasks     []*setupTask
	autoSetupDone bool
	downloadCh    chan downloadProgressMsg

	// 通用状态
	err      error
	done     bool
	quitting bool

	modelsPath   string
	agentsDir    string
	mindxConfig  *core.MindxConfig
	workspaceDir string

	width    int
	height   int
	renderer *glamour.TermRenderer
}

type firstRunResult struct {
	SelectedProvider string
	SelectedModel    string
	APIKey           string
	ResolvedKeys     map[string]string
	Err              error

	// 自动安装结果（由 wizard 内部执行，不再询问用户）
	DaemonOK      bool
	DaemonErr     error
	PythonOK      bool
	PythonInfo    core.PythonConfig
	PythonErr     error
	EmbedderOK    bool
	EmbedderModel string
	PathOK        bool
	PathErr       error
	WebUIReady    bool
}

// ── Constructor & helpers ─────────────────────────────────────────────────

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
		if v := os.Getenv(p.APIKey); v != "" {
			keys[p.Name] = v
		} else if credStore != nil {
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
	for _, ml := range modelList {
		modelItems = append(modelItems, ml)
	}
	mlist := list.New(modelItems, md, minContentWidth-4, 8)
	mlist.SetShowStatusBar(false)
	mlist.SetShowPagination(false)
	mlist.SetShowTitle(false)
	mlist.SetFilteringEnabled(false)

	m := &firstRunModel{
		step:         0,
		providerList: pl,
		providers:    providerList,
		modelList:    mlist,
		models:       modelList,
		apiKeyInput:  ti,
		modelsPath:   modelsPath,
		agentsDir:    agentsDir,
		mindxConfig:  mindxConfig,
		workspaceDir: workspaceDir,
		renderer:     initGlamour(minContentWidth),
		width:        80,
		height:       24,
	}

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

	p := tea.NewProgram(m, tea.WithoutSignals())
	finalModel, err := p.Run()
	if err != nil {
		return firstRunResult{Err: err}
	}

	fm := finalModel.(*firstRunModel)
	if fm.quitting {
		return firstRunResult{Err: fmt.Errorf("%s", i18n.T("wizard.error.user.cancelled"))}
	}

	// 从 autoTasks 中提取结果
	result := firstRunResult{
		SelectedProvider: fm.selectedProvider.Name,
		SelectedModel:    fm.selectedModel.Name,
		APIKey:           fm.apiKeyInput.Value(),
		ResolvedKeys:     fm.preResolvedKeys,
	}
	if len(fm.autoTasks) > 0 {
		result.DaemonOK = fm.autoTasks[0].Status == taskDone
		result.DaemonErr = fm.autoTasks[0].Err
		result.PythonOK = fm.autoTasks[1].Status == taskDone || fm.autoTasks[1].Status == taskSkipped
		result.PythonErr = fm.autoTasks[1].Err
		result.PythonInfo = fm.pythonInfoFromTask(fm.autoTasks[1])
		result.EmbedderOK = fm.autoTasks[2].Status == taskDone || fm.autoTasks[2].Status == taskSkipped
		result.EmbedderModel = fm.autoTasks[2].Detail
		if runtime.GOOS == "windows" && len(fm.autoTasks) > 3 {
			result.PathOK = fm.autoTasks[3].Status == taskDone || fm.autoTasks[3].Status == taskSkipped
			result.PathErr = fm.autoTasks[3].Err
		} else {
			result.PathOK = true // 非 Windows 不需要 PATH 设置
		}
	}

	webDir := filepath.Join(workspaceDir, "web")
	if _, err := os.Stat(webDir); err == nil {
		result.WebUIReady = true
	}

	return result
}

func (m *firstRunModel) pythonInfoFromTask(t *setupTask) core.PythonConfig {
	info := DetectPython()
	if t.Detail != "" {
		info.VenvPath = t.Detail
	}
	return info
}

func parseProviderAndModels(providersPath, modelsPath string, providerKeys map[string]string) ([]providerItem, []modelItem, error) {
	var providerItems []providerItem

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
	for _, mi := range modelConfig.Models {
		desc := strings.TrimSpace(mi.Description)
		enabled := false
		if providerKeys != nil && providerKeys[mi.Provider] != "" {
			enabled = true
		}
		items = append(items, modelItem{
			Name:     mi.Name,
			desc:     desc,
			BaseURL:  mi.BaseURL,
			CredRef:  mi.APIKey,
			Provider: mi.Provider,
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

// ── Bubble Tea Init / Update / View ───────────────────────────────────────

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
		return m.updateAutoSetup(msg)
	case 4:
		return m.updateComplete(msg)
	}
	return m, nil
}

// ── Step 0: Provider selection ────────────────────────────────────────────

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

// ── Step 1: API Key input ─────────────────────────────────────────────────

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

// ── Step 2: Model selection ───────────────────────────────────────────────

func (m *firstRunModel) updateModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
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
					m.initAutoSetupTasks()
					autoCmd := m.startAutoSetup()
					m.step = 3
					return m, autoCmd
				}
			}
		case "s", "S":
			if m.modelConfigured {
				if item := m.modelList.SelectedItem(); item != nil {
					if mi, ok := item.(modelItem); ok {
						m.selectedModel = mi
						m.initAutoSetupTasks()
						autoCmd := m.startAutoSetup()
						m.step = 3
						return m, autoCmd
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

// ── Step 3: Auto Setup Progress ───────────────────────────────────────────

// initAutoSetupTasks 初始化自动安装任务列表。
func (m *firstRunModel) initAutoSetupTasks() {
	tasks := []*setupTask{
		{Name: i18n.T("wizard.autosetup.task.daemon"), spinner: spinner.New(spinner.WithSpinner(spinner.Dot))},
		{Name: i18n.T("wizard.autosetup.task.python"), spinner: spinner.New(spinner.WithSpinner(spinner.Dot))},
		{Name: i18n.T("wizard.autosetup.task.embedder"), spinner: spinner.New(spinner.WithSpinner(spinner.Dot))},
	}
	// Windows 额外需要 PATH 配置任务
	if runtime.GOOS == "windows" {
		tasks = append(tasks, &setupTask{
			Name:    i18n.T("wizard.autosetup.task.path"),
			spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		})
	}
	m.autoTasks = tasks
	m.downloadCh = make(chan downloadProgressMsg, 100)
	m.autoSetupDone = false
}

// startAutoSetup 在 goroutine 中并行执行所有安装任务。
func (m *firstRunModel) startAutoSetup() tea.Cmd {
	wd := m.workspaceDir
	taskCount := len(m.autoTasks)

	// Daemon 安装
	go func(idx int) {
		err := SetupDaemon(wd)
		if err == nil && DaemonInstalled(wd) {
			m.markTaskDone(idx, taskDone, nil, "")
		} else {
			m.markTaskDone(idx, taskFailed, err, "")
		}
	}(0)

	// Python 环境
	go func(idx int) {
		pyInfo, err := SetupPython(wd)
		detail := ""
		if err == nil {
			detail = pyInfo.VenvPath
			venvExists := false
			if pyInfo.VenvPath != "" {
				if _, statErr := os.Stat(pyInfo.VenvPath); statErr == nil {
					venvExists = true
				}
			}
			if venvExists {
				m.markTaskDone(idx, taskDone, nil, detail)
			} else if pyInfo.Detected {
				m.markTaskDone(idx, taskSkipped, err, detail)
			} else {
				m.markTaskDone(idx, taskFailed, err, detail)
			}
		} else {
			if pyInfo.Detected {
				m.markTaskDone(idx, taskSkipped, err, detail)
			} else {
				m.markTaskDone(idx, taskFailed, err, detail)
			}
		}
	}(1)

	// Embedder 模型下载
	go func(idx int) {
		m.runEmbedderDownload(idx)
	}(2)

	// PATH 配置（仅 Windows）
	if runtime.GOOS == "windows" && taskCount > 3 {
		go func(idx int) {
			exe, exeErr := os.Executable()
			if exeErr != nil {
				m.markTaskDone(idx, taskFailed, exeErr, "")
				return
			}
			installDir := filepath.Dir(exe)
			if CheckInPath(installDir) {
				m.markTaskDone(idx, taskSkipped, nil, installDir)
				return
			}
			if _, pathErr := AddToSystemPath(installDir); pathErr != nil {
				m.markTaskDone(idx, taskFailed, pathErr, "")
			} else {
				m.markTaskDone(idx, taskDone, nil, installDir)
			}
		}(3)
	}

	return tea.Batch(append([]tea.Cmd{m.listenTaskResults(taskCount)}, m.spinnerTickCmds()...)...)
}

// markTaskDone 安全地标记任务完成状态（从 goroutine 调用）。
func (m *firstRunModel) markTaskDone(idx int, status taskStatus, err error, detail string) {
	if idx >= 0 && idx < len(m.autoTasks) {
		m.autoTasks[idx].Status = status
		m.autoTasks[idx].Err = err
		m.autoTasks[idx].Detail = detail
	}
}

// spinnerTickCmds 返回所有 autoTasks 中 spinner 的 Tick 命令，用于驱动动画。
func (m *firstRunModel) spinnerTickCmds() []tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range m.autoTasks {
		cmds = append(cmds, t.spinner.Tick)
	}
	return cmds
}

func (m *firstRunModel) listenTaskResults(expected int) tea.Cmd {
	return func() tea.Msg {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			doneCount := 0
			for _, t := range m.autoTasks {
				if t.Status == taskDone || t.Status == taskFailed || t.Status == taskSkipped {
					doneCount++
				}
			}
			if doneCount >= expected {
				return autoSetupDoneMsg{}
			}
		}
		return autoSetupDoneMsg{}
	}
}

// runEmbedderDownload 在 goroutine 中执行 embedder 模型下载并通过 channel 报告进度。
func (m *firstRunModel) runEmbedderDownload(taskIdx int) {
	defer close(m.downloadCh)

	cacheDir := filepath.Join(m.workspaceDir, "data", "models")
	modelID := "Xenova/chinese-clip-vit-base-patch16"
	modelFile := "onnx/model_q4.onnx"
	dstPath := filepath.Join(cacheDir, filepath.Base(modelFile))

	m.markTaskDone(taskIdx, taskRunning, nil, "")

	if _, err := os.Stat(dstPath); err == nil {
		m.downloadCh <- downloadProgressMsg{Done: true}
		m.markTaskDone(taskIdx, taskSkipped, nil, "model_q4.onnx")
		return
	}

	observer := &downloadObserver{ch: m.downloadCh, taskIdx: taskIdx}

	downloader, err := goragutils.NewModelDownloader(cacheDir)
	if err != nil {
		m.downloadCh <- downloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.downloader.create.failed"), err)}
		m.markTaskDone(taskIdx, taskFailed, err, "")
		return
	}
	downloader.WithObserver(observer)

	files := []string{modelFile}
	if _, err := downloader.Download(modelID, files); err != nil {
		m.downloadCh <- downloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.download.failed.err"), err)}
		m.markTaskDone(taskIdx, taskFailed, err, "")
		return
	}

	srcPath := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), modelFile)

	src, err := os.Open(srcPath)
	if err != nil {
		m.downloadCh <- downloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.open.failed"), err)}
		m.markTaskDone(taskIdx, taskFailed, err, "")
		return
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(dstPath)
	if err != nil {
		m.downloadCh <- downloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.create.failed"), err)}
		m.markTaskDone(taskIdx, taskFailed, err, "")
		return
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		m.downloadCh <- downloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.copy.failed"), err)}
		m.markTaskDone(taskIdx, taskFailed, err, "")
		return
	}

	srcDir := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)))
	_ = os.RemoveAll(srcDir)

	m.downloadCh <- downloadProgressMsg{
		Done:   true,
		Status: i18n.T("setup.memory.model.download.complete"),
	}
	m.markTaskDone(taskIdx, taskDone, nil, "model_q4.onnx")
}

func (m *firstRunModel) updateAutoSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case downloadProgressMsg:
		// 更新 embedder 任务的状态文本
		if len(m.autoTasks) > 2 {
			if msg.Err != nil {
				m.autoTasks[2].Status = taskFailed
				m.autoTasks[2].Err = msg.Err
			} else if msg.Done {
				m.autoTasks[2].Status = taskDone
				if m.autoTasks[2].Detail == "" || m.autoTasks[2].Status == taskRunning {
					m.autoTasks[2].Detail = "model_q4.onnx"
				}
			}
			if msg.Status != "" {
				m.autoTasks[2].Detail = msg.Status
			}
		}
	case autoSetupDoneMsg:
		m.autoSetupDone = true
		m.step = 4
		return m, nil
	}

	// 更新 spinner 动画
	var cmds []tea.Cmd
	for _, t := range m.autoTasks {
		if t.Status == taskRunning || t.Status == taskPending {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// ── Step 4: Complete ──────────────────────────────────────────────────────

func (m *firstRunModel) updateComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ", "s", "S":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// ── Render methods ────────────────────────────────────────────────────────

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

func (m *firstRunModel) renderAutoSetup() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown(fmt.Sprintf("**%s**\n\n", i18n.T("wizard.autosetup.title"))))
	b.WriteString(m.renderMarkdown(i18n.T("wizard.autosetup.desc") + "\n\n"))

	for _, t := range m.autoTasks {
		line := fmt.Sprintf("  %s %s", t.statusIcon(), t.Name)
		if t.Detail != "" && t.Status != taskRunning {
			line += fmt.Sprintf(" — %s", t.Detail)
		}
		if t.Status == taskFailed && t.Err != nil {
			line += fmt.Sprintf(" (%s)", t.Err.Error())
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	if !m.autoSetupDone {
		b.WriteString(m.renderMarkdown(i18n.T("wizard.autosetup.waiting")))
	} else {
		b.WriteString(m.renderMarkdown(i18n.T("wizard.autosetup.done.hint")))
	}

	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderComplete() string {
	var items []string

	// Provider
	if m.selectedProvider.Name != "" {
		items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.provider"), m.selectedProvider.DisplayName))
	}

	// Model
	if m.selectedModel.Name != "" {
		items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.model"), m.selectedModel.Name))
	}

	// Daemon
	if len(m.autoTasks) > 0 {
		switch m.autoTasks[0].Status {
		case taskDone:
			items = append(items, i18n.T("wizard.complete.item.daemon.installed"))
		case taskFailed:
			items = append(items, i18n.T("wizard.complete.item.daemon.failed"))
		default:
			items = append(items, i18n.T("wizard.complete.item.daemon.skipped"))
		}
	}

	// Python
	if len(m.autoTasks) > 1 {
		pyInfo := DetectPython()
		venvPath := filepath.Join(m.workspaceDir, ".venv")
		_, venvExists := os.Stat(venvPath)
		if pyInfo.Detected && venvExists == nil {
			items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.python.ready"), pyInfo.Version))
		} else if pyInfo.Detected {
			items = append(items, fmt.Sprintf(i18n.T("wizard.complete.item.python.no.venv"), pyInfo.Version))
		} else {
			items = append(items, i18n.T("wizard.complete.item.python.missing"))
		}
	}

	// Embedder
	if len(m.autoTasks) > 2 {
		modelPath := filepath.Join(m.workspaceDir, "data", "models", "model_q4.onnx")
		if _, err := os.Stat(modelPath); err == nil {
			items = append(items, i18n.T("wizard.complete.item.embedder.ready"))
		} else {
			items = append(items, i18n.T("wizard.complete.item.embedder.pending"))
		}
	}

	// PATH (Windows only)
	if runtime.GOOS == "windows" && len(m.autoTasks) > 3 {
		if CheckInPath(m.autoTasks[3].Detail) || m.autoTasks[3].Status == taskDone {
			items = append(items, i18n.T("wizard.complete.item.path.added"))
		} else if m.autoTasks[3].Status == taskFailed {
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
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.title")))
	cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.status.header")))
	for _, item := range items {
		cb.WriteString(item + "\n\n")
	}
	cb.WriteString("---\n\n")
	cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.header")))
	cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.tui.detail")))
	cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.usage.webui.detail")))
	if runtime.GOOS == "windows" {
		cb.WriteString(fmt.Sprintf("%s\n\n", i18n.T("wizard.complete.windows.restart")))
	}
	cb.WriteString(i18n.T("wizard.complete.finish"))

	return m.paddedView(borderStyle.Render(m.renderMarkdown(cb.String())))
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
		content = m.renderAutoSetup()
	case 4:
		content = m.renderComplete()
	}
	return tea.View{
		Content:   style.GradientTitle("") + "\n\n" + content,
		AltScreen: true,
	}
}
