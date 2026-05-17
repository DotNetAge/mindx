package client

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/goreact"
	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/mindx/internal/client/component/choices"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/component/notify"
	"github.com/DotNetAge/mindx/internal/client/component/statusbar"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	appcore "github.com/DotNetAge/mindx/internal/core"
)

const (
	ansiClearScreen = "\x1b[2J\x1b[H" // 跨平台清屏: macOS/Linux/Windows 10+
)

type rootModel struct {
	program      *tea.Program
	conversation *conv.ConversationPanel
	statusBar    *statusbar.StatusBar
	input        *input.InputArea
	notifBar     *notify.NotificationBar
	choices      *choices.ChoicesPanel

	app         *appcore.App
	chatManager *chatSessionManager
	registry    *SlashCommandRegistry
	masterAgent *goreact.Agent

	eventCancel      func()
	executing        bool
	needsClearScreen bool
}

func NewProgram(cfg *appcore.MindxConfig) *tea.Program {
	m := &rootModel{
		conversation: conv.New(),
		statusBar:    statusbar.New(),
		input:        input.New(),
		notifBar:     notify.New(),
		choices:      choices.New(),
		registry:     BuiltinCommands(),
	}

	var err error
	m.app, err = appcore.DefaultApp(cfg)
	if err != nil {
		m.notifBar.Add(data.Notification{Message: fmt.Sprintf("初始化失败: %v", err), Level: data.NotifError})
	} else {
		m.masterAgent, err = m.app.GetMaster()
		if err != nil {
			m.notifBar.Add(data.Notification{Message: fmt.Sprintf("Agent不可用: %v", err), Level: data.NotifError})
		}
		m.chatManager = newChatSessionManager(m.app)
		m.loadCommands()

		if err := m.restoreLastSession(); err != nil {
			m.notifBar.Add(data.Notification{Message: fmt.Sprintf("会话恢复失败（首次启动）: %v", err), Level: data.NotifInfo})
		} else {
			m.needsClearScreen = true
		}

		m.populateWelcome()
	}

	p := tea.NewProgram(m)
	m.program = p
	return p
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
}

func (m *rootModel) populateWelcome() {
	if m.app == nil {
		return
	}
	m.conversation.WelcomeData = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0",
		ModelName: "unknown",
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.conversation.WelcomeData.Workspace = sessionMeta.GetProjectDir()
		m.conversation.WelcomeData.SessionID = sessionMeta.SessionID
	}

	if m.masterAgent != nil {
		m.conversation.WelcomeData.AgentName = m.masterAgent.Name()
		if m.masterAgent.Model() != nil {
			m.conversation.WelcomeData.ModelName = m.masterAgent.Model().Name
		}
		sid := m.masterAgent.SessionID()
		if sid != "" {
			m.conversation.WelcomeData.SessionID = sid
		}
	}
}

func (m *rootModel) restoreLastSession() error {
	if m.app == nil {
		return fmt.Errorf("app not initialized")
	}

	sessDB := m.app.SessDB()
	if sessDB == nil {
		return fmt.Errorf("session store not available")
	}

	ctx := context.Background()
	sessions, err := sessDB.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no existing sessions (first launch)")
	}

	latestSession := sessions[0]
	m.app.SetCurrentSessionMeta(&latestSession)

	return nil
}

func (m *rootModel) startEventLoop() {
	if m.app == nil || m.masterAgent == nil {
		return
	}
	m.stopEventLoop()

	go func() {
		bus := m.masterAgent.Reactor().EventBus()
		if bus == nil {
			return
		}
		eventCh, cancel := bus.Subscribe()
		m.eventCancel = cancel

		for evt := range eventCh {
			switch evt.Type {
			case goreactcore.ThinkingDelta:
				m.program.Send(clientmsg.ThinkingDeltaMsg{SessionID: evt.SessionID, Content: toString(evt.Data)})
			case goreactcore.ThinkingDone:
				m.program.Send(clientmsg.ThinkingDoneMsg{SessionID: evt.SessionID})
			case goreactcore.ActionStart:
				toolName, params, estimatedTok := extractActionStartData(evt.Data)
				m.program.Send(clientmsg.ActionStartMsg{
					SessionID:    evt.SessionID,
					ToolName:     toolName,
					EstimatedTok: estimatedTok,
					Params:       params,
				})
			case goreactcore.ActionProgress:
				m.program.Send(clientmsg.ActionProgressMsg{SessionID: evt.SessionID, ToolName: "", Progress: toString(evt.Data)})
			case goreactcore.ActionResult:
				toolName, success, resultStr, errStr := extractActionResultData(evt.Data)
				m.program.Send(clientmsg.ActionResultMsg{
					SessionID: evt.SessionID,
					ToolName:  toolName,
					Success:   success,
					Result:    resultStr,
					Error:     errStr,
				})
			case goreactcore.FinalAnswer:
				m.program.Send(clientmsg.FinalAnswerMsg{SessionID: evt.SessionID, Content: toString(evt.Data)})
			case goreactcore.Error:
				m.program.Send(clientmsg.AgentErrorMsg{SessionID: evt.SessionID, Error: errors.New(toString(evt.Data))})
			}
		}
	}()
}

func (m *rootModel) stopEventLoop() {
	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
}

func toString(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", data)
}

func extractActionStartData(data any) (toolName string, params map[string]any, estimatedTok int) {
	if d, ok := data.(goreactcore.ActionStartData); ok {
		return d.ToolName, d.Params, d.PredictedTokens
	}
	return "", nil, 0
}

