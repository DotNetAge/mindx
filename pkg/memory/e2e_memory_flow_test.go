package memory_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goharnesssession "github.com/DotNetAge/goharness/session"
	goraglogging "github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/memory"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// TestE2E_CompactionMemoryPersistRecall 对「压缩 → 记忆落盘 → 记忆召回」整条路径做独立端到端测试。
//
// 与当前系统使用完全一致的配置：
//   - 配置来源：~/.mindx/mindx.json + settings/models.yml + settings/providers.yml（core.DefaultApp）
//   - 默认模型：THUDM/GLM-Z1-9B-0414（siliconflow，context_length 131072）
//   - Embedder：~/.mindx/data/models/model.onnx（BGE 512 维）
//   - 会话记录：~/.mindx/sessions/ 下的真实会话
//   - 记忆库：~/.mindx/memory/shared/vectors/shared.db
//   - 记忆装配：SetMemory(RAGMemoryAdapter)，与 handler_session.go 一致
//
// 安全策略：
//   - 通过环境变量 MINDX_E2E=1 显式启用（会调用真实 LLM、消耗 token、读写真实记忆库）
//   - 可通过 MINDX_E2E_SESSION_ID 指定目标会话
//   - 若 daemon 正在运行，为避免 bbolt 文件锁冲突，自动改用 shared.db 副本库
//
// 验证点：
//  1. 压缩生效：ForceCompact 后 cursor 移动到消息末尾
//  2. 记忆落盘：该会话在向量库中出现压缩时间点之后的新记录
//  3. 记忆召回：RetrieveBySession / Retrieve 能读回新记录，三段式字段（title/summary/content）完整
func TestE2E_CompactionMemoryPersistRecall(t *testing.T) {
	if os.Getenv("MINDX_E2E") != "1" {
		t.Skip("设置 MINDX_E2E=1 以运行端到端测试（会调用真实 LLM 并读写 ~/.mindx 记忆库）")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("获取用户主目录失败: %v", err)
	}
	workspace := filepath.Join(home, ".mindx")

	// ── 1. 加载与当前系统一致的配置 ─────────────────────────────
	cfg, err := core.LoadMindxConfig(workspace)
	if err != nil {
		t.Fatalf("加载 mindx 配置失败: %v", err)
	}
	app, err := core.DefaultApp(cfg)
	if err != nil {
		t.Fatalf("构建应用失败: %v", err)
	}
	t.Logf("配置文件: %s", filepath.Join(workspace, "mindx.json"))
	t.Logf("默认模型: %s (context_length=%d)", cfg.LastModel, app.ModelContextLength())

	emb := app.Embedder()
	if emb == nil {
		t.Skip("当前配置未启用 embedder（model.onnx 不可用），无法测试记忆链路")
	}
	t.Logf("embedder: 维度=%d", emb.Dim())

	modelCfg := app.ResolveDefaultModel()
	if modelCfg == nil || modelCfg.Name == "" {
		t.Skip("当前配置未解析到默认模型，无法测试压缩链路")
	}

	// ── 2. 确定目标会话（优先环境变量，否则自动挑选活跃窗口最大的真实会话）──
	// 压缩是一次性操作：压缩后 cursor 移到末尾，活跃窗口变空，无法重复触发。
	// 因此测试必须自动选择一个仍有活跃消息的会话，保证可重复运行。
	sess, err := pickE2ETargetSession(ctx, app, os.Getenv("MINDX_E2E_SESSION_ID"))
	if err != nil {
		t.Fatalf("选择目标会话失败: %v", err)
	}
	if sess == nil {
		t.Skip("所有真实会话的活跃窗口均为空（均已压缩或没有新消息），无法触发压缩")
	}
	sessionID := sess.ID()

	// ── 3. 构建 RAGMemory（真实库；daemon 运行中则用副本避免 bbolt 锁冲突）──
	memoryDir := filepath.Join(workspace, "memory")
	if isMindxDaemonRunning() {
		t.Log("检测到 daemon 正在运行，使用 shared.db 副本构建测试库，避免 bbolt 文件锁冲突")
		memoryDir = t.TempDir()
		if err := copySharedDB(filepath.Join(workspace, "memory"), memoryDir); err != nil {
			t.Fatalf("复制共享记忆库失败: %v", err)
		}
	}
	rag, err := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
		AgentName: "_shared",
		MemoryDir: memoryDir,
		Embedder:  emb,
		Logger:    goraglogging.DefaultConsoleLogger(),
	})
	if err != nil {
		t.Fatalf("构建 RAGMemory 失败: %v", err)
	}
	defer rag.Close(ctx)
	t.Logf("记忆库: %s", filepath.Join(memoryDir, "shared", "vectors", "shared.db"))

	// ── 4. 目标会话已由 pickE2ETargetSession 加载完成 ──────────
	t.Logf("会话 %s: agent=%s project=%s", sessionID, sess.AgentName(), sess.ProjectDir())

	tokens := sess.CurrentWindowTokens()
	active := sess.Current()
	t.Logf("会话窗口: tokens=%d, 活跃消息=%d, 模型上限=%d", tokens, len(active), app.ModelContextLength())

	// ── 5. 注入记忆存储（对齐 handler_session.go）──
	// 压缩链路由 Compactor（agents.NewCompactor）驱动，Summarizer 概念已移除。
	// mindx 主代码当前未装配 Compactor，此 e2e 测试的压缩摘要部分需待主代码装配后完整验证。
	sess.SetMemory(mindxses.NewRAGMemoryAdapter(rag, sess.AgentName(), sess.ProjectDir()))
	t.Logf("已注入 RAG 记忆存储（模型 %s）", modelCfg.Name)

	// ── 6. 压缩前基线 ──────────────────────────────────────────
	before := sess.ContextUsage()
	beforeMem, err := rag.RetrieveBySession(ctx, sessionID, 100)
	if err != nil {
		t.Fatalf("压缩前召回会话记忆失败: %v", err)
	}
	t.Logf("压缩前: cursor=%d, 窗口 tokens=%d, 该会话已有记忆=%d 条",
		before.Cursor, before.WindowTokens, len(beforeMem))

	// ── 7. 触发压缩（真实 LLM 摘要，耗时可能数分钟）──────────────
	compactStart := time.Now()
	t.Logf("开始 ForceCompact（真实 LLM 摘要）...")
	sess.ForceCompact(ctx)
	after := sess.ContextUsage()
	t.Logf("压缩耗时: %s", time.Since(compactStart).Round(time.Second))
	t.Logf("压缩后: cursor=%d, 窗口 tokens=%d", after.Cursor, after.WindowTokens)

	// 验证 1：压缩生效 —— cursor 必须移动
	if after.Cursor <= before.Cursor {
		t.Fatalf("压缩未生效: cursor 未移动（before=%d after=%d）", before.Cursor, after.Cursor)
	}

	// ── 8. 验证 2：记忆落盘 ────────────────────────────────────
	afterMem, err := rag.RetrieveBySession(ctx, sessionID, 100)
	if err != nil {
		t.Fatalf("压缩后召回会话记忆失败: %v", err)
	}
	t.Logf("压缩后该会话记忆=%d 条（压缩前 %d 条）", len(afterMem), len(beforeMem))

	// 按 ID 差集判定新记忆：LLM 生成的时间戳可能是会话内的历史事件时间
	// （早于压缩时刻），不能按时间过滤，必须对比压缩前后的 ID 集合。
	beforeIDs := make(map[string]bool, len(beforeMem))
	for _, m := range beforeMem {
		beforeIDs[m.ID] = true
	}
	var newOnes []goharnessmemory.MemoryChunk
	for _, m := range afterMem {
		if !beforeIDs[m.ID] {
			newOnes = append(newOnes, m)
		}
	}
	if len(newOnes) == 0 {
		t.Fatalf("记忆未落盘: 压缩后未发现新 ID 的记忆（压缩前 %d 条，压缩后 %d 条）",
			len(beforeMem), len(afterMem))
	}

	// ── 9. 验证 3：记忆召回 + 三段式字段完整性 ──────────────────
	for i, m := range newOnes {
		t.Logf("新记忆[%d]: ID=%s SessionID=%s", i, m.ID, m.SessionID)
		t.Logf("  title  : %q", m.Title)
		t.Logf("  summary: %q", m.Summary)
		t.Logf("  tags   : %v", m.Tags)
		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "...(截断)"
		}
		t.Logf("  content: %q", content)
		t.Logf("  时间戳 : %s", m.Timestamp.Format("2006-01-02 15:04:05"))

		if m.SessionID != sessionID {
			t.Errorf("新记忆[%d] SessionID=%q，期望 %q", i, m.SessionID, sessionID)
		}
		if m.Content == "" {
			t.Errorf("新记忆[%d] content 为空", i)
		}
		if m.Summary == "" {
			t.Errorf("新记忆[%d] summary 为空（三段式要求 summary 必须有核心结论）", i)
		}
		if m.AgentName == "" {
			t.Errorf("新记忆[%d] agent_name 为空", i)
		}
	}

	// 语义召回验证：用第一条新记忆的 title（回退 summary）作为查询词
	query := newOnes[0].Title
	if query == "" {
		query = newOnes[0].Summary
	}
	if query != "" {
		hits, rErr := rag.Retrieve(ctx, query,
			goharnessmemory.WithMemorySessionID(sessionID),
			goharnessmemory.WithMemoryLimit(5),
		)
		if rErr != nil {
			t.Fatalf("语义召回失败: %v", rErr)
		}
		t.Logf("语义召回（查询 %q）命中 %d 条", query, len(hits))
		for _, h := range hits {
			t.Logf("  命中: title=%q summary=%q", h.Title, h.Summary)
		}
		if len(hits) == 0 {
			t.Errorf("语义召回无结果（查询词 %q）", query)
		}
	}

	t.Logf("端到端链路验证通过: 压缩(⌄ %d 条消息) → 落盘(%d 条新记忆) → 召回(%d 条字段完整)",
		after.Cursor-before.Cursor, len(newOnes), len(newOnes))
}

