package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/timer"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/goreact"
	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/mindx/internal/client/component/choices"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/component/notify"
	"github.com/DotNetAge/mindx/internal/client/component/statusbar"
	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	appcore "github.com/DotNetAge/mindx/internal/core"
)

type rootModel struct {
	program          *tea.Program
	conversationList conv.ConversationList
	welcome          *welcome.WelcomePanel
	statusBar        *statusbar.StatusBar
	input            *input.InputArea
	notifBar         *notify.NotificationBar
	choices          *choices.ChoicesPanel
	viewport         viewport.Model
	termWidth        int
	termHeight       int

	app      *appcore.App
	registry *SlashCommandRegistry
	agent    *goreact.Agent

	eventDone   chan struct{}
	msgCh       chan tea.Msg
	executing   bool
	postExitCmd string

	// pendingPermission tracks an active permission request waiting for user response.
	// Non-nil while the choices panel is showing a permission question.
	pendingPermission *pendingPermissionState
}

// pendingPermissionState holds the state needed to respond to a permission request.
type pendingPermissionState struct {
	responder goreactcore.PermissionResponder
	req       clientmsg.PermissionRequestMsg
}

var pendingPostExitCmd string

func NewProgram(cfg *appcore.MindxConfig) error {
	m := &rootModel{
		conversationList: conv.NewConversationList(),
		welcome:          welcome.New(),
		statusBar:        statusbar.New(),
		input:            input.New(),
		notifBar:         notify.New(),
		choices:          choices.New(),
		viewport:         viewport.New(),
	}

	var err error
	m.app, err = appcore.DefaultApp(cfg)
	if err != nil {
		m.notifBar.Add(data.Notification{Message: fmt.Sprintf("初始化失败: %v", err), Level: data.NotifError})
	} else {
		m.agent, err = m.app.CurrentAgent()
		if err != nil {
			m.notifBar.Add(data.Notification{Message: fmt.Sprintf("Agent不可用: %v", err), Level: data.NotifError})
		}
		m.registry = BuiltinCommands(CommandDeps{
			App:     m.app,
			OnClear: func() { m.program.Send(clientmsg.ClearScreenMsg{}) },
			OnExit:  func() { m.program.Send(clientmsg.ExitMsg{}) },
			OnDoctor: func() {
				m.postExitCmd = "doctor"
			},
		})
		m.loadCommands()

		m.populateWelcome()
	}

	fmt.Print("\x1b[2J\x1b[H")
	p := tea.NewProgram(m)
	m.program = p
	if _, err := p.Run(); err != nil {
		return err
	}

	if pendingPostExitCmd == "doctor" {
		fmt.Print("\n🔧 正在启动诊断向导...\n\n")
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("获取可执行路径失败: %w", err)
		}
		cmd := exec.Command(self, "doctor")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("诊断向导执行失败: %w", err)
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

func messagesToConversations(sessionID, agentName string, msgs []goreactcore.Message) []conv.Conversation {
	var convs []conv.Conversation
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			conv := conv.Conversation{
				SessionID: sessionID,
				AgentName: agentName,
				Status:    conv.StatusDone,
				CreatedAt: time.UnixMilli(msg.Timestamp),
				Question:  conv.Question{Text: msg.Content},
				Output:    conv.Output{},
			}
			convs = append(convs, conv)
		case "assistant":
			if len(convs) > 0 {
				last := &convs[len(convs)-1]
				last.Output.Entries = append(last.Output.Entries, conv.OutputEntry{Role: "assistant", Content: msg.Content})
			}
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

	// Prefer CurrentSessionMeta (always set after /chat switch),
	// fall back to agent.SessionID() (set on startup via WithSession).
	sessionMeta := m.app.CurrentSessionMeta()
	sessionID := ""
	agentName := ""
	if sessionMeta != nil && sessionMeta.SessionID != "" {
		sessionID = sessionMeta.SessionID
		agentName = sessionMeta.AgentName
	} else if m.agent != nil {
		sessionID = m.agent.SessionID()
	}
	if sessionID == "" {
		return
	}
	if agentName == "" && m.agent != nil {
		agentName = m.agent.Name()
	}

	ctx := context.Background()
	msgs, err := sessDB.Get(ctx, sessionID)
	if err != nil || len(msgs) == 0 {
		return
	}
	convs := messagesToConversations(sessionID, agentName, msgs)
	m.conversationList.Conversations = append(m.conversationList.Conversations, convs...)
}

func (m *rootModel) populateWelcome() {
	if m.app == nil {
		return
	}
	m.welcome.Data = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0 Beta",
		ModelName: "unknown",
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.welcome.Data.Workspace = sessionMeta.GetProjectDir()
		m.welcome.Data.SessionID = sessionMeta.SessionID
	}

	if m.agent != nil {
		m.welcome.Data.AgentName = m.agent.Name()
		m.statusBar.AgentName = m.agent.Name()
		if m.agent.Model() != nil {
			m.welcome.Data.ModelName = m.agent.Model().Name
			m.statusBar.ModelName = m.agent.Model().Name
		}
		sid := m.agent.SessionID()
		if sid != "" {
			m.welcome.Data.SessionID = sid
		}
		m.loadSessionHistory()
	}
}

