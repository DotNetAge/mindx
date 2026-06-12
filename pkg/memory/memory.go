package memory

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/gorag"
	goragcore "github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedder"
	goragindexer "github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	querypkg "github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/gorag/store/doc/bleve"
	graphgograph "github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/DotNetAge/gorag/store/vector/govector"
	"github.com/DotNetAge/goreact/memory"
)

var _ memory.Memory = (*RAGMemory)(nil)

// RAGMemory implements memory.Memory using GoRAG's HybridIndexer as the backend.
type RAGMemory struct {
	indexer    *gorag.HybridIndexer
	embedder   goragcore.Embedder
	memoryType memory.MemoryType
	graphStore goragcore.GraphStore
	logger     logging.Logger
}

type RAGMemoryOption func(*RAGMemory)

type MemoryConfig struct {
	MemoryType memory.MemoryType

	AgentName string

	MemoryDir string

	SessionDir string

	Logger logging.Logger

	Embedder goragcore.Embedder

	ReadOnly bool

	// GraphStore 可选。提供外部已创建的图数据库实例，用于 LLMIndexer 的实体/关系写入，
	// 也供外部直接查询（GetNode, GetNeighbors, GetMultiHopPaths 等）。
	// 当 LLMConfig 配置且此项为空时，会自动创建。
	GraphStore goragcore.GraphStore

	// LLMConfig 可选。非空时启用 LLMIndexer，在语义索引基础上增加知识图谱
	// 实体/关系索引与标签系统（tags, summary, entity_ids）。
	LLMConfig *goragindexer.ModelConfig
}

func (c MemoryConfig) dataDir() string {
	switch c.MemoryType {
	case memory.MemoryTypeSession:
		return filepath.Join(c.SessionDir, "memory")
	default:
		return filepath.Join(c.MemoryDir, c.AgentName)
	}
}

func NewEmbedderFromConfig(modelPath string) (goragcore.Embedder, error) {
	if modelPath == "" {
		return nil, nil
	}
	emb, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelPath))
	if err != nil {
		return nil, fmt.Errorf("memory: create embedder: %w", err)
	}
	return emb, nil
}

func NewRAGMemory(indexer *gorag.HybridIndexer, opts ...RAGMemoryOption) *RAGMemory {
	m := &RAGMemory{
		indexer:    indexer,
		memoryType: memory.MemoryTypeLongTerm,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewRAGMemoryFromConfig(cfg MemoryConfig) (*RAGMemory, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("memory: embedder is required")
	}
	if cfg.AgentName == "" {
		return nil, fmt.Errorf("memory: agent name is required")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("memory: logger is required (pass cfg.Logger to share the application logger)")
	}
	if cfg.MemoryDir == "" && cfg.MemoryType != memory.MemoryTypeSession {
		return nil, fmt.Errorf("memory: memory dir is required for %s memory type", memoryTypeLabel(cfg.MemoryType))
	}
	if cfg.SessionDir == "" && cfg.MemoryType == memory.MemoryTypeSession {
		return nil, fmt.Errorf("memory: session dir is required for session memory type")
	}

	dataDir := cfg.dataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("memory: create data directory %s: %w", dataDir, err)
	}

	vecDir := filepath.Join(dataDir, "vectors")
	if mkErr := os.MkdirAll(vecDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: create vector directory %s: %w", vecDir, mkErr)
	}
	vs, err := govector.NewStore(
		govector.WithCollection(cfg.AgentName),
		govector.WithDimension(cfg.Embedder.Dim()),
		govector.WithDBPath(filepath.Join(vecDir, cfg.AgentName+".db")),
		govector.WithHNSW(true),
		govector.WithReadOnly(cfg.ReadOnly),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: create vector store: %w", err)
	}

	ftDir := filepath.Join(dataDir, "fulltexts")
	if mkErr := os.MkdirAll(ftDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: create fulltext directory %s: %w", ftDir, mkErr)
	}
	fts, err := bleve.NewBleveStore(filepath.Join(ftDir, cfg.AgentName+".bleve"))
	if err != nil {
		return nil, fmt.Errorf("memory: create fulltext store: %w", err)
	}

	logger := cfg.Logger
	logger.Info("memory: initializing RAG indexer",
		"agent", cfg.AgentName,
		"memory_type", memoryTypeLabel(cfg.MemoryType),
		"vector_dim", cfg.Embedder.Dim(),
		"vector_dir", vecDir,
		"fulltext_dir", ftDir,
	)

	indexer, err := gorag.NewHybridIndexer(
		logger, vs, nil, fts, nil, cfg.Embedder,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: create hybrid indexer: %w", err)
	}

	// ── LLMIndexer 模式（取代 semanticIndexer + graphIndexer）─────────
	var graphStore goragcore.GraphStore
	if cfg.LLMConfig != nil {
		// 使用外部已创建的 GraphStore，或自动创建
		if cfg.GraphStore != nil {
			graphStore = cfg.GraphStore
			logger.Info("memory: using external GraphStore for LLMIndexer")
		} else {
			graphDir := filepath.Join(dataDir, "knowledge-graph")
			if mkErr := os.MkdirAll(graphDir, 0755); mkErr != nil {
				return nil, fmt.Errorf("memory: create graph directory: %w", mkErr)
			}
			gs, gErr := graphgograph.NewGraphStore(filepath.Join(graphDir, "graph.db"))
			if gErr != nil {
				return nil, fmt.Errorf("memory: create graph store: %w", gErr)
			}
			graphStore = gs
		}

		// 删除 semanticIndexer（LLMIndexer 替代语义分块 + 实体提取）
		indexer.RemoveIndexer("semantic")

		llmIdx := goragindexer.New(
			*cfg.LLMConfig,
			cfg.Embedder,
			vs,
			graphStore,
			goragindexer.WithLLMLogger(logger),
		)
		indexer.AddIndexer(llmIdx, 0.8)

		logger.Info("memory: LLMIndexer enabled (replaces semantic+graph)",
			"model", cfg.LLMConfig.Model,
			"lang", cfg.LLMConfig.Language,
			"ontology", cfg.LLMConfig.Ontology,
			"max_tokens", cfg.LLMConfig.MaxTokens,
		)
	}

	return &RAGMemory{
		indexer:    indexer,
		embedder:   cfg.Embedder,
		memoryType: cfg.MemoryType,
		graphStore: graphStore,
		logger:     logger,
	}, nil
}

