package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

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
	ansiClearScreen = "\x1b[2J\x1b[H"
)

type rootModel struct {
	program      *tea.Program
	conversation *conv.ConversationPanel
	statusBar    *statusbar.StatusBar
	input        *input.InputArea
	notifBar     *notify.NotificationBar
	choices      *choices.ChoicesPanel

	app         *appcore.App
	registry    *SlashCommandRegistry
	agent *goreact.Agent

	eventDone        chan struct{}
	executing        bool
	needsClearScreen bool
	postExitCmd      string
}

var pendingPostExitCmd string

func NewProgram(cfg *appcore.MindxConfig) error {
	m := &rootModel{
		conversation: conv.New(),
		statusBar:    statusbar.New(),
		input:        input.New(),
		notifBar:     notify.New(),
		choices:      choices.New(),
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

		if err := m.restoreLastSession(); err != nil {
			m.notifBar.Add(data.Notification{Message: fmt.Sprintf("会话恢复失败（首次启动）: %v", err), Level: data.NotifInfo})
		} else {
			m.needsClearScreen = true
		}

		m.populateWelcome()
	}

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

func (m *rootModel) populateWelcome() {
	if m.app == nil {
		return
	}
	m.conversation.WelcomeData = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0 Beta",
		ModelName: "unknown",
	}

	sessionMeta := m.app.CurrentSessionMeta()
	if sessionMeta != nil {
		m.conversation.WelcomeData.Workspace = sessionMeta.GetProjectDir()
		m.conversation.WelcomeData.SessionID = sessionMeta.SessionID
	}

	if m.agent != nil {
		m.conversation.WelcomeData.AgentName = m.agent.Name()
		if m.agent.Model() != nil {
			m.conversation.WelcomeData.ModelName = m.agent.Model().Name
		}
		sid := m.agent.SessionID()
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
		if m.agent != nil {
			meta, err := m.app.CreateSession(m.agent.Name())
			if err == nil {
				m.app.SetCurrentSessionMeta(meta)
				return nil
			}
		}
	}

	if len(sessions) == 0 {
		return nil
	}

	latestSession := sessions[0]
	m.app.SetCurrentSessionMeta(&latestSession)

	return nil
}

func (m *rootModel) startEventLoop() {
	if m.app == nil || m.agent == nil {
		return
	}
	m.stopEventLoop()

	m.eventDone = make(chan struct{})

	go func() {
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
				switch evt.Type {
				case goreactcore.ThinkingDelta:
					m.program.Send(clientmsg.ThinkingDeltaMsg{SessionID: evt.SessionID, Content: toString(evt.Data)})
				case goreactcore.ThinkingDone:
					m.program.Send(clientmsg.ThinkingDoneMsg{SessionID: evt.SessionID})
				case goreactcore.ActionStart:
					if data, ok := evt.Data.(goreactcore.ActionStartData); ok {
						m.program.Send(clientmsg.ActionStartMsg{
							SessionID:    evt.SessionID,
							ToolCount:    data.ToolCount,
							ToolNames:    data.ToolNames,
							EstimatedTok: data.TotalPredictedTokens,
						})
					}
				case goreactcore.ToolExecStart:
					if data, ok := evt.Data.(goreactcore.ToolExecStartData); ok {
						m.program.Send(clientmsg.ToolExecStartMsg{
							SessionID:    evt.SessionID,
							ToolName:     data.ToolName,
							Params:       data.Params,
							EstimatedTok: data.PredictedTokens,
						})
					}
				case goreactcore.ToolExecEnd:
					if data, ok := evt.Data.(goreactcore.ToolExecEndData); ok {
						m.program.Send(clientmsg.ToolExecEndMsg{
							SessionID: evt.SessionID,
							ToolName:  data.ToolName,
							Success:   data.Success,
							Result:    data.Result,
							Error:     data.Error,
								Duration:  data.Duration,
						})
					}
				case goreactcore.ActionProgress:
					if data, ok := evt.Data.(goreactcore.ActionProgressData); ok {
						m.program.Send(clientmsg.ActionProgressMsg{
							SessionID:      evt.SessionID,
							CompletedCount: data.CompletedCount,
							TotalCount:     data.TotalCount,
							Status:         data.Status,
						})
					}
				case goreactcore.ActionEnd:
					if data, ok := evt.Data.(goreactcore.ActionEndData); ok {
						m.program.Send(clientmsg.ActionEndMsg{
							SessionID:    evt.SessionID,
							TotalTools:   data.TotalTools,
							SuccessCount: data.SuccessCount,
							FailedCount:  data.FailedCount,
							Summary:      data.Summary,
						})
					}
				case goreactcore.ExecutionSummary:
					if data, ok := evt.Data.(goreactcore.ExecutionSummaryData); ok {
						m.program.Send(clientmsg.ExecutionSummaryMsg{
							SessionID:  evt.SessionID,
							Duration:   data.TotalDuration,
							TokensUsed: data.TokensUsed,
							ToolCalls:  data.ToolCalls,
						})
						}

				case goreactcore.FinalAnswer:
					m.program.Send(clientmsg.FinalAnswerMsg{SessionID: evt.SessionID, Content: toString(evt.Data)})
				case goreactcore.Error:
					m.program.Send(clientmsg.AgentErrorMsg{SessionID: evt.SessionID, Error: errors.New(toString(evt.Data))})
				}
			case <-m.eventDone:
				return
			}
		}
	}()
}

func (m *rootModel) stopEventLoop() {
	if m.eventDone != nil {
		close(m.eventDone)
		m.eventDone = nil
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
		clientmsg.ActionProgressMsg, clientmsg.ToolExecStartMsg, clientmsg.ToolExecEndMsg,
		clientmsg.ActionEndMsg, clientmsg.FinalAnswerMsg,
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
		m.handlePostExit()
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
	if m.agent == nil {
		m.executing = false
		return m, m.notifBar.Add(data.Notification{Message: "Agent未初始化", Level: data.NotifError})
	}

	m.executing = true
	agent := m.agent
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

	switch e.Name {
	case "chat":
		m.refreshAfterChatOp(result)
	case "model":
		if len(e.Args) > 0 && result.Success {
			modelName := e.Args[0]
			if modelCfg := m.app.Models().Get(modelName); modelCfg != nil && m.agent != nil {
				m.agent.SetModel(modelCfg)
				m.statusBar.ModelName = modelCfg.Name
				m.conversation.WelcomeData.ModelName = modelCfg.Name
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
	}

	m.conversation.Clear()
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