func (m *rootModel) startEventLoop() {
	if m.app == nil || m.agent == nil {
		return
	}
	m.stopEventLoop()

	m.eventDone = make(chan struct{})
	m.msgCh = make(chan tea.Msg, 2048)

	go func() {
		defer func() {
			if p := recover(); p != nil {
				fmt.Printf("[mindx] event subscriber panicked: %v\n", p)
			}
		}()

		bus := m.agent.Reactor().EventBus()
		if bus == nil {
			return
		}
		eventCh, cancel := bus.Subscribe()
		defer cancel()

		for {
			select {
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				msg := m.convertEvent(evt)
				if msg != nil {
					select {
					case m.msgCh <- msg:
					default:
					}
				}
			case <-m.eventDone:
				return
			}
		}
	}()

	go func() {
		defer func() {
			if p := recover(); p != nil {
				fmt.Printf("[mindx] msg forwarder panicked: %v\n", p)
			}
		}()

		for {
			select {
			case msg, ok := <-m.msgCh:
				if !ok {
					return
				}
				// Fire-and-forget to prevent blocking the event pipeline
				// when bubbletea's render cycle is under load.
				go m.program.Send(msg)
			case <-m.eventDone:
				return
			}
		}
	}()
}

func (m *rootModel) convertEvent(evt goreactcore.ReactEvent) tea.Msg {
	switch evt.Type {
	case goreactcore.ThinkingDelta:
		return clientmsg.ThinkingDeltaMsg{SessionID: evt.SessionID, Content: toString(evt.Data)}
	case goreactcore.ThinkingDone:
		doneMsg := clientmsg.ThinkingDoneMsg{SessionID: evt.SessionID}
		if thought, ok := evt.Data.(map[string]any); ok {
			doneMsg.ThoughtData = thought
			if reasoning, ok := thought["reasoning"].(string); ok {
				doneMsg.Reasoning = reasoning
			}
			if decision, ok := thought["decision"].(string); ok {
				doneMsg.Decision = decision
			}
			if isFinal, ok := thought["is_final"].(bool); ok {
				doneMsg.IsFinal = isFinal
			}
		}
		return doneMsg
	case goreactcore.ActionStart:
		if data, ok := evt.Data.(goreactcore.ActionStartData); ok {
			return clientmsg.ActionStartMsg{
				SessionID:    evt.SessionID,
				ToolCount:    data.ToolCount,
				ToolNames:    data.ToolNames,
				EstimatedTok: data.TotalPredictedTokens,
			}
		}
	case goreactcore.ToolExecStart:
		if data, ok := evt.Data.(goreactcore.ToolExecStartData); ok {
			return clientmsg.ToolExecStartMsg{
				SessionID:    evt.SessionID,
				ToolName:     data.ToolName,
				Params:       data.Params,
				EstimatedTok: data.PredictedTokens,
			}
		}
	case goreactcore.ToolExecEnd:
		if data, ok := evt.Data.(goreactcore.ToolExecEndData); ok {
			return clientmsg.ToolExecEndMsg{
				SessionID:  evt.SessionID,
				ToolName:   data.ToolName,
				ToolCallID: data.ToolCallID,
				Success:    data.Success,
				Result:     data.Result,
				Error:      data.Error,
				Duration:   data.Duration,
			}
		}
	case goreactcore.ActionProgress:
		if data, ok := evt.Data.(goreactcore.ActionProgressData); ok {
			return clientmsg.ActionProgressMsg{
				SessionID:      evt.SessionID,
				CompletedCount: data.CompletedCount,
				TotalCount:     data.TotalCount,
				Status:         data.Status,
			}
		}
	case goreactcore.ActionEnd:
		if data, ok := evt.Data.(goreactcore.ActionEndData); ok {
			return clientmsg.ActionEndMsg{
				SessionID:    evt.SessionID,
				TotalTools:   data.TotalTools,
				SuccessCount: data.SuccessCount,
				FailedCount:  data.FailedCount,
				Summary:      data.Summary,
			}
		}
	case goreactcore.ExecutionSummary:
		if data, ok := evt.Data.(goreactcore.ExecutionSummaryData); ok {
			return clientmsg.ExecutionSummaryMsg{
				SessionID:  evt.SessionID,
				Duration:   data.TotalDuration,
				TokensUsed: data.TokensUsed,
				ToolCalls:  data.ToolCalls,
			}
		}
	case goreactcore.FinalAnswer:
		return clientmsg.FinalAnswerMsg{SessionID: evt.SessionID, Content: toString(evt.Data)}
	case goreactcore.Error:
		return clientmsg.AgentErrorMsg{SessionID: evt.SessionID, Error: errors.New(toString(evt.Data))}
	case goreactcore.LLMTimeout:
		if data, ok := evt.Data.(goreactcore.LLMTimeoutData); ok {
			return clientmsg.LLMTimeoutMsg{
				SessionID: evt.SessionID,
				Timeout:   data.Timeout,
				Elapsed:   data.Elapsed,
				Error:     data.Error,
			}
		}
	case goreactcore.PermissionRequest:
		if data, ok := evt.Data.(goreactcore.PermissionRequestData); ok {
			questions := make([]clientmsg.QuestionData, len(data.Questions))
			for i, q := range data.Questions {
				opts := make([]string, len(q.Options))
				for j, o := range q.Options {
					opts[j] = o.Label
				}
				questions[i] = clientmsg.QuestionData{
					Question:    q.Question,
					Header:      q.Header,
					Options:     opts,
					MultiSelect: q.MultiSelect,
				}
			}
			return clientmsg.PermissionRequestMsg{
				ToolName:      data.ToolName,
				Reason:        data.Reason,
				SecurityLevel: int(data.SecurityLevel),
				Questions:     questions,
			}
		}
	}
	return nil
}