func WithMemoryType(t memory.MemoryType) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.memoryType = t
	}
}

func WithEmbedder(embedder goragcore.Embedder) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.embedder = embedder
	}
}

func (m *RAGMemory) MemoryType() memory.MemoryType {
	return m.memoryType
}

func (m *RAGMemory) Indexer() *gorag.HybridIndexer {
	return m.indexer
}

// GraphStore 返回 LLMIndexer 使用的图数据库实例。
// 可用于直接执行图查询（GetNode, GetNeighbors, GetMultiHopPaths 等）。
// 仅当 LLMIndexer 启用时非空。
func (m *RAGMemory) GraphStore() goragcore.GraphStore {
	return m.graphStore
}

func (m *RAGMemory) Retrieve(ctx context.Context, query string, opts ...memory.RetrieveOption) ([]memory.MemoryRecord, error) {
	cfg := memory.DefaultRetrieveConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if m.indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	q := m.buildQuery(query)
	if q == nil {
		return nil, nil
	}

	hits, err := m.indexer.Search(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("memory retrieve failed: %w", err)
	}

	if len(hits) == 0 {
		return nil, nil
	}

	var records []memory.MemoryRecord
	for _, hit := range hits {
		record := hitToRecord(hit)
		if record == nil {
			continue
		}

		if cfg.Types != nil && !containsType(cfg.Types, record.Type) {
			continue
		}
		if cfg.MinScore > 0 && float64(hit.Score) < cfg.MinScore {
			continue
		}

		record.Score = float64(hit.Score)
		records = append(records, *record)
	}

	if cfg.Limit > 0 && len(records) > cfg.Limit {
		records = records[:cfg.Limit]
	}

	return records, nil
}

func (m *RAGMemory) Store(ctx context.Context, record memory.MemoryRecord) (string, error) {
	if m.indexer == nil {
		return "", fmt.Errorf("indexer not initialized")
	}

	if record.ID == "" {
		record.ID = generateID()
	}

	content := buildContentForStore(record)
	chunk, err := m.indexer.Add(ctx, content)
	if err != nil {
		return "", fmt.Errorf("memory store failed: %w", err)
	}

	if chunk == nil {
		return "", fmt.Errorf("memory store returned nil chunk")
	}

	return record.ID, nil
}

func (m *RAGMemory) Update(ctx context.Context, id string, record memory.MemoryRecord) error {
	if m.indexer == nil {
		return fmt.Errorf("indexer not initialized")
	}

	if id == "" {
		return memory.ErrMemoryNotFound
	}

	err := m.indexer.Remove(ctx, id)
	if err != nil {
		return fmt.Errorf("memory update failed to remove old record %s: %w", id, err)
	}

	record.ID = id
	content := buildContentForStore(record)
	_, err = m.indexer.Add(ctx, content)
	if err != nil {
		return fmt.Errorf("memory update failed to add new record: %w", err)
	}

	return nil
}

func (m *RAGMemory) Delete(ctx context.Context, id string) error {
	if m.indexer == nil {
		return fmt.Errorf("indexer not initialized")
	}

	if id == "" {
		return memory.ErrMemoryNotFound
	}

	err := m.indexer.Remove(ctx, id)
	if err != nil {
		return fmt.Errorf("memory delete failed: %w", err)
	}

	return nil
}

