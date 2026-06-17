package svc

import (
	"context"
	"encoding/json"
	"fmt"

	goharnesssession "github.com/DotNetAge/goharness/session"
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
		filtered := make([]goharnesssession.SessionInfo, 0)
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

type sessionDeleteParams struct {
	SessionID string `json:"session_id"`
}

func (d *Daemon) handleSessionDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionDeleteParams
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

	// Capture project dir and agent from session meta before deleting,
	// so we can remove the directory from the file watchlist.
	var projectDir, agentName string
	meta, metaErr := sessDB.GetSessionMeta(p.SessionID)
	if metaErr == nil && meta != nil {
		projectDir = meta.ProjectWorkingDir
		agentName = meta.AgentName
	}

	if err := sessDB.DeleteSession(context.Background(), p.SessionID); err != nil {
		return nil, fmt.Errorf("delete session %q failed: %w", p.SessionID, err)
	}

	// Remove project directory from file watchlist so it stops being auto-indexed.
	if projectDir != "" && d.memoryWatch != nil {
		if err := d.memoryWatch.RemoveWatch(projectDir, agentName); err != nil {
			d.logger.Warn("failed to remove project dir from watchlist",
				"dir", projectDir,
				"agent", agentName,
				"error", err,
			)
		}
	}

	return map[string]any{
		"session_id": p.SessionID,
		"deleted":    true,
	}, nil
}

type sessionCreateParams struct {
	Agent      string `json:"agent"`
	ProjectDir string `json:"project_dir"`
}

func (d *Daemon) handleSessionCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Agent == "" {
		return nil, fmt.Errorf("agent is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	// Pass project_dir as a session option so it gets persisted to session meta
	var opts []goharnesssession.SessionOption
	if p.ProjectDir != "" {
		opts = append(opts, goharnesssession.WithProjectDirOption(p.ProjectDir))
	}

	info, err := sessDB.Create(context.Background(), p.Agent, opts...)
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}

	// Add project directory to file watchlist for auto-indexing (RAG).
	if p.ProjectDir != "" && d.memoryWatch != nil {
		if err := d.memoryWatch.AddWatch(p.ProjectDir, p.Agent); err != nil {
			d.logger.Warn("failed to add project dir to watchlist",
				"dir", p.ProjectDir,
				"agent", p.Agent,
				"error", err,
			)
		}
	}

	return map[string]any{
		"session_id":  info.SessionID,
		"agent_name":  info.AgentName,
		"created_at":  info.CreatedAt,
		"project_dir": info.ProjectDir,
	}, nil
}

type sessionFileActionParams struct {
	SessionID string   `json:"session_id"`
	Files     []string `json:"files,omitempty"`
}

// getOrLoadSession 尝试从 activeSessions 获取 session，
// 如果会话已结束（goroutine 已退出），则从持久化存储重建。
// 如果存储不可用或 session 在磁盘上也不存在，则创建一个空 session
// 兜底（后续 ConfirmModify/Rollback 会返回空列表而非报错）。
func (d *Daemon) getOrLoadSession(sessionID string) (*goharnesssession.Session, error) {
	val, ok := d.activeSessions.Load(sessionID)
	if ok {
		return val.(*goharnesssession.Session), nil
	}

	sess := goharnesssession.NewSession(sessionID, "")
	sessDB := d.app.SessDB()
	if sessDB != nil {
		// 从持久化存储重建 session，触发 lazy-load 以恢复 modifyFiles
		sess = goharnesssession.NewSession(sessionID, "",
			goharnesssession.WithStore(sessDB),
		)
		// 触发懒加载：加载历史消息和 modify_files
		//
		// 即使加载失败（如 session 目录已不存在），ensureLoaded 内部也会
		// 标记 loaded=true 并静默返回空 session，不会阻塞流程。
		sess.All()
	}

	return sess, nil
}

func (d *Daemon) handleSessionConfirmFiles(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionFileActionParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, err
	}

	confirmed, err := sess.ConfirmModify(p.Files...)
	if err != nil {
		return nil, fmt.Errorf("confirm files failed: %w", err)
	}

	return map[string]any{
		"session_id": p.SessionID,
		"confirmed":  confirmed,
	}, nil
}

func (d *Daemon) handleSessionRollbackFiles(_ context.Context, params json.RawMessage) (any, error) {
	var p sessionFileActionParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, err
	}

	rolledBack, err := sess.Rollback(p.Files...)
	if err != nil {
		return nil, fmt.Errorf("rollback files failed: %w", err)
	}

	return map[string]any{
		"session_id":  p.SessionID,
		"rolled_back": rolledBack,
	}, nil
}

type sessionTruncateParams struct {
	SessionID string `json:"session_id"`
}

func (d *Daemon) handleSessionTruncate(ctx context.Context, params json.RawMessage) (any, error) {
	var p sessionTruncateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q failed: %w", p.SessionID, err)
	}

	// Find the last user message — we truncate everything after it
	msgs := sess.All()
	lastUserIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return nil, fmt.Errorf("no user message found to truncate at")
	}

	if err := sess.Truncate(ctx, lastUserIdx); err != nil {
		return nil, fmt.Errorf("truncate session %q failed: %w", p.SessionID, err)
	}

	d.logger.Info("session truncated for retry",
		"session_id", p.SessionID,
		"messages_kept", lastUserIdx,
	)

	return map[string]any{
		"session_id":    p.SessionID,
		"messages_kept": lastUserIdx,
		"truncated":     true,
	}, nil
}
