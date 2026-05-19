package conv

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/timer"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

const tickInterval = 800 * time.Millisecond

type ConversationList struct {
	Conversations []Conversation
	width         int
	height        int
	viewport      viewport.Model
	timer         timer.Model
	contentDirty  bool
}

func NewConversationList() ConversationList {
	return ConversationList{
		viewport: viewport.New(),
		timer:    timer.New(100*365*24*time.Hour, timer.WithInterval(tickInterval)),
	}
}

func (l ConversationList) Init() tea.Cmd {
	return l.timer.Init()
}

func (l ConversationList) Update(e tea.Msg) (ConversationList, tea.Cmd) {
	switch e := e.(type) {
	case msg.WindowResizeMsg:
		l.width = e.Width
		l.height = e.Height
		l.viewport.SetWidth(e.Width)
		vh := e.Height
		if vh > 2 {
			vh -= 2
		}
		l.viewport.SetHeight(vh)

	case msg.ClearScreenMsg:
		l.Conversations = nil
		l.contentDirty = true

	case tea.MouseWheelMsg:
		l.viewport, _ = l.viewport.Update(e)

	case msg.ThinkingDeltaMsg, msg.ThinkingDoneMsg,
		msg.ActionStartMsg, msg.ToolExecStartMsg, msg.ToolExecEndMsg,
		msg.ActionEndMsg, msg.FinalAnswerMsg, msg.AgentErrorMsg,
		msg.LLMTimeoutMsg,
		msg.SessionDoneMsg, msg.ExecutionSummaryMsg,
		msg.CollapseToggleMsg, msg.ThinkCollapseMsg,
		msg.ActionProgressMsg:

		l.contentDirty = true
		var cmds []tea.Cmd
		sessionID := getSessionID(e)
		found := false
		for i := len(l.Conversations) - 1; i >= 0; i-- {
			conv := l.Conversations[i]
			if conv.SessionID != sessionID {
				continue
			}
			found = true
			newConv, cmd := UpdateConversation(conv, e)
			l.Conversations[i] = newConv
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			break
		}
		if !found && sessionID != "" {
			return l, func() tea.Msg {
				return msg.AgentErrorMsg{
					SessionID: sessionID,
					Error:     fmt.Errorf("事件 SessionID %q 未找到匹配的会话 (当前有 %d 个会话)", sessionID, len(l.Conversations)),
				}
			}
		}
		return l, tea.Batch(cmds...)

	case timer.TickMsg:
		newTimer, timerCmd := l.timer.Update(e)
		l.timer = newTimer
		now := time.Now()
		for i, conv := range l.Conversations {
			newConv, _ := UpdateConversation(conv, msg.TickMsg{Time: now})
			l.Conversations[i] = newConv
		}
		if l.hasActiveStreaming() {
			l.contentDirty = true
		}
		return l, timerCmd
	}

	return l, nil
}

func (l ConversationList) View() string {
	if len(l.Conversations) == 0 {
		return ""
	}

	var content string
	for _, conv := range l.Conversations {
		convView := ViewConversation(conv, l.width)
		if convView == "" {
			continue
		}
		if content != "" {
			content += "\n\n"
		}
		content += convView
	}

	if content == "" {
		return ""
	}

	if l.width == 0 {
		l.viewport.SetWidth(80)
	} else {
		l.viewport.SetWidth(l.width)
	}
	vh := l.height
	if vh > 2 {
		vh -= 2
	}
	l.viewport.SetHeight(vh)
	l.viewport.SetContent(content)
	if l.contentDirty {
		l.viewport.GotoBottom()
		l.contentDirty = false
	}
	return l.viewport.View()
}

func (l ConversationList) ViewportUpdate(e tea.Msg) {
	l.viewport, _ = l.viewport.Update(e)
}

func (l ConversationList) Clear() {
	l.Conversations = nil
	l.contentDirty = true
}

func (l *ConversationList) MarkDirty() {
	l.contentDirty = true
}

func (l ConversationList) hasActiveStreaming() bool {
	for _, conv := range l.Conversations {
		for _, round := range conv.Rounds {
			if round.Thought.IsActive {
				return true
			}
		}
	}
	return false
}

func getSessionID(e tea.Msg) string {
	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		return e.SessionID
	case msg.ThinkingDoneMsg:
		return e.SessionID
	case msg.ActionStartMsg:
		return e.SessionID
	case msg.ToolExecStartMsg:
		return e.SessionID
	case msg.ToolExecEndMsg:
		return e.SessionID
	case msg.ActionEndMsg:
		return e.SessionID
	case msg.FinalAnswerMsg:
		return e.SessionID
	case msg.AgentErrorMsg:
		return e.SessionID
	case msg.SessionDoneMsg:
		return e.SessionID
	case msg.ExecutionSummaryMsg:
		return e.SessionID
	case msg.CollapseToggleMsg:
		return e.SessionID
	case msg.ThinkCollapseMsg:
		return e.SessionID
	case msg.ActionProgressMsg:
		return e.SessionID
	default:
		return ""
	}
}
