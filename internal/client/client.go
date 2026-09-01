package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/timer"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/goharness/events"
	goharnesslogging "github.com/DotNetAge/goharness/logging"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/client/component/changes"
	"github.com/DotNetAge/mindx/internal/client/component/choices"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/component/dialog"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/component/notify"
	"github.com/DotNetAge/mindx/internal/client/component/permission"
	"github.com/DotNetAge/mindx/internal/client/component/sidebar"
	"github.com/DotNetAge/mindx/internal/client/component/statusbar"
	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	appcore "github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

const (
	overlayNone = iota
	overlayConnectProvider
	overlayConnectAPIKey
	overlayConnectModel
	// overlayModelSelect 是 /model 无参数回车触发的模型选择浮层，
	// 与 connect 流程的 overlayConnectModel 互不干扰（后者绑定 connectModels）。
	overlayModelSelect
	// overlayAgentSelect 是 /agent 无参数回车触发的 Agent 选择浮层。
	overlayAgentSelect
)

type rootModel struct {
	program              *tea.Program
	streamList           conv.StreamList
	welcome              *welcome.WelcomePanel
	statusBar            *statusbar.StatusBar
	sidebar              *sidebar.Sidebar
	input                *input.InputArea
	notifBar             *notify.NotificationBar
	askChoices           *choices.ChoicesPanel // AskUser 内联选项面板（替代浮层对话框）
	askChoicesActive     bool                  // 区分 ChoiceSelectedMsg 来自 AskUser 面板还是授权栏
	providerDlg          *dialog.ListDialog
	apiKeyDlg            *dialog.InputDialog
	modelDlg             *dialog.ListDialog // connect 流程的模型选择
	modelSelectDlg       *dialog.ListDialog // /model 命令的模型选择浮层
	agentSelectDlg       *dialog.ListDialog // /agent 命令的 Agent 选择浮层
	agentSelectNames     []string           // 浮层列表项与 Agents().List() 的名称映射
	connectProvider      string
	connectProviderNames []string
	connectAPIKey        string
	connectModels        []string
	connectModelNames    []string
	daemonAddr           string
	fileTracker          *changes.Tracker
	activeOverlay        int
	permBar              permission.PermissionBar
	viewport             viewport.Model
	termWidth            int
	termHeight           int
	leftWidth            int
	rightWidth           int
	scrollToBottom       bool

	app      *appcore.App
	registry *SlashCommandRegistry

	executing   bool
	postExitCmd string

	// currentCancel cancels the running agent execution (for interrupt/stop).
	currentCancel context.CancelFunc

	// pendingAskUserData tracks an active AskUserPending event (non-blocking) for the dialog overlay.
	pendingAskUserData *events.AskUserPendingData

	// rpcAskUserQuestions stores AskUser questions received via RPC (used when daemon is connected).
	rpcAskUserQuestions []struct {
		Question    string
		Options     []string
		MultiSelect bool
	}

	// RPC client for daemon communication.
	rpc          *daemonRPCClient
	rpcConnected bool

	// currentSessionID tracks the active session ID used in RPC messages.
	currentSessionID string

	// taskTracker 从 TaskCreate/TaskUpdate 工具结果中提取任务进度并同步到侧栏。
	taskTracker *taskTracker
}

func (m *rootModel) getLogger() goharnesslogging.Logger {
	if m.app != nil {
		return m.app.Logger()
	}
	return nil
}

var pendingPostExitCmd string

func NewProgram(cfg *appcore.MindxConfig) error {
	m := &rootModel{
		streamList:     conv.NewStreamList(),
		welcome:        welcome.New(),
		statusBar:      statusbar.New(),
		sidebar:        sidebar.New(),
		input:          input.New(),
		notifBar:       notify.New(),
		askChoices:     choices.New(),
		providerDlg:    dialog.NewListDialog(i18n.T("client.ui.dialog.provider.select")),
		apiKeyDlg:      dialog.NewInputDialog("API key", "API key"),
		modelDlg:       dialog.NewListDialog(i18n.T("client.ui.dialog.model.select")),
		modelSelectDlg: dialog.NewListDialog(i18n.T("client.ui.dialog.model.select")),
		agentSelectDlg: dialog.NewListDialog(i18n.T("client.ui.dialog.agent.select")),
		viewport:       viewport.New(),
		daemonAddr:     ":1314",
	}
	m.taskTracker = newTaskTracker()

	var err error
	m.app, err = appcore.DefaultApp(cfg)
	if err != nil {
		m.notifBar.Add(data.Notification{Message: fmt.Sprintf(i18n.T("client.notify.init.failed"), err), Level: data.NotifError})
	} else {
		m.registry = BuiltinCommands(CommandDeps{
			App:     m.app,
			OnClear: func() { m.program.Send(clientmsg.ClearScreenMsg{}) },
			OnExit:  func() { m.program.Send(clientmsg.ExitMsg{}) },
			OnDoctor: func() {
				m.postExitCmd = "doctor"
			},
			OnConnect: func() { m.startConnectFlow() },
			OnAgentSwitch: func(name string) {
				m.program.Send(clientmsg.AgentSwitchMsg{AgentName: name})
			},
		})
		m.loadCommands()

		if m.app != nil {
			if _, err := m.app.EnsureSession(); err != nil {
				fmt.Fprintf(os.Stderr, "\nFATAL: EnsureSession failed at startup: %v\n", err)
				os.Exit(1)
			}
		}
		m.populateWelcome()

		// Wire ModelConfig pricing into TUI components
		m.wirePricing()
	}

	fmt.Print("\x1b[2J\x1b[H")
	p := tea.NewProgram(m)
	m.program = p

	// Resolve initial session ID for RPC messages
	if m.app != nil {
		meta := m.app.CurrentSessionMeta()
		if meta != nil {
			m.currentSessionID = meta.SessionID
		}
	}

	// Connect to daemon for RPC mode
	m.connectDaemon()

	if _, err := p.Run(); err != nil {
		return err
	}

	if pendingPostExitCmd == "doctor" {
		fmt.Print("\n🔧 " + i18n.T("client.doctor.starting") + "\n\n")
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf(i18n.T("error.executable.path"), err)
		}
		cmd := exec.Command(self, "doctor")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(i18n.T("error.doctor.execute"), err)
		}
	}

	return nil
}

func (m *rootModel) handlePostExit() {
	pendingPostExitCmd = m.postExitCmd
}

func (m *rootModel) loadCommands() {
	cmds := m.registry.List()
	for _, c := range cmds {
		m.input.Commands = append(m.input.Commands, input.SlashCommand{
			Name:        c.Name,
			Description: c.Description,
		})
	}

	agents := m.app.Agents().List()
	for _, a := range agents {
		m.input.Agents = append(m.input.Agents, data.AgentInfo{
			Name:        a.Name,
			Description: a.Description,
		})
	}

	models := m.app.Models().List()
	for _, ml := range models {
		m.input.Models = append(m.input.Models, input.ModelItem{
			Name:        ml.Name,
			Description: ml.Description,
		})
	}

	sessions, _ := loadRecentSessions(m.app)
	m.input.Sessions = sessions
}

