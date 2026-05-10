package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gorag"
	goragcore "github.com/DotNetAge/gorag/core"
	querypkg "github.com/DotNetAge/gorag/query"
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
type RAGMemory struct {
	indexer  *gorag.HybridIndexer
	embedder goragcore.Embedder
}

// RAGMemoryOption configures RAGMemory creation.
type RAGMemoryOption func(*RAGMemory)

// NewRAGMemory creates a new RAGMemory backed by a HybridIndexer.
func NewRAGMemory(indexer *gorag.HybridIndexer, opts ...RAGMemoryOption) *RAGMemory {
	m := &RAGMemory{
		indexer: indexer,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithEmbedder sets the embedder for creating semantic queries during retrieval.
func WithEmbedder(embedder goragcore.Embedder) RAGMemoryOption {
	return func(m *RAGMemory) {
		m.embedder = embedder
	}
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


