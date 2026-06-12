package client

import (
	"context"
	"encoding/json"
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
	overlaySelect
	overlayOptions
	overlayConnectProvider
	overlayConnectAPIKey
	overlayConnectModel
)

type rootModel struct {
	program              *tea.Program
	conversationList     conv.ConversationList
	welcome              *welcome.WelcomePanel
	statusBar            *statusbar.StatusBar
	sidebar              *sidebar.Sidebar
	input                *input.InputArea
	notifBar             *notify.NotificationBar
	selectDlg            *dialog.SelectDialog
	optionsDlg           *dialog.OptionsDialog
	providerDlg          *dialog.ListDialog
	apiKeyDlg            *dialog.InputDialog
	modelDlg             *dialog.ListDialog
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

	// pendingAskUser tracks an active LLM question (AskUser tool) waiting for user response.
	pendingAskUser *events.AskUserRequestData

	// rpcAskUserQuestions stores AskUser questions received via RPC (used when daemon is connected).
	rpcAskUserQuestions []struct {
		Question    string
		Options     []string
		MultiSelect bool
	}

	// pendingPermission tracks a tool security permission request waiting for user response.
	pendingPermission *events.PermissionRequestData

	// RPC client for daemon communication.
	rpc          *daemonRPCClient
	rpcConnected bool

	// pendingCorrelationID links UI interactions (ask_user, permission) back to daemon RPC.
	pendingCorrelationID string

	// pendingPermParams stores permission request params for grant forwarding.
	pendingPermParams map[string]any

	// currentSessionID tracks the active session ID used in RPC messages.
	currentSessionID string
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
		conversationList: conv.NewConversationList(),
		welcome:          welcome.New(),
		statusBar:        statusbar.New(),
		sidebar:          sidebar.New(),
		input:            input.New(),
		notifBar:         notify.New(),
		selectDlg:        dialog.NewSelectDialog(""),
		optionsDlg:       dialog.NewOptionsDialog(""),
		providerDlg:      dialog.NewListDialog(i18n.T("client.ui.dialog.provider.select")),
		apiKeyDlg:        dialog.NewInputDialog("API key", "API key"),
		modelDlg:         dialog.NewListDialog(i18n.T("client.ui.dialog.model.select")),
		viewport:         viewport.New(),
		daemonAddr:       ":1314",
	}

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
		})
		m.loadCommands()

		if m.app != nil {
			if _, err := m.app.EnsureSession(); err != nil {
				fmt.Fprintf(os.Stderr, "\nFATAL: EnsureSession failed at startup: %v\n", err)
				os.Exit(1)
			}
		}
		m.populateWelcome()

		// Wire CostRegistry pricing into TUI components
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
	sessions, err := sessDB.ListSessions(ctx)
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