func loadRecentSessions(app *appcore.App) ([]input.SessionItem, error) {
	sessDB := app.SessDB()
	if sessDB == nil {
		return []input.SessionItem{
			{ID: "new", IsSpecial: true, SpecialType: "new"},
			{ID: "clear", IsSpecial: true, SpecialType: "clear"},
		}, nil
	}

	ctx := context.Background()
	sessions, err := goharnesssession.ListSessions(ctx, sessDB)
	if err != nil || len(sessions) == 0 {
		return []input.SessionItem{
			{ID: "new", IsSpecial: true, SpecialType: "new"},
			{ID: "clear", IsSpecial: true, SpecialType: "clear"},
		}, nil
	}

	var items []input.SessionItem
	maxSessions := 10
	if len(sessions) < maxSessions {
		maxSessions = len(sessions)
	}
	for i := 0; i < maxSessions; i++ {
		s := sessions[i]
		preview := ""
		if len(s.Messages) > 0 {
			preview = s.Messages[0].Content
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
		}
		items = append(items, input.SessionItem{
			ID:        s.SessionID,
			AgentName: s.AgentName,
			Preview:   preview,
		})
	}

	items = append(items, input.SessionItem{ID: "new", IsSpecial: true, SpecialType: "new"})
	items = append(items, input.SessionItem{ID: "clear", IsSpecial: true, SpecialType: "clear"})

	return items, nil
}

func (m *rootModel) loadSessionHistory() {
	if m.app == nil {
		return
	}
	sessDB := m.app.SessDB()
	if sessDB == nil {
		return
	}

	sessionMeta := m.app.CurrentSessionMeta()
	sessionID := ""
	agentName := ""
	if sessionMeta != nil && sessionMeta.SessionID != "" {
		sessionID = sessionMeta.SessionID
		agentName = sessionMeta.AgentName
	}
	if sessionID == "" {
		return
	}
	if agentName == "" {
		agentName = m.app.CurrentAgentName()
	}

	s, loadErr := goharnesssession.Load(context.Background(), sessionID, agentName, sessDB, m.getLogger(),
		goharnesssession.WithModelContextResolver(m.app.ModelContextLength),
	)
	if loadErr != nil || s == nil {
		return
	}
	msgs := s.All()
	m.streamList.Streams = append(m.streamList.Streams,
		conv.StreamsFromMessages(sessionID, agentName, msgs)...)
	m.scrollToBottom = true
}

func (m *rootModel) loadSessionTokenUsage(sessionID, agentName string) {
	if m.app == nil || m.app.TokenUsageStore() == nil {
		return
	}
	if sessionID == "" {
		return
	}

	ctx := context.Background()
	records, err := m.app.TokenUsageStore().Query(ctx, goharnesssession.TokenUsageFilter{
		SessionID: sessionID,
	})
	if err != nil || len(records) == 0 {
		return
	}

	// Accumulate all historical records and seed the TUI components
	var totalInput, totalOutput, totalCached, totalAll int
	for _, r := range records {
		totalInput += r.PromptTokens
		totalOutput += r.CompletionTokens
		totalCached += r.CachedTokens
		// 计费口径：prompt + completion - cached，与后端 chargeableTokens 保持一致
		actual := r.PromptTokens + r.CompletionTokens - r.CachedTokens
		if actual < 0 {
			actual = 0
		}
		totalAll += actual
	}

	// Reset and seed StatusBar
	m.statusBar.TokensTotal = totalAll
	m.statusBar.PromptTokens = totalInput
	m.statusBar.CompletionTokens = totalOutput
	m.statusBar.CachedTokens = totalCached

	// Reset and seed Sidebar
	m.sidebar.PromptTokens = totalInput
	m.sidebar.CompletionTokens = totalOutput
	m.sidebar.CachedTokens = totalCached
	m.sidebar.TotalTokens = totalAll
	if len(records) > 0 {
		if records[len(records)-1].ModelName != "" {
			m.sidebar.ModelName = records[len(records)-1].ModelName
		}
	}
}

func (m *rootModel) populateWelcome() {
	if m.app == nil {
		return
	}
	m.welcome.Data = data.WelcomeData{
		Version:   m.app.Config().AppVersion,
		ModelName: "unknown",
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.welcome.Data.Workspace = sessionMeta.ProjectDir
		m.welcome.Data.SessionID = sessionMeta.SessionID
	}
	// Fallback to actual working directory when no session is loaded yet
	if m.welcome.Data.Workspace == "" {
		if wd, err := os.Getwd(); err == nil {
			m.welcome.Data.Workspace = wd
		}
	}

	// Initialize file change tracker with the project directory
	if m.fileTracker == nil && m.welcome.Data.Workspace != "" {
		m.fileTracker = changes.NewTracker(m.welcome.Data.Workspace)
	}

	agentName := m.app.CurrentAgentName()
	m.welcome.Data.AgentName = agentName
	m.statusBar.AgentName = agentName

	// Get model info from config
	if cfg := m.app.Config(); cfg != nil && cfg.LastModel != "" {
		if modelCfg := m.app.Models().Get(cfg.LastModel); modelCfg != nil {
			m.welcome.Data.ModelName = displayName(modelCfg.Title, modelCfg.Name)
			m.updateModelDisplay(modelCfg)
		}
	}

	if sessionMeta != nil && sessionMeta.SessionID != "" {
		m.welcome.Data.SessionID = sessionMeta.SessionID
	}
	m.loadSessionHistory()
	m.sidebar.SetWelcomeData(m.welcome.Data)
}

func (m *rootModel) wirePricing() {
	if m.app == nil {
		return
	}

	// StatusBar: total cost function using ModelConfig or default fallback
	m.statusBar.CostFn = func(modelName string, promptTokens, completionTokens, cachedTokens int) float64 {
		per1MIn, per1MOut, per1MInCache := appcore.DefaultInputCost, appcore.DefaultOutputCost, 0.0
		if model := m.app.Models().Get(modelName); model != nil {
			per1MIn, per1MOut, per1MInCache = model.CostPer1MIn, model.CostPer1MOut, model.CostPer1MInCache
		}
		return appcore.CalculateCost(per1MIn, per1MOut, per1MInCache, int64(promptTokens), int64(completionTokens), int64(cachedTokens))
	}

	// Sidebar: per-component cost breakdown using ModelConfig or default fallback
	m.sidebar.CostFunc = func(modelName string, promptTokens, completionTokens, cachedTokens int) (float64, float64, float64) {
		per1MIn, per1MOut, per1MInCache := appcore.DefaultInputCost, appcore.DefaultOutputCost, 0.0
		if model := m.app.Models().Get(modelName); model != nil {
			per1MIn, per1MOut, per1MInCache = model.CostPer1MIn, model.CostPer1MOut, model.CostPer1MInCache
		}
		netInput := promptTokens - cachedTokens
		if netInput < 0 {
			netInput = 0
		}
		inputCost := per1MIn / 1_000_000 * float64(netInput)
		outputCost := per1MOut / 1_000_000 * float64(completionTokens)
		cachedCost := per1MInCache / 1_000_000 * float64(cachedTokens)
		return inputCost, outputCost, cachedCost
	}
}

func displayName(title, name string) string {
	if title != "" {
		return title
	}
	return name
}

// modelKeyOf 构造模型参照组合串：Provider + "/" + Name；Provider 为空时退化为仅 Name。
// 与 goharness config 的 modelKey 格式保持一致，参照字段（LastModel/DefaultModel）用它消除同名歧义。
func modelKeyOf(provider, name string) string {
	if provider == "" {
		return name
	}
	return provider + "/" + name
}

func (m *rootModel) providerDisplayName(providerName string) string {
	if m.app == nil || providerName == "" {
		return providerName
	}
	p := m.app.Models().GetProvider(providerName)
	if p != nil && p.Title != "" {
		return p.Title
	}
	return providerName
}

