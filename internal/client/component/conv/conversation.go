package conv

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

type Conversation struct {
	SessionID string
	AgentName string
	Status    Status
	CreatedAt time.Time

	Question  Question
	Thinking  Thinking
	Rounds    []ThoughtActionRound
	Output    Output
	Error     ErrorMsg
}

func NewConversation(sessionID, agentName, questionText string) Conversation {
	return Conversation{
		SessionID: sessionID,
		AgentName: agentName,
		Status:    StatusThinking,
		CreatedAt: time.Now(),
		Question:  Question{Text: questionText},
		Thinking:  NewThinking(),
	}
}

func (c *Conversation) currentRound() *ThoughtActionRound {
	if len(c.Rounds) == 0 {
		return nil
	}
	return &c.Rounds[len(c.Rounds)-1]
}

func (c *Conversation) ensureCurrentRound() {
	if len(c.Rounds) == 0 || c.currentRound().Action.Completed {
		c.Rounds = append(c.Rounds, NewThoughtActionRound())
	}
}

func UpdateConversation(m Conversation, e tea.Msg) (Conversation, tea.Cmd) {
	switch e := e.(type) {
	case msg.ThinkingDoneMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		newThought, _ := UpdateThought(m.currentRound().Thought, e)
		m.currentRound().Thought = newThought
		newThinking, _ := UpdateThinking(m.Thinking, e)
		m.Thinking = newThinking
		return m, nil

	case msg.ThinkingDeltaMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		newThought, _ := UpdateThought(m.currentRound().Thought, e)
		m.currentRound().Thought = newThought
		return m, nil

	case msg.ActionStartMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		current := m.currentRound()
		newAction, cmd := UpdateAction(current.Action, e)
		current.Action = newAction
		m.Status = StatusExecuting
		return m, cmd

	case msg.ToolUseDeltaMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		newAction, _ := UpdateAction(m.currentRound().Action, e)
		m.currentRound().Action = newAction
		if m.Status != StatusExecuting {
			m.Status = StatusExecuting
		}
		return m, nil

	case msg.ContentDeltaMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		newOutput, _ := UpdateOutput(m.Output, e)
		m.Output = newOutput
		if m.Status != StatusResponding {
			m.Status = StatusResponding
		}
		return m, nil

	case msg.ToolExecStartMsg, msg.ToolExecEndMsg,
		msg.ActionEndMsg, msg.ExecutionSummaryMsg,
		msg.CollapseToggleMsg, msg.ActionProgressMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			if _, isCollapse := e.(msg.CollapseToggleMsg); !isCollapse {
				return m, nil
			}
		}
		if m.currentRound() == nil {
			return m, nil
		}
		newAction, cmd := UpdateAction(m.currentRound().Action, e)
		m.currentRound().Action = newAction
		if _, ok := e.(msg.ActionEndMsg); ok {
			m.Status = StatusResponding
		}
		return m, cmd

	case msg.FinalAnswerMsg, msg.AgentErrorMsg, msg.LLMTimeoutMsg:
		if m.Status == StatusDone {
			return m, nil
		}
		switch e.(type) {
		case msg.AgentErrorMsg:
			newError, _ := UpdateErrorMsg(m.Error, e)
			m.Error = newError
			m.Status = StatusError
		case msg.LLMTimeoutMsg:
			newOutput, cmd := UpdateOutput(m.Output, e)
			m.Output = newOutput
			m.Status = StatusError
			return m, cmd
		default:
			newOutput, cmd := UpdateOutput(m.Output, e)
			m.Output = newOutput
			m.Status = StatusResponding
			return m, cmd
		}
		return m, nil

	case msg.SessionDoneMsg:
		m.Status = StatusDone
		for i := range m.Rounds {
			m.Rounds[i].Thought.IsActive = false
		}
		return m, nil

	case msg.TickMsg:
		for i := range m.Rounds {
			newThought, _ := UpdateThought(m.Rounds[i].Thought, e)
			m.Rounds[i].Thought = newThought
			newAction, _ := UpdateAction(m.Rounds[i].Action, e)
			m.Rounds[i].Action = newAction
		}
		newThinking, _ := UpdateThinking(m.Thinking, e)
		m.Thinking = newThinking
		return m, nil

	case msg.ThinkCollapseMsg:
		if m.currentRound() != nil {
			newThought, cmd := UpdateThought(m.currentRound().Thought, e)
			m.currentRound().Thought = newThought
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func ViewConversation(m Conversation, width int) string {
	questionView := ViewQuestion(m.Question, width)
	thinkingView := ViewThinking(m.Thinking)

	var roundsView strings.Builder
	for _, round := range m.Rounds {
		thoughtView := ViewThought(round.Thought)
		actionView := ViewAction(round.Action, width)

		if thoughtView != "" || actionView != "" {
			if questionView != "" || thinkingView != "" || roundsView.Len() > 0 {
				roundsView.WriteString("\n")
			}
			if thoughtView != "" {
				roundsView.WriteString(thoughtView)
			}
			if actionView != "" {
				if thoughtView != "" {
					roundsView.WriteString("\n")
				}
				roundsView.WriteString(actionView)
			}
		}
	}

	outputView := ViewOutput(m.Output, width)
	errorView := ViewErrorMsg(m.Error, width)

	if questionView == "" && thinkingView == "" && roundsView.Len() == 0 && outputView == "" && errorView == "" {
		return ""
	}

	var b viewBuilder
	if questionView != "" {
		b.writeString(questionView)
	}
	if thinkingView != "" {
		if questionView != "" {
			b.writeString("\n")
		}
		b.writeString(thinkingView)
	}
	if roundsView.Len() > 0 {
		if questionView != "" || thinkingView != "" {
			b.writeString("\n")
		}
		b.writeString(roundsView.String())
	}
	if outputView != "" {
		if questionView != "" || thinkingView != "" || roundsView.Len() > 0 {
			b.writeString("\n")
		}
		b.writeString(outputView)
	}
	if errorView != "" {
		if b.len() > 0 {
			b.writeString("\n\n")
		}
		b.writeString(errorView)
	}
	return b.String()
}
