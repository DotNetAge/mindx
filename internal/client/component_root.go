package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
)

var currentRenderWidth int

type rootModel struct {
	contentPanel *ContentPanel
	statusBar    StatusBar
	inputBox     InputBox
	choicesPanel ChoicesPanel

	app *core.App

	sessionReg  *sessionRegistry
	outputCh    chan tea.Msg
	chatManager *chatSessionManager

	registry         *SlashCommandRegistry
	currentAgent     string
	currentModel     string
	currentSessionID string

	mindxConfig *core.MindxConfig

	executing     bool
	currentCancel context.CancelFunc
}

func NewProgram(mindxConfig *core.MindxConfig) *tea.Program {
	registry := BuiltinCommands()

	redirectOutputForTUI()

	return tea.NewProgram(&rootModel{
		contentPanel: NewContentPanel(),
		statusBar:    NewStatusBar(),
		inputBox:     NewInputBox(registry),
		choicesPanel: NewChoicesPanel(),
		sessionReg:   newSessionRegistry(),
		outputCh:     make(chan tea.Msg, 256),
		registry:     registry,
		chatManager:  GetChatSessionManager(),
		mindxConfig:  mindxConfig,
	})
}

// redirectOutputForTUI 重定向所有日志输出，防止干扰 TUI 界面
// 包括：标准库 log、slog（GoReact 默认使用）、以及 MindX/Goreact 的自定义 logger
func redirectOutputForTUI() {
	log.SetOutput(io.Discard)

	slog.SetDefault(slog.New(discardHandler{}))
}

// discardHandler 是一个丢弃所有日志的 slog.Handler 实现
type discardHandler struct{}

func (h discardHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (h discardHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler            { return h }
func (h discardHandler) WithGroup(string) slog.Handler                 { return h }

func (m *rootModel) Init() tea.Cmd {
	var err error
	m.app, err = core.DefaultApp()
	if err != nil {
		return func() tea.Msg { return err }
	}

	// 注入 ZapLogger：日志输出到文件，不污染 TUI 屏幕
	zapLogger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   "logs/mindx-tui.log",
		MaxSize:    100, // MB
		MaxBackups: 7,
		MaxAge:     30, // days
		Compress:   true,
		Console:    false, // 关键：不输出到 stdout/stderr！
	})
	m.app.SetLogger(zapLogger)

	m.registry.SetQueryFunc(func(queryType, name string) (string, error) {
		switch queryType {
		case "agents":
			return listAgentsLocal(m.app)
		case "models":
			return listModelsLocal(m.app)
		case "skills":
			return listSkillsLocal(m.app)
		default:
			return "", fmt.Errorf("unknown query type: %s", queryType)
		}
	})

	m.statusBar.SetConnected(true)
	m.showWelcome()

	agents := loadAgentsFromApp(m.app)
	m.inputBox.SetAgents(agents)
	m.registry.SetAgents(agents)

	if len(agents) > 0 {
		m.currentAgent = agents[0].name
		m.currentModel = agents[0].model
		m.statusBar.SetAgent(m.currentAgent, m.currentModel)
		m.contentPanel.UpdateWelcomeAgent(m.currentAgent)
	}

	if m.mindxConfig != nil && m.mindxConfig.LastAgent != "" {
		m.currentAgent = m.mindxConfig.LastAgent
		for _, a := range agents {
			if a.name == m.mindxConfig.LastAgent {
				m.currentModel = a.model
				break
			}
		}
		m.statusBar.SetAgent(m.currentAgent, m.currentModel)
	}

	return m.loadOrInitSession()
}

