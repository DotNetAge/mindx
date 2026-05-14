package memory

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DotNetAge/gorag"
	goragcore "github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedder"
	"github.com/DotNetAge/gorag/logging"
	querypkg "github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/gorag/store/doc/bleve"
	"github.com/DotNetAge/gorag/store/vector/govector"
	"github.com/DotNetAge/goreact/core"
)

var _ core.Memory = (*RAGMemory)(nil)

// RAGMemory implements core.Memory using GoRAG's HybridIndexer as the backend.
// It maps MemoryRecord operations to HybridIndexer's Add/Search/Remove,
// enabling semantic retrieval over stored knowledge with hybrid (vector + BM25 + graph) search.
//
// Data mapping:
//   - MemoryRecord.ID -> Chunk.Content header [ID:xxx]
//   - MemoryRecord.Content -> Chunk.Content body
//   - MemoryRecord.Type/Tags/Title -> Chunk.Content headers
//   - Retrieve query -> SemanticQuery through HybridIndexer
//
// Storage layout (determined by MemoryType at construction):
//
//	MemoryTypeLongTerm -> <MemoryDir>/<AgentName>/{vectors,fulltexts,graphs}/
//	MemoryTypeSession  -> <SessionDir>/memory/{vectors,fulltexts,graphs}/
type RAGMemory struct {
	indexer    *gorag.HybridIndexer
	embedder   goragcore.Embedder
	memoryType core.MemoryType
	logger     logging.Logger
}

// RAGMemoryOption configures RAGMemory creation.
type RAGMemoryOption func(*RAGMemory)

// MemoryConfig configures a RAGMemory instance for a specific agent and memory type.
// It determines storage paths and provides the dependencies needed for the HybridIndexer.
type MemoryConfig struct {
	// MemoryType determines storage layout:
	//   MemoryTypeLongTerm (default) -> <MemoryDir>/<AgentName>/
	//   MemoryTypeSession            -> <SessionDir>/memory/
	MemoryType core.MemoryType

	// AgentName is used as the data filename and collection name.
	// Required for both memory types.
	AgentName string

	// MemoryDir is the base memory directory for LongTerm storage.
	// Typically ~/.mindx/memory/. Required for MemoryTypeLongTerm.
	MemoryDir string

	// SessionDir is the session sandbox directory for Session storage.
	// Required for MemoryTypeSession.
	SessionDir string

	// Logger for the HybridIndexer. Optional; defaults to a no-op logger.
	Logger logging.Logger

	// Embedder is required for semantic vector search.
	Embedder goragcore.Embedder
}

// dataDir resolves the RAG storage root directory based on MemoryType.
func (c MemoryConfig) dataDir() string {
	switch c.MemoryType {
	case core.MemoryTypeSession:
		return filepath.Join(c.SessionDir, "memory")
	default:
		return filepath.Join(c.MemoryDir, c.AgentName)
	}
}

// NewEmbedderFromConfig 根据 Embedder 模型文件路径创建 ChineseClipEmbedder。
// 如果 modelPath 为空，返回 nil（Memory 不可用）。
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

// NewRAGMemory creates a new RAGMemory backed by a pre-built HybridIndexer.
func NewRAGMemory(indexer *gorag.HybridIndexer, opts ...RAGMemoryOption) *RAGMemory {
	m := &RAGMemory{
		indexer:    indexer,
		memoryType: core.MemoryTypeLongTerm,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewRAGMemoryFromConfig creates a new RAGMemory with full store initialization.
// It creates vector, fulltext, and graph stores under the resolved data directory
// (determined by MemoryConfig.MemoryType), then constructs a HybridIndexer with all stores.
//
// LongTerm example:
//
//	mem, err := NewRAGMemoryFromConfig(MemoryConfig{
//	    MemoryType: core.MemoryTypeLongTerm,
//	    AgentName:  "code-reviewer",
//	    MemoryDir:  filepath.Join(os.Getenv("HOME"), ".mindx", "memory"),
//	    Embedder:   embedder,
//	})
//
// Session example:
//
//	mem, err := NewRAGMemoryFromConfig(MemoryConfig{
//	    MemoryType: core.MemoryTypeSession,
//	    AgentName:  "code-reviewer",
//	    SessionDir: sessionInfo.SessionDir,
//	    Embedder:   embedder,
//	})
func NewRAGMemoryFromConfig(cfg MemoryConfig) (*RAGMemory, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("memory: embedder is required")
	}
	if cfg.AgentName == "" {
		return nil, fmt.Errorf("memory: agent name is required")
	}
	if cfg.MemoryDir == "" && cfg.MemoryType != core.MemoryTypeSession {
		return nil, fmt.Errorf("memory: memory dir is required for %s memory type", memoryTypeLabel(cfg.MemoryType))
	}
	if cfg.SessionDir == "" && cfg.MemoryType == core.MemoryTypeSession {
		return nil, fmt.Errorf("memory: session dir is required for session memory type")
	}

	dataDir := cfg.dataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("memory: create data directory %s: %w", dataDir, err)
	}

	// --- Vector store ---
	vecDir := filepath.Join(dataDir, "vectors")
	if mkErr := os.MkdirAll(vecDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: create vector directory %s: %w", vecDir, mkErr)
	}
	vs, err := govector.NewStore(
		govector.WithCollection(cfg.AgentName),
		govector.WithDimension(cfg.Embedder.Dim()),
		govector.WithDBPath(filepath.Join(vecDir, cfg.AgentName+".db")),
		govector.WithHNSW(true),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: create vector store: %w", err)
	}

	// --- Fulltext (bleve) store ---
	ftDir := filepath.Join(dataDir, "fulltexts")
	if mkErr := os.MkdirAll(ftDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: create fulltext directory %s: %w", ftDir, mkErr)
	}
	fts, err := bleve.NewBleveStore(filepath.Join(ftDir, cfg.AgentName+".bleve"))
	if err != nil {
		return nil, fmt.Errorf("memory: create fulltext store: %w", err)
	}

	// Logger: if not explicitly provided, auto-create a per-instance file logger
	// at ~/.mindx/logs/memory/<AgentName>.log
	logger := cfg.Logger
	if logger == nil {
		logger = newMemoryFileLogger(cfg.AgentName)
	}
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}

	// --- HybridIndexer ---
	// Both memory types use semantic + fulltext only (no graph store).
	indexer, err := gorag.NewHybridIndexer(
		logger, vs, nil, fts, nil, cfg.Embedder,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: create hybrid indexer: %w", err)
	}

	return &RAGMemory{
		indexer:    indexer,
		embedder:   cfg.Embedder,
		memoryType: cfg.MemoryType,
		logger:     logger,
	}, nil
}

