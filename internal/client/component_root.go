package client

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/DotNetAge/gort/pkg/gateway"
)

// rootModel 是整个 TUI 的根组件，负责组装子组件和消息路由。
type rootModel struct {
	// 子组件
	contentPanel *ContentPanel
	statusBar    StatusBar
	inputBox     InputBox
	choicesPanel ChoicesPanel

	// 网络
	client *gateway.Client

	// 会话管理 — Per-Session Goroutines
	sessionReg *sessionRegistry
	outputCh   chan tea.Msg // sessionLoop 和 fallback handler 的输出通道
	sessionSeq int

	// 持久化会话状态 (chat.json)
	chatManager *chatSessionManager

	// 共享状态
	registry         *SlashCommandRegistry
	currentAgent     string
	currentModel     string
	currentSessionID string // 当前活跃的 sessionID，与服务端一致

	// 执行状态
	executing bool
}

// NewProgram 创建并返回 Bubble Tea 程序。
func NewProgram() *tea.Program {
	registry := BuiltinCommands()

	return tea.NewProgram(&rootModel{
		contentPanel: NewContentPanel(),
		statusBar:    NewStatusBar(),
		inputBox:     NewInputBox(registry),
		choicesPanel: NewChoicesPanel(),
		sessionReg:   newSessionRegistry(),
		outputCh:     make(chan tea.Msg, 256),
		registry:     registry,
		chatManager:  GetChatSessionManager(),
	})
}

func (m *rootModel) Init() tea.Cmd {
	addr := os.Getenv("MINDX_WS_ADDR")
	if addr == "" {
		addr = "localhost:1314"
	}
	wsPath := os.Getenv("MINDX_WS_PATH")
	if wsPath == "" {
		wsPath = "/ws"
	}
	wsURL := fmt.Sprintf("ws://%s%s", addr, wsPath)
	m.client = gateway.NewClient(wsURL)

	// 设置命令发送回调：用于 /models, /agents, /job-xxx 等服务器命令
	m.registry.SetCommandSender(func(name, args string) (string, error) {
		if !m.client.IsConnected() {
			return "", fmt.Errorf("未连接到服务器")
		}
		return m.client.SendCommand(name, args)
	})

	// 注册所有事件 handler，内部按 sessionRegistry 分发到 sessionRuntime 专属通道
	RegisterHandlers(m.client, m.sessionReg, m.outputCh)

	return tea.Batch(
		connectWithRetry(m.client, connectAttempt{}),
		m.loadOrInitSession(),
	)
}

