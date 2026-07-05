package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

func (d *Daemon) handleSessionList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionListParams
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

func (d *Daemon) handleSessionGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionGetParams
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
	var p rpc.SessionGetParams
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

func (d *Daemon) handleSessionDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionDeleteParams
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

	// Capture project dir from session meta before deleting,
	// so we can clean up the index state.
	var projectDir string
	meta, metaErr := sessDB.GetSessionMeta(p.SessionID)
	if metaErr == nil && meta != nil {
		projectDir = meta.ProjectWorkingDir
	}

	if err := sessDB.DeleteSession(context.Background(), p.SessionID); err != nil {
		return nil, fmt.Errorf("delete session %q failed: %w", p.SessionID, err)
	}

	// Clean up index state for this session's project dir
	if projectDir != "" && d.indexStateStore != nil {
		if absDir, absErr := filepath.Abs(projectDir); absErr == nil {
			d.indexStateStore.Remove(absDir)
			d.logger.Info("session.delete: removed index state for session",
				"session_id", p.SessionID,
				"project_dir", projectDir,
			)
		}
	}

	return map[string]any{
		"session_id": p.SessionID,
		"deleted":    true,
	}, nil
}

func (d *Daemon) handleSessionCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Agent == "" {
		return nil, fmt.Errorf("agent is required")
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	d.logger.Info("session.create: called",
		"agent", p.Agent,
		"project_dir", p.ProjectDir,
		"kbWatch_available", d.kbWatch != nil,
	)

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	// Design rule: only one session per (agent, project_dir) pair is allowed.
	existingSessions, err := sessDB.ListSessions(context.Background())
	if err == nil {
		for _, s := range existingSessions {
			if s.AgentName == p.Agent && sameDirectory(s.ProjectDir, p.ProjectDir) {
				d.logger.Warn("session.create: duplicate session rejected",
					"agent", p.Agent,
					"project_dir", p.ProjectDir,
					"existing_session_id", s.SessionID,
				)
				return nil, fmt.Errorf("duplicate session: agent %q already has a session for directory %q",
					p.Agent, p.ProjectDir)
			}
		}
	}

	// Pass project_dir as a session option so it gets persisted to session meta.
	opts := []goharnesssession.SessionOption{
		goharnesssession.WithProjectDirOption(p.ProjectDir),
	}
	info, err := sessDB.Create(context.Background(), p.Agent, opts...)
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}

	d.logger.Info("session.create: new session created (manual indexing mode)",
		"session_id", info.SessionID,
		"project_dir", info.ProjectDir,
	)

	// Note: Auto-indexing is disabled. Users add files to the index manifest
	// manually via the File Explorer (clicking the cloud icon next to each file).
	// Indexing is triggered via kb.manifest.start/stop per-session.

	return map[string]any{
		"session_id":  info.SessionID,
		"agent_name":  info.AgentName,
		"created_at":  info.CreatedAt,
		"project_dir": info.ProjectDir,
	}, nil
}

// sameDirectory compares two directory paths after normalization.
func sameDirectory(dir1, dir2 string) bool {
	abs1, err1 := filepath.Abs(dir1)
	abs2, err2 := filepath.Abs(dir2)
	if err1 != nil || err2 != nil {
		return dir1 == dir2
	}
	return abs1 == abs2
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

	var sess *goharnesssession.Session
	sessDB := d.app.SessDB()
	if sessDB != nil {
		// Try to load existing session from persistent store.
		var loadErr error
		sess, loadErr = goharnesssession.Load(sessionID, "", sessDB)
		if loadErr != nil {
			// Session not found in store — create empty session as fallback
			// so ConfirmModify/Rollback return empty lists instead of errors.
			sess = goharnesssession.NewSession(sessionID, "")
			return sess, nil
		}
		// Trigger lazy-load to restore messages and modify_files.
		sess.All()
	} else {
		sess = goharnesssession.NewSession(sessionID, "")
	}

	return sess, nil
}

func (d *Daemon) handleSessionConfirmFiles(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionFileActionParams
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
	var p rpc.SessionFileActionParams
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

func (d *Daemon) handleSessionTruncate(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionTruncateParams
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