func (m *rootModel) updateModelDisplay(model *config.ModelConfig) {
	if model == nil {
		return
	}
	m.statusBar.ModelName = displayName(model.Title, model.Name)
	if model.Provider != "" {
		m.statusBar.Provider = m.providerDisplayName(model.Provider)
	}
}

func (m *rootModel) updateActiveDialog(msg any) (tea.Model, tea.Cmd) {
	switch m.activeOverlay {
	case overlayConnectProvider:
		newDlg, cmd := m.providerDlg.Update(msg)
		m.providerDlg = newDlg
		return m, cmd
	case overlayConnectAPIKey:
		newDlg, cmd := m.apiKeyDlg.Update(msg)
		m.apiKeyDlg = newDlg
		return m, cmd
	case overlayConnectModel:
		newDlg, cmd := m.modelDlg.Update(msg)
		m.modelDlg = newDlg
		return m, cmd
	case overlayModelSelect:
		newDlg, cmd := m.modelSelectDlg.Update(msg)
		m.modelSelectDlg = newDlg
		return m, cmd
	case overlayAgentSelect:
		newDlg, cmd := m.agentSelectDlg.Update(msg)
		m.agentSelectDlg = newDlg
		return m, cmd
	}
	return m, nil
}

func (m *rootModel) windowSizeMsg() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: m.termWidth, Height: m.termHeight}
}

func (m *rootModel) updateWithState(msg tea.Msg, state string, scroll bool) (tea.Model, tea.Cmd) {
	m.statusBar.CurrentState = state
	return m.updateConversation(msg, scroll)
}

// updateConversation updates the conversation list with msg and optionally
// scrolls the viewport to the bottom. Used by most Update cases to eliminate
// the repeated 3-line pattern.
func (m *rootModel) updateConversation(msg tea.Msg, scrollToBottom bool) (tea.Model, tea.Cmd) {
	if scrollToBottom {
		m.scrollToBottom = true
	}
	newList, cmd := m.streamList.Update(msg)
	m.streamList = newList
	return m, cmd
}

// ============================================================
// Overlay key routing (AskUser dialog)
// ============================================================

func (m *rootModel) handleOverlayKey(e tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	return m.updateActiveDialog(e)
}

func (m *rootModel) handleOverlayPaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	return m.updateActiveDialog(msg)
}

// activateInlineAskUser 把 AskUser 问题内联渲染进对话流，并激活选项面板。
// 面板活动期间输入栏隐藏（见 View），用户提交答案后自动恢复。
func (m *rootModel) activateInlineAskUser() {
	var question string
	var options []string
	var multiSelect bool

	if m.pendingAskUserData != nil && len(m.pendingAskUserData.Questions) > 0 {
		q := m.pendingAskUserData.Questions[0]
		question, options, multiSelect = q.Question, q.Options, q.MultiSelect
	} else if len(m.rpcAskUserQuestions) > 0 {
		q := m.rpcAskUserQuestions[0]
		question, options, multiSelect = q.Question, q.Options, q.MultiSelect
	} else {
		return
	}

	sessionID := m.currentSessionID
	if sessionID == "" && m.app != nil {
		if meta := m.app.CurrentSessionMeta(); meta != nil {
			sessionID = meta.SessionID
		}
	}
	if sessionID != "" {
		m.streamList.AppendQuestion(sessionID, question)
	}

	newPanel, _ := m.askChoices.Update(clientmsg.ShowChoicesMsg{
		Prompt:         question,
		Options:        options,
		MultiSelect:    multiSelect,
		AllowTextInput: true,
	})
	m.askChoices = newPanel
}

// mapAskUserReply builds the answer from the dialog result and sends it as a user message.
// In the non-blocking AskUser flow, the LLM loop has already exited; the user's answer
// re-enters the loop as a new user message.
func (m *rootModel) mapAskUserReply(isMulti bool, index int, indices []int, customText string, cancelled bool) {
	var options []string
	useRPC := m.rpcConnected && len(m.rpcAskUserQuestions) > 0

	if m.pendingAskUserData != nil && len(m.pendingAskUserData.Questions) > 0 {
		q := m.pendingAskUserData.Questions[0]
		options = q.Options
	} else if useRPC {
		q := m.rpcAskUserQuestions[0]
		options = q.Options
	} else {
		return
	}

	m.pendingAskUserData = nil
	m.rpcAskUserQuestions = nil

	if cancelled {
		return
	}

	var answer string
	if isMulti {
		var parts []string
		for _, idx := range indices {
			if idx >= 0 && idx < len(options) {
				parts = append(parts, options[idx])
			}
		}
		if customText != "" {
			if len(parts) > 0 {
				parts = append(parts, customText)
			} else {
				answer = customText
			}
		}
		if len(parts) > 0 {
			answer = strings.Join(parts, ", ")
		}
	} else {
		if customText != "" {
			answer = customText
		} else if index >= 0 && index < len(options) {
			answer = options[index]
		}
	}
	if answer == "" {
		return
	}

	// Non-blocking: send answer as user message to re-enter the LLM loop.
	if useRPC {
		if m.currentSessionID != "" {
			m.rpcSendMessage(answer)
		}
	} else {
		// Local path: use the question text as a new user message.
		// The answer is appended to the session as user input.
		m.program.Send(clientmsg.UserSendMsg{Text: answer})
	}
}

// ============================================================
// Connect flow: Provider → API Key → Model
// ============================================================

func (m *rootModel) startConnectFlow() {
	if m.app == nil {
		m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.system.uninitialized"), Level: data.NotifError})
		return
	}
	providers := m.app.Models().Providers()
	displayNames := make([]string, 0, len(providers))
	m.connectProviderNames = make([]string, 0, len(providers))
	for _, p := range providers {
		displayNames = append(displayNames, displayName(p.Title, p.Name))
		m.connectProviderNames = append(m.connectProviderNames, p.Name)
	}
	if len(displayNames) == 0 {
		m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.no.provider"), Level: data.NotifWarning})
		return
	}
	m.providerDlg.SetItems(displayNames)
	m.providerDlg.Update(m.windowSizeMsg())
	m.activeOverlay = overlayConnectProvider
}

func (m *rootModel) modelsForProvider(providerName string) []string {
	if m.app == nil {
		return nil
	}
	allModels := m.app.Models().ListRaw()
	var display []string
	var names []string
	for _, mc := range allModels {
		if mc.Provider == providerName {
			display = append(display, displayName(mc.Title, mc.Name))
			names = append(names, mc.Name)
		}
	}
	m.connectModelNames = names
	return display
}

func (m *rootModel) saveConnectResult(modelName string) {
	if m.app == nil || m.connectProvider == "" {
		return
	}

	reg := m.app.Models()

	// 规则3: TUI不应修改Provider的APIKey字段，应将实际值存入CredentialStore。
	if m.connectAPIKey != "" {
		credStore := appcore.NewCredentialStore(m.app.Settings().UserPreferences())
		_ = credStore.Set(m.connectProvider, m.connectAPIKey)
	}

	cfg := m.app.Config()
	if cfg != nil {
		cfg.DefaultProvider = m.connectProvider
		if modelName != "" {
			cfg.LastModel = modelKeyOf(m.connectProvider, modelName)
		}
		_ = cfg.Save()
	}

	if modelName != "" {
		if modelCfg := reg.Get(modelName); modelCfg != nil {
			m.welcome.Data.ModelName = displayName(modelCfg.Title, modelCfg.Name)
			m.updateModelDisplay(modelCfg)
			if cfg := m.app.Config(); cfg != nil {
				cfg.LastModel = modelKeyOf(modelCfg.Provider, modelName)
				_ = cfg.Save()
			}
			_ = reg.Save(modelCfg)
		}
		// no-op
		_ = m.connectAPIKey // nolint:SA9003
		// APIKey已存入CredentialStore（上方），无需再持久化模型配置
		// no-op
	}

	label := fmt.Sprintf(i18n.T("client.notify.connected"), m.connectProvider)
	if modelName != "" {
		label += fmt.Sprintf(" / %s", modelName)
	}
	m.notifBar.Add(data.Notification{Message: label, Level: data.NotifSuccess})

	m.connectProvider = ""
	m.connectAPIKey = ""
	m.connectModels = nil
}