// loadOrInitSession 加载已有的会话或从服务器初始化新会话。
func (m *rootModel) loadOrInitSession() tea.Cmd {
	return func() tea.Msg {
		if m.chatManager.Exists() {
			session, err := m.chatManager.Load()
			if err == nil && session.AgentName != "" && session.SessionID != "" {
				m.currentAgent = session.AgentName
				m.currentSessionID = session.SessionID
				m.statusBar.SetAgent(m.currentAgent, m.currentModel)
				return sessionLoadedMsg{agentName: session.AgentName, sessionID: session.SessionID}
			}
		}
		return sessionInitRequiredMsg{}
	}
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ---- 窗口事件 ----
	case tea.WindowSizeMsg:
		currentRenderWidth = msg.Width

		// 强制为底部预留足够空间：
		// - StatusBar: 1行
		// - InputBox: 1行
		// - Suggestion: 最多5行（maxSuggestionRows）
		// - 额外缓冲: 1行
		const bottomReservedLines = 13 // 1(状态栏) + 1(Input边框) + 10(Suggestion) + 1(缓冲)

		if msg.Height > bottomReservedLines {
			m.contentPanel.SetSize(msg.Width, msg.Height-bottomReservedLines)
		} else {
			m.contentPanel.SetSize(msg.Width, msg.Height/2)
		}

		m.statusBar.SetWidth(msg.Width)
		m.inputBox.SetWidth(msg.Width)
		return m, nil

	case tea.MouseWheelMsg:
		m.contentPanel.Update(msg)
		return m, nil

	case tea.MouseMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			if !m.inputBox.IsFocused() && !m.inputBox.hidden {
				m.inputBox.textarea.Focus()
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

	// ---- 键盘事件 ----
	case tea.KeyPressMsg:
		// 将键盘滚动事件传递给 ContentPanel.viewport（当 InputBox 不聚焦时）
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

	// ---- 连接事件 ----
	case connectedMsg:
		m.statusBar.SetConnected(true)
		m.showWelcome()
		return m, tea.Batch(fetchCommands(m.client), fetchAgents(m.client))

	case connectAttempt:
		return m, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
			return connectWithRetry(m.client, connectAttempt{count: msg.count})
		})

	// ---- Agent / 命令数据 ----
	case agentsFetchedMsg:
		m.inputBox.SetAgents(msg.agents)
		m.registry.SetAgents(msg.agents)
		if msg.masterName != "" {
			m.currentAgent = msg.masterName
			for _, a := range msg.agents {
				if a.name == msg.masterName {
					m.currentModel = a.model
					break
				}
			}
		}
		if m.currentAgent == "" && len(msg.agents) > 0 {
			m.currentAgent = msg.agents[0].name
			m.currentModel = msg.agents[0].model
		}
		m.statusBar.SetAgent(m.currentAgent, m.currentModel)
		// 更新 Welcome 中的 AgentName
		m.contentPanel.UpdateWelcomeAgent(m.currentAgent)

		if m.currentSessionID != "" {
			m.chatManager.Update(m.currentAgent, m.currentSessionID)
		}

		return m, nil

	case commandsFetchedMsg:
		m.registry.SyncRemoteCommands(msg.commands)

		var models, skills []gateway.CommandMeta
		for _, c := range msg.commands {
			switch c.Category {
			case "model":
				models = append(models, c)
			case "skill":
				skills = append(skills, c)
			}
		}
		m.registry.SetModels(models)
		m.registry.SetSkills(skills)
		return m, nil

	// ---- 发送消息 ----
	case sendMsg:
		return m.handleSend(msg)

	// ---- Agent 切换 ----
	case agentSwitchMsg:
		return m.handleAgentSwitch(msg)

	// ---- 清屏 ----
	case clearScreenMsg:
		m.contentPanel.ClearAll()
		return m, nil

	// ---- 会话状态加载/初始化 ----
	case sessionLoadedMsg:
		return m, nil // 已经在 loadOrInitSession 中设置了状态

	case sessionInitRequiredMsg:
		if m.client.IsConnected() {
			return m, m.initSessionFromServer()
		}
		return m, nil

	// ---- Session 更新（来自 sessionLoop goroutine） ----
	case agentAnswerUpdateMsg:
		return m.handleSessionUpdate(msg)

	// ---- Session 完成（来自 sessionLoop goroutine） ----
	case agentAnswerDoneMsg:
		return m.handleSessionDone(msg)

	// ---- 原始事件（未知 session 的 fallback） ----
	case rawEvent:
		return m.handleRawEvent(msg)

	// ---- 错误 ----
	case errMsg:
		m.handleGlobalError(msg.Error())
		return m, nil

	// ---- 退出程序 ----
	case exitMsg:
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
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "MindX Chat"
	return v
}

// ---- 键盘处理 ----

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

	// 如果刚从 suggestion 补全，直接处理键盘事件（不经过 suggestion）
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

// ---- 发送消息处理 ----