func (m *rootModel) stopEventLoop() {
	if m.eventDone != nil {
		close(m.eventDone)
		m.eventDone = nil
	}
	if m.msgCh != nil {
		close(m.msgCh)
		m.msgCh = nil
	}
}

func toString(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", data)
}

func (m *rootModel) Init() tea.Cmd {
	m.startEventLoop()
	return m.conversationList.Init()
}

func (m *rootModel) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := e.(type) {
	case tea.WindowSizeMsg:
		w := clientmsg.WindowResizeMsg{Width: msg.Width, Height: msg.Height}
		m.dispatchToAll(w)
		m.resizeViewport(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		m.input.Executing = m.executing
		_, inputCmd := m.input.Update(msg)
		newVp, vpCmd := m.viewport.Update(msg)
		m.viewport = newVp
		return m, tea.Batch(inputCmd, vpCmd)

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
		m.statusBar.CurrentState = "空闲"
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.AgentErrorMsg:
		m.executing = false
		// Don't override "已取消" status when cancel triggered the error
		if !errors.Is(msg.Error, context.Canceled) {
			m.statusBar.CurrentState = "出错"
		}
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case timer.TickMsg:
		m.statusBar.Tick()
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ThinkingDeltaMsg, clientmsg.ThinkingDoneMsg:
		m.statusBar.CurrentState = "思考中"
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ToolExecStartMsg, clientmsg.ToolExecEndMsg,
		clientmsg.ActionProgressMsg:
		m.statusBar.CurrentState = "执行中"
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ExecutionSummaryMsg:
		m.statusBar.Update(msg)
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ActionEndMsg:
		m.statusBar.CurrentState = "正在获取结果"
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.FinalAnswerMsg:
		m.statusBar.CurrentState = "完成"
		m.statusBar.Update(msg)
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ActionStartMsg:
		m.statusBar.CurrentState = "执行中"
		m.statusBar.Update(msg)
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.CollapseToggleMsg, clientmsg.ThinkCollapseMsg:
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ClearScreenMsg:
		m.statusBar.CurrentState = "空闲"
		newList, cmd := m.conversationList.Update(msg)
		m.conversationList = newList
		return m, cmd

	case clientmsg.ChoiceSelectedMsg:
		_, cmd := m.choices.Update(msg)
		// If there's a pending permission request, respond to it
		if m.pendingPermission != nil {
			pp := m.pendingPermission
			m.pendingPermission = nil // Clear before responding
			if msg.Index < 0 {
				// esc / cancelled
				go pp.responder.Respond(goreactcore.PermissionResult{
					Behavior: goreactcore.PermissionDeny,
					Message:  "user cancelled",
				})
			} else if len(pp.req.Questions) > 0 {
				// AskUser: selected a choice
				q := pp.req.Questions[0]
				selected := q.Options[msg.Index]
				go pp.responder.Respond(goreactcore.PermissionResult{
					Behavior: goreactcore.PermissionAllow,
					UpdatedInput: map[string]any{
						"answers": map[string]any{
							q.Question: selected,
						},
					},
				})
			} else {
				// Simple allow/deny
				if msg.Index == 0 {
					go pp.responder.Respond(goreactcore.PermissionResult{
						Behavior: goreactcore.PermissionAllow,
						Message:  "user approved",
					})
				} else {
					go pp.responder.Respond(goreactcore.PermissionResult{
						Behavior: goreactcore.PermissionDeny,
						Message:  "user denied",
					})
				}
			}
		}
		return m, cmd

	case clientmsg.NotifTimeoutMsg:
		_, cmd := m.notifBar.Update(msg)
		return m, cmd

	case clientmsg.ShowChoicesMsg:
		_, cmd := m.choices.Update(msg)
		return m, cmd

	case clientmsg.PermissionRequestMsg:
		m.statusBar.CurrentState = "等待选择"
		// Store pending permission responder
		responder := m.agent.Reactor().PermissionResponder()
		m.pendingPermission = &pendingPermissionState{
			responder: responder,
			req:       msg,
		}
		// Show choices panel with questions
		if len(msg.Questions) > 0 {
			q := msg.Questions[0]
			if len(q.Options) > 0 {
				return m, func() tea.Msg {
					return clientmsg.ShowChoicesMsg{
						Options: q.Options,
						Prompt:  q.Question,
					}
				}
			}
		} else {
			// Simple allow/deny
			return m, func() tea.Msg {
				return clientmsg.ShowChoicesMsg{
					Options: []string{"Allow", "Deny"},
					Prompt:  "🔒 " + msg.ToolName + ": " + msg.Reason,
				}
			}
		}
		return m, nil

	case clientmsg.SessionLoadedMsg:
		m.statusBar.Update(msg)
		return m, nil

	case clientmsg.ExecutionCancelMsg:
		m.executing = false
		m.statusBar.CurrentState = "已取消"
		if m.agent != nil {
			m.agent.Cancel()
		}
		return m, nil

	case clientmsg.ExitMsg:
		m.handlePostExit()
		m.stopEventLoop()
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
	// Initial estimate-based height; View() overrides this dynamically.
	headerEstimate := 8
	footerEstimate := 5
	separators := 2
	vpHeight := termHeight - headerEstimate - footerEstimate - separators
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.SetWidth(termWidth)
	m.viewport.SetHeight(vpHeight)
}

func (m *rootModel) handleSend(e clientmsg.UserSendMsg) (tea.Model, tea.Cmd) {
	if m.executing {
		return m, m.notifBar.Add(data.Notification{Message: "已有消息正在处理", Level: data.NotifWarning})
	}
	if m.agent == nil {
		m.executing = false
		return m, m.notifBar.Add(data.Notification{Message: "Agent未初始化", Level: data.NotifError})
	}

	m.executing = true
	m.statusBar.SessionStart = time.Now()
	m.statusBar.SessionDuration = 0
	agent := m.agent
	sessionID := agent.SessionID()
	if sessionID == "" {
		sessionID = "main"
	}

	newConv := conv.NewConversation(sessionID, agent.Name(), e.Text)
	m.conversationList.Conversations = append(m.conversationList.Conversations, newConv)
	m.conversationList.MarkDirty()

	go func() {
		defer func() {
			if p := recover(); p != nil {
				m.program.Send(clientmsg.AgentErrorMsg{
					SessionID: sessionID,
					Error:     fmt.Errorf("handleSend panic: %v", p),
				})
			}
		}()

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

	newAgent, err := m.app.ResolveAgent(e.AgentName)
	if err != nil {
		return m, m.notifBar.Add(data.Notification{Message: fmt.Sprintf("Agent %q 不可用: %v", e.AgentName, err), Level: data.NotifError})
	}

	m.agent = newAgent
	m.statusBar.AgentName = newAgent.Name()
	if newAgent.Model() != nil {
		m.statusBar.ModelName = newAgent.Model().Name
	}
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

	switch e.Name {
	case "chat":
		m.refreshAfterChatOp(result)
	case "model":
		if len(e.Args) > 0 && result.Success {
			modelName := e.Args[0]
			if modelCfg := m.app.Models().Get(modelName); modelCfg != nil && m.agent != nil {
				m.agent.SetModel(modelCfg)
				m.statusBar.ModelName = modelCfg.Name
				m.welcome.Data.ModelName = modelCfg.Name
				if cfg := m.app.Config(); cfg != nil {
					cfg.LastModel = modelName
					_ = cfg.Save()
				}
			}
		}
		m.input.Models, _ = reloadModels(m.app)
	case "doctor":
		m.handlePostExit()
		m.stopEventLoop()
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

func (m *rootModel) refreshAfterChatOp(result CommandResult) {
	if !result.Success {
		return
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.statusBar.Update(clientmsg.SessionLoadedMsg{
			AgentName: sessionMeta.AgentName,
			SessionID: sessionMeta.SessionID,
		})
		m.statusBar.AgentName = sessionMeta.AgentName
	}
	if m.agent != nil && m.agent.Model() != nil {
		m.statusBar.ModelName = m.agent.Model().Name
	}

	m.conversationList.Clear()
	m.loadSessionHistory()
	newSessions, _ := loadRecentSessions(m.app)
	m.input.Sessions = newSessions
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
	welcomeView := m.welcome.View()
	notifView := m.notifBar.View()
	choicesView := m.choices.View()
	statusView := m.statusBar.View()
	inputView := m.input.View()

	// Build header: welcome + optional notifications + choices
	var header strings.Builder
	header.WriteString(welcomeView)
	if notifView != "" {
		header.WriteString("\n")
		header.WriteString(notifView)
	}
	if choicesView != "" {
		header.WriteString("\n")
		header.WriteString(choicesView)
	}
	headerStr := header.String()

	// Build footer: statusbar + input
	var footer strings.Builder
	footer.WriteString(statusView)
	footer.WriteString("\n")
	footer.WriteString(inputView)
	footerStr := footer.String()

	// Dynamically compute viewport height from actual rendered line counts.
	// This accounts for the input suggestion dropdown making the footer taller.
	headerLines := strings.Count(headerStr, "\n") + 1
	footerLines := strings.Count(footerStr, "\n") + 1
	separators := 2
	vpHeight := m.termHeight - headerLines - footerLines - separators
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.SetWidth(m.termWidth)
	m.viewport.SetHeight(vpHeight)

	// Update viewport content from conversation list
	m.viewport.SetContent(m.conversationList.View())

	// Compose full layout
	var out strings.Builder
	out.WriteString(headerStr)
	out.WriteString("\n")
	out.WriteString(m.viewport.View())
	out.WriteString("\n")
	out.WriteString(footerStr)

	v := tea.NewView(out.String())
	v.AltScreen = true
	return v
}
