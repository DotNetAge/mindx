package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// sanitizeDirName converts a filesystem path to a safe directory name (same logic as memory package).
func sanitizeDirName(absPath string) string {
	replacer := strings.NewReplacer(
		string(filepath.Separator), "_",
		":", "_",
		"~", "_",
	)
	name := replacer.Replace(absPath)
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

func (d *Daemon) handleMemoryQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryQueryParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	opts := []goharnessmemory.RetrieveOption{}
	if p.Limit > 0 {
		opts = append(opts, goharnessmemory.WithMemoryLimit(p.Limit))
	}
	if p.MinScore > 0 {
		opts = append(opts, goharnessmemory.WithMinScore(p.MinScore))
	}

	chunks, err := mem.Retrieve(context.Background(), p.Query, opts...)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	if chunks == nil {
		return []goharnessmemory.MemoryChunk{}, nil
	}
	return chunks, nil
}

func (d *Daemon) handleMemoryStore(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryStoreParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	chunk := goharnessmemory.MemoryChunk{
		Summary:   p.Title,
		Content:   p.Content,
		Timestamp: time.Now(),
	}

	id, err := mem.Store(context.Background(), chunk)
	if err != nil {
		return nil, fmt.Errorf("memory store failed: %w", err)
	}

	return map[string]string{"id": id}, nil
}

func (d *Daemon) handleMemoryDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	if err := mem.Delete(context.Background(), p.ID); err != nil {
		return nil, fmt.Errorf("memory delete failed: %w", err)
	}

	return map[string]string{"status": "ok", "deleted_id": p.ID}, nil
}

// ---------------------------------------------------------------------------
// memory.chunks — 分页获取 RAG Chunk 列表（翻书式遍历接口）
// ---------------------------------------------------------------------------

func (d *Daemon) handleMemoryChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryChunksParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize <= 0 || p.PageSize > 200 {
		p.PageSize = 50
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	indexer := mem.Semantic()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	admin, ok := indexer.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("indexer does not support admin operations")
	}

	offset := (p.Page - 1) * p.PageSize
	chunks, total, err := admin.List(context.Background(), offset, p.PageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	items := make([]rpc.ChunkItem, 0, len(chunks))
	for _, c := range chunks {
		mimeType, _ := c.Metadata[goragcore.MetaMimeType].(string)
		headingLevel := 0
		if hl, ok := c.Metadata[goragcore.MetaHeadingLevel].(float64); ok {
			headingLevel = int(hl)
		}
		var headingPath []string
		if hp, ok := c.Metadata[goragcore.MetaHeadingPath].([]string); ok {
			headingPath = hp
		} else if s, ok := c.Metadata[goragcore.MetaHeadingPath].(string); ok && s != "" {
			headingPath = []string{s}
		}

		items = append(items, rpc.ChunkItem{
			ID:       c.ID,
			ParentID: c.ParentID,
			DocID:    c.DocID,
			MIMEType: mimeType,
			Content:  c.Content,
			Metadata: c.Metadata,
			ChunkMeta: rpc.ChunkMetaItem{
				Index:        c.Index,
				StartPos:     c.StartPos,
				EndPos:       c.EndPos,
				HeadingLevel: headingLevel,
				HeadingPath:  headingPath,
			},
		})
	}

	hasMore := len(items) == p.PageSize

	d.logger.Info("memory.chunks called", "page", p.Page, "page_size", p.PageSize, "returned", len(items), "total", total, "has_more", hasMore)

	return rpc.MemoryChunksResult{
		Chunks:   items,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
		HasMore:  hasMore,
	}, nil
}

// ---------------------------------------------------------------------------
// memory.get_chunks — 按文档ID获取全部分块（一次性拉取单文档所有Chunk）
// ---------------------------------------------------------------------------

func (d *Daemon) handleMemoryGetChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryGetChunksParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.DocID == "" {
		return nil, fmt.Errorf("doc_id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	indexer := mem.Semantic()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	admin, ok := indexer.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("indexer does not support admin operations")
	}

	// List all chunks and filter by DocID since GetChunks takes chunk IDs, not docID
	allChunks, _, err := admin.List(context.Background(), 0, 1000000, nil)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	var matched []goragcore.Chunk
	for _, c := range allChunks {
		if c.DocID == p.DocID {
			matched = append(matched, c)
		}
	}

	items := make([]rpc.ChunkItem, 0, len(matched))
	for _, c := range matched {
		mimeType, _ := c.Metadata[goragcore.MetaMimeType].(string)
		headingLevel := 0
		if hl, ok := c.Metadata[goragcore.MetaHeadingLevel].(float64); ok {
			headingLevel = int(hl)
		}
		var headingPath []string
		if hp, ok := c.Metadata[goragcore.MetaHeadingPath].([]string); ok {
			headingPath = hp
		} else if s, ok := c.Metadata[goragcore.MetaHeadingPath].(string); ok && s != "" {
			headingPath = []string{s}
		}

		items = append(items, rpc.ChunkItem{
			ID:       c.ID,
			ParentID: c.ParentID,
			DocID:    c.DocID,
			MIMEType: mimeType,
			Content:  c.Content,
			Metadata: c.Metadata,
			ChunkMeta: rpc.ChunkMetaItem{
				Index:        c.Index,
				StartPos:     c.StartPos,
				EndPos:       c.EndPos,
				HeadingLevel: headingLevel,
				HeadingPath:  headingPath,
			},
		})
	}

	d.logger.Info("memory.get_chunks called", "doc_id", p.DocID, "returned", len(items))

	return struct {
		DocID  string          `json:"doc_id"`
		Chunks []rpc.ChunkItem `json:"chunks"`
		Count  int             `json:"count"`
	}{
		DocID:  p.DocID,
		Chunks: items,
		Count:  len(items),
	}, nil
}