func extractActionResultData(data any) (toolName string, success bool, result, errStr string) {
	if d, ok := data.(goreactcore.ActionResultData); ok {
		return d.ToolName, d.Success, d.Result, d.Error
	}
	return "", false, "", ""
}

func (m *rootModel) Init() tea.Cmd {
	m.startEventLoop()
	if m.needsClearScreen {
		return func() tea.Msg {
			fmt.Print(ansiClearScreen)
			return nil
		}
	}
	return nil
}

func (m *rootModel) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := e.(type) {
	case tea.WindowSizeMsg:
		w := clientmsg.WindowResizeMsg{Width: msg.Width, Height: msg.Height}
		m.dispatchToAll(w)

	case tea.KeyPressMsg:
		_, cmd := m.input.Update(msg)
		return m, cmd

	case tea.MouseWheelMsg:
		m.conversation.ViewportUpdate(msg)
		return m, nil

	case clientmsg.WindowResizeMsg:
		m.dispatchToAll(msg)

	case clientmsg.UserSendMsg:
		return m.handleSend(msg)

	case clientmsg.AgentSwitchMsg:
		return m.handleAgentSwitch(msg)

	case clientmsg.SlashCommandMsg:
		return m.handleSlashCommand(msg)

	case clientmsg.SessionDoneMsg, clientmsg.AgentErrorMsg:
		m.executing = false
		_, cmd := m.conversation.Update(msg)
		return m, cmd

	case clientmsg.ThinkingDeltaMsg, clientmsg.ThinkingDoneMsg,
		clientmsg.ActionProgressMsg, clientmsg.ActionResultMsg, clientmsg.FinalAnswerMsg,
		clientmsg.TickMsg, clientmsg.CollapseToggleMsg, clientmsg.ThinkCollapseMsg,
		clientmsg.ClearScreenMsg:
		_, cmd := m.conversation.Update(msg)
		return m, cmd

	case clientmsg.ActionStartMsg:
		m.statusBar.Update(msg)
		_, cmd := m.conversation.Update(msg)
		return m, cmd

	case clientmsg.ChoiceSelectedMsg:
		_, cmd := m.choices.Update(msg)
		return m, cmd

	case clientmsg.NotifTimeoutMsg:
		_, cmd := m.notifBar.Update(msg)
		return m, cmd

	case clientmsg.ShowChoicesMsg:
		_, cmd := m.choices.Update(msg)
		return m, cmd

	case clientmsg.SessionLoadedMsg:
		m.statusBar.Update(msg)
		return m, nil

	case clientmsg.ExitMsg:
		m.stopEventLoop()
		return m, tea.Quit
	}

	return m, nil
}

func (m *rootModel) dispatchToAll(w clientmsg.WindowResizeMsg) {
	m.conversation.Update(w)
	m.statusBar.Update(w)
	m.input.Update(w)
	m.notifBar.Update(w)
}

func (m *rootModel) handleSend(e clientmsg.UserSendMsg) (tea.Model, tea.Cmd) {
	if m.executing {
		return m, m.notifBar.Add(data.Notification{Message: "已有消息正在处理", Level: data.NotifWarning})
	}
	if m.masterAgent == nil {
		m.executing = false
		return m, m.notifBar.Add(data.Notification{Message: "Agent未初始化", Level: data.NotifError})
	}

	m.executing = true
	agent := m.masterAgent

	_, err := m.chatManager.getOrCreateSession(agent)
	if err != nil {
		m.executing = false
		return m, m.notifBar.Add(data.Notification{Message: fmt.Sprintf("会话创建失败: %v", err), Level: data.NotifError})
	}

	sessionID := agent.SessionID()

	go func() {
		_, err := agent.Ask(sessionID, e.Text)
		if err != nil {
			m.program.Send(clientmsg.AgentErrorMsg{SessionID: sessionID, Error: err})
		}
		m.program.Send(clientmsg.SessionDoneMsg{SessionID: sessionID})
	}()

	return m, nil
}

func (m *rootModel) handleAgentSwitch(e clientmsg.AgentSwitchMsg) (tea.Model, tea.Cmd) {
	if m.executing {
		m.stopEventLoop()
		m.executing = false
	}

	_, err := m.app.ResolveAgent(e.AgentName)
	if err != nil {
		return m, m.notifBar.Add(data.Notification{Message: fmt.Sprintf("Agent %q 不可用: %v", e.AgentName, err), Level: data.NotifError})
	}

	_, _ = m.statusBar.Update(clientmsg.AgentSwitchMsg{AgentName: e.AgentName})
	m.startEventLoop()

	return m, m.notifBar.Add(data.Notification{
		Message: fmt.Sprintf("已切换到 Agent: %s", e.AgentName),
		Level:   data.NotifInfo,
	})
}

func (m *rootModel) handleSlashCommand(e clientmsg.SlashCommandMsg) (tea.Model, tea.Cmd) {
	cmd := m.registry.Get(e.Name)
	if cmd == nil {
		return m, m.notifBar.Add(data.Notification{Message: fmt.Sprintf("未知命令: /%s", e.Name), Level: data.NotifWarning})
	}

	result := cmd.Run(e.Args)
	if result.Message != "" {
		return m, m.notifBar.Add(data.Notification{Message: result.Message, Level: data.NotifInfo})
	}
	return m, nil
}

func (m *rootModel) View() tea.View {
	convView := m.conversation.View()
	inputView := m.input.View()
	notifView := m.notifBar.View()

	var out string
	if convView != "" {
		out = convView + "\n"
	}
	if notifView != "" {
		out += notifView + "\n"
	}
	out += inputView
	return tea.NewView(out)
}