// ============================================================
// Daemon health check (1-minute interval)
// ============================================================

type daemonCheckResultMsg struct {
	Status clientmsg.DaemonConnStatus
}

func checkDaemonCmd(addr string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(60 * time.Second)
		status := probeDaemon(addr)
		return daemonCheckResultMsg{Status: status}
	}
}

func probeDaemon(addr string) clientmsg.DaemonConnStatus {
	var host string
	if strings.HasPrefix(addr, ":") {
		host = "localhost" + addr
	} else if !strings.Contains(addr, ":") {
		host = addr + ":1314"
	} else {
		host = addr
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return clientmsg.DaemonDisconnected
	}
	_ = conn.Close()
	return clientmsg.DaemonConnected
}

// ============================================================
// Main bubbletea update loop
// ============================================================

func (m *rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.streamList.Init(),
		checkDaemonCmd(m.daemonAddr),
	)
}

func (m *rootModel) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := e.(type) {
	case tea.WindowSizeMsg:
		w := clientmsg.WindowResizeMsg{Width: msg.Width, Height: msg.Height}
		m.dispatchToAll(w)
		m.resizeViewport(msg.Width, msg.Height)
		m.providerDlg.Update(msg)
		m.apiKeyDlg.Update(msg)
		m.modelDlg.Update(msg)
		m.modelSelectDlg.Update(msg)
		m.agentSelectDlg.Update(msg)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.handlePostExit()
			return m, tea.Quit
		}

		// Priority 1: dialog overlay (connect flow)
		if m.activeOverlay != overlayNone {
			return m.handleOverlayKey(msg)
		}

		// Priority 2: inline AskUser choices（输入栏此时隐藏）
		if m.askChoices.Visible {
			newPanel, cmd := m.askChoices.Update(msg)
			m.askChoices = newPanel
			return m, cmd
		}

		// Priority 3: permission bar (tool security)
		if m.permBar.Visible {
			newBar, cmd := permission.UpdatePermissionBar(m.permBar, msg)
			m.permBar = newBar
			return m, cmd
		}

		m.input.Executing = m.executing
		_, inputCmd := m.input.Update(msg)
		newVp, vpCmd := m.viewport.Update(msg)
		m.viewport = newVp
		return m, tea.Batch(inputCmd, vpCmd)

	case tea.PasteMsg:
		if m.activeOverlay != overlayNone {
			return m.handleOverlayPaste(msg)
		}
		inp, cmd := m.input.Update(msg)
		m.input = inp
		return m, cmd

	case tea.MouseWheelMsg:
		newVp, cmd := m.viewport.Update(msg)
		m.viewport = newVp
		return m, cmd

	case clientmsg.WindowResizeMsg:
		m.dispatchToAll(msg)

	case clientmsg.UserSendMsg:
		return m.handleSend(msg)

	case clientmsg.AgentSwitchMsg:
		return m.handleAgentSwitch(msg)

	case clientmsg.SlashCommandMsg:
		return m.handleSlashCommand(msg)

	case clientmsg.SessionDoneMsg:
		m.executing = false
		m.currentCancel = nil
		return m.updateWithState(msg, i18n.T("client.status.idle"), true)

	case clientmsg.AgentErrorMsg:
		m.executing = false
		if !errors.Is(msg.Error, context.Canceled) {
			m.statusBar.CurrentState = i18n.T("client.status.error")
		}
		return m.updateConversation(msg, false)

	case timer.TickMsg:
		m.statusBar.Tick()
		return m.updateConversation(msg, false)

	case daemonCheckResultMsg:
		m.statusBar.DaemonStatus = msg.Status
		return m, checkDaemonCmd(m.daemonAddr)

	case clientmsg.ThinkingDeltaMsg, clientmsg.ThinkingDoneMsg:
		return m.updateWithState(msg, i18n.T("client.status.thinking"), true)

	case clientmsg.ContentDeltaMsg:
		return m.updateWithState(msg, i18n.T("client.status.thinking"), true)

	case clientmsg.ToolExecEndMsg:
		// 任务工具的结果驱动侧栏任务面板实时刷新（非任务工具无副作用）。
		if m.taskTracker.applyToolResult(msg.ToolName, msg.Result) {
			m.sidebar.SetTasks(m.taskTracker.snapshot())
		}
		return m.updateWithState(msg, i18n.T("client.status.executing"), true)

	case clientmsg.ToolUseDeltaMsg, clientmsg.ToolExecStartMsg:
		return m.updateWithState(msg, i18n.T("client.status.executing"), true)

	case clientmsg.ExecutionSummaryMsg:
		m.statusBar.Update(msg)
		m.sidebar.AddTokenUsage(
			msg.TokensUsed.PromptTokens,
			msg.TokensUsed.CompletionTokens,
			msg.TokensUsed.CachedTokens,
			msg.TokensUsed.TotalTokens,
			m.statusBar.ModelName,
		)
		return m.updateConversation(msg, true)

	case clientmsg.FinalAnswerMsg:
		m.statusBar.CurrentState = i18n.T("client.status.complete")
		m.statusBar.Update(msg)
		return m.updateConversation(msg, true)

	case clientmsg.CollapseToggleMsg, clientmsg.ThinkCollapseMsg:
		return m.updateConversation(msg, false)

	case clientmsg.ToggleToolsFoldMsg:
		// 输入栏发出时不携带会话归属，此处补齐后转发给对话流。
		if msg.SessionID == "" {
			msg.SessionID = m.currentSessionID
		}
		return m.updateConversation(msg, false)

	case clientmsg.ClearScreenMsg:
		return m.updateWithState(msg, i18n.T("client.status.idle"), false)

	case clientmsg.MaxTurnsReachedMsg:
		// 正常边界（非错误）：结束执行态并给出建议提示。
		m.executing = false
		m.statusBar.CurrentState = i18n.T("client.status.complete")
		return m.updateConversation(msg, true)

	case clientmsg.TaskSummaryMsg, clientmsg.SubtaskSpawnedMsg, clientmsg.SubtaskCompletedMsg:
		return m.updateConversation(msg, true)

	// --- Inline AskUser: 问题入对话流 + 内联 Choices ---

	case clientmsg.AskUserEventMsg:
		m.statusBar.CurrentState = i18n.T("client.status.waiting.answer")
		m.activateInlineAskUser()
		m.askChoicesActive = m.askChoices.Visible
		return m, nil

	// --- Connect flow: Provider → API Key → Model ---

	case dialog.ListDialogResult:
		switch m.activeOverlay {
		case overlayConnectProvider:
			m.activeOverlay = overlayNone
			if !msg.Cancelled && msg.Index >= 0 && msg.Index < len(m.connectProviderNames) {
				m.connectProvider = m.connectProviderNames[msg.Index]
				m.activeOverlay = overlayConnectAPIKey
				m.apiKeyDlg = dialog.NewInputDialog("API key", "API key")
				m.apiKeyDlg.Visible = true
				m.apiKeyDlg.Update(m.windowSizeMsg())
			}
		case overlayConnectModel:
			m.activeOverlay = overlayNone
			if !msg.Cancelled && msg.Index >= 0 && msg.Index < len(m.connectModelNames) {
				m.saveConnectResult(m.connectModelNames[msg.Index])
			}
		case overlayModelSelect:
			m.activeOverlay = overlayNone
			if !msg.Cancelled && msg.Index >= 0 {
				models := m.app.Models().List()
				if msg.Index < len(models) {
					return m.handleSlashCommand(clientmsg.SlashCommandMsg{Name: "model", Args: []string{models[msg.Index].Name}})
				}
			}
		case overlayAgentSelect:
			m.activeOverlay = overlayNone
			if !msg.Cancelled && msg.Index >= 0 && msg.Index < len(m.agentSelectNames) {
				return m.handleSlashCommand(clientmsg.SlashCommandMsg{Name: "agent", Args: []string{m.agentSelectNames[msg.Index]}})
			}
		}
		return m, nil

	case dialog.InputDialogResult:
		if m.activeOverlay == overlayConnectAPIKey {
			m.activeOverlay = overlayNone
			if !msg.Cancelled && msg.Value != "" {
				m.connectAPIKey = msg.Value
				m.connectModels = m.modelsForProvider(m.connectProvider)
				if len(m.connectModels) > 0 {
					m.modelDlg = dialog.NewListDialog(i18n.T("client.ui.dialog.model.select"))
					m.modelDlg.SetItems(m.connectModels)
					m.modelDlg.Update(m.windowSizeMsg())
					m.activeOverlay = overlayConnectModel
				} else {
					m.saveConnectResult("")
				}
			}
		}
		return m, nil

	// --- Permission request (tool security) ---

	case clientmsg.PermissionRequestMsg:
		m.statusBar.CurrentState = i18n.T("client.status.waiting.choice")
		newBar, _ := permission.UpdatePermissionBar(m.permBar, msg)
		m.permBar = newBar
		return m, nil

	case clientmsg.ChoiceSelectedMsg:
		// 来源分流：内联 AskUser 面板 → 组装答案作为用户消息重入对话循环；
		// 授权栏 → 发送对应魔术词。
		if m.askChoicesActive {
			m.askChoicesActive = false
			m.statusBar.CurrentState = i18n.T("client.status.idle")
			cancelled := msg.Index < 0 && len(msg.Indices) == 0
			m.mapAskUserReply(len(msg.Indices) > 0, msg.Index, msg.Indices, msg.CustomText, cancelled)
			return m, nil
		}

		m.statusBar.CurrentState = i18n.T("client.status.idle")
		// 记录授权请求来源会话 ID，用于构造带目标魔法词；随后清空 permBar。
		pendingSessionID := m.permBar.SessionID
		m.permBar = permission.PermissionBar{}

		if msg.Index < 0 {
			// User cancelled → nothing to do (LLM loop already paused)
			return m, nil
		}

		if msg.Index == permission.PermissionAllow || msg.Index == permission.PermissionAllowSession {
			magicWord := "PermissionAllow"
			if msg.Index == permission.PermissionAllowSession {
				magicWord = "PermissionAllowSession"
			}
			// 子智能体授权冒泡：携带目标子会话 ID 精确路由，避免多个子会话
			// 并发挂起时按先到先服务错位；主会话自身授权请求 SessionID 为空，
			// 保持无目标旧行为。
			if pendingSessionID != "" {
				magicWord += ": " + pendingSessionID
			}
			m.rpcSendMagicWord(magicWord)
		} else if msg.Index == permission.PermissionDeny {
			// 拒绝同样需要送达：主会话自身授权 → 合成拒绝结果并继续循环；
			// 子会话授权冒泡 → 路由拒绝让子会话换路执行，而非挂到 10 分钟超时。
			magicWord := "PermissionDeny"
			if pendingSessionID != "" {
				magicWord += ": " + pendingSessionID
			}
			m.rpcSendMagicWord(magicWord)
		}
		return m, nil

	// --- svc 事件对齐：会话队列 / 压缩 / 中断 / 用量 / 调度与升级广播 ---

	case clientmsg.MessageQueuedMsg:
		return m.updateWithState(msg, i18n.T("client.status.queued"), false)

	case clientmsg.MessageProcessingMsg:
		return m.updateWithState(msg, i18n.T("client.status.executing"), false)

	case clientmsg.LLMCancelledMsg:
		m.executing = false
		m.statusBar.CurrentState = i18n.T("client.status.cancelled")
		return m.updateConversation(msg, false)

	case clientmsg.LLMTimeoutMsg:
		return m.updateConversation(msg, false)

	case clientmsg.CompactionMsg, clientmsg.ContextCompactionMsg:
		m.notifBar.Add(data.Notification{
			Message: i18n.T("client.notify.context.compacted"),
			Level:   data.NotifInfo,
		})
		return m, nil

	case clientmsg.PermissionDeniedMsg:
		m.notifBar.Add(data.Notification{
			Message: fmt.Sprintf(i18n.T("client.notify.permission.denied"), msg.Reason),
			Level:   data.NotifWarning,
		})
		return m, nil

	case clientmsg.TokenUsageRecordedMsg:
		rec := msg.Record
		m.sidebar.AddTokenUsage(rec.PromptTokens, rec.CompletionTokens, rec.CachedTokens, rec.TotalTokens, rec.ModelName)
		return m, nil

	case clientmsg.ScheduleJobMsg:
		m.notifBar.Add(data.Notification{
			Message: fmt.Sprintf(i18n.T("client.notify.schedule.job"), msg.Status, msg.Content),
			Level:   data.NotifInfo,
		})
		return m, nil

	case clientmsg.DaemonUpdateMsg:
		if msg.Phase == "installed" {
			m.notifBar.Add(data.Notification{
				Message: fmt.Sprintf(i18n.T("client.notify.update.installed"), msg.Version),
				Level:   data.NotifWarning,
			})
		} else {
			m.notifBar.Add(data.Notification{
				Message: fmt.Sprintf(i18n.T("client.notify.update.available"), msg.Version),
				Level:   data.NotifInfo,
			})
		}
		return m, nil

	// --- Notifications ---

	case clientmsg.NotifTimeoutMsg:
		_, cmd := m.notifBar.Update(msg)
		return m, cmd

	case clientmsg.SessionLoadedMsg:
		m.statusBar.Update(msg)
		m.loadSessionTokenUsage(msg.SessionID, msg.AgentName)
		return m, nil

	case clientmsg.ExecutionCancelMsg:
		m.executing = false
		m.statusBar.CurrentState = i18n.T("client.status.cancelled")
		if m.rpcConnected {
			m.rpcCancelExecution()
		} else if m.currentCancel != nil {
			m.currentCancel()
			m.currentCancel = nil
		}
		return m, nil

	case tea.InterruptMsg:
		m.executing = false
		m.handlePostExit()
		return m, tea.Quit

	case clientmsg.ExitMsg:
		m.handlePostExit()
		return m, tea.Quit
	}

	return m, nil
}