// WithMemoryType sets the memory type for a RAGMemory created via NewRAGMemory.
func WithMemoryType(t core.MemoryType) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.memoryType = t
	}
}

// WithEmbedder sets the embedder for creating semantic queries during retrieval.
func WithEmbedder(embedder goragcore.Embedder) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.embedder = embedder
	}
}

// MemoryType returns the memory type of this RAGMemory instance.
func (m *RAGMemory) MemoryType() core.MemoryType {
	return m.memoryType
}

// Retrieve searches memory for records relevant to the query.
// Uses the HybridIndexer's Search with a semantic query for best results.
func (m *RAGMemory) Retrieve(ctx context.Context, query string, opts ...core.RetrieveOption) ([]core.MemoryRecord, error) {
	cfg := core.DefaultRetrieveConfig()
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

	var records []core.MemoryRecord
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

// Store persists a new memory record and returns its ID.
// The content is added to the HybridIndexer with structured headers for metadata.
func (m *RAGMemory) Store(ctx context.Context, record core.MemoryRecord) (string, error) {
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

// Update modifies an existing memory record by ID.
// Implements remove-then-add pattern since HybridIndexer doesn't have native update.
func (m *RAGMemory) Update(ctx context.Context, id string, record core.MemoryRecord) error {
	if m.indexer == nil {
		return fmt.Errorf("indexer not initialized")
	}

	if id == "" {
		return core.ErrMemoryNotFound
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

// Delete removes a memory record by ID from the index.
func (m *RAGMemory) Delete(ctx context.Context, id string) error {
	if m.indexer == nil {
		return fmt.Errorf("indexer not initialized")
	}

	if id == "" {
		return core.ErrMemoryNotFound
	}

	err := m.indexer.Remove(ctx, id)
	if err != nil {
		return fmt.Errorf("memory delete failed: %w", err)
	}

	return nil
}

// SyncProjectDir incrementally indexes files from projectDir into this memory store.
// cacheDir is where the file mtime cache is persisted (e.g., ~/.mindx/memory/<AgentName>/project/).
// Returns a detailed sync result with per-file error information.
func (m *RAGMemory) SyncProjectDir(ctx context.Context, projectDir, cacheDir string) *ProjectSyncResult {
	pi := NewProjectIndexer(m.indexer, cacheDir, m.logger)
	return pi.Sync(ctx, projectDir)
}

// Close shuts down the underlying HybridIndexer and releases resources,
// including the per-instance log file if one was created.
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

// newMemoryFileLogger creates a per-instance file logger at
// ~/.mindx/logs/memory/<AgentName>.log. Returns nil if the mindx
// home directory cannot be determined or the log file cannot be created.
func newMemoryFileLogger(agentName string) logging.Logger {
	if agentName == "" {
		return nil
	}

	mindxHome := os.Getenv("MINDX_WORKSPACE")
	if mindxHome == "" {
		if runtime.GOOS == "windows" {
			if appData := os.Getenv("APPDATA"); appData != "" {
				mindxHome = filepath.Join(appData, "mindx")
			}
		}
		if mindxHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil
			}
			mindxHome = filepath.Join(homeDir, ".mindx")
		}
	}

	logDir := filepath.Join(mindxHome, "logs", "memory")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil
	}

	logFile := filepath.Join(logDir, agentName+".log")
	logger, err := logging.DefaultFileLogger(logFile)
	if err != nil {
		return nil
	}
	return logger
}

func (m *RAGMemory) buildQuery(query string) goragcore.Query {
	if m.embedder != nil {
		return querypkg.NewSemanticQuery(query, m.embedder)
	}
	return m.indexer.NewQuery(query)
}

func buildContentForStore(record core.MemoryRecord) string {
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

func hitToRecord(hit goragcore.Hit) *core.MemoryRecord {
	content := hit.Content
	id := hit.ID
	title := ""
	memType := core.MemoryTypeLongTerm
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

	return &core.MemoryRecord{
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

func containsType(types []core.MemoryType, t core.MemoryType) bool {
	for _, v := range types {
		if v == t {
			return true
		}
	}
	return false
}

func memoryTypeLabel(t core.MemoryType) string {
	switch t {
	case core.MemoryTypeSession:
		return "session"
	case core.MemoryTypeLongTerm:
		return "longterm"
	default:
		return "unknown"
	}
}

func parseMemoryType(s string) core.MemoryType {
	switch s {
	case "session":
		return core.MemoryTypeSession
	case "longterm":
		return core.MemoryTypeLongTerm
	default:
		return core.MemoryTypeLongTerm
	}
}