func listAgentsLocal(app *core.App) (string, error) {
	registry := app.Agents()
	agentList := registry.List()

	var result []map[string]string
	for _, agent := range agentList {
		result = append(result, map[string]string{
			"name":        agent.Name,
			"role":        agent.Role,
			"description": agent.Description,
			"model":       agent.Model,
		})
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func listModelsLocal(app *core.App) (string, error) {
	models := app.Models().List()
	var result []map[string]string
	for _, model := range models {
		result = append(result, map[string]string{
			"name":        model.Name,
			"description": model.Description,
		})
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func listSkillsLocal(app *core.App) (string, error) {
	m, err := app.GetMaster()
	if err != nil {
		return "[]", nil
	}
	if m.Reactor() == nil {
		return "[]", nil
	}
	skills := m.Reactor().SkillRegistry().ListSkills()

	var result []map[string]string
	for _, skill := range skills {
		result = append(result, map[string]string{
			"name":        skill.Name,
			"description": skill.Description,
		})
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func loadAgentsFromApp(app *core.App) []agentInfo {
	registry := app.Agents()
	agentList := registry.List()

	var infos []agentInfo
	for _, a := range agentList {
		infos = append(infos, agentInfo{
			name:  a.Name,
			model: a.Model,
		})
	}
	return infos
}

func newSessionID(agentName string) string {
	return fmt.Sprintf("%s-%s-%x", agentName, time.Now().Format("20060102-150405"), time.Now().UnixNano()%0xffff)
}

func (m *rootModel) loadOrInitSession() tea.Cmd {
	return func() tea.Msg {
		if m.mindxConfig != nil && m.mindxConfig.LastSessionID != "" && m.mindxConfig.LastAgent != "" {
			m.currentAgent = m.mindxConfig.LastAgent
			m.currentSessionID = m.mindxConfig.LastSessionID
			m.chatManager.Update(m.currentAgent, m.currentSessionID)
			m.loadExistingSessionMeta(m.currentAgent, m.currentSessionID)
			return sessionLoadedMsg{agentName: m.currentAgent, sessionID: m.currentSessionID}
		}

		if m.chatManager.Exists() {
			session, err := m.chatManager.Load()
			if err == nil && session.AgentName != "" && session.SessionID != "" {
				m.currentAgent = session.AgentName
				m.currentSessionID = session.SessionID
				m.statusBar.SetAgent(m.currentAgent, m.currentModel)
				m.loadExistingSessionMeta(session.AgentName, session.SessionID)
				return sessionLoadedMsg{agentName: session.AgentName, sessionID: session.SessionID}
			}
		}

		if m.app.Settings().MasterAgent != "" {
			m.currentAgent = m.app.Settings().MasterAgent

			meta, err := m.app.CreateSession(m.currentAgent)
			if err != nil {
				m.currentSessionID = newSessionID(m.currentAgent)
			} else {
				m.currentSessionID = meta.SessionID
			}

			m.chatManager.Update(m.currentAgent, m.currentSessionID)
			return sessionLoadedMsg{agentName: m.currentAgent, sessionID: m.currentSessionID}
		}

		return sessionInitRequiredMsg{}
	}
}

// loadExistingSessionMeta attempts to load session metadata for an existing session.
// This is best-effort: if meta.json doesn't exist (e.g., sessions created before
// this feature), the app falls back to defaults without error.
func (m *rootModel) loadExistingSessionMeta(agentName, sessionID string) {
	if m.app == nil || m.app.SessDB() == nil {
		return
	}
	meta, err := m.app.SessDB().GetMeta(context.Background(), sessionID)
	if err == nil && meta != nil {
		m.app.SetCurrentSessionMeta(meta)
	}
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			m.handleGlobalError(fmt.Sprintf("内部错误: %v", r))
		}
	}()

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		currentRenderWidth = msg.Width

		const bottomReservedLines = 13

		if msg.Height > bottomReservedLines {
			m.contentPanel.SetSize(msg.Width, msg.Height-bottomReservedLines)
		} else {
			m.contentPanel.SetSize(msg.Width, msg.Height/2)
		}

		m.statusBar.SetWidth(msg.Width)
		m.inputBox.SetWidth(msg.Width)
		return m, nil

	case tea.MouseMsg:
		m.contentPanel.Update(msg)
		return m, nil
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			if !m.inputBox.IsFocused() && !m.inputBox.hidden {
				contentHeight := m.contentPanel.height
				if contentHeight > 0 && mouse.Y > contentHeight {
					m.inputBox.textarea.Focus()
				}
			}
		}
		return m, nil

	case tea.PasteMsg:
		if !m.executing {
			ib, cmd := m.inputBox.HandlePaste(msg)
			m.inputBox = ib
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "down", "pgup", "pgdown", "home", "end":
			if !m.inputBox.IsFocused() || m.executing {
				m.contentPanel.Update(msg)
				return m, nil
			}
		}
		return m.handleKeyPress(msg)

	case suggestionCompleteMsg:
		m.inputBox.InsertText(msg.text)
		return m, nil

	case localDisplayMsg:
		answer := m.contentPanel.LatestAnswer()
		if answer == nil {
			sessionID := fmt.Sprintf("local-%d", time.Now().UnixMilli())
			answer = m.contentPanel.CreateAnswer(sessionID, "system")
		}
		answer.AppendResult(msg.markdown)
		m.contentPanel.refreshOnUpdate()
		return m, nil

	case sendMsg:
		return m.handleSend(msg)

	case agentSwitchMsg:
		return m.handleAgentSwitch(msg)

	case clearScreenMsg:
		m.contentPanel.ClearAll()
		return m, nil

	case sessionLoadedMsg:
		return m, nil

	case sessionInitRequiredMsg:
		if m.currentAgent == "" {
			m.currentAgent = "default"
		}
		m.currentSessionID = newSessionID(m.currentAgent)
		m.chatManager.Update(m.currentAgent, m.currentSessionID)
		return m, nil

	case agentAnswerUpdateMsg:
		return m.handleSessionUpdate(msg)

	case agentAnswerDoneMsg:
		return m.handleSessionDone(msg)

	case errMsg:
		m.handleGlobalError(msg.Error())
		return m, nil

	case exitMsg:
		if m.currentCancel != nil {
			m.currentCancel()
		}
		m.saveSessionOnExit()
		return m, tea.Quit
	}

	return m, nil
}

func (m *rootModel) View() tea.View {
	if m.contentPanel == nil {
		return tea.NewView("Loading...")
	}

	parts := []string{
		m.contentPanel.View(),
		m.statusBar.View(),
	}

	if m.choicesPanel.IsVisible() {
		parts = append(parts, m.choicesPanel.View())
	} else {
		parts = append(parts, m.inputBox.View())
		if m.inputBox.HasSuggestion() {
			parts = append(parts, m.inputBox.SuggestionView())
		}
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, parts...))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion // 启用鼠标模式（支持滚轮）
	v.WindowTitle = "MindX Chat"
	return v
}

func (m *rootModel) handleKeyPress(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.executing {
		return m, nil
	}

	if m.choicesPanel.IsVisible() {
		cp, cmd := m.choicesPanel.Update(msg)
		m.choicesPanel = cp
		return m, cmd
	}

	kp, isKey := msg.(tea.KeyPressMsg)

	if isKey && kp.String() == "enter" && m.inputBox.JustCompleted() {
		ib, cmd := m.inputBox.HandleKey(kp)
		m.inputBox = ib
		return m, cmd
	}

	if m.inputBox.HasSuggestion() && isKey {
		switch kp.String() {
		case "up", "down", "enter", "tab":
			ib, cmd := m.inputBox.UpdateSuggestions(msg)
			m.inputBox = ib
			return m, cmd
		case "esc":
			ib, _ := m.inputBox.UpdateSuggestions(msg)
			m.inputBox = ib
			return m, nil
		}
	}

	if isKey {
		ib, cmd := m.inputBox.HandleKey(kp)
		m.inputBox = ib
		return m, cmd
	}

	return m, nil
}

func (m *rootModel) handleSend(msg sendMsg) (tea.Model, tea.Cmd) {
	m.executing = true

	text := msg.text

	if strings.HasPrefix(text, "@") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) >= 1 && len(parts[0]) > 1 {
			targetAgent := parts[0][1:]

			for _, a := range m.inputBox.suggestAg.agents {
				if a.name == targetAgent {
					m.currentAgent = targetAgent
					m.currentModel = a.model
					m.statusBar.SetAgent(m.currentAgent, m.currentModel)
					break
				}
			}
		}

		if len(parts) >= 2 {
			text = strings.TrimSpace(parts[1])
		} else {
			text = ""
		}
	}

	if strings.TrimSpace(text) == "" {
		m.executing = false
		return m, nil
	}

	sessionID := m.getOrCreateSessionID()

	answer := m.contentPanel.CreateAnswer(sessionID, m.currentAgent)
	m.sessionReg.add(sessionID, answer)
	answer.AppendResult(msg.text)

	answer.StartThinking()
	m.contentPanel.refreshOnUpdate()

	agent, err := m.app.ResolveAgent(m.currentAgent)
	if err != nil {
		m.executing = false
		return m, func() tea.Msg { return err }
	}

	_, cancel := context.WithCancel(context.Background())
	m.currentCancel = cancel

	eventCh, cancelEvents := agent.EventsFiltered(func(e goreactcore.ReactEvent) bool {
		switch e.Type {
		case goreactcore.ThinkingDelta, goreactcore.ThinkingDone, goreactcore.ActionStart,
			goreactcore.ActionProgress, goreactcore.ActionResult, goreactcore.FinalAnswer,
			goreactcore.ExecutionSummary, goreactcore.Error, goreactcore.SubtaskSpawned,
			goreactcore.SubtaskCompleted, goreactcore.ClarifyNeeded, goreactcore.PermissionRequest,
			goreactcore.PermissionDenied, goreactcore.CycleEnd, goreactcore.TaskSummary:
			return true
		default:
			return false
		}
	})

	go func() {
		defer func() {
			cancelEvents()
			if r := recover(); r != nil {
				// 捕获详细堆栈信息
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				errMsg := fmt.Sprintf("agent execution panic:\n\nError: %v\n\nStack Trace:\n%s", r, stackTrace)
				trySend(m.outputCh, fmt.Errorf("%s", errMsg))
			}
		}()
		_, err = agent.Ask(sessionID, text)
		if err != nil {
			trySend(m.outputCh, err)
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				errMsg := fmt.Sprintf("event consumer panic:\n\nError: %v\n\nStack Trace:\n%s", r, stackTrace)
				trySend(m.outputCh, fmt.Errorf("%s", errMsg))
			}
		}()
		m.consumeEvents(eventCh, sessionID)
	}()

	return m, waitEvent(m.outputCh)
}