func (m *rootModel) dispatchToAll(w clientmsg.WindowResizeMsg) {
	m.welcome.Update(w)
	m.statusBar.Update(w)
	m.input.Update(w)
	m.notifBar.Update(w)
	newPanel, _ := m.askChoices.Update(w)
	m.askChoices = newPanel
	// streamList 不在此分发：它必须使用 viewport 实际宽度（leftWidth），
	// 全屏宽度会导致 markdown 按整屏渲染、右侧被 Sidebar 遮挡（见 resizeViewport）。
}

const (
	// resizeViewport 在首帧前无法得知各区域实际行数，只能用保守估计初始化；
	// 首帧起由 View() 的逐行统计经 updateViewportHeights 接管，消除双轨制抖动。
	resizeHeaderEstimate = 8
	resizeFooterEstimate = 5
	layoutSeparators     = 2
	resizeMinViewport    = 5
	viewMinViewport      = 3
)

func (m *rootModel) resizeViewport(termWidth, termHeight int) {
	m.termWidth = termWidth
	m.termHeight = termHeight

	m.leftWidth = termWidth*3/4 - 1
	if m.leftWidth < 40 {
		m.leftWidth = termWidth - 30
	}
	m.rightWidth = termWidth - m.leftWidth - 1
	if m.rightWidth < 20 {
		m.rightWidth = 20
	}

	vpHeight := termHeight - resizeHeaderEstimate - resizeFooterEstimate - layoutSeparators
	if vpHeight < resizeMinViewport {
		vpHeight = resizeMinViewport
	}
	m.updateViewportHeights(vpHeight)

	// 对话流宽度 = viewport 实际宽度。全屏宽度会让内容渲染到 Sidebar 底下。
	m.streamList.Update(clientmsg.WindowResizeMsg{Width: m.leftWidth, Height: vpHeight})

	m.sidebar.Update(clientmsg.WindowResizeMsg{Width: m.rightWidth - 2, Height: vpHeight})
}