func (m *RAGMemory) SyncProjectDir(ctx context.Context, projectDir, cacheDir string) *ProjectSyncResult {
	pi := NewIndexService(m.indexer, cacheDir, m.logger)
	return pi.Sync(ctx, projectDir)
}

// MemoryStats summarizes the RAG indexing progress.
type MemoryStats struct {
	TotalFiles   int `json:"total_files"`
	IndexedFiles int `json:"indexed_files"`
	TotalChunks  int `json:"total_chunks"`
}

// Stats returns indexing statistics for the given project directory.
func (m *RAGMemory) Stats(ctx context.Context, projectDir, cacheDir string) *MemoryStats {
	stats := &MemoryStats{}

	// Count total discoverable files by walking the project dir
	// (same ignore logic as IndexService)
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(projectDir, path)
		if info.IsDir() {
			name := info.Name()
			if name != "." && (strings.HasPrefix(name, ".") || DefaultIgnoredDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip ignored files (check via relative path)
		if relPath == "" || strings.HasPrefix(relPath, ".") {
			return nil
		}
		stats.TotalFiles++
		return nil
	})

	// Count indexed files from the IndexService cache
	if cacheDir != "" {
		pi := NewIndexService(m.indexer, cacheDir, m.logger)
		if err := pi.cache.LoadFromFile(pi.cacheDir); err == nil {
			stats.IndexedFiles = len(pi.cache.Files)
		}
	}

	// Count total chunks via Indexer
	if m.indexer != nil {
		count, err := m.indexer.Count(ctx)
		if err == nil {
			stats.TotalChunks = count
		}
	}

	return stats
}

func (m *RAGMemory) Close(ctx context.Context) error {
	var errs []error

	if m.indexer != nil {
		if err := m.indexer.Close(ctx); err != nil {
			errs = append(errs, err)
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

func (m *RAGMemory) buildQuery(query string) goragcore.Query {
	if m.embedder != nil {
		return querypkg.NewSemanticQuery(query, m.embedder)
	}
	return m.indexer.NewQuery(query)
}

func buildContentForStore(record memory.MemoryRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[ID:%s]\n", record.ID))
	sb.WriteString(fmt.Sprintf("[Type:%s]\n", memoryTypeLabel(record.Type)))
	if record.Title != "" {
		sb.WriteString(fmt.Sprintf("[Title:%s]\n", record.Title))
	}
	if len(record.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("[Tags:%s]\n", strings.Join(record.Tags, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString(record.Content)
	return sb.String()
}

func hitToRecord(hit goragcore.Hit) *memory.MemoryRecord {
	content := hit.Content
	id := hit.ID
	title := ""
	memType := memory.MemoryTypeLongTerm
	tags := []string{}

	lines := strings.Split(content, "\n")
	bodyStart := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ID:") && strings.HasSuffix(line, "]") {
			id = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[ID:")
		} else if strings.HasPrefix(line, "[Type:") && strings.HasSuffix(line, "]") {
			typeStr := strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[Type:")
			memType = parseMemoryType(typeStr)
		} else if strings.HasPrefix(line, "[Title:") && strings.HasSuffix(line, "]") {
			title = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[Title:")
		} else if strings.HasPrefix(line, "[Tags:") && strings.HasSuffix(line, "]") {
			tagsStr := strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[Tags:")
			if tagsStr != "" {
				tags = strings.Split(tagsStr, ",")
				for i := range tags {
					tags[i] = strings.TrimSpace(tags[i])
				}
			}
		} else if line == "" && bodyStart == 0 && i > 0 {
			bodyStart = i + 1
		}
	}

	if bodyStart > 0 && bodyStart < len(lines) {
		content = strings.Join(lines[bodyStart:], "\n")
	}

	return &memory.MemoryRecord{
		ID:      id,
		Type:    memType,
		Title:   title,
		Content: content,
		Tags:    tags,
		Score:   float64(hit.Score),
	}
}

func generateID() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixMilli())
}

func containsType(types []memory.MemoryType, t memory.MemoryType) bool {
	for _, v := range types {
		if v == t {
			return true
		}
	}
	return false
}

func memoryTypeLabel(t memory.MemoryType) string {
	switch t {
	case memory.MemoryTypeSession:
		return "session"
	case memory.MemoryTypeLongTerm:
		return "longterm"
	default:
		return "unknown"
	}
}

func parseMemoryType(s string) memory.MemoryType {
	switch s {
	case "session":
		return memory.MemoryTypeSession
	case "longterm":
		return memory.MemoryTypeLongTerm
	default:
		return memory.MemoryTypeLongTerm
	}
}