// isMindxDaemonRunning 检测 mindx daemon 是否正在运行（监听 :1314）。
// daemon 独占 shared.db 的 bbolt 文件锁，运行中时测试必须改用副本库。
func isMindxDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", "localhost:1314", 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// copySharedDB 将真实记忆库的 shared.db 复制到临时目录。
// 源库不存在时返回 nil（测试将使用全新空库）。
func copySharedDB(srcMemoryDir, dstMemoryDir string) error {
	src := filepath.Join(srcMemoryDir, "shared", "vectors", "shared.db")
	dst := filepath.Join(dstMemoryDir, "shared", "vectors", "shared.db")
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// pickE2ETargetSession 挑选端到端测试的目标会话。
//
// 压缩是一次性操作：ForceCompact 成功后 cursor 移到消息末尾，活跃窗口变空，
// 同一会话无法重复触发压缩。因此测试不能固定使用某个会话，必须自动挑选
// 当前仍有活跃窗口的真实会话，保证可重复运行。
//
// 选择策略：
//   - 环境变量 MINDX_E2E_SESSION_ID 指定时，直接使用该会话（由调用方校验窗口）
//   - 未指定时，遍历 ~/.mindx/sessions 下所有真实会话，选择活跃窗口 tokens
//     最大且超过 ForceCompact 100K 阈值的会话；没有满足条件的会话时返回 nil
//
// 返回 nil 表示当前没有可压缩的会话（调用方应 Skip）。
func pickE2ETargetSession(ctx context.Context, app *core.App, forcedID string) (*goharnesssession.Session, error) {
	sessDB := app.SessDB()
	if sessDB == nil {
		return nil, fmt.Errorf("会话存储不可用")
	}

	loadSession := func(id string) (*goharnesssession.Session, error) {
		sess, err := goharnesssession.Load(ctx, id, "", sessDB, app.Logger(),
			goharnesssession.WithModelContextResolver(app.ModelContextLength))
		if err != nil {
			return nil, err
		}
		sess.All() // 触发懒加载，恢复消息与游标
		return sess, nil
	}

	if forcedID != "" {
		return loadSession(forcedID)
	}

	// 自动选择：取活跃窗口 tokens 最大且超过 ForceCompact 阈值的会话
	const forceCompactThreshold int64 = 100_000
	infos, err := goharnesssession.ListSessions(ctx, sessDB)
	if err != nil {
		return nil, fmt.Errorf("列出会话失败: %w", err)
	}

	var best *goharnesssession.Session
	var bestTokens int64
	for _, info := range infos {
		if info.SessionID == "" {
			continue
		}
		sess, err := loadSession(info.SessionID)
		if err != nil {
			continue // 单个会话加载失败不影响整体挑选
		}
		tokens := sess.CurrentWindowTokens()
		if tokens > bestTokens {
			bestTokens = tokens
			best = sess
		}
	}
	if best == nil || bestTokens <= forceCompactThreshold {
		return nil, nil
	}
	return best, nil
}