// updateViewportHeights 是视口高度写入的唯一入口：viewport 与侧边栏必须同帧一致，
// 分散写入正是原先 resizeViewport 固定值与 View 动态值互相打架的根源。
func (m *rootModel) updateViewportHeights(vpHeight int) {
	m.viewport.SetWidth(m.leftWidth)
	m.viewport.SetHeight(vpHeight)
	m.sidebar.SyncHeight(vpHeight)
}

func (m *rootModel) handleSend(e clientmsg.UserSendMsg) (tea.Model, tea.Cmd) {
	if m.executing {
		return m, m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.message.processing"), Level: data.NotifWarning})
	}

	agentName := m.app.CurrentAgentName()
	sessionMeta := m.app.CurrentSessionMeta()
	sessionID := ""
	if sessionMeta != nil {
		sessionID = sessionMeta.SessionID
	}

	preview := e.Text
	if len([]rune(preview)) > 80 {
		preview = string([]rune(preview)[:80]) + "..."
	}

	m.streamList.AppendUserMessage(sessionID, agentName, e.Text)

	// Use RPC path when connected to daemon
	if m.rpcConnected {
		text := e.Text
		if !strings.HasPrefix(text, "@") {
			text = "@" + agentName + " " + text
		}
		m.currentSessionID = sessionID
		m.rpcSendMessage(text)
		return m, nil
	}

	// Fallback: in-process execution
	m.executing = true
	m.statusBar.CurrentState = i18n.T("client.status.thinking")
	m.statusBar.SessionStart = time.Now()
	m.statusBar.SessionDuration = 0

	if sessionMeta == nil || sessionMeta.SessionID == "" {
		panic(fmt.Sprintf("FATAL: no active session meta (app.EnsureSession must be called before send), agent=%s", m.app.CurrentAgentName()))
	}

	if l := m.getLogger(); l != nil {
		l.Info("user send (local)", "session", sessionID, "preview", preview)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.currentCancel = cancel

	go func() {
		defer cancel()
		defer func() {
			if p := recover(); p != nil {
				if l := m.getLogger(); l != nil {
					l.Error("handleSend panic", fmt.Errorf("%v", p), "session", sessionID)
				}
				m.program.Send(clientmsg.AgentErrorMsg{
					SessionID: sessionID,
					Error:     fmt.Errorf("handleSend panic: %v", p),
				})
			}
		}()

		rt, err := m.app.CurrentRuntime()
		if err != nil {
			m.program.Send(clientmsg.AgentErrorMsg{SessionID: sessionID, Error: fmt.Errorf(i18n.T("client.notify.runtime.unavailable"), err)})
			m.program.Send(clientmsg.SessionDoneMsg{SessionID: sessionID})
			return
		}

		s := m.app.NewSessionFromMeta()
		if s == nil {
			m.program.Send(clientmsg.AgentErrorMsg{SessionID: sessionID, Error: fmt.Errorf("%s", i18n.T("client.notify.session.create.failed"))})
			m.program.Send(clientmsg.SessionDoneMsg{SessionID: sessionID})
			return
		}

		ask := rt.Ask(agentName, e.Text, s).WithContext(ctx)

		ask.OnContent(func(c string) {
			m.program.Send(clientmsg.ContentDeltaMsg{SessionID: sessionID, Content: c})
		})
		ask.OnThinking(func(c string) {
			m.program.Send(clientmsg.ThinkingDeltaMsg{SessionID: sessionID, Content: c})
		})
		ask.OnToolUseDelta(func(d events.ToolUseDeltaData) {
			m.program.Send(clientmsg.ToolUseDeltaMsg{
				SessionID: sessionID, Index: d.Index, ID: d.ID, Name: d.Name, Arguments: d.Arguments,
			})
		})
		ask.OnThinkingDone(func() {
			m.program.Send(clientmsg.ThinkingDoneMsg{SessionID: sessionID})
		})
		ask.OnToolStart(func(d events.ToolExecStartData) {
			if m.fileTracker != nil {
				m.fileTracker.ToolExecStart(d.Params)
			}
			m.program.Send(clientmsg.ToolExecStartMsg{
				SessionID: sessionID, ToolName: d.ToolName, Params: d.Params,
			})
		})
		ask.OnToolEnd(func(d events.ToolExecEndData) {
			var diffText string
			var diffAdds, diffDels int
			var diffFile string
			if m.fileTracker != nil {
				m.fileTracker.ToolExecEnd()
				changes := m.fileTracker.Snapshot()
				m.sidebar.SetFileChanges(changes)
				if len(changes) > 0 {
					last := changes[len(changes)-1]
					diffText = last.Diff
					diffAdds = last.Additions
					diffDels = last.Deletions
					diffFile = last.File
				}
			}
			m.program.Send(clientmsg.ToolExecEndMsg{
				SessionID: sessionID, ToolName: d.ToolName, ToolCallID: d.ToolCallID,
				Success: d.Success, Result: d.Result, Error: d.Error, Duration: d.Duration,
				DiffText: diffText, DiffAdds: diffAdds, DiffDels: diffDels, DiffFile: diffFile,
			})
		})
		ask.OnExecutionSummary(func(d events.ExecutionSummaryData) {
			m.program.Send(clientmsg.ExecutionSummaryMsg{
				SessionID: sessionID, Duration: d.TotalDuration, TokensUsed: d.TokensUsed, ToolCalls: d.ToolCalls,
			})
		})
		ask.OnLoopEnd(func(d events.CycleInfo) {
			// TerminationReason 是判定「本轮是否产出最终答案」的权威信号
			// （completed 等价于 OpenAI finish_reason=stop），必须透传给对话流，
			// 供其精确区分过程 content 与最终答案（推理区语义）。
			m.program.Send(clientmsg.IterationMsg{
				SessionID: sessionID, Iteration: d.Iteration,
				TerminationReason: d.TerminationReason, Duration: d.Duration,
			})
		})
		var tokenUsage goharnesssession.TokenUsage
		ask.OnTokenUsageRecorded(func(d goharnesssession.TokenUsageRecord) {
			tokenUsage.PromptTokens += d.PromptTokens
			tokenUsage.CompletionTokens += d.CompletionTokens
			tokenUsage.CachedTokens += d.CachedTokens
			tokenUsage.ReasoningTokens += d.ReasoningTokens
			// 累计计费口径，与后端 chargeableTokens 保持一致
			actual := d.PromptTokens + d.CompletionTokens - d.CachedTokens
			if actual < 0 {
				actual = 0
			}
			tokenUsage.TotalTokens += actual
			tokenUsage.Timestamp = d.Timestamp
			m.program.Send(clientmsg.ExecutionSummaryMsg{
				SessionID:  sessionID,
				TokensUsed: tokenUsage,
			})
		})
		// NOTE: Old WithGrantCache / localGrantCache non-blocking permission
		// flow has been removed. Permission resumption now flows through
		// the PermissionAllow / PermissionDeny magic words (see
		// agents.resolvePermissionMagicWord). The runtime intercepts the
		// magic word before it reaches the LLM, drains
		// session.PendingPermission, and runs the tool (Allow) or appends
		// a "Permission Denied" result (Deny).

		ask.OnAskUserPending(func(d events.AskUserPendingData) {
			m.pendingAskUserData = &d
			m.program.Send(clientmsg.AskUserEventMsg{})
		})
		ask.OnPermissionPending(func(d events.PermissionPendingData) {
			m.program.Send(clientmsg.PermissionRequestMsg{
				ToolName: d.ToolName, Reason: d.Reason, SecurityLevel: int(d.SecurityLevel),
			})
		})
		ask.OnError(func(errStr string) {
			m.program.Send(clientmsg.AgentErrorMsg{SessionID: sessionID, Error: errors.New(errStr)})
		})
		ask.OnLLMTimeout(func(d events.LLMTimeoutData) {
			m.program.Send(clientmsg.LLMTimeoutMsg{
				SessionID: sessionID, Timeout: d.Timeout, Elapsed: d.Elapsed, Error: d.Error,
			})
		})
		ask.OnLLMCancelled(func(d events.LLMCancelledData) {
			m.program.Send(clientmsg.LLMCancelledMsg{
				SessionID: sessionID, Elapsed: d.Elapsed,
			})
		})
		ask.OnLLMRetry(func(d events.LLMRetryData) {
			m.program.Send(clientmsg.LLMRetryMsg{
				SessionID: sessionID, Provider: d.Provider, Model: d.Model,
				StatusCode: d.StatusCode, Attempt: d.Attempt, MaxAttempts: d.MaxAttempts,
				RetryAfter: d.RetryAfter, Error: d.Error, Phase: d.Phase,
			})
		})
		ask.OnMaxTurnsReached(func(d events.MaxTurnsReachedData) {
			m.program.Send(clientmsg.MaxTurnsReachedMsg{
				SessionID: sessionID, TurnsCompleted: d.TurnsCompleted, MaxTurns: d.MaxTurns, Suggestion: d.Suggestion,
			})
		})
		ask.OnAnswer(func(answer string) {
			m.program.Send(clientmsg.FinalAnswerMsg{SessionID: sessionID, Content: answer})
		})

		_, err = ask.Run()
		if err != nil {
			m.program.Send(clientmsg.AgentErrorMsg{SessionID: sessionID, Error: err})
		}
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: sessionID})
	}()

	return m, nil
}