func (m *rootModel) handleSend(msg sendMsg) (tea.Model, tea.Cmd) {
	if !m.client.IsConnected() {
		m.sessionSeq++
		sessionID := fmt.Sprintf("disconnected-%d", m.sessionSeq)
		answer := m.contentPanel.CreateAnswer(sessionID, m.currentAgent)
		answer.AppendError("未连接到服务器")
		return m, nil
	}

	text := msg.text

	// 解析 @agent_name content 格式：先切换 Agent，再发送内容
	if strings.HasPrefix(text, "@") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) >= 1 && len(parts[0]) > 1 {
			targetAgent := parts[0][1:] // 去掉 @ 符号

			// 查找并切换到目标 Agent
			for _, a := range m.inputBox.suggestAg.agents {
				if a.name == targetAgent {
					m.currentAgent = targetAgent
					m.currentModel = a.model
					m.statusBar.SetAgent(m.currentAgent, m.currentModel)
					break
				}
			}
		}

		// 提取要发送的内容（去掉 @agent_name 部分）
		if len(parts) >= 2 {
			text = strings.TrimSpace(parts[1])
		} else {
			text = ""
		}
	}

	// 如果没有内容，只切换不发送
	if strings.TrimSpace(text) == "" {
		return m, nil
	}

	m.executing = true

	// 使用当前持久化的 sessionID（与服务端一致）
	agentName := m.currentAgent
	if agentName == "" {
		agentName = "default"
	}
	sessionID := m.currentSessionID
	if sessionID == "" {
		m.sessionSeq++
		sessionID = fmt.Sprintf("session-%s-%d", agentName, m.sessionSeq)
		m.currentSessionID = sessionID
	}

	// 创建 AgentAnswer 并注册到 sessionRegistry
	answer := m.contentPanel.CreateAnswer(sessionID, m.currentAgent)
	m.sessionReg.add(sessionID, answer)

	// 追加用户消息
	answer.AppendResult(msg.text)

	// 发送到服务器 + 开始监听 outputCh（传递 sessionID 给服务端）
	return m, tea.Batch(
		sendToServerWithSession(m.client, text, sessionID),
		waitEvent(m.outputCh),
	)
}

// ---- Session 更新处理 ----

func (m *rootModel) handleSessionUpdate(msg agentAnswerUpdateMsg) (tea.Model, tea.Cmd) {
	answer := m.sessionReg.get(msg.sessionID)
	if answer == nil {
		// session 未注册时自动创建（服务端可能在客户端注册 session 之前就推送事件）
		answer = m.contentPanel.CreateAnswer(msg.sessionID, "agent")
		m.sessionReg.add(msg.sessionID, answer)
	}

	m.routeToAnswer(answer, msg.contentType, msg.content)
	m.contentPanel.refreshOnUpdate()
	return m, waitEvent(m.outputCh)
}

// ---- Session 完成处理 ----

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

// ---- Agent 切换 ----

func (m *rootModel) handleAgentSwitch(msg agentSwitchMsg) (tea.Model, tea.Cmd) {
	m.currentAgent = msg.agentName
	m.statusBar.SetAgent(m.currentAgent, m.currentModel)

	// 切换 Agent 时生成新 sessionID 并持久化
	m.sessionSeq++
	newSessionID := fmt.Sprintf("session-%s-%d", m.currentAgent, m.sessionSeq)
	m.currentSessionID = newSessionID
	if err := m.chatManager.Update(m.currentAgent, newSessionID); err != nil {
		return m, func() tea.Msg {
			return errMsg(fmt.Errorf("保存会话状态失败: %v", err))
		}
	}

	return m, nil
}

// ---- 原始事件路由（未知 session 的 fallback） ----

func (m *rootModel) handleRawEvent(ev rawEvent) (tea.Model, tea.Cmd) {
	sid := ev.sessionID
	if sid == "" {
		if answer := m.contentPanel.LatestAnswer(); answer != nil {
			m.routeToAnswer(answer, ev.contentType, ev.content)
		}
		if ev.contentType == "complete" || ev.contentType == "error" {
			m.executing = false
					}
		return m, waitEvent(m.outputCh)
	}

	answer := m.contentPanel.FindAnswer(sid)
	if answer == nil {
		answer = m.contentPanel.CreateAnswer(sid, "agent")
		m.sessionReg.add(sid, answer)
	}
	m.routeToAnswer(answer, ev.contentType, ev.content)
	m.contentPanel.refreshOnUpdate()

	if ev.contentType == "complete" || ev.contentType == "error" {
		m.sessionReg.remove(sid)
		if m.sessionReg.count() == 0 {
			m.executing = false
					}
	}

	return m, waitEvent(m.outputCh)
}