func messagesToConversations(sessionID, agentName string, msgs []goharnesssession.Message) []conv.Conversation {
	var convs []conv.Conversation
	var current *conv.Conversation

	// 按 tool_call_id 索引待匹配的工具调用（与 Web 前端逻辑对齐）
	type pendingToolCall struct {
		name string
		args map[string]any
	}
	pendingToolCalls := map[string]pendingToolCall{}

	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			c := conv.NewConversation(sessionID, agentName, msg.Content)
			c.Status = conv.StatusDone
			convs = append(convs, c)
			current = &convs[len(convs)-1]

		case "assistant":
			if current == nil {
				continue
			}
			// 1) 还原思想流（reasoning_content 是 assistant 消息内嵌字段）
			if msg.ReasoningContent != "" {
				current.EnsureCurrentRound()
				if rnd := current.CurrentRound(); rnd != nil {
					rnd.ThoughtContent = msg.ReasoningContent
				}
			}
			// 2) 收集所有 tool_calls 到 Map（goharness 扁平格式 {id, name, arguments}）
			for _, tc := range msg.ToolCalls {
				var argsMap map[string]any
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &argsMap)
				}
				pendingToolCalls[tc.ID] = pendingToolCall{
					name: tc.Name,
					args: argsMap,
				}
			}
			// 3) 正文内容 → Output
			if msg.Content != "" {
				current.Output.Entries = append(current.Output.Entries, conv.OutputEntry{Role: "assistant", Content: msg.Content})
			}

		case "tool":
			if current == nil {
				continue
			}
			// 通过 tool_call_id 精确匹配工具调用结果
			match, ok := pendingToolCalls[msg.ToolCallID]
			if !ok {
				match = pendingToolCall{name: "工具"}
			}

			current.EnsureCurrentRound()
			if rnd := current.CurrentRound(); rnd != nil {
				success := true
				resultText := msg.Content
				if strings.HasPrefix(resultText, "[") && strings.Contains(resultText, "] error:") {
					success = false
				}
				rnd.Action.Steps = append(rnd.Action.Steps, conv.ActionStep{
					ToolName:   match.name,
					Status:     map[bool]conv.ActionStepStatus{true: conv.ActionStepDone, false: conv.ActionStepFailed}[success],
					Params:     match.args,
					ResultText: resultText,
					Collapsed:  false,
				})
			}
			delete(pendingToolCalls, msg.ToolCallID)
		}
	}

	// 标记所有还原的 Action 为已完成
	for i := range convs {
		convs[i].Status = conv.StatusDone
		for j := range convs[i].Rounds {
			convs[i].Rounds[j].Action.Completed = true
		}
	}
	return convs
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

	ctx := context.Background()
	msgs, err := sessDB.Get(ctx, sessionID)
	if err != nil || len(msgs) == 0 {
		return
	}
	convs := messagesToConversations(sessionID, agentName, msgs)
	m.conversationList.Conversations = append(m.conversationList.Conversations, convs...)
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
		totalAll += r.TotalTokens
	}

	// Reset and seed StatusBar
	m.statusBar.TokensTotal = totalAll
	m.statusBar.InputTokens = totalInput
	m.statusBar.OutputTokens = totalOutput
	m.statusBar.CachedTokens = totalCached

	// Reset and seed Sidebar
	m.sidebar.InputTokens = totalInput
	m.sidebar.OutputTokens = totalOutput
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

	// StatusBar: total cost function using CostRegistry or hardcoded fallback
	m.statusBar.CostFn = func(modelName string, inputTokens, outputTokens, cachedTokens int) float64 {
		costs := m.app.Costs()
		if costs != nil {
			if mc, ok := costs.Get(modelName); ok {
				return appcore.CalculateCost(mc, int64(inputTokens), int64(outputTokens), int64(cachedTokens))
			}
		}
		return data.CalculateCost(data.GetPricing(modelName), inputTokens, outputTokens, cachedTokens)
	}

	// Sidebar: per-component cost breakdown using CostRegistry or hardcoded fallback
	m.sidebar.CostFunc = func(modelName string, inputTokens, outputTokens, cachedTokens int) (float64, float64, float64) {
		costs := m.app.Costs()
		if costs != nil {
			if mc, ok := costs.Get(modelName); ok {
				inputCost := 0.0
				if mc.CostPer1MIn > 0 {
					inputCost = mc.CostPer1MIn / 1_000_000 * float64(inputTokens)
				}
				outputCost := 0.0
				if mc.CostPer1MOut > 0 {
					outputCost = mc.CostPer1MOut / 1_000_000 * float64(outputTokens)
				}
				cachedCost := 0.0
				if mc.CostPer1MInCached > 0 && cachedTokens > 0 {
					cachedCost = mc.CostPer1MInCached / 1_000_000 * float64(cachedTokens)
				}
				return inputCost, outputCost, cachedCost
			}
		}
		p := data.GetPricing(modelName)
		inputCost := float64(inputTokens) / 1_000_000 * p.InputPrice
		outputCost := float64(outputTokens) / 1_000_000 * p.OutputPrice
		cachedCost := float64(cachedTokens) / 1_000_000 * p.CachedPrice
		return inputCost, outputCost, cachedCost
	}
}