// resolveSessionForAgent 落实"Agent 切换即 Session 切换"的强规则：
// 从服务端读取当前 ProjectDir 下的全部会话，过滤出新 Agent 所属的会话并取
// 最新一条；不存在则在该 Agent + ProjectDir 下创建新会话。
func (m *rootModel) resolveSessionForAgent(agentName string) (*goharnesssession.SessionInfo, error) {
	projectDir := ""
	if meta := m.app.CurrentSessionMeta(); meta != nil && meta.ProjectDir != "" {
		projectDir = meta.ProjectDir
	} else if wd, err := os.Getwd(); err == nil {
		projectDir = wd
	}

	ctx := context.Background()
	sessions, err := goharnesssession.ListSessions(ctx, m.app.SessDB())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var latest *goharnesssession.SessionInfo
	for i := range sessions {
		s := &sessions[i]
		if s.AgentName != agentName || s.ProjectDir != projectDir {
			continue
		}
		if latest == nil || s.LastActivityAt.After(latest.LastActivityAt) {
			latest = s
		}
	}
	if latest != nil {
		return m.app.SwitchSession(latest.SessionID)
	}
	return m.app.CreateSession(agentName, projectDir)
}

func (m *rootModel) handleAgentSwitch(e clientmsg.AgentSwitchMsg) (tea.Model, tea.Cmd) {
	if m.executing {
		m.executing = false
	}

	// Update the app config with the new agent name
	cfg := m.app.Config()
	if cfg != nil {
		cfg.LastAgent = e.AgentName
		_ = cfg.Save()
	}

	// 强规则：Agent 变更必须联动 Session 切换（复用最新或创建新的），
	// 失败不阻断切换本身——保留旧会话上下文并提示用户。
	newMeta, sessErr := m.resolveSessionForAgent(e.AgentName)
	var sessNotifCmd tea.Cmd
	if sessErr != nil {
		sessNotifCmd = m.notifBar.Add(data.Notification{
			Message: fmt.Sprintf(i18n.T("client.notify.session.resolve.failed"), e.AgentName, sessErr),
			Level:   data.NotifWarning,
		})
	}

	m.statusBar.AgentName = e.AgentName

	// Update model display from the new agent's configured model
	agent := m.app.Agents().Get(e.AgentName)
	if agent != nil && agent.Model != "" {
		if modelCfg := m.app.Models().Get(agent.Model); modelCfg != nil {
			m.updateModelDisplay(modelCfg)
		}
	}

	// Session 相关数据全部刷新：对话流按新会话历史重建，
	// 输入框会话列表、侧栏欢迎信息、任务面板随新会话归零。
	m.streamList.Clear()
	m.loadSessionHistory()
	if newSessions, err := loadRecentSessions(m.app); err == nil {
		m.input.Sessions = newSessions
	}
	m.taskTracker.reset()
	m.sidebar.SetTasks(nil)
	if newMeta != nil {
		m.currentSessionID = newMeta.SessionID
		m.statusBar.Update(clientmsg.SessionLoadedMsg{
			AgentName: newMeta.AgentName,
			SessionID: newMeta.SessionID,
		})
		m.welcome.Data.SessionID = newMeta.SessionID
	}
	m.sidebar.SetWelcomeData(m.welcome.Data)
	m.scrollToBottom = true

	switchedCmd := m.notifBar.Add(data.Notification{
		Message: fmt.Sprintf(i18n.T("client.notify.agent.switched"), e.AgentName),
		Level:   data.NotifInfo,
	})
	return m, tea.Batch(sessNotifCmd, switchedCmd)
}

