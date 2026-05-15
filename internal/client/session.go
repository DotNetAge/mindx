package client

import (
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/mindx/internal/client/data"
	appcore "github.com/DotNetAge/mindx/internal/core"
)

type chatSessionManager struct {
	app *appcore.App
}

func newChatSessionManager(app *appcore.App) *chatSessionManager {
	return &chatSessionManager{app: app}
}

func (m *chatSessionManager) getOrCreateSession(agent *goreact.Agent) (*data.ChatSession, error) {
	sid := agent.SessionID()
	if sid == "" {
		meta, err := m.app.CreateSession(agent.Name())
		if err != nil {
			return nil, err
		}
		return &data.ChatSession{
			SessionID: meta.SessionID,
			AgentName: agent.Name(),
		}, nil
	}
	return &data.ChatSession{
		SessionID: sid,
		AgentName: agent.Name(),
	}, nil
}
