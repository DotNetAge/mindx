package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/rpc"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
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

	sessions, err := goharnesssession.ListSessions(context.Background(), sessDB)
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

	// 消息数据：通过 Session 对象获取（Session 是消息的权威来源，必须经过 cursor 过滤）
	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q failed: %w", p.SessionID, err)
	}

	var messages []goharnesssession.Message
	if !p.IncludeSlid {
		messages = sess.Current() // 只返回活跃窗口 messages[cursor:]
	} else {
		messages = sess.All() // 返回全部消息（用于调试/CLI）
	}

	// 元数据：session 级别的元数据（title, project_working_dir, message_count 等），由
	// mindx FileSessionStore 管理（meta.json），不在 goharness Session 对象的职责范围内。
	// 这里直接调用 FileSessionStore 是元数据操作，不是会话数据操作。
	var meta any
	if sessDB := d.app.SessDB(); sessDB != nil {
		if m, err := sessDB.GetSessionMeta(p.SessionID); err == nil {
			meta = m
		}
	}

	return map[string]any{
		"session_id": p.SessionID,
		"messages":   d.enrichMessages(messages),
		"meta":       meta,
	}, nil
}

// enrichMessages enriches each message's token_usage with computed fields
// (actual_tokens, cost) for the frontend, and ensures token_usage is never nil.
func (d *Daemon) enrichMessages(msgs []goharnesssession.Message) []map[string]any {
	// Build pricing from the current model's cost config
	pricing := d.buildSessionPricing()

	// Serialize to JSON then deserialize to maps so we can inject computed fields
	data, _ := json.Marshal(msgs)
	rawMsgs := make([]map[string]any, len(msgs))
	_ = json.Unmarshal(data, &rawMsgs)

	for i, msg := range msgs {
		if msg.Usage != nil {
			rawMsgs[i]["actual_tokens"] = msg.Usage.ActualTokens()
			rawMsgs[i]["cost"] = msg.Usage.Cost(pricing)
		} else {
			// Ensure token_usage is never nil/absent to prevent client-side undefined errors
			rawMsgs[i]["token_usage"] = map[string]any{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
				"cached_tokens":     0,
				"reasoning_tokens":  0,
				"actual_tokens":     0,
				"cost":              0,
			}
		}
	}

	return rawMsgs
}