func displayName(title, name string) string {
	if title != "" {
		return title
	}
	return name
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
	case overlaySelect:
		newDlg, cmd := m.selectDlg.Update(msg)
		m.selectDlg = newDlg
		return m, cmd
	case overlayOptions:
		newDlg, cmd := m.optionsDlg.Update(msg)
		m.optionsDlg = newDlg
		return m, cmd
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
	newList, cmd := m.conversationList.Update(msg)
	m.conversationList = newList
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

// activateAskUserOverlay sets up the appropriate dialog from pendingAskUser state.
func (m *rootModel) activateAskUserOverlay() {
	var question string
	var options []string
	var multiSelect bool

	if m.pendingAskUser != nil && len(m.pendingAskUser.Questions) > 0 {
		q := m.pendingAskUser.Questions[0]
		question = q.Question
		options = q.Options
		multiSelect = q.MultiSelect
	} else if len(m.rpcAskUserQuestions) > 0 {
		q := m.rpcAskUserQuestions[0]
		question = q.Question
		options = q.Options
		multiSelect = q.MultiSelect
	} else {
		return
	}

	title := question
	if len([]rune(title)) > 20 {
		title = string([]rune(title)[:20]) + "…"
	}

	if multiSelect {
		m.optionsDlg.Title = title
		m.optionsDlg.SetOptions(question, options)
		m.activeOverlay = overlayOptions
	} else {
		m.selectDlg.Title = title
		m.selectDlg.SetOptions(question, options)
		m.activeOverlay = overlaySelect
	}
}

// mapAskUserReply builds the answer map from the dialog result and calls Reply() (or RPC).
func (m *rootModel) mapAskUserReply(isMulti bool, index int, indices []int, customText string, cancelled bool) {
	var question string
	var options []string
	useRPC := m.rpcConnected && len(m.rpcAskUserQuestions) > 0

	if m.pendingAskUser != nil && len(m.pendingAskUser.Questions) > 0 {
		q := m.pendingAskUser.Questions[0]
		question = q.Question
		options = q.Options
	} else if useRPC {
		q := m.rpcAskUserQuestions[0]
		question = q.Question
		options = q.Options
	} else {
		return
	}

	pp := m.pendingAskUser
	m.pendingAskUser = nil
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

	if useRPC {
		m.rpcReplyAskUser(map[string]string{question: answer})
	} else {
		pp.Reply(map[string]string{question: answer})
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
			cfg.LastModel = modelName
		}
		_ = cfg.Save()
	}

	if modelName != "" {
		if modelCfg := reg.Get(modelName); modelCfg != nil {
			m.welcome.Data.ModelName = displayName(modelCfg.Title, modelCfg.Name)
			m.updateModelDisplay(modelCfg)
			if cfg := m.app.Config(); cfg != nil {
				cfg.LastModel = modelName
				_ = cfg.Save()
			}
			_ = reg.Save(modelCfg)
		}
	} else if m.connectAPIKey != "" {
		// APIKey已存入CredentialStore（上方），无需再持久化模型配置
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
	host := "localhost"
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
	conn.Close()
	return clientmsg.DaemonConnected
}

// ============================================================
// Main bubbletea update loop
// ============================================================

func (m *rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.conversationList.Init(),
		checkDaemonCmd(m.daemonAddr),
	)
}

