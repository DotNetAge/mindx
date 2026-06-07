package conv

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

// ConversationRound represents one Think-Act cycle iteration.
type ConversationRound struct {
	ThoughtContent string
	Action         Action
}

type Conversation struct {
	SessionID string
	AgentName string
	Status    Status
	CreatedAt time.Time

	Question Question
	Thinking Thinking
	Rounds   []ConversationRound
	Output   Output
	Error    ErrorMsg

	// MaxTurnsNotice stores a friendly suggestion when max turns are reached.
	// This is displayed as an informational notice (not an error).
	MaxTurnsNotice string
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

func (c *Conversation) currentRound() *ConversationRound {
	if len(c.Rounds) == 0 {
		return nil
	}
	return &c.Rounds[len(c.Rounds)-1]
}

// EnsureCurrentRound is the exported version of ensureCurrentRound for cross-package use
// (e.g., session history restoration in client package).
func (c *Conversation) EnsureCurrentRound() {
	c.ensureCurrentRound()
}

// CurrentRound is the exported version of currentRound for cross-package use.
func (c *Conversation) CurrentRound() *ConversationRound {
	return c.currentRound()
}

func (c *Conversation) ensureCurrentRound() {
	if len(c.Rounds) == 0 {
		c.Rounds = append(c.Rounds, ConversationRound{})
	}
}

func UpdateConversation(m Conversation, e tea.Msg) (Conversation, tea.Cmd) {
	switch e := e.(type) {
	case msg.ThinkingDeltaMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		if curr := m.currentRound(); curr != nil && curr.ThoughtContent != "" && !m.Thinking.IsActive {
			m.Rounds = append(m.Rounds, ConversationRound{})
		}
		m.ensureCurrentRound()
		m.currentRound().ThoughtContent += e.Content
		newThinking, _ := UpdateThinking(m.Thinking, e)
		m.Thinking = newThinking
		return m, nil

	case msg.ThinkingDoneMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		if e.Content != "" {
			m.currentRound().ThoughtContent = e.Content
		}
		newThinking, _ := UpdateThinking(m.Thinking, e)
		m.Thinking = newThinking
		return m, nil

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

	case msg.ToolExecStartMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		m.ensureCurrentRound()
		newAction, cmd := UpdateAction(m.currentRound().Action, e)
		m.currentRound().Action = newAction
		m.Status = StatusExecuting
		return m, cmd

	case msg.ToolExecEndMsg:
		if m.Status == StatusDone || m.Status == StatusError {
			return m, nil
		}
		if m.currentRound() == nil {
			return m, nil
		}
		newAction, cmd := UpdateAction(m.currentRound().Action, e)
		m.currentRound().Action = newAction
		return m, cmd

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

	case msg.ExecutionSummaryMsg:
		if m.currentRound() == nil {
			return m, nil
		}
		newAction, cmd := UpdateAction(m.currentRound().Action, e)
		m.currentRound().Action = newAction
		return m, cmd

	case msg.FinalAnswerMsg, msg.AgentErrorMsg, msg.LLMTimeoutMsg, msg.MaxTurnsReachedMsg:
		if m.Status == StatusDone {
			return m, nil
		}
		switch e := e.(type) {
		case msg.AgentErrorMsg:
			newError, _ := UpdateErrorMsg(m.Error, e)
			m.Error = newError
			m.Status = StatusError
		case msg.LLMTimeoutMsg:
			newOutput, cmd := UpdateOutput(m.Output, e)
			m.Output = newOutput
			m.Status = StatusError
			return m, cmd
		case msg.MaxTurnsReachedMsg:
			m.MaxTurnsNotice = e.Suggestion
			m.Status = StatusDone // 正常完成（只是到达边界）
			return m, nil
		default:
			newOutput, cmd := UpdateOutput(m.Output, e)
			m.Output = newOutput
			m.Status = StatusResponding
			return m, cmd
		}
		return m, nil

	case msg.CollapseToggleMsg:
		if m.currentRound() != nil {
			newAction, _ := UpdateAction(m.currentRound().Action, e)
			m.currentRound().Action = newAction
		}
		return m, nil

	case msg.SessionDoneMsg:
		m.Status = StatusDone
		for i := range m.Rounds {
			m.Rounds[i].Action.Completed = true
		}
		return m, nil

	case msg.TickMsg:
		for i := range m.Rounds {
			newAction, _ := UpdateAction(m.Rounds[i].Action, e)
			m.Rounds[i].Action = newAction
		}
		newThinking, _ := UpdateThinking(m.Thinking, e)
		m.Thinking = newThinking
		return m, nil
	}

	return m, nil
}

func ViewConversation(m Conversation, width int) string {
	questionView := ViewQuestion(m.Question, width)
	thinkingView := ViewThinking(m.Thinking)

	var roundsView strings.Builder
	for _, round := range m.Rounds {
		var tokensSuffix string
		thoughtView := ViewThought(round.ThoughtContent, 0, 0, false, tokensSuffix)
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
	if m.MaxTurnsNotice != "" {
		if b.len() > 0 {
			b.writeString("\n\n")
		}
		b.writeString(viewMaxTurnsNotice(m.MaxTurnsNotice, width))
	}
	return b.String()
}

func viewMaxTurnsNotice(notice string, width int) string {
	yellow := lipgloss.NewStyle().Foreground(style.ThemeYellow).Bold(true)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemeYellow).
		Padding(0, 1).
		Width(width - 4)

	icon := yellow.Render("💡")
	msg := lipgloss.NewStyle().Foreground(style.ThemeYellow).Render(notice)
	return border.Render(icon + " " + msg)
}