func (m *rootModel) consumeEvents(eventCh <-chan goreactcore.ReactEvent, sessionID string) {
	for event := range eventCh {
		contentType := string(event.Type)
		content := stringifyEventData(event.Data)

		trySend(m.outputCh, agentAnswerUpdateMsg{
			sessionID:   sessionID,
			contentType: contentType,
			content:     content,
		})
	}
	trySend(m.outputCh, agentAnswerDoneMsg{sessionID: sessionID})
}

func stringifyEventData(data interface{}) string {
	if data == nil {
		return ""
	}

	switch v := data.(type) {
	case string:
		return v
	default:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonData)
	}
}

func (m *rootModel) getOrCreateSessionID() string {
	if m.currentSessionID != "" {
		return m.currentSessionID
	}

	m.currentSessionID = newSessionID(m.currentAgent)
	m.chatManager.Update(m.currentAgent, m.currentSessionID)
	return m.currentSessionID
}

func (m *rootModel) handleSessionUpdate(msg agentAnswerUpdateMsg) (tea.Model, tea.Cmd) {
	answer := m.sessionReg.get(msg.sessionID)
	if answer == nil {
		answer = m.contentPanel.CreateAnswer(msg.sessionID, "agent")
		m.sessionReg.add(msg.sessionID, answer)
	}

	m.routeToAnswer(answer, msg.contentType, msg.content)

	spinnerCmd := answer.Update(msg)
	m.contentPanel.refreshOnUpdate()
	return m, tea.Batch(waitEvent(m.outputCh), spinnerCmd)
}