func (m *rootModel) handleSlashCommand(e clientmsg.SlashCommandMsg) (tea.Model, tea.Cmd) {
	cmd := m.registry.Get(e.Name)
	if cmd == nil {
		return m, m.notifBar.Add(data.Notification{Message: fmt.Sprintf(i18n.T("client.notify.command.unknown"), e.Name), Level: data.NotifWarning})
	}

	result := cmd.Run(e.Args)

	switch e.Name {
	case "chat":
		clearCmd := m.refreshAfterChatOp(result)
		if result.Message != "" {
			level := data.NotifInfo
			if result.Success {
				level = data.NotifSuccess
			}
			return m, tea.Batch(
				m.notifBar.Add(data.Notification{Message: result.Message, Level: level}),
				clearCmd,
			)
		}
		return m, clearCmd
	case "model":
		// 无参数：升级为居中浮层选择器（替代原先塞进通知栏的多行文本列表）。
		if len(e.Args) == 0 {
			return m.openModelSelectDialog()
		}
		if result.Success {
			modelName := e.Args[0]
			if modelCfg := m.app.Models().Get(modelName); modelCfg != nil {
				m.welcome.Data.ModelName = displayName(modelCfg.Title, modelCfg.Name)
				m.updateModelDisplay(modelCfg)
				if cfg := m.app.Config(); cfg != nil {
					cfg.LastModel = modelKeyOf(modelCfg.Provider, modelName)
					_ = cfg.Save()
				}
			}
			result.Message = fmt.Sprintf(i18n.T("client.notify.model.switched"), modelName)
		}
		m.input.Models, _ = reloadModels(m.app)
	case "agent":
		// 无参数：打开 Agent 浮层选择器（Role (name) - Description 格式）。
		if len(e.Args) == 0 {
			return m.openAgentSelectDialog()
		}
	case "doctor":
		m.handlePostExit()
		return m, tea.Quit
	}

	if result.Message != "" {
		level := data.NotifInfo
		if result.Success {
			level = data.NotifSuccess
		}
		return m, m.notifBar.Add(data.Notification{Message: result.Message, Level: level})
	}
	return m, nil
}

func (m *rootModel) refreshAfterChatOp(result CommandResult) tea.Cmd {
	if !result.Success {
		return nil
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.statusBar.Update(clientmsg.SessionLoadedMsg{
			AgentName: sessionMeta.AgentName,
			SessionID: sessionMeta.SessionID,
		})
		m.statusBar.AgentName = sessionMeta.AgentName
		m.welcome.Data.SessionID = sessionMeta.SessionID
	}

	// Update model display from config
	if cfg := m.app.Config(); cfg != nil && cfg.LastModel != "" {
		if modelCfg := m.app.Models().Get(cfg.LastModel); modelCfg != nil {
			m.updateModelDisplay(modelCfg)
		}
	}

	m.streamList.Clear()
	m.loadSessionHistory()
	newSessions, _ := loadRecentSessions(m.app)
	m.input.Sessions = newSessions

	// 会话切换后任务快照随之失效：清空追踪并同步侧栏。
	// 历史会话的任务状态在 KVStore 中持久存在，但 TUI 不回放（与对话流同策略）。
	m.taskTracker.reset()
	m.sidebar.SetTasks(nil)

	return func() tea.Msg {
		return clientmsg.ClearScreenMsg{}
	}
}

// openModelSelectDialog 激活 /model 的居中浮层选择器。
// 列表项与 m.app.Models().List() 同序，ListDialogResult.Index 直接映射回模型。
func (m *rootModel) openModelSelectDialog() (tea.Model, tea.Cmd) {
	if m.app == nil || m.app.Models() == nil {
		return m, m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.system.uninitialized"), Level: data.NotifWarning})
	}
	models := m.app.Models().List()
	if len(models) == 0 {
		return m, m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.no.provider"), Level: data.NotifWarning})
	}
	names := make([]string, len(models))
	for i, ml := range models {
		names[i] = displayName(ml.Title, ml.Name)
	}
	m.modelSelectDlg = dialog.NewListDialog(i18n.T("client.ui.dialog.model.select"))
	m.modelSelectDlg.SetItems(names)
	m.modelSelectDlg.Update(m.windowSizeMsg())
	m.activeOverlay = overlayModelSelect
	return m, nil
}

// openAgentSelectDialog 激活 /agent 的居中浮层选择器。
// 列表项格式为 "Role (name) - Description"，agentSelectNames 保持与
// Agents().List() 同序，ListDialogResult.Index 映射回 Agent 名称后
// 走既有 handleAgentSwitch 链路（状态栏/配置持久化零改动）。
func (m *rootModel) openAgentSelectDialog() (tea.Model, tea.Cmd) {
	if m.app == nil || m.app.Agents() == nil {
		return m, m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.system.uninitialized"), Level: data.NotifWarning})
	}
	agents := m.app.Agents().List()
	if len(agents) == 0 {
		return m, m.notifBar.Add(data.Notification{Message: i18n.T("client.notify.no.provider"), Level: data.NotifWarning})
	}
	items := make([]string, len(agents))
	m.agentSelectNames = make([]string, len(agents))
	for i, a := range agents {
		m.agentSelectNames[i] = a.Name
		if a.Role != "" {
			items[i] = fmt.Sprintf("%s (%s) - %s", a.Role, a.Name, a.Description)
		} else {
			items[i] = displayName(a.Name, a.Name) + " - " + a.Description
		}
	}
	m.agentSelectDlg = dialog.NewListDialog(i18n.T("client.ui.dialog.agent.select"))
	m.agentSelectDlg.SetItems(items)
	m.agentSelectDlg.Update(m.windowSizeMsg())
	m.activeOverlay = overlayAgentSelect
	return m, nil
}

func reloadModels(app *appcore.App) ([]input.ModelItem, error) {
	if app == nil || app.Models() == nil {
		return []input.ModelItem{}, nil
	}
	models := app.Models().List()
	var items []input.ModelItem
	for _, ml := range models {
		items = append(items, input.ModelItem{
			Name:        ml.Name,
			Description: ml.Description,
		})
	}
	return items, nil
}

func (m *rootModel) View() tea.View {
	notifView := m.notifBar.View()
	statusView := m.statusBar.View()
	inputView := m.input.View()
	permView := permission.ViewPermissionBar(m.permBar, m.termWidth)
	choicesView := m.askChoices.View()

	headerStr := notifView

	m.input.Hidden = m.permBar.Visible || m.askChoices.Visible
	bottomArea := inputView

	// 底部区域互斥：授权栏 > 内联 AskUser 选项 > 输入栏。
	switch {
	case m.permBar.Visible:
		bottomArea = permView
	case m.askChoices.Visible:
		bottomArea = choicesView
	}

	headerLines := strings.Count(headerStr, "\n") + 1
	statusLines := strings.Count(statusView, "\n") + 1
	bottomLines := strings.Count(bottomArea, "\n") + 1
	vpHeight := m.termHeight - headerLines - statusLines - bottomLines - layoutSeparators
	if vpHeight < viewMinViewport {
		vpHeight = viewMinViewport
	}
	m.updateViewportHeights(vpHeight)

	m.viewport.SetContent(m.streamList.View())

	if m.scrollToBottom {
		m.viewport.GotoBottom()
		m.scrollToBottom = false
	}

	mainArea := m.viewport.View()

	sideArea := m.sidebar.View()

	layout := lipgloss.JoinHorizontal(lipgloss.Top, mainArea, sideArea)

	full := lipgloss.JoinVertical(lipgloss.Left,
		headerStr,
		layout,
		statusView,
		bottomArea,
	)

	// Dialog overlay: render full-screen centered if active (connect flow).
	if m.activeOverlay != overlayNone {
		var modal string
		switch m.activeOverlay {
		case overlayConnectProvider:
			modal = m.providerDlg.View()
		case overlayConnectAPIKey:
			modal = m.apiKeyDlg.View()
		case overlayConnectModel:
			modal = m.modelDlg.View()
		case overlayModelSelect:
			modal = m.modelSelectDlg.View()
		case overlayAgentSelect:
			modal = m.agentSelectDlg.View()
		}
		if modal != "" {
			full = lipgloss.Place(m.termWidth, m.termHeight, lipgloss.Center, lipgloss.Center, modal)
		}
	}

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
