package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	goragutils "github.com/DotNetAge/gorag/utils"
	goreactcore "github.com/DotNetAge/goreact/core"
	"gopkg.in/yaml.v3"

	"github.com/DotNetAge/mindx/internal/core"
)

const minContentWidth = 60

var borderStyle = lipgloss.NewStyle().Padding(1, 2)

type modelItem struct {
	Name        string
	desc        string
	BaseURL     string
	CredRef     string
}

func (i modelItem) Title() string       { return i.Name }
func (i modelItem) Description() string { return i.desc }
func (i modelItem) FilterValue() string { return i.Name }

type downloadProgressMsg struct {
	current   int64
	total     int64
	file      string
	done      bool
	err       error
	modelPath string
	status    string
}

type downloadObserver struct {
	ch chan<- downloadProgressMsg
}

func (o *downloadObserver) OnEvent(event goragutils.DownloadEvent) {
	msg := downloadProgressMsg{
		current: event.Current,
		total:   event.Total,
		file:    event.File,
	}
	switch event.Type {
	case goragutils.EventStart:
		msg.status = fmt.Sprintf("正在下载 %s...", event.File)
	case goragutils.EventProgress:
		downloadedMB := float64(event.Current) / (1024 * 1024)
		if event.Total > 0 {
			totalMB := float64(event.Total) / (1024 * 1024)
			msg.status = fmt.Sprintf("下载中  %.0f / %.0f MB", downloadedMB, totalMB)
		} else {
			msg.status = fmt.Sprintf("下载中  %.0f MB", downloadedMB)
		}
	case goragutils.EventComplete:
		msg.status = fmt.Sprintf("%s 下载完成", event.File)
	}
	o.ch <- msg
}

func runModelDownload(cacheDir string, ch chan<- downloadProgressMsg) {
	defer close(ch)

	modelID := "Xenova/chinese-clip-vit-base-patch16"
	modelFile := "onnx/model_q4.onnx"
	dstPath := filepath.Join(cacheDir, filepath.Base(modelFile))

	if _, err := os.Stat(dstPath); err == nil {
		ch <- downloadProgressMsg{
			done:      true,
			modelPath: dstPath,
		}
		return
	}

	observer := &downloadObserver{ch: ch}

	downloader, err := goragutils.NewModelDownloader(cacheDir)
	if err != nil {
		ch <- downloadProgressMsg{done: true, err: fmt.Errorf("创建下载器失败: %w", err)}
		return
	}
	downloader.WithObserver(observer)

	files := []string{modelFile}
	if _, err := downloader.Download(modelID, files); err != nil {
		ch <- downloadProgressMsg{done: true, err: fmt.Errorf("下载失败: %w", err)}
		return
	}

	srcPath := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), modelFile)

	src, err := os.Open(srcPath)
	if err != nil {
		ch <- downloadProgressMsg{done: true, err: fmt.Errorf("打开下载模型失败: %w", err)}
		return
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		ch <- downloadProgressMsg{done: true, err: fmt.Errorf("创建模型文件失败: %w", err)}
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		ch <- downloadProgressMsg{done: true, err: fmt.Errorf("复制模型文件失败: %w", err)}
		return
	}

	// 清理下载器缓存目录，避免文件重复
	srcDir := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)))
	os.RemoveAll(srcDir)

	ch <- downloadProgressMsg{
		done:      true,
		modelPath: dstPath,
		status:    "模型下载完成",
	}
}

type firstRunModel struct {
	step int

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

	pythonChoice    bool
	pythonSubmitted bool
	pythonDetected  bool
	pythonVersion   string
	pythonInfo      core.PythonConfig

	memoryChoice    bool
	memorySubmitted bool
	memoryState     int
	downloadCh      chan downloadProgressMsg
	progressBar     progress.Model
	downloadErr     error
	downloadStatus  string
	embedderModel   string
	workspaceDir    string

	pathChoice    bool
	pathSubmitted bool
	installDir    string
	pathInPath    bool

	modelConfigured  bool
	apiKeyConfigured bool

	width  int
	height int

	renderer *glamour.TermRenderer
}