func (m *rootModel) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := e.(type) {
	case tea.WindowSizeMsg:
		w := clientmsg.WindowResizeMsg{Width: msg.Width, Height: msg.Height}
		m.dispatchToAll(w)
		m.resizeViewport(msg.Width, msg.Height)
		m.selectDlg.Update(msg)
		m.optionsDlg.Update(msg)
		m.providerDlg.Update(msg)
		m.apiKeyDlg.Update(msg)
		m.modelDlg.Update(msg)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.handlePostExit()
			return m, tea.Quit
		}

		// Priority 1: dialog overlay (AskUser)
		if m.activeOverlay != overlayNone {
			return m.handleOverlayKey(msg)
		}

		// Priority 2: permission bar (tool security)
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

	case clientmsg.ToolExecStartMsg, clientmsg.ToolExecEndMsg:
		return m.updateWithState(msg, i18n.T("client.status.executing"), true)

	case clientmsg.ExecutionSummaryMsg:
		m.statusBar.Update(msg)
		m.sidebar.AddTokenUsage(
			msg.TokensUsed.InputTokens,
			msg.TokensUsed.OutputTokens,
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

	case clientmsg.ClearScreenMsg:
		return m.updateWithState(msg, i18n.T("client.status.idle"), false)

	// --- Dialog overlay: AskUser questions from the LLM ---

	case clientmsg.AskUserEventMsg:
		m.statusBar.CurrentState = i18n.T("client.status.waiting.answer")
		m.activateAskUserOverlay()
		return m, nil

	case dialog.SelectDialogResult:
		m.activeOverlay = overlayNone
		m.statusBar.CurrentState = i18n.T("client.status.idle")
		m.mapAskUserReply(false, msg.Index, nil, msg.CustomText, msg.Cancelled)
		return m, nil

	case dialog.OptionsDialogResult:
		m.activeOverlay = overlayNone
		m.statusBar.CurrentState = i18n.T("client.status.idle")
		m.mapAskUserReply(true, 0, msg.Indices, msg.CustomText, msg.Cancelled)
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
		m.statusBar.CurrentState = i18n.T("client.status.idle")
		if m.rpcConnected {
			params := m.pendingPermParams
			m.pendingPermParams = nil
			if msg.Index < 0 {
				m.rpcReplyPermission("deny", nil, "user cancelled")
			} else if msg.Index == 0 {
				m.rpcReplyPermission("grant", params, "")
			} else {
				m.rpcReplyPermission("deny", nil, "user denied")
			}
		} else if m.pendingPermission != nil {
			pp := m.pendingPermission
			m.pendingPermission = nil
			if msg.Index < 0 {
				go pp.Deny("user cancelled")
			} else if msg.Index == 0 {
				go pp.Grant(nil)
			} else {
				go pp.Deny("user denied")
			}
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
	m.conversationList, _ = m.conversationList.Update(w)
	m.statusBar.Update(w)
	m.input.Update(w)
	m.notifBar.Update(w)
}

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

	headerEstimate := 8
	footerEstimate := 5
	separators := 2
	vpHeight := termHeight - headerEstimate - footerEstimate - separators
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.SetWidth(m.leftWidth)
	m.viewport.SetHeight(vpHeight)

	m.sidebar.Update(clientmsg.WindowResizeMsg{Width: m.rightWidth - 2, Height: vpHeight})
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

	newConv := conv.NewConversation(sessionID, agentName, e.Text)
	m.conversationList.Conversations = append(m.conversationList.Conversations, newConv)
	m.conversationList.MarkDirty()

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
				SessionID: sessionID, ToolName: d.ToolName, Params: d.Params, EstimatedTok: d.PredictedTokens,
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
		ask.OnCycleEnd(func(d events.CycleInfo) {
			m.program.Send(clientmsg.IterationMsg{SessionID: sessionID, Iteration: d.Iteration})
		})
		var tokenUsage goharnesssession.TokenUsage
		ask.OnTokenUsageRecorded(func(d goharnesssession.TokenUsageRecord) {
			tokenUsage.InputTokens += d.PromptTokens
			tokenUsage.OutputTokens += d.CompletionTokens
			tokenUsage.CachedTokens += d.CachedTokens
			tokenUsage.ReasoningTokens += d.ReasoningTokens
			tokenUsage.TotalTokens += d.TotalTokens
			tokenUsage.Timestamp = d.Timestamp
			m.program.Send(clientmsg.ExecutionSummaryMsg{
				SessionID:  sessionID,
				TokensUsed: tokenUsage,
			})
		})
		ask.OnAskUser(func(d events.AskUserRequestData) {
			m.pendingAskUser = &d
			m.program.Send(clientmsg.AskUserEventMsg{})
		})
		ask.OnPermissionRequest(func(d events.PermissionRequestData) {
			m.pendingPermission = &d
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

	m.statusBar.AgentName = e.AgentName

	// Update model display from the new agent's configured model
	agent := m.app.Agents().Get(e.AgentName)
	if agent != nil && agent.Model != "" {
		if modelCfg := m.app.Models().Get(agent.Model); modelCfg != nil {
			m.updateModelDisplay(modelCfg)
		}
	}

	return m, m.notifBar.Add(data.Notification{
		Message: fmt.Sprintf(i18n.T("client.notify.agent.switched"), e.AgentName),
		Level:   data.NotifInfo,
	})
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
		if len(e.Args) > 0 && result.Success {
			modelName := e.Args[0]
			if modelCfg := m.app.Models().Get(modelName); modelCfg != nil {
				m.welcome.Data.ModelName = displayName(modelCfg.Title, modelCfg.Name)
				m.updateModelDisplay(modelCfg)
				if cfg := m.app.Config(); cfg != nil {
					cfg.LastModel = modelName
					_ = cfg.Save()
				}
			}
		}
		m.input.Models, _ = reloadModels(m.app)
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

	m.conversationList.Clear()
	m.loadSessionHistory()
	newSessions, _ := loadRecentSessions(m.app)
	m.input.Sessions = newSessions

	return func() tea.Msg {
		return clientmsg.ClearScreenMsg{}
	}
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

	headerStr := notifView

	m.input.Hidden = m.permBar.Visible
	bottomArea := inputView

	// When the permission bar is visible, replace the input area with it.
	if m.permBar.Visible {
		bottomArea = permView
	}

	headerLines := strings.Count(headerStr, "\n") + 1
	statusLines := strings.Count(statusView, "\n") + 1
	bottomLines := strings.Count(bottomArea, "\n") + 1
	separators := 2
	vpHeight := m.termHeight - headerLines - statusLines - bottomLines - separators
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.SetWidth(m.leftWidth)
	m.viewport.SetHeight(vpHeight)

	m.viewport.SetContent(m.conversationList.View())

	if m.scrollToBottom {
		m.viewport.GotoBottom()
		m.scrollToBottom = false
	}

	mainArea := m.viewport.View()

	m.sidebar.SyncHeight(vpHeight)
	sideArea := m.sidebar.View()

	layout := lipgloss.JoinHorizontal(lipgloss.Top, mainArea, sideArea)

	full := lipgloss.JoinVertical(lipgloss.Left,
		headerStr,
		layout,
		statusView,
		bottomArea,
	)

	// Dialog overlay: render full-screen centered if active (AskUser).
	if m.activeOverlay != overlayNone {
		var modal string
		switch m.activeOverlay {
		case overlaySelect:
			modal = m.selectDlg.View()
		case overlayOptions:
			modal = m.optionsDlg.View()
		case overlayConnectProvider:
			modal = m.providerDlg.View()
		case overlayConnectAPIKey:
			modal = m.apiKeyDlg.View()
		case overlayConnectModel:
			modal = m.modelDlg.View()
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