func (m *rootModel) handleSessionDone(msg agentAnswerDoneMsg) (tea.Model, tea.Cmd) {
	if answer := m.sessionReg.get(msg.sessionID); answer != nil {
		m.contentPanel.refreshOnUpdate()
	}

	m.sessionReg.remove(msg.sessionID)

	if m.sessionReg.count() == 0 {
		m.executing = false
	}

	return m, waitEvent(m.outputCh)
}

func (m *rootModel) handleAgentSwitch(msg agentSwitchMsg) (tea.Model, tea.Cmd) {
	m.currentAgent = msg.agentName
	m.statusBar.SetAgent(m.currentAgent, m.currentModel)

	m.currentSessionID = newSessionID(m.currentAgent)
	if err := m.chatManager.Update(m.currentAgent, m.currentSessionID); err != nil {
		return m, func() tea.Msg {
			return fmt.Errorf("保存会话状态失败: %v", err)
		}
	}

	return m, nil
}

func (m *rootModel) routeToAnswer(answer *AgentAnswer, contentType, content string) {
	switch contentType {
	case "thinking", "ThinkingDelta", "thinking_delta", "THINKING_DELTA":
		answer.AppendThinking(content)
	case "thinking_done", "ThinkingDone":
		answer.SetThinkingDone(content)
	case "final_answer", "FinalAnswer", "FINAL_ANSWER", "result":
		if strings.Contains(content, `"reasoning"`) || strings.Contains(content, `"Reasoning"`) {
			answer.AppendThinking(content)
		} else {
			answer.AppendResult(content)
		}
	case "error":
		answer.AppendError(content)
	case "table", "todo", "options", "plain":
		answer.AppendTyped(content)
	case "action_start":
		toolName, estimatedTokens := parseActionStart(content)
		answer.AppendAction(toolName, estimatedTokens)
	case "action_progress":
		answer.SetActionProgress(content)
	case "action_result":
		parseActionResult(answer, content)
	}
}

