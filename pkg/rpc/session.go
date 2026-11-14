package rpc

import "encoding/json"

// SessionCreateParams are the params for session.create.
type SessionCreateParams struct {
	Agent      string `json:"agent"`
	ProjectDir string `json:"project_dir,omitempty"`
	// Team 团队会话绑定的团队名（空 = 个人会话）。会话级事实：创建时写入并随
	// meta 持久化，重载后 System Prompt 的团队段重建与原会话一致。
	// 校验：团队存在、已雇佣、Leader == Agent。
	Team string `json:"team,omitempty"`
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
	// MemoryOnly 为 true 时只把活跃窗口沉淀为记忆，不移动 cursor。
	// 上下文窗口原封不动，因此不产生「清窗口」带来的前缀缓存失效，
	// 也不受 ForceCompact 的 100K 下限约束。
	MemoryOnly bool `json:"memory_only,omitempty"`
}

// SessionRepairParams are the params for session.repair_pairing.
type SessionRepairParams struct {
	SessionID string `json:"session_id"`
	// DryRun 为 true 时只检测不断言，用于修复前预览。
	DryRun bool `json:"dry_run,omitempty"`
}

// SessionRepairResult 是 session.repair_pairing 的结果。
type SessionRepairResult struct {
	SessionID string `json:"session_id"`
	// BreakAt 是断裂在活跃窗口内的索引；-1 表示配对完整。
	BreakAt int `json:"break_at"`
	// Repaired 表示本次是否执行了修复。
	Repaired bool `json:"repaired"`
	// RemovedToolCalls 是随坏轮次回滚的 tool_call 数量。
	RemovedToolCalls int `json:"removed_tool_calls"`
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

func (c *Client) SessionCreate(agent, projectDir string, team ...string) (json.RawMessage, error) {
	params := SessionCreateParams{Agent: agent, ProjectDir: projectDir}
	if len(team) > 0 {
		params.Team = team[0]
	}
	return c.CallWithTimeout("session.create", params)
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

// SessionCompact 手动触发一次上下文压缩（LLM 摘要式滑动窗口）。
//
// 服务端走 ForceCompact，不校验 needsCompaction()，即使远未达 80% 阈值也会执行。
// 压缩本身会真实调用一次 LLM 生成摘要（消耗 token），且 cursor 前移不可逆
// （session.yml 的完整历史仍保留，只影响活跃窗口），调用方应先展示占用再决定。
func (c *Client) SessionCompact(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.compact", SessionCompactParams{SessionID: sessionID})
}

// SessionExtractMemory 只提取记忆，不压缩上下文窗口。
//
// 与 SessionCompact 的差别：压缩是「提取记忆 + 清空窗口」的一体化操作，
// 清窗口会让后续几轮的前缀缓存全部失效；本方法只做前一半，把会话里的
// 决策/踩坑/路径沉淀成可检索记忆，窗口保持不变。
func (c *Client) SessionExtractMemory(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("session.compact", SessionCompactParams{
		SessionID:  sessionID,
		MemoryOnly: true,
	})
}

// SessionRepairPairing 检查并修复会话窗口的工具调用配对断裂。
//
// 断裂成因通常是并行工具调用返回图片时，图片消息被插在 tool 响应之间，
// 使 assistant 声明的多个 tool_calls 未被连续回应。这类会话上的任何整窗
// 请求（含压缩/记忆提取）都会被接口以 400 拒绝。
// 修复只丢弃坏轮次的 assistant/tool，用户消息会原样回补。
func (c *Client) SessionRepairPairing(sessionID string, dryRun bool) (json.RawMessage, error) {
	return c.CallWithTimeout("session.repair_pairing", SessionRepairParams{
		SessionID: sessionID,
		DryRun:    dryRun,
	})
}
