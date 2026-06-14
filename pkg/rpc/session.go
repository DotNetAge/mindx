package rpc

import "encoding/json"

// SessionCreateParams are the params for session.create.
type SessionCreateParams struct {
	Agent      string `json:"agent"`
	ProjectDir string `json:"project_dir,omitempty"`
}

// SessionGetParams are the params for session.get.
type SessionGetParams struct {
	SessionID string `json:"session_id"`
}

// SessionListParams are the params for session.list.
type SessionListParams struct {
	Agent string `json:"agent,omitempty"`
}

// SessionDeleteParams are the params for session.delete.
type SessionDeleteParams struct {
	SessionID string `json:"session_id"`
}

// SessionMetaParams are the params for session.meta.
type SessionMetaParams struct {
	SessionID string `json:"session_id"`
}

// SessionFileActionParams are the params for session.confirm_files and session.rollback_files.
type SessionFileActionParams struct {
	SessionID string   `json:"session_id"`
	Files     []string `json:"files,omitempty"`
}

func (c *Client) SessionCreate(agent, projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.create", SessionCreateParams{
		Agent: agent, ProjectDir: projectDir,
	})
}

func (c *Client) SessionList(agent string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.list", SessionListParams{Agent: agent})
}

func (c *Client) SessionGet(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.get", SessionGetParams{SessionID: sessionID})
}

func (c *Client) SessionDelete(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.delete", SessionDeleteParams{SessionID: sessionID})
}

func (c *Client) SessionMeta(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.meta", SessionMetaParams{SessionID: sessionID})
}

func (c *Client) SessionConfirmFiles(sessionID string, files []string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.confirm_files", SessionFileActionParams{
		SessionID: sessionID, Files: files,
	})
}

func (c *Client) SessionRollbackFiles(sessionID string, files []string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.rollback_files", SessionFileActionParams{
		SessionID: sessionID, Files: files,
	})
}
