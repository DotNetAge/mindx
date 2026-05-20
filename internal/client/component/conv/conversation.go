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
	Reasoning Reasoning
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
		Reasoning: NewReasoning(),
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
		newReasoning, cmd := UpdateReasoning(m.Reasoning, e)
		m.Reasoning = newReasoning
		return m, cmd

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
		newReasoning, reasoningCmd := UpdateReasoning(m.Reasoning, e)
		m.Reasoning = newReasoning

		for i := range m.Rounds {
			newThought, _ := UpdateThought(m.Rounds[i].Thought, e)
			m.Rounds[i].Thought = newThought
			newAction, _ := UpdateAction(m.Rounds[i].Action, e)
			m.Rounds[i].Action = newAction
		}
		return m, reasoningCmd

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
	reasoningView := ViewReasoning(m.Reasoning)

	var roundsView strings.Builder
	for _, round := range m.Rounds {
		actionView := ViewAction(round.Action, width)

		if actionView != "" {
			if questionView != "" || reasoningView != "" || roundsView.Len() > 0 {
				roundsView.WriteString("\n")
			}
			roundsView.WriteString(actionView)
		}
	}

	outputView := ViewOutput(m.Output, width)
	errorView := ViewErrorMsg(m.Error, width)

	if questionView == "" && reasoningView == "" && roundsView.Len() == 0 && outputView == "" && errorView == "" {
		return ""
	}

	var b viewBuilder
	if questionView != "" {
		b.writeString(questionView)
	}
	if reasoningView != "" {
		if questionView != "" {
			b.writeString("\n")
		}
		b.writeString(reasoningView)
	}
	if roundsView.Len() > 0 {
		if questionView != "" || reasoningView != "" {
			b.writeString("\n")
		}
		b.writeString(roundsView.String())
	}
	if outputView != "" {
		if questionView != "" || reasoningView != "" || roundsView.Len() > 0 {
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
