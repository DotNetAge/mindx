package svc

import (
	"context"
	"encoding/json"
	"fmt"

	goreactcore "github.com/DotNetAge/goreact/core"
)

type sessionListParams struct {
	Agent string `json:"agent,omitempty"`
}

func (d *Daemon) handleSessionList(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	sessions, err := sessDB.ListSessions(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}

	if p.Agent != "" {
		filtered := make([]goreactcore.SessionInfo, 0)
		for i := range sessions {
			if sessions[i].AgentName == p.Agent {
				filtered = append(filtered, sessions[i])
			}
		}
		sessions = filtered
	}

	return sessions, nil
}

type sessionGetParams struct {
	SessionID string `json:"session_id"`
}

func (d *Daemon) handleSessionGet(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	info, err := sessDB.Get(context.Background(), p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q failed: %w", p.SessionID, err)
	}

	meta, metaErr := sessDB.GetSessionMeta(p.SessionID)

	result := map[string]any{
		"session_id": p.SessionID,
		"messages":   info,
	}
	if metaErr == nil && meta != nil {
		result["meta"] = meta
	}
	return result, nil
}

func (d *Daemon) handleSessionMeta(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	meta, err := sessDB.GetSessionMeta(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session meta %q failed: %w", p.SessionID, err)
	}

	return meta, nil
}
