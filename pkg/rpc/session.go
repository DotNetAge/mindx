package rpc

import "encoding/json"

// SessionCreateParams are the params for session.create.
type SessionCreateParams struct {
	Agent      string `json:"agent"`
	ProjectDir string `json:"project_dir,omitempty"`
}

// SessionGetParams are the params for session.get.
type SessionGetParams struct {
	SessionID   string `json:"session_id"`
	IncludeSlid bool   `json:"include_slid,omitempty"` // 为 true 时返回全部消息（含已滑出的历史）
}

// SessionListParams are the params for session.list.
type SessionListParams struct {
	Agent string `json:"agent,omitempty"`
}

// SessionLatestByDirParams are the params for session.latest_by_dir.
type SessionLatestByDirParams struct {
	ProjectDir string `json:"project_dir"`
}

// SessionDeleteParams are the params for session.delete.
type SessionDeleteParams struct {
	SessionID string `json:"session_id"`
}

// SessionMetaParams are the params for session.meta.
type SessionMetaParams struct {
	SessionID string `json:"session_id"`
}

// SessionRenameParams are the params for session.rename（用户编辑会话标题）。
// title 允许为空：置空后回退首条 user 消息自动补录语义。
type SessionRenameParams struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
}

// SessionFileActionParams are the params for session.confirm_files and session.rollback_files.
type SessionFileActionParams struct {
	SessionID string   `json:"session_id"`
	Files     []string `json:"files,omitempty"`
}

// SessionTruncateParams are the params for session.truncate.
type SessionTruncateParams struct {
	SessionID string `json:"session_id"`
}

// SessionContextParams are the params for session.context.
type SessionContextParams struct {
	SessionID string `json:"session_id"`
}

// SessionDeleteRoundParams are the params for session.delete_round.
type SessionDeleteRoundParams struct {
	SessionID string `json:"session_id"`
	MessageID int64  `json:"id"`
}

// SessionCompactParams are the params for session.compact.
// 压缩统一走 LLM 摘要式滑动窗口（TryCompact）；MicroCompact 已整体拆除
// （改写中间消息会破坏 KV 缓存，属于负优化）。
type SessionCompactParams struct {
	SessionID string `json:"session_id"`
}

// ContextWindowUsage is the result of session.context.
// It mirrors goharness/session.ContextWindowUsage.
type ContextWindowUsage struct {
	WindowTokens       int64   `json:"window_tokens"`
	MaxWindowSize      int64   `json:"max_window_size"`
	UsageRatio         float64 `json:"usage_ratio"`
	MessageCount       int     `json:"message_count"`
	Cursor             int     `json:"cursor"`
	ActiveMessageCount int     `json:"active_message_count"`
	TotalActualTokens  int64   `json:"total_actual_tokens"`
	TotalCost          float64 `json:"total_cost"`
}

func (c *Client) SessionCreate(agent, projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.create", SessionCreateParams{
		Agent: agent, ProjectDir: projectDir,
	})
}

func (c *Client) SessionList(agent string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.list", SessionListParams{Agent: agent})
}

func (c *Client) SessionLatestByDir(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.latest_by_dir", SessionLatestByDirParams{ProjectDir: projectDir})
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

// SessionRename 更新会话标题（用户可编辑）。
func (c *Client) SessionRename(sessionID, title string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.rename", SessionRenameParams{SessionID: sessionID, Title: title})
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

func (c *Client) SessionTruncate(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.truncate", SessionTruncateParams{SessionID: sessionID})
}

func (c *Client) SessionContext(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.context", SessionContextParams{SessionID: sessionID})
}
