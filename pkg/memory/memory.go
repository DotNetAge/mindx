package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	semantic goragcore.Indexer // SemanticIndexer（统一记忆存储）
	embedder goragcore.Embedder
	logger   logging.Logger
}

type RAGMemoryOption func(*RAGMemory)

type MemoryConfig struct {
	AgentName string

	MemoryDir string

	Logger logging.Logger

	Embedder goragcore.Embedder

	ReadOnly bool
}

func (c MemoryConfig) dataDir() string {
	return filepath.Join(c.MemoryDir, "shared")
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

// NewRAGMemory 创建一个 RAGMemory 实例。
// semanticIdx 始终非空；graphIdx 可为 nil（仅使用语义检索）。
func NewRAGMemory(semanticIdx goragcore.Indexer, opts ...RAGMemoryOption) *RAGMemory {
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
		return nil, fmt.Errorf("memory: embedder is required")
	}
	if cfg.AgentName == "" {
		return nil, fmt.Errorf("memory: agent name is required")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("memory: logger is required (pass cfg.Logger to share the application logger)")
	}
	if cfg.MemoryDir == "" {
		return nil, fmt.Errorf("memory: memory dir is required")
	}

	dataDir := cfg.dataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("memory: create data directory %s: %w", dataDir, err)
	}

	logger := cfg.Logger

	// ── SemanticIndexer（统一记忆存储）───────────────────────
	semVecDir := filepath.Join(dataDir, "vectors")
	if mkErr := os.MkdirAll(semVecDir, 0755); mkErr != nil {
		return nil, fmt.Errorf("memory: create semantic vector directory %s: %w", semVecDir, mkErr)
	}
	semVS, err := govector.NewStore(
		govector.WithCollection("shared_sem"),
		govector.WithDimension(cfg.Embedder.Dim()),
		govector.WithDBPath(filepath.Join(semVecDir, "shared.db")),
		govector.WithHNSW(true),
		govector.WithReadOnly(cfg.ReadOnly),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: create semantic vector store: %w", err)
	}
	semIdx := goragindexer.NewSemanticIndexer(semVS, cfg.Embedder,
		goragindexer.WithSemanticLogger(logger),
	)

	m := &RAGMemory{
		semantic: semIdx,
		embedder: cfg.Embedder,
		logger:   logger,
	}

	logger.Info("memory: initialized",
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
func (m *RAGMemory) Semantic() goragcore.Indexer {
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
		"agent_name": chunk.AgentName,
		"session_id": chunk.SessionID,
		"summary":    chunk.Summary,
		"tags":       tagStrs,
		"content":    chunk.Content,
		"title":      chunk.Summary,
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

	if m.semantic != nil {
		if err := m.semantic.StoreChunk(ctx, coreChunk); err != nil {
			return fmt.Errorf("memory store chunk failed: %w", err)
		}
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
		return nil, fmt.Errorf("semantic indexer not initialized")
	}

	q := m.buildQueryWithFilter(query, cfg)
	if q == nil {
		return nil, nil
	}

	hits, err := idx.Search(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("memory retrieve failed: %w", err)
	}

	if len(hits) == 0 {
		return nil, nil
	}

	var chunks []memory.MemoryChunk
	for _, hit := range hits {
		chunk := hitToChunk(hit)
		if chunk == nil {
			continue
		}

		if cfg.MinScore > 0 && float64(hit.Score) < cfg.MinScore {
			continue
		}

		chunks = append(chunks, *chunk)
	}

	if cfg.Limit > 0 && len(chunks) > cfg.Limit {
		chunks = chunks[:cfg.Limit]
	}

	return chunks, nil
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
		return fmt.Errorf("semantic indexer not initialized")
	}
	if id == "" {
		return memory.ErrMemoryNotFound
	}
	if err := idx.Remove(ctx, id); err != nil {
		return fmt.Errorf("memory update failed to remove old record %s: %w", id, err)
	}
	chunk.ID = id
	return m.storeMemoryChunk(ctx, chunk)
}

func (m *RAGMemory) Delete(ctx context.Context, id string) error {
	idx := m.semantic
	if idx == nil {
		return fmt.Errorf("semantic indexer not initialized")
	}

	if id == "" {
		return memory.ErrMemoryNotFound
	}

	err := idx.Remove(ctx, id)
	if err != nil {
		return fmt.Errorf("memory delete failed: %w", err)
	}

	return nil
}

func (m *RAGMemory) Close(ctx context.Context) error {
	var errs []error

	if m.semantic != nil {
		if closer, ok := m.semantic.(io.Closer); ok {
			if err := closer.Close(); err != nil {
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

	return q
}

func hitToChunk(hit goragcore.Hit) *memory.MemoryChunk {
	chunk := &memory.MemoryChunk{
		ID:      hit.ID,
		Content: hit.Content,
	}

	// Extract metadata fields from Vector search results
	if hit.Metadata != nil {
		if a, ok := hit.Metadata["agent_name"].(string); ok && a != "" {
			chunk.AgentName = a
		}
		if s, ok := hit.Metadata["session_id"].(string); ok {
			chunk.SessionID = s
		}
		if s, ok := hit.Metadata["summary"].(string); ok && s != "" {
			chunk.Summary = s
		} else {
			chunk.Summary = hit.Title
		}
		if t, ok := hit.Metadata["tags"]; ok {
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
		if ts, ok := hit.Metadata["timestamp"]; ok {
			switch v := ts.(type) {
			case float64:
				chunk.Timestamp = time.UnixMilli(int64(v))
			case int64:
				chunk.Timestamp = time.UnixMilli(v)
			}
		}
		if c, ok := hit.Metadata["content"].(string); ok && c != "" {
			chunk.Content = c
		}
	}

	if chunk.Summary == "" {
		chunk.Summary = hit.Title
	}

	return chunk
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
