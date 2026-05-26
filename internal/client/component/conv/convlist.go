package conv

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

const tickInterval = 800 * time.Millisecond

type ConversationList struct {
	Conversations []Conversation
	width         int
	height        int
	timer         timer.Model
}

func NewConversationList() ConversationList {
	return ConversationList{
		timer: timer.New(100*365*24*time.Hour, timer.WithInterval(tickInterval)),
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

	case msg.ClearScreenMsg:
		l.Conversations = nil

	case timer.TickMsg:
		newTimer, timerCmd := l.timer.Update(e)
		l.timer = newTimer
		now := time.Now()
		for i, conv := range l.Conversations {
			newConv, _ := UpdateConversation(conv, msg.TickMsg{Time: now})
			l.Conversations[i] = newConv
		}
		return l, timerCmd

	case msg.ThinkingDeltaMsg, msg.ThinkingDoneMsg,
		msg.ToolExecStartMsg, msg.ToolExecEndMsg,
		msg.FinalAnswerMsg, msg.AgentErrorMsg,
		msg.LLMTimeoutMsg,
		msg.SessionDoneMsg, msg.ExecutionSummaryMsg,
		msg.CollapseToggleMsg, msg.ThinkCollapseMsg:

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
	}

	return l, nil
}

func (l ConversationList) View() string {
	if len(l.Conversations) == 0 {
		return ""
	}

	// Render newest-to-oldest, building from top to bottom.
	// Bubbletea/terminal handles truncation for lines beyond screen height.
	var parts []string
	for i := len(l.Conversations) - 1; i >= 0; i-- {
		convView := ViewConversation(l.Conversations[i], l.width)
		if convView == "" {
			continue
		}
		parts = append([]string{convView}, parts...)
	}

	return strings.Join(parts, "\n\n")
}

func (l ConversationList) Clear() {
	l.Conversations = nil
}

func (l *ConversationList) MarkDirty() {}

func getSessionID(e tea.Msg) string {
	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		return e.SessionID
	case msg.ThinkingDoneMsg:
		return e.SessionID
	case msg.ToolExecStartMsg:
		return e.SessionID
	case msg.ToolExecEndMsg:
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
	default:
		return ""
	}
}