// ---------------------------------------------------------------------------
// memory.count — 获取 RAG 索引中的分块总数
// ---------------------------------------------------------------------------

func (d *Daemon) handleMemoryCount(_ context.Context, _ json.RawMessage) (any, error) {
	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	indexer := mem.Semantic()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	admin, ok := indexer.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("indexer does not support admin operations")
	}

	count, err := admin.Count(context.Background())
	if err != nil {
		return nil, fmt.Errorf("memory count failed: %w", err)
	}

	d.logger.Info("memory.count called", "count", count)

	return rpc.MemoryCountResult{Count: count}, nil
}

// ---------------------------------------------------------------------------
// memory.list_by_session — 按会话 ID 列出所有 MemoryChunk（分页，按时间倒序）
// ---------------------------------------------------------------------------

func (d *Daemon) handleMemoryListBySession(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryListBySessionParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	indexer := mem.Semantic()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	admin, ok := indexer.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("indexer does not support admin operations")
	}

	// 分页遍历所有 chunk，按 session_id 过滤
	var matched []rpc.MemoryChunkItem
	const pageSize = 200
	offset := 0
	for {
		chunks, _, err := admin.List(context.Background(), offset, pageSize, nil)
		if err != nil {
			return nil, fmt.Errorf("list chunks failed: %w", err)
		}
		if len(chunks) == 0 {
			break
		}
		for _, c := range chunks {
			sessionID, _ := c.Metadata["session_id"].(string)
			if sessionID != p.SessionID {
				continue
			}
			chunk := chunkHitToMemoryItem(c)
			if chunk != nil {
				matched = append(matched, *chunk)
			}
		}
		if len(chunks) < pageSize {
			break
		}
		offset += pageSize
	}

	// 按时间倒序（最新在前）
	for i := 0; i < len(matched); i++ {
		for j := i + 1; j < len(matched); j++ {
			if matched[i].Timestamp < matched[j].Timestamp {
				matched[i], matched[j] = matched[j], matched[i]
			}
		}
	}

	d.logger.Info("memory.list_by_session called",
		"session_id", p.SessionID,
		"returned", len(matched))

	return rpc.MemoryListBySessionResult{
		Chunks: matched,
		Count:  len(matched),
	}, nil
}

// chunkHitToMemoryItem converts a core.Chunk to a MemoryChunkItem.
func chunkHitToMemoryItem(c goragcore.Chunk) *rpc.MemoryChunkItem {
	item := &rpc.MemoryChunkItem{
		ID:      c.ID,
		Content: c.Content,
	}

	if c.Metadata != nil {
		if a, ok := c.Metadata["agent_name"].(string); ok {
			item.AgentName = a
		}
		if s, ok := c.Metadata["session_id"].(string); ok {
			item.SessionID = s
		}
		if s, ok := c.Metadata["summary"].(string); ok && s != "" {
			item.Summary = s
		} else {
			item.Summary = c.Title
		}
		if t, ok := c.Metadata["tags"]; ok {
			switch v := t.(type) {
			case []string:
				item.Tags = v
			case []any:
				for _, tag := range v {
					if s, ok := tag.(string); ok {
						item.Tags = append(item.Tags, s)
					}
				}
			}
		}
		if ts, ok := c.Metadata["timestamp"]; ok {
			switch v := ts.(type) {
			case float64:
				item.Timestamp = int64(v)
			case int64:
				item.Timestamp = v
			}
		}
	}

	if item.Summary == "" {
		item.Summary = c.Title
	}

	return item
}

// ---------------------------------------------------------------------------
// memory.update — 更新一条 MemoryChunk
// ---------------------------------------------------------------------------

func (d *Daemon) handleMemoryUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MemoryUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	// 从 indexer 获取当前 chunk，保留未修改的字段
	indexer := mem.Semantic()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	admin, ok := indexer.(goragindexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("indexer does not support admin operations")
	}

	// Use a zero-offset list with a generous page size to find the specific chunk.
	chunks, _, err := admin.List(context.Background(), 0, 1_000_000, nil)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	var existing *goharnessmemory.MemoryChunk
	for _, c := range chunks {
		if c.ID == p.ID {
			chunk := hitToMemoryChunk(c)
			existing = chunk
			break
		}
	}
	if existing == nil {
		return nil, fmt.Errorf("memory chunk %q not found", p.ID)
	}

	// Apply updates
	if p.Summary != "" {
		existing.Summary = p.Summary
	}
	if p.Content != "" {
		existing.Content = p.Content
	}
	if p.Tags != nil {
		existing.Tags = p.Tags
	}

	if err := mem.Update(context.Background(), p.ID, *existing); err != nil {
		return nil, fmt.Errorf("memory update failed: %w", err)
	}

	d.logger.Info("memory.update called", "id", p.ID)

	return map[string]string{"status": "ok", "id": p.ID}, nil
}

// hitToMemoryChunk converts a core.Chunk to a goharness MemoryChunk for update purposes.
func hitToMemoryChunk(c goragcore.Chunk) *goharnessmemory.MemoryChunk {
	chunk := &goharnessmemory.MemoryChunk{
		ID:      c.ID,
		Content: c.Content,
	}

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
