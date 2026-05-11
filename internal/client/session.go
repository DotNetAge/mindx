package client

import (
	tea "charm.land/bubbletea/v2"
)

type sessionRegistry struct {
	answers map[string]*AgentAnswer
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		answers: make(map[string]*AgentAnswer),
	}
}

func (r *sessionRegistry) add(sessionID string, answer *AgentAnswer) {
	r.answers[sessionID] = answer
}

func (r *sessionRegistry) remove(sessionID string) {
	delete(r.answers, sessionID)
}

func (r *sessionRegistry) get(sessionID string) *AgentAnswer {
	return r.answers[sessionID]
}

func (r *sessionRegistry) count() int {
	return len(r.answers)
}

func (r *sessionRegistry) clear() {
	r.answers = make(map[string]*AgentAnswer)
}

func trySend(ch chan<- tea.Msg, msg tea.Msg) bool {
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}

func waitEvent(outputCh <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-outputCh
	}
}