// buildSessionPricing builds a PricingUnit from the daemon's current model cost config.
// Returns a zero-value PricingUnit if the model cost is not available.
func (d *Daemon) buildSessionPricing() goharnesssession.PricingUnit {
	if d.modelName == "" {
		return goharnesssession.PricingUnit{}
	}
	mc, ok := d.app.Costs().Get(d.modelName)
	if !ok {
		return goharnesssession.PricingUnit{}
	}
	return goharnesssession.PricingUnit{
		InputPricePer1M:  mc.CostPer1MIn,
		OutputPricePer1M: mc.CostPer1MOut,
	}
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
	meta, metaErr := goharnesssession.GetSessionMeta(context.Background(), sessDB, p.SessionID)
	if metaErr == nil && meta != nil {
		projectDir = meta.ProjectDir
	}

	if err := goharnesssession.DeleteSession(context.Background(), sessDB, p.SessionID); err != nil {
		return nil, fmt.Errorf("delete session %q failed: %w", p.SessionID, err)
	}

	// Note: per-session index state cleanup is handled by the Indexer lifecycle.
	_ = projectDir // kept for future use
	if absDir, absErr := filepath.Abs(projectDir); absErr == nil {
		pi, piErr := d.getIndexer(absDir)
		if piErr == nil && pi != nil {
			pi.Stop()
			d.logger.Info("session.delete: stopped indexing for session",
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
		"graph_indexer_available", d.graphIndexer != nil,
	)

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	// Design rule: only one session per (agent, project_dir) pair is allowed.
	existingSessions, err := goharnesssession.ListSessions(context.Background(), sessDB)
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
	info, err := goharnesssession.CreateSession(context.Background(), sessDB, p.Agent, opts...)
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}

	d.logger.Info("session.create: new session created (manual indexing mode)",
		"session_id", info.SessionID,
		"project_dir", info.ProjectDir,
	)

	// Note: Auto-indexing is disabled. Users add files to the index manifest
	// manually via the File Explorer (clicking the cloud icon next to each file).
	// Indexing is triggered via kb.index.start/stop per-session.

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
		sess, loadErr = goharnesssession.Load(context.Background(), sessionID, "", sessDB, d.logger)
		if loadErr != nil {
			// Session not found in store — create empty session as fallback
			// so ConfirmModify/Rollback return empty lists instead of errors.
			sess, _ = goharnesssession.New("", "", "", sessDB, d.logger)
			return sess, nil
		}
		// Trigger lazy-load to restore messages and modify_files.
		sess.All()
	} else {
		sess, _ = goharnesssession.New("", "", "", nil, d.logger)
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

// handleSessionContext returns the current context window usage for a session.
// The calculation is consistent with GoHarness's MicroCompact method (estimateWindowTokensV2).
// It uses the session's maxWindowSize (if set) or falls back to the default model's context_length.
func (d *Daemon) handleSessionContext(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionContextParams
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

	// Build pricing from the current model's cost config
	pricing := d.buildSessionPricing()

	// Get the session's native context usage (uses maxWindowSize if configured)
	usage := sess.ContextUsage(pricing)

	// If the session doesn't have maxWindowSize configured (0), fall back to
	// the default model's context_length, so the ratio is meaningful.
	if usage.MaxWindowSize == 0 {
		modelCfg := d.app.ResolveDefaultModel()
		if modelCfg != nil && modelCfg.ContextLength > 0 {
			usage.MaxWindowSize = modelCfg.ContextLength
			if usage.MaxWindowSize > 0 {
				usage.UsageRatio = float64(usage.WindowTokens) / float64(usage.MaxWindowSize)
			}
		}
	}

	return map[string]any{
		"session_id":           p.SessionID,
		"window_tokens":        usage.WindowTokens,
		"max_window_size":      usage.MaxWindowSize,
		"usage_ratio":          usage.UsageRatio,
		"message_count":        usage.MessageCount,
		"cursor":               usage.Cursor,
		"active_message_count": usage.ActiveMessageCount,
		"total_actual_tokens":  usage.TotalActualTokens,
		"total_cost":           usage.TotalCost,
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

func (d *Daemon) handleSessionDeleteRound(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionDeleteRoundParams
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

	if err := sess.DeleteRound(ctx, p.MessageID); err != nil {
		return nil, fmt.Errorf("delete round failed: %w", err)
	}

	d.logger.Info("session round deleted",
		"session_id", p.SessionID,
		"message_id", p.MessageID,
	)

	return map[string]any{
		"session_id": p.SessionID,
		"message_id": p.MessageID,
		"deleted":    true,
	}, nil
}

func (d *Daemon) handleSessionCompact(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionCompactParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if p.Mode == "" {
		p.Mode = "full"
	}

	sess, err := d.getOrLoadSession(p.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q failed: %w", p.SessionID, err)
	}

	// 确保会话有 maxWindowSize 配置，否则 TryCompact/TryMicroCompact 无法触发
	if sess.MaxWindowSize() <= 0 {
		if modelCfg := d.app.ResolveDefaultModel(); modelCfg != nil && modelCfg.ContextLength > 0 {
			sess.SetMaxWindowSize(modelCfg.ContextLength)
		}
	}

	// 设置 LLM 摘要器，使 TryCompact 能生成语义摘要
	if modelCfg := d.app.ResolveDefaultModel(); modelCfg != nil && modelCfg.Name != "" {
		sess.SetSummarizer(goharnesssession.NewLLMSummarizer(*modelCfg))
	}

	// 绑定 RAG 记忆存储，使压缩摘要持久化到 RAG indexer（浏览器可读）
	if d.sharedMemory != nil {
		sess.SetMemory(mindxses.NewRAGMemoryAdapter(d.sharedMemory, sess.AgentName(), sess.ProjectDir()))
	}

	d.logger.Info("session.compact: triggered",
		"session_id", p.SessionID,
		"mode", p.Mode,
		"max_window_size", sess.MaxWindowSize(),
		"has_summarizer", d.app.ResolveDefaultModel() != nil,
	)

	// 绑定事件处理器，TryCompact/TryMicroCompact 会自动调用它们广播事件
	gw := d.gw
	sid := p.SessionID
	var beforeTokens int64

	sess.SetCompactStartHandler(func(windowTokens, maxWindowSize int64) {
		beforeTokens = windowTokens
		d.logger.Info("[session] compact start",
			"session_id", sid,
			"window_tokens", windowTokens,
		)
		if gw != nil {
			gw.BroadcastNotification("compact_start", map[string]any{
				"session_id": sid,
				"data": map[string]any{
					"window_tokens":   windowTokens,
					"max_window_size": maxWindowSize,
				},
			})
		}
	})

	sess.SetCompactDoneHandler(func(messagesSlid int, windowTokens int64) {
		var ratio float64
		if beforeTokens > 0 {
			ratio = float64(windowTokens) / float64(beforeTokens)
		}
		d.logger.Info("[session] compact done",
			"session_id", sid,
			"messages_slid", messagesSlid,
			"window_tokens", windowTokens,
			"ratio", ratio,
		)
		if gw != nil {
			gw.BroadcastNotification("compact_done", map[string]any{
				"session_id": sid,
				"data": map[string]any{
					"messages_slid":   messagesSlid,
					"window_tokens":   windowTokens,
					"max_window_size": sess.MaxWindowSize(),
					"ratio":           ratio,
				},
			})
		}
	})

	sess.SetMicroCompactStartHandler(func(windowTokens, maxWindowSize int64) {
		beforeTokens = windowTokens
		d.logger.Info("[session] micro-compact start",
			"session_id", sid,
			"window_tokens", windowTokens,
		)
		if gw != nil {
			gw.BroadcastNotification("micro_compact_start", map[string]any{
				"session_id": sid,
				"data": map[string]any{
					"window_tokens":   windowTokens,
					"max_window_size": maxWindowSize,
				},
			})
		}
	})

	sess.SetMicroCompactDoneHandler(func(compressed, deduped int, windowTokens int64) {
		var ratio float64
		if beforeTokens > 0 {
			ratio = float64(windowTokens) / float64(beforeTokens)
		}
		d.logger.Info("[session] micro-compact done",
			"session_id", sid,
			"compressed", compressed,
			"deduped", deduped,
			"window_tokens", windowTokens,
			"ratio", ratio,
		)
		if gw != nil {
			gw.BroadcastNotification("micro_compact_done", map[string]any{
				"session_id": sid,
				"data": map[string]any{
					"compressed":      compressed,
					"deduped":         deduped,
					"window_tokens":   windowTokens,
					"max_window_size": sess.MaxWindowSize(),
					"ratio":           ratio,
				},
			})
		}
	})

	switch p.Mode {
	case "micro":
		performed := sess.TryMicroCompact()
		if !performed {
			d.logger.Info("session.compact: micro compact skipped (below threshold or nothing to compress)",
				"session_id", p.SessionID)
		}
	default:
		// ForceCompact 不检查 needsCompaction()，由前端自行判断按钮可用性
		sess.ForceCompact(ctx)
	}

	// Note: 不清除 compact handler，因为 Runtime（ask loop）会重新设置自己的 handler。
	// 如果这里清除，后续 Runtime 自动压缩时将丢失 CompactStart/CompactDone 事件广播。

	// Return updated context usage after compaction
	usage := sess.ContextUsage(d.buildSessionPricing())

	d.logger.Info("session.compact: done",
		"session_id", p.SessionID,
		"mode", p.Mode,
		"window_tokens", usage.WindowTokens,
		"usage_ratio", usage.UsageRatio,
	)

	return map[string]any{
		"session_id":           p.SessionID,
		"mode":                 p.Mode,
		"window_tokens":        usage.WindowTokens,
		"max_window_size":      usage.MaxWindowSize,
		"usage_ratio":          usage.UsageRatio,
		"message_count":        usage.MessageCount,
		"cursor":               usage.Cursor,
		"active_message_count": usage.ActiveMessageCount,
		"total_actual_tokens":  usage.TotalActualTokens,
		"total_cost":           usage.TotalCost,
	}, nil
}
