package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/DotNetAge/goharness/memory"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/embedder"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/logging"
	querypkg "github.com/DotNetAge/gorag/v2/query"
	"github.com/DotNetAge/gorag/v2/store/vector/govector"
)

var _ memory.Memory = (*RAGMemory)(nil)

// RAGMemory implements memory.Memory using SemanticIndexer for unified memory storage.
// All agents' memories are stored in the same vector store, differentiated by metadata
// fields (agent_name, session_id) for filter-based retrieval.
type RAGMemory struct {
	semantic goragindexer.Indexer // SemanticIndexer（统一记忆存储）
	embedder goragcore.Embedder
	logger   logging.Logger
}

type RAGMemoryOption func(*RAGMemory)

type MemoryConfig struct {
	AgentName string

	MemoryDir string

	Logger logging.Logger

	Embedder goragcore.Embedder
}

func (c MemoryConfig) dataDir() string {
	return filepath.Join(c.MemoryDir, "shared")
}

func NewEmbedderFromConfig(modelPath string) (goragcore.Embedder, error) {
	if modelPath == "" {
		return nil, nil
	}
	emb, err := embedder.NewBGEEmbedder(
		embedder.WithBGEModelFile(modelPath),
		embedder.WithBGEDimension(512),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: 创建 embedder 失败: %w", err)
	}
	return emb, nil
}

// NewRAGMemory 创建一个 RAGMemory 实例。
// semanticIdx 始终非空；graphIdx 可为 nil（仅使用语义检索）。
func NewRAGMemory(semanticIdx goragindexer.Indexer, opts ...RAGMemoryOption) *RAGMemory {
	m := &RAGMemory{
		semantic: semanticIdx,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewRAGMemoryFromConfig(cfg MemoryConfig) (*RAGMemory, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("memory: embedder 是必填项")
	}
	if cfg.AgentName == "" {
		return nil, fmt.Errorf("memory: agent name 是必填项")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("memory: logger 是必填项，用于日志记录")
	}
	if cfg.MemoryDir == "" {
		return nil, fmt.Errorf("memory: memory dir 是必填项")
	}

	dataDir := cfg.dataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("memory: 创建 data目录 %s 失败: %w", dataDir, err)
	}

	logger := cfg.Logger

	// ── SemanticIndexer（统一记忆存储）───────────────────────
	semVecDir := filepath.Join(dataDir, "vectors")
	if mkErr := os.MkdirAll(semVecDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: 创建语义向量目录 %s 失败: %w", semVecDir, mkErr)
	}
	semVS, err := govector.NewStore(
		govector.WithCollection("shared_sem"),
		govector.WithDimension(cfg.Embedder.Dim()),
		govector.WithDBPath(filepath.Join(semVecDir, "shared.db")),
		govector.WithHNSW(true),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: 创建语义向量存储 %s 失败: %w", semVecDir, err)
	}
	semIdx, semErr := goragindexer.NewSemanticIndexer(semVS, cfg.Embedder,
		goragindexer.WithSemanticLogger(logger),
	)
	if semErr != nil {
		return nil, fmt.Errorf("memory: 创建语义索引器失败: %w", semErr)
	}

	m := &RAGMemory{
		semantic: semIdx,
		embedder: cfg.Embedder,
		logger:   logger,
	}

	logger.Info("memory: 初始化完成",
		"agent", cfg.AgentName,
		"vector_dim", cfg.Embedder.Dim(),
	)

	return m, nil
}

func WithEmbedder(embedder goragcore.Embedder) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.embedder = embedder
	}
}

// Semantic 返回 SemanticIndexer，用于统一记忆存储。
func (m *RAGMemory) Semantic() goragindexer.Indexer {
	return m.semantic
}