func parseActionStart(content string) (string, int) {
	parts := strings.SplitN(content, "|", 2)
	toolName := parts[0]
	estimatedTokens := 0
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &estimatedTokens)
	}
	return toolName, estimatedTokens
}

func parseActionResult(answer *AgentAnswer, content string) {
	if strings.HasPrefix(content, "success|") {
		rest := strings.TrimPrefix(content, "success|")
		answer.MarkActionDone(rest)
	} else if strings.HasPrefix(content, "failed|") {
		rest := strings.TrimPrefix(content, "failed|")
		answer.MarkActionFailed(rest)
	}
}

func (m *rootModel) handleGlobalError(errMsg string) {
	if m.sessionReg.count() == 0 {
		if answer := m.contentPanel.LatestAnswer(); answer != nil {
			answer.AppendError(errMsg)
			m.contentPanel.refreshOnUpdate()
		}
	} else {
		for _, answer := range m.sessionReg.answers {
			if answer != nil {
				answer.AppendError(errMsg)
			}
		}
		m.contentPanel.refreshOnUpdate()
	}

	m.sessionReg.clear()
	m.executing = false
}

func (m *rootModel) saveSessionOnExit() {
	if m.currentAgent != "" && m.currentSessionID != "" {
		if err := m.chatManager.Update(m.currentAgent, m.currentSessionID); err != nil {
			// 静默处理错误，避免干扰 TUI 界面
		}
	}

	if m.mindxConfig != nil {
		m.mindxConfig.LastAgent = m.currentAgent
		m.mindxConfig.LastSessionID = m.currentSessionID
		_ = m.mindxConfig.Save()
	}
}

func (m *rootModel) showWelcome() {
	appTitle := "MindX"
	version := "2.0"
	workspace := os.Getenv("MINDX_WORKSPACE")
	if workspace == "" {
		workspace = "default"
	}
	sessionID := fmt.Sprintf("%x", time.Now().UnixNano())

	var projectDir string
	if m.app != nil && m.app.CurrentSessionMeta() != nil {
		projectDir = m.app.CurrentSessionMeta().GetProjectDir()
	}

	m.contentPanel.ShowWelcome(appTitle, version, workspace, sessionID, "本地模式", projectDir)
}
