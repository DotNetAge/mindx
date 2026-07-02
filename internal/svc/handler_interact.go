package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gort/pkg/gateway"
)

type executionResumeParams struct {
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	ClientID  string `json:"client_id,omitempty"`
}

// handleExecutionResume is a deprecated no-op kept for backward compatibility
// with older clients that still call execution.resume to "cache" a grant.
//
// In the current permission architecture, permission resumption is driven
// entirely by the PermissionAllow / PermissionDeny magic words — when the UI
// resends the last user message (which now contains the magic word), the
// runtime detects it via resolvePermissionMagicWord, drains
// session.PendingPermission, and either runs the tool (Allow) or appends a
// "Permission Denied" result (Deny). No server-side grant cache is needed.
//
// See goharness/agents/runtime.go (resolvePermissionMagicWord) and
// goharness/session/session.go (PendingPermission).
func (d *Daemon) handleExecutionResume(ctx context.Context, raw json.RawMessage) (any, error) {
	var p executionResumeParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid execution.resume params: %w", err)
	}

	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" {
		clientID = p.ClientID
	}

	d.logger.Info("execution.resume: deprecated no-op (use PermissionAllow magic word)",
		"client_id", clientID,
		"session_id", p.SessionID,
		"tool", p.ToolName,
	)

	return map[string]any{
		"status":     "ok",
		"deprecated": true,
		"session_id": p.SessionID,
	}, nil
}

type askUserReplyParams struct {
	CorrelationID string            `json:"correlation_id"`
	Answers       map[string]string `json:"answers"`
}

func (d *Daemon) handleAskUserReply(_ context.Context, raw json.RawMessage) (any, error) {
	var p askUserReplyParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid ask_user.reply params: %w", err)
	}

	d.interactMu.Lock()
	interact, ok := d.pendingInteractions[p.CorrelationID]
	if ok {
		delete(d.pendingInteractions, p.CorrelationID)
	}
	d.interactMu.Unlock()

	if !ok {
		return nil, fmt.Errorf("pending ask_user not found: %s", p.CorrelationID)
	}
	if interact.replyFn == nil {
		return nil, fmt.Errorf("ask_user has no reply function: %s", p.CorrelationID)
	}

	interact.replyFn(p.Answers)
	return map[string]string{"status": "ok"}, nil
}

type permissionReplyParams struct {
	CorrelationID string         `json:"correlation_id"`
	Action        string         `json:"action"`
	Params        map[string]any `json:"params,omitempty"`
	Reason        string         `json:"reason,omitempty"`
}

func (d *Daemon) handlePermissionReply(_ context.Context, raw json.RawMessage) (any, error) {
	var p permissionReplyParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid permission.reply params: %w", err)
	}

	d.interactMu.Lock()
	interact, ok := d.pendingInteractions[p.CorrelationID]
	if ok {
		delete(d.pendingInteractions, p.CorrelationID)
	}
	d.interactMu.Unlock()

	if !ok {
		return nil, fmt.Errorf("pending permission not found: %s", p.CorrelationID)
	}

	switch p.Action {
	case "grant":
		if interact.grantFn == nil {
			return nil, fmt.Errorf("permission has no grant function: %s", p.CorrelationID)
		}
		interact.grantFn(p.Params)
	case "deny":
		if interact.denyFn == nil {
			return nil, fmt.Errorf("permission has no deny function: %s", p.CorrelationID)
		}
		reason := p.Reason
		if reason == "" {
			reason = "user denied"
		}
		interact.denyFn(reason)
	default:
		return nil, fmt.Errorf("invalid permission action: %s (must be 'grant' or 'deny')", p.Action)
	}

	return map[string]string{"status": "ok"}, nil
}

func (d *Daemon) handleMessageCancel(_ context.Context, _ json.RawMessage) (any, error) {
	d.logger.Info("message.cancel called, cancelling all running executions")
	d.clientCancels.Range(func(key, value any) bool {
		if cancel, ok := value.(context.CancelFunc); ok {
			cancel()
		}
		d.clientCancels.Delete(key)
		return true
	})
	return map[string]string{"status": "ok"}, nil
}