// StoreMemoryChunks stores memory chunks directly with full Vector metadata for filter-based retrieval.
func (m *RAGMemory) StoreMemoryChunks(ctx context.Context, chunks []memory.MemoryChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	for _, chunk := range chunks {
		if chunk.ID == "" && chunk.Content != "" {
			chunk.ID = contentHash(chunk.Content)
		}
		if err := m.storeMemoryChunk(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// storeMemoryChunk stores a single MemoryChunk with full Vector metadata.
func (m *RAGMemory) storeMemoryChunk(ctx context.Context, chunk memory.MemoryChunk) error {
	content := chunk.Summary
	if chunk.Content != "" {
		content = chunk.Summary + "\n" + chunk.Content
	}

	tagStrs := make([]string, len(chunk.Tags))
	copy(tagStrs, chunk.Tags)

	metadata := map[string]any{
		"agent_name":  chunk.AgentName,
		"session_id":  chunk.SessionID,
		"project_dir": chunk.ProjectDir,
		"summary":     chunk.Summary,
		"tags":        tagStrs,
		"content":     chunk.Content,
		"title":       chunk.Summary,
	}
	if !chunk.Timestamp.IsZero() {
		metadata["timestamp"] = chunk.Timestamp.UnixMilli()
	}

	coreChunk := &goragcore.Chunk{
		ID:       chunk.ID,
		Content:  content,
		Title:    chunk.Summary,
		DocID:    chunk.AgentName,
		Metadata: metadata,
	}

	store, ok := m.semantic.(goragindexer.IndexerStore)
	if !ok {
		return fmt.Errorf("memory: 语义索引器不支持存储操作")
	}
	doc := goragcore.NewStructuredDocFromChunks([]goragcore.Chunk{*coreChunk})
	if err := store.Save(ctx, doc); err != nil {
		return fmt.Errorf("memory: 存储 chunk 失败: %w", err)
	}
	return nil
}

// Retrieve implements memory.Memory.
func (m *RAGMemory) Retrieve(ctx context.Context, query string, opts ...memory.RetrieveOption) ([]memory.MemoryChunk, error) {
	cfg := memory.DefaultRetrieveConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	idx := m.semantic
	if idx == nil {
		return nil, fmt.Errorf("memory: 语义索引器未初始化")
	}

	q := m.buildQueryWithFilter(query, cfg)
	if q == nil {
		return nil, nil
	}

	hitResult, err := idx.Search(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("memory: 检索失败: %w", err)
	}

	if hitResult == nil || len(hitResult.Chunks) == 0 {
		return nil, nil
	}

	var chunks []memory.MemoryChunk
	for _, chunkHit := range hitResult.Chunks {
		chunk := chunkHitToChunk(chunkHit)
		if chunk == nil {
			continue
		}

		if cfg.MinScore > 0 && float64(chunkHit.Score) < cfg.MinScore {
			continue
		}

		chunks = append(chunks, *chunk)
	}

	if cfg.Limit > 0 && len(chunks) > cfg.Limit {
		chunks = chunks[:cfg.Limit]
	}

	return chunks, nil
}

// RetrieveLatest 按时间倒序取出当前 AgentName+ProjectDir 范围内最新的 N 条记忆。
// 不依赖向量检索，通过 Indexer.List 分页拉取所有 chunk，按 metadata 过滤后
// 按 timestamp 倒序取前 limit 条。
//
// 用于 memmache.md 中"记忆缓冲区固定取最新10条"的需求：每次 LLM 调用前
// 取最新记忆拼到系统指令区末尾。
//
// 实现 memory.LatestRetriever 可选接口。
func (m *RAGMemory) RetrieveLatest(ctx context.Context, agentName, projectDir string, limit int) ([]memory.MemoryChunk, error) {
	idx := m.semantic
	if idx == nil {
		return nil, fmt.Errorf("memory: 语义索引器未初始化")
	}
	if limit <= 0 {
		limit = 10
	}
	if ctx == nil {
		ctx = context.Background()
	}

	admin, ok := idx.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("memory: 语义索引器不支持 List 操作")
	}

	// 分页拉取所有 chunks，按 metadata 过滤
	var matched []memory.MemoryChunk
	const pageSize = 200
	offset := 0
	totalHits := 0
	for {
		chunks, total, err := admin.List(ctx, offset, pageSize, nil)
		if err != nil {
			return nil, fmt.Errorf("memory: 检索最新记忆失败: %w", err)
		}
		if len(chunks) == 0 {
			break
		}
		totalHits = total
		for _, c := range chunks {
			chunk := coreChunkToChunk(c)
			if chunk == nil {
				m.logger.Debug("RetrieveLatest: coreChunkToChunk returned nil", "chunk_id", c.ID)
				continue
			}
			m.logger.Debug("RetrieveLatest: chunk metadata",
				"chunk_id", c.ID, "agent_name", chunk.AgentName,
				"project_dir", chunk.ProjectDir, "summary", chunk.Summary)
			// 过滤 agent_name
			if agentName != "" && chunk.AgentName != agentName {
				m.logger.Debug("RetrieveLatest: filtered by agent_name",
					"want", agentName, "got", chunk.AgentName, "chunk_id", c.ID)
				continue
			}
			// 过滤 project_dir
			if projectDir != "" && chunk.ProjectDir != projectDir {
				m.logger.Debug("RetrieveLatest: filtered by project_dir",
					"want", projectDir, "got", chunk.ProjectDir, "chunk_id", c.ID)
				continue
			}
			matched = append(matched, *chunk)
		}
		if len(chunks) < pageSize {
			break
		}
		offset += pageSize
	}
	m.logger.Debug("RetrieveLatest: summary",
		"total_hits", totalHits, "matched", len(matched),
		"agent_name", agentName, "project_dir", projectDir)

	// 按 timestamp 倒序排序（最新在前）
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	// 取前 limit 条
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}

// RetrieveBySession 实现 memory.SessionRetriever 可选接口：按 sessionID 取最新记忆。
// 无视 agentName / projectDir 过滤，作为 RetrieveLatest 的兜底。
func (m *RAGMemory) RetrieveBySession(ctx context.Context, sessionID string, limit int) ([]memory.MemoryChunk, error) {
	idx := m.semantic
	if idx == nil {
		return nil, fmt.Errorf("memory: 语义索引器未初始化")
	}
	if limit <= 0 {
		limit = 10
	}
	if ctx == nil {
		ctx = context.Background()
	}

	admin, ok := idx.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("memory: 语义索引器不支持 List 操作")
	}

	var matched []memory.MemoryChunk
	const pageSize = 200
	offset := 0
	totalHits := 0
	for {
		chunks, total, err := admin.List(ctx, offset, pageSize, nil)
		if err != nil {
			return nil, fmt.Errorf("memory: 检索会话记忆失败: %w", err)
		}
		if len(chunks) == 0 {
			break
		}
		totalHits = total
		for _, c := range chunks {
			chunk := coreChunkToChunk(c)
			if chunk == nil {
				continue
			}
			// 仅按 session_id 过滤
			if sessionID != "" && chunk.SessionID != sessionID {
				continue
			}
			matched = append(matched, *chunk)
		}
		if len(chunks) < pageSize {
			break
		}
		offset += pageSize
	}
	m.logger.Debug("RetrieveBySession: summary",
		"total_hits", totalHits, "matched", len(matched),
		"session_id", sessionID)

	// 按 timestamp 倒序排序（最新在前）
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	// 取前 limit 条
	if len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}

// Store implements memory.Memory.
func (m *RAGMemory) Store(ctx context.Context, chunk memory.MemoryChunk) (string, error) {
	if chunk.ID == "" && chunk.Content != "" {
		chunk.ID = contentHash(chunk.Content)
	}
	if err := m.storeMemoryChunk(ctx, chunk); err != nil {
		return "", err
	}
	return chunk.ID, nil
}

// Update is not part of the memory.Memory interface and is not supported in the chunk-based model.
// Deprecated: use Delete then Store instead.
func (m *RAGMemory) Update(ctx context.Context, id string, chunk memory.MemoryChunk) error {
	idx := m.semantic
	if idx == nil {
		return fmt.Errorf("memory: 语义索引器未初始化")
	}
	admin, ok := idx.(goragindexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("memory: 语义索引器不支持 Remove 操作")
	}
	if id == "" {
		return memory.ErrMemoryNotFound
	}
	if err := admin.Remove(ctx, id); err != nil {
		return fmt.Errorf("memory update failed to remove old record %s: %w", id, err)
	}
	chunk.ID = id
	return m.storeMemoryChunk(ctx, chunk)
}

func (m *RAGMemory) Delete(ctx context.Context, id string) error {
	idx := m.semantic
	if idx == nil {
		return fmt.Errorf("memory: 语义索引器未初始化")
	}

	if id == "" {
		return memory.ErrMemoryNotFound
	}

	admin, ok := idx.(goragindexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("memory: 语义索引器不支持 Remove 操作")
	}

	err := admin.Remove(ctx, id)
	if err != nil {
		return fmt.Errorf("memory: 删除记忆失败 %s: %w", id, err)
	}

	return nil
}

func (m *RAGMemory) Close(ctx context.Context) error {
	var errs []error

	if m.semantic != nil {
		if closer, ok := m.semantic.(goragindexer.IndexerCloser); ok {
			if err := closer.Close(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if closer, ok := m.logger.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("memory close: %v", errs)
	}
	return nil
}

// buildQueryWithFilter builds a semantic query with optional filters for short/long-term memory.
func (m *RAGMemory) buildQueryWithFilter(query string, cfg memory.RetrieveConfig) goragcore.Query {
	if m.embedder == nil {
		return nil
	}

	q := querypkg.NewSemanticQuery(query, m.embedder)

	if cfg.AgentName != "" {
		q.AddFilter(memory.FilterKeyAgentName, cfg.AgentName)
	}

	if cfg.SessionID != "" {
		q.AddFilter(memory.FilterKeySessionID, cfg.SessionID)
	}

	if cfg.ProjectDir != "" {
		q.AddFilter(memory.FilterKeyProjectDir, cfg.ProjectDir)
	}

	return q
}

// chunkToMemoryChunk 将 gorag core.Chunk 转换为 memory.MemoryChunk。
func chunkToMemoryChunk(c *goragcore.Chunk) *memory.MemoryChunk {
	if c == nil {
		return nil
	}
	chunk := &memory.MemoryChunk{
		ID:      c.ID,
		Content: c.Content,
	}

	// Extract metadata fields from Vector search results
	if c.Metadata != nil {
		if a, ok := c.Metadata["agent_name"].(string); ok && a != "" {
			chunk.AgentName = a
		}
		if s, ok := c.Metadata["session_id"].(string); ok {
			chunk.SessionID = s
		}
		if p, ok := c.Metadata["project_dir"].(string); ok {
			chunk.ProjectDir = p
		}
		if s, ok := c.Metadata["summary"].(string); ok && s != "" {
			chunk.Summary = s
		} else {
			chunk.Summary = c.Title
		}
		if t, ok := c.Metadata["tags"]; ok {
			switch v := t.(type) {
			case []string:
				if len(v) > 0 {
					chunk.Tags = v
				}
			case []any:
				for _, tag := range v {
					if s, ok := tag.(string); ok {
						chunk.Tags = append(chunk.Tags, s)
					}
				}
			}
		}
		if ts, ok := c.Metadata["timestamp"]; ok {
			switch v := ts.(type) {
			case float64:
				chunk.Timestamp = time.UnixMilli(int64(v))
			case int64:
				chunk.Timestamp = time.UnixMilli(v)
			}
		}
		if ct, ok := c.Metadata["content"].(string); ok && ct != "" {
			chunk.Content = ct
		}
	}

	if chunk.Summary == "" {
		chunk.Summary = c.Title
	}

	return chunk
}

// coreChunkToChunk 将 gorag core.Chunk（值类型）转换为 memory.MemoryChunk。
func coreChunkToChunk(c goragcore.Chunk) *memory.MemoryChunk {
	return chunkToMemoryChunk(&c)
}

// chunkHitToChunk 将 gorag core.ChunkHit 转换为 memory.MemoryChunk。
func chunkHitToChunk(hit goragcore.ChunkHit) *memory.MemoryChunk {
	return chunkToMemoryChunk(hit.Chunk)
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