// routeToAnswer 根据 contentType 将内容路由到 AgentAnswer 的正确区域。
func (m *rootModel) routeToAnswer(answer *AgentAnswer, contentType, content string) {
	switch contentType {
	case "thinking":
		answer.AppendThinking(content)
	case "thinking_done":
		answer.SetThinkingDone(content)
	case "result":
		answer.AppendResult(content)
	case "error":
		answer.AppendError(content)
	case "table", "todo", "options", "plain":
		answer.AppendTyped(content)
	case "action_start":
		// content 格式: "toolName|predictedTokens"
		toolName, estimatedTokens := parseActionStart(content)
		answer.AppendAction(toolName, estimatedTokens)
	case "action_progress":
		answer.SetActionProgress(content)
	case "action_result":
		// content 格式: "success|duration|resultText" 或 "failed|errorText"
		parseActionResult(answer, content)
	}
}

// parseActionStart 解析 action_start 内容，返回 toolName 和 estimatedTokens。
func parseActionStart(content string) (string, int) {
	parts := strings.SplitN(content, "|", 2)
	toolName := parts[0]
	estimatedTokens := 0
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &estimatedTokens)
	}
	return toolName, estimatedTokens
}

// parseActionResult 解析 action_result 内容，更新对应 ActionStep 的状态。
func parseActionResult(answer *AgentAnswer, content string) {
	if strings.HasPrefix(content, "success|") {
		rest := strings.TrimPrefix(content, "success|")
		answer.MarkActionDone(rest)
	} else if strings.HasPrefix(content, "failed|") {
		rest := strings.TrimPrefix(content, "failed|")
		answer.MarkActionFailed(rest)
	}
}

// handleGlobalError 处理全局错误：路由到所有活跃 session 并清理执行状态。
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

// initSessionFromServer 从服务器获取初始会话信息（MasterAgent 和 sessionID）。
func (m *rootModel) initSessionFromServer() tea.Cmd {
	return func() tea.Msg {
		agentResult, err := m.client.SendCommand("agents", "")
		if err != nil {
			return errMsg(fmt.Errorf("获取 Agent 列表失败: %w", err))
		}

		var agents []map[string]string
		if err := json.Unmarshal([]byte(agentResult), &agents); err != nil {
			return errMsg(fmt.Errorf("解析 Agent 列表失败: %w", err))
		}

		masterAgent := "master"
		if len(agents) > 0 {
			if name, ok := agents[0]["name"]; ok && name != "" {
				masterAgent = name
			}
		}

		sessionResult, err := m.client.SendCommand("init", "")
		if err != nil {
			return errMsg(fmt.Errorf("初始化会话失败: %w", err))
		}

		var sessionResp struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(sessionResult), &sessionResp); err != nil || sessionResp.SessionID == "" {
			m.sessionSeq++
			sessionResp.SessionID = fmt.Sprintf("session-%s-%d", masterAgent, m.sessionSeq)
		}

		m.currentAgent = masterAgent
		m.currentSessionID = sessionResp.SessionID
		m.statusBar.SetAgent(m.currentAgent, m.currentModel)

		if err := m.chatManager.Update(masterAgent, sessionResp.SessionID); err != nil {
			return errMsg(fmt.Errorf("保存会话状态失败: %v", err))
		}

		return nil
	}
}

// saveSessionOnExit 程序退出前保存当前会话状态到 chat.json。
func (m *rootModel) saveSessionOnExit() {
	if m.currentAgent != "" && m.currentSessionID != "" {
		if err := m.chatManager.Update(m.currentAgent, m.currentSessionID); err != nil {
			fmt.Printf("警告: 保存会话状态失败: %v\n", err)
		}
	}
}

// ---- Welcome ----

func (m *rootModel) showWelcome() {
	appTitle := "MindX"
	version := "2.0"
	workspace := os.Getenv("MINDX_WORKSPACE")
	if workspace == "" {
		workspace = "default"
	}
	sessionID := fmt.Sprintf("%x", time.Now().UnixNano())
	m.contentPanel.ShowWelcome(appTitle, version, workspace, sessionID, "连接中...")
}