type firstRunResult struct {
	SelectedModel string
	CredRef       string
	APIKey        string
	Err           error

	DaemonSetup    bool
	PythonSetup    bool
	PythonInfo     core.PythonConfig
	EmbedderModel  string
	PathSetup      bool
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

func runFirstRunWizard(modelsPath, agentsDir, workspaceDir string, mindxConfig *core.MindxConfig) firstRunResult {
	modelList, err := parseModelsForWizard(modelsPath)
	if err != nil {
		return firstRunResult{Err: fmt.Errorf("解析模型配置失败: %w", err)}
	}
	if len(modelList) == 0 {
		return firstRunResult{Err: fmt.Errorf("模型配置文件中没有可用模型")}
	}

	ti := textinput.New()
	ti.Placeholder = "请输入 API Key..."
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.Focus()

	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	d.SetSpacing(0)
	d.SetHeight(2)

	var items []list.Item
	for _, m := range modelList {
		items = append(items, m)
	}
	l := list.New(items, d, minContentWidth-4, 8)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)

	pythonInfo := DetectPython()

	m := &firstRunModel{
		step:           0,
		modelList:      l,
		models:         modelList,
		apiKeyInput:    ti,
		modelsPath:     modelsPath,
		agentsDir:      agentsDir,
		mindxConfig:    mindxConfig,
		pythonDetected: pythonInfo.Detected,
		pythonVersion:  pythonInfo.Version,
		pythonInfo:     pythonInfo,
		progressBar:    progress.New(progress.WithWidth(minContentWidth-12)),
		memoryState:    0,
		workspaceDir:   workspaceDir,
		pathChoice:     true,
		renderer:       initGlamour(minContentWidth),
		width:          80,
		height:         24,
	}

	m.modelConfigured = mindxConfig.DefaultModel != ""

	if m.modelConfigured {
		credStore := core.NewCredentialStore(workspaceDir)
		for _, model := range modelList {
			if model.Name == mindxConfig.DefaultModel {
				if key, err := credStore.Get(model.CredRef); err == nil && key != "" {
					m.apiKeyConfigured = true
				}
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

	if mindxConfig.DefaultModel != "" {
		for i, model := range modelList {
			if model.Name == mindxConfig.DefaultModel {
				l.Select(i)
				break
			}
		}
	}

	m.daemonChoice = DaemonInstalled(workspaceDir)

	m.pythonChoice = true // 默认创建虚拟环境，用户按 Enter 直接确认

	modelPath := filepath.Join(workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		m.memoryState = 2
		m.embedderModel = "model_q4.onnx"
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
		SelectedModel: fm.selectedModel.Name,
		CredRef:       fm.selectedModel.CredRef,
		APIKey:        fm.apiKeyInput.Value(),
		DaemonSetup:   fm.daemonChoice,
		PythonSetup:   fm.pythonChoice,
		PythonInfo:    fm.pythonInfo,
		EmbedderModel: fm.embedderModel,
		PathSetup:     fm.pathChoice,
	}
}

func parseModelsForWizard(path string) ([]modelItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Models []goreactcore.ModelConfig `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var items []modelItem
	for _, m := range config.Models {
		desc := strings.TrimSpace(m.Description)
		items = append(items, modelItem{
			Name:    m.Name,
			desc:    desc,
			BaseURL: m.BaseURL,
			CredRef: m.APIKey,
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := m.contentWidth()
		m.apiKeyInput.SetWidth(cw - 8)
		m.progressBar = progress.New(progress.WithWidth(cw - 12))
		m.renderer = initGlamour(cw)
		m.modelList.SetWidth(cw - 4)
	}

	switch m.step {
	case 0:
		return m.updateModelSelect(msg)
	case 1:
		return m.updateAPIKeyInput(msg)
	case 2:
		return m.updateDaemonCheck(msg)
	case 3:
		return m.updatePythonCheck(msg)
	case 4:
		return m.updateMemoryConfig(msg)
	case 5:
		return m.updatePathSetup(msg)
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
		case "enter":
			if item := m.modelList.SelectedItem(); item != nil {
				if mi, ok := item.(modelItem); ok {
					m.selectedModel = mi
					m.apiKeyInput.Placeholder = fmt.Sprintf("请输入 %s 的 API Key...", mi.Name)
					m.apiKeyInput.Focus()
					m.step = 1
					cmd := textinput.Blink
					return m, cmd
				}
			}
		case "s", "S":
			if m.modelConfigured {
				if item := m.modelList.SelectedItem(); item != nil {
					if mi, ok := item.(modelItem); ok {
						m.selectedModel = mi
						m.step = 2
						return m, nil
					}
				}
			}
		}
	}

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
			m.daemonChoice = !m.daemonChoice
		case "enter":
			m.daemonSubmitted = true
			m.step = 3
			return m, nil
		case "s", "S":
			if DaemonInstalled(m.workspaceDir) {
				m.step = 3
				return m, nil
			}
		}
	}
	return m, nil
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
			m.step = 4
			return m, nil
		case "left", "right":
			if m.pythonDetected {
				m.pythonChoice = !m.pythonChoice
			}
		case "enter":
			if !m.pythonDetected {
				m.pythonChoice = true
				m.pythonSubmitted = true
				m.step = 4
				return m, nil
			}
			m.pythonSubmitted = true
			m.step = 4
			return m, nil
		case "s", "S":
			if _, err := os.Stat(filepath.Join(m.workspaceDir, ".venv")); err == nil {
				m.pythonSubmitted = true
				m.step = 4
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *firstRunModel) updateMemoryConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.memoryState {
		case 0:
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				m.quitting = true
				return m, tea.Quit
			case "left", "right":
				m.memoryChoice = !m.memoryChoice
			case "enter":
				m.memorySubmitted = true
				if m.memoryChoice {
					m.memoryState = 1
					m.downloadCh = make(chan downloadProgressMsg, 100)
					cacheDir := filepath.Join(m.workspaceDir, "data", "models")
					m.embedderModel = "model_q4.onnx"
					go runModelDownload(cacheDir, m.downloadCh)
					return m, m.listenDownloadCmd()
				}
				if runtime.GOOS == "windows" {
					m.step = 5
					return m, nil
				}
				m.done = true
				return m, tea.Quit
			}

		case 1:
			if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.quitting = true
				return m, tea.Quit
			}

		case 2:
			if msg.String() == "enter" || msg.String() == " " || msg.String() == "s" || msg.String() == "S" {
				if runtime.GOOS == "windows" {
					m.step = 5
					return m, nil
				}
				m.done = true
				return m, tea.Quit
			}
		}

	case downloadProgressMsg:
		if m.memoryState == 1 {
			if msg.err != nil {
				m.downloadErr = msg.err
				m.embedderModel = ""
				m.downloadStatus = "下载失败"
				m.memoryState = 2
				return m, nil
			}
			if msg.done {
				m.downloadStatus = "处理中..."
				m.memoryState = 2
				return m, nil
			}
			if msg.status != "" {
				m.downloadStatus = msg.status
			}
			var progCmd tea.Cmd
			if msg.total > 0 {
				progCmd = m.progressBar.SetPercent(float64(msg.current) / float64(msg.total))
			} else if msg.current > 0 {
				progCmd = m.progressBar.SetPercent(0.5)
			}
			return m, tea.Batch(progCmd, m.listenDownloadCmd())
		}
	}

	return m, nil
}

func (m *firstRunModel) listenDownloadCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.downloadCh
		if !ok {
			return nil
		}
		return msg
	}
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
			m.done = true
			return m, tea.Quit
		case "s", "S":
			if m.pathInPath {
				m.pathSubmitted = true
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *firstRunModel) renderModelSelect() string {
	var b strings.Builder
	md := "选择默认模型\n\n"
	md += "请选择一个 AI 模型作为默认对话模型。\n\n"
	help := "↑↓ 选择  **Enter** 确认  **Esc** 退出"
	if m.modelConfigured {
		help = "↑↓ 选择  **Enter** 确认  **S** 跳过 (使用已有配置)  **Esc** 退出"
	}
	b.WriteString(m.renderMarkdown(md))
	b.WriteString(m.modelList.View())
	b.WriteString("\n")
	b.WriteString(m.renderMarkdown(help))
	return m.paddedView(borderStyle.Render(b.String()))
}

func (m *firstRunModel) renderAPIKeyInput() string {
	var b strings.Builder
	b.WriteString(m.renderMarkdown("API Key 配置\n\n"))
	b.WriteString(fmt.Sprintf("模型: **%s**\n\n", m.selectedModel.Name))
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

func (m *firstRunModel) renderDaemonCheck() string {
	var b strings.Builder
	installed := DaemonInstalled(m.workspaceDir)
	if installed {
		b.WriteString(m.renderMarkdown(fmt.Sprintf(
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
	var b strings.Builder
	switch m.memoryState {
	case 0:
		md := `💾 记忆体配置

🔴 **Embedder 模型未下载**

Embedder 模型用于将文本向量化，实现语义搜索和 RAG 记忆。

下载 Chinese-CLIP 模型后，Agent 可以跨会话检索历史知识。
不下载则仅有基础会话记忆。

是否下载 Embedder 模型?

` + m.yesNoIndicator(m.memoryChoice) + `

← → 切换  **Enter** 确认  **Esc** 退出`
		b.WriteString(m.renderMarkdown(md))

	case 1:
		b.WriteString(m.renderMarkdown("⏳ 正在下载 Embedder 模型...\n\n"))
		if m.downloadStatus != "" {
			b.WriteString(m.renderMarkdown(m.downloadStatus))
			b.WriteString("\n\n")
		}
		b.WriteString(m.progressBar.View())
		b.WriteString("\n\n")
		b.WriteString(m.renderMarkdown("请等待下载完成..."))

	case 2:
		if m.downloadErr != nil {
			b.WriteString(m.renderMarkdown(fmt.Sprintf(
				"❌ 模型下载失败\n\n错误: %s\n\n你可以稍后运行 `mindx doctor` 重新尝试下载。\n\n**Enter** 继续",
				m.downloadErr.Error(),
			)))
		} else {
			b.WriteString(m.renderMarkdown("💾 记忆体配置\n\n✅ **Embedder 模型已就绪**\n\n记忆体功能已启用，支持语义搜索和 RAG 跨会话检索。\n\n**Enter** 继续  **S** 跳过"))
		}
	}
	return m.paddedView(borderStyle.Render(b.String()))
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

func (m *firstRunModel) View() tea.View {
	content := ""
	switch m.step {
	case 0:
		content = m.renderModelSelect()
	case 1:
		content = m.renderAPIKeyInput()
	case 2:
		content = m.renderDaemonCheck()
	case 3:
		content = m.renderPythonCheck()
	case 4:
		content = m.renderMemoryConfig()
	case 5:
		content = m.renderPathSetup()
	}
	return tea.NewView(content)
}
