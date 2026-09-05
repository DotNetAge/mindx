package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/goharness/agents"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
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

// handleSessionLatestByDir 按工作目录过滤所有会话，返回该目录下最近活跃的 0-1 个会话。
// 用于「打开工作目录」流程：客户端据此跨 Agent 定位该目录最近使用的会话，
// 并切换到该会话对应的 Agent。
func (d *Daemon) handleSessionLatestByDir(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SessionLatestByDirParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	sessions, err := goharnesssession.ListSessions(context.Background(), sessDB)
	if err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}

	dir := TrimLastSlash(p.ProjectDir)
	var latest *goharnesssession.SessionInfo
	for i := range sessions {
		if TrimLastSlash(sessions[i].ProjectDir) != dir {
			continue
		}
		if latest == nil || sessions[i].LastActivityAt.After(latest.LastActivityAt) {
			latest = &sessions[i]
		}
	}

	// 目录下无会话：返回 null，客户端据此只清空对话流、不自动新建
	if latest == nil {
		return nil, nil
	}
	return latest, nil
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
	if sess == nil {
		return nil, fmt.Errorf("get session %q failed: session is nil", p.SessionID)
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

	// 待确认修改文件：读取会话追踪的修改文件列表（重载后仍持久化，见 goharness
	// Session.SaveModifyFiles），结合 backup 计算 diff 供前端 FileReviewBar 恢复显示。
	modFiles := sess.GetModifyFiles()
	fileInfos := make([]fileDiffInfo, 0, len(modFiles))
	for _, fp := range modFiles {
		info := computeFileDiff(sess, fp)
		// 仅返回可用的文件：diff 非空（有真实修改）或确为新文件；
		// 当前文件已被删除时 computeFileDiff 返回空 diff 且非新文件，跳过。
		if info.Diff != "" || info.IsNew {
			fileInfos = append(fileInfos, info)
		}
	}

	return map[string]any{
		"session_id":   p.SessionID,
		"messages":     d.enrichMessages(messages),
		"meta":         meta,
		"modify_files": fileInfos,
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

// buildSessionPricing 构建当前模型的定价信息。
// 与 resolveSessionModelContextLength 一致：动态解析当前默认模型，
// 用户切换模型后立即生效，避免使用启动时固化的模型名导致价格过期或为空。
// 模型不可用或未配置价格时，回退到默认价格（DefaultInputCost/DefaultOutputCost）。
func (d *Daemon) buildSessionPricing() goharnesssession.PricingUnit {
	per1MIn, per1MOut, per1MInCache := core.DefaultInputCost, core.DefaultOutputCost, 0.0
	if d.app != nil {
		if modelCfg := d.app.ResolveDefaultModel(); modelCfg != nil {
			per1MIn, per1MOut, per1MInCache = modelCfg.CostPer1MIn, modelCfg.CostPer1MOut, modelCfg.CostPer1MInCache
		}
	}
	return goharnesssession.PricingUnit{
		InputPricePer1M:  per1MIn,
		OutputPricePer1M: per1MOut,
		CachePricePer1M:  per1MInCache,
	}
}

// resolveSessionModelContextLength 返回当前默认模型的上下文窗口大小（ContextLength）。
// 作为 modelContextResolver 回调注入到 session，每次需要窗口大小时动态调用，
// 保证用户切换模型后窗口大小立即更新——窗口大小是模型能力的函数，不是会话的固定属性。
//
// 模型未配置或 ContextLength <= 0 时返回 0，禁用自动压缩。
// 直接委托给 App.ModelContextLength，避免逻辑重复。
func (d *Daemon) resolveSessionModelContextLength() int64 {
	if d.app == nil {
		return 0
	}
	return d.app.ModelContextLength()
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

	d.logger.Info("session.delete: called", "session_id", p.SessionID)

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	if err := goharnesssession.DeleteSession(context.Background(), sessDB, p.SessionID); err != nil {
		return nil, fmt.Errorf("delete session %q failed: %w", p.SessionID, err)
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

	// 雇佣校验：会话可用入口统一走雇佣视图，只有已雇佣的 Agent 才能创建会话
	if agents := d.app.Agents(); agents != nil {
		cfg := agents.Get(p.Agent)
		if cfg == nil {
			return nil, fmt.Errorf("未找到智能体 %q，请确认名称是否正确", p.Agent)
		}
		if !core.AgentIsHired(cfg) {
			return nil, fmt.Errorf("智能体 %q 尚未雇佣，无法创建会话（可执行 mindx agent hire %s 启用）", p.Agent, p.Agent)
		}
	}

	d.logger.Info("session.create: called",
		"agent", p.Agent,
		"project_dir", p.ProjectDir,
	)

	sessDB := d.app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("session store not available")
	}

	// 允许同一 (agent, project_dir) 存在多个会话（1:N）：
	// 前端切换智能体时按「当前目录下最新会话」复用，无则新建。

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
	// resolver 必须注入到每个 session（包括从持久化加载的），
	// 否则 ModelContextLength() 返回 0，窗口大小永远是 0。
	resolverOpt := goharnesssession.WithModelContextResolver(d.resolveSessionModelContextLength)
	if sessDB != nil {
		// Try to load existing session from persistent store.
		var loadErr error
		sess, loadErr = goharnesssession.Load(context.Background(), sessionID, "", sessDB, d.logger, resolverOpt)
		if loadErr != nil {
			// 会话在存储中不存在：回退创建空会话，
			// 让 ConfirmModify/Rollback 等返回空列表而非错误。
			// New 要求 agentName/projectDir 非空且 store 非 nil，不能传空参数。
			sess, loadErr = goharnesssession.New(
				"fallback:"+sessionID, // agentName 必须非空
				"",
				"fallback:"+sessionID, // projectDir 必须非空
				sessDB, d.logger, resolverOpt,
			)
			if loadErr != nil {
				return nil, fmt.Errorf("创建空会话 %q 失败: %w", sessionID, loadErr)
			}
			return sess, nil
		}
		// Trigger lazy-load to restore messages and modify_files.
		sess.All()
	} else {
		return nil, fmt.Errorf("无法创建会话 %q: SessionStore 未配置", sessionID)
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

	// ContextUsage 已通过 modelContextResolver 回调动态获取当前模型的 ContextLength，
	// 无需 fallback —— resolver 在 session 创建时注入，每次调用都读取最新的默认模型配置。
	usage := sess.ContextUsage(pricing)

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

	// 窗口大小已通过 modelContextResolver 回调动态获取，无需手动 SetMaxWindowSize。
	// 绑定 RAG 记忆存储，使压缩摘要持久化到 RAG indexer（浏览器可读）
	if d.sharedMemory != nil {
		sess.SetMemory(mindxses.NewRAGMemoryAdapter(d.sharedMemory, sess.AgentName(), sess.ProjectDir()))
	}

	// 补充装配 Compactor：活跃会话由 rt.SessionConfigs() 自动装配（daemon.go 主流程），
	// 但非活跃会话经 getOrLoadSession 持久化加载时只有 resolver 注入，compactor 为 nil。
	// 缺失时 ForceCompact 走「未配置压缩器」分支，会静默移动 cursor 而不生成任何记忆
	// 分块——曾致压缩完成后零记忆落盘（假成功）。此处与 SetMemory 一样补齐装配。
	if rt, rtErr := d.app.ResolveRuntime(sess.AgentName()); rtErr == nil && rt != nil {
		sess.SetCompactor(agents.NewCompactor(rt))
	}

	d.logger.Info("session.compact: triggered",
		"session_id", p.SessionID,
		"mode", p.Mode,
		"max_window_size", sess.ModelContextLength(),
		"has_model", d.app.ResolveDefaultModel() != nil,
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
					"max_window_size": sess.ModelContextLength(),
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
					"max_window_size": sess.ModelContextLength(),
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
