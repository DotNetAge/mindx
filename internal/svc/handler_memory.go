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

	offset := (p.Page - 1) * p.PageSize
	hits, err := indexer.List(context.Background(), offset, p.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	chunks := make([]rpc.ChunkItem, 0, len(hits))
	for _, h := range hits {
		parentID, _ := h.Metadata["parent_id"].(string)
		mimeType, _ := h.Metadata["mime_type"].(string)
		chunks = append(chunks, rpc.ChunkItem{
			ID:       h.ID,
			ParentID: parentID,
			DocID:    h.DocID,
			MIMEType: mimeType,
			Content:  h.Content,
			Metadata: h.Metadata,
			ChunkMeta: rpc.ChunkMetaItem{
				Index:        h.ChunkMeta.Index,
				StartPos:     h.ChunkMeta.StartPos,
				EndPos:       h.ChunkMeta.EndPos,
				HeadingLevel: h.ChunkMeta.HeadingLevel,
				HeadingPath:  h.ChunkMeta.HeadingPath,
			},
		})
	}

	hasMore := len(chunks) == p.PageSize

	// Get the total count for proper pagination
	total, err := indexer.Count(context.Background())
	if err != nil {
		total = offset + len(chunks) // fallback estimate
	}

	d.logger.Info("memory.chunks called", "page", p.Page, "page_size", p.PageSize, "returned", len(chunks), "total", total, "has_more", hasMore)

	return rpc.MemoryChunksResult{
		Chunks:   chunks,
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

	chunks, err := indexer.GetChunks(context.Background(), p.DocID)
	if err != nil {
		return nil, fmt.Errorf("get chunks by doc_id failed: %w", err)
	}

	items := make([]rpc.ChunkItem, 0, len(chunks))
	for _, c := range chunks {
		meta := map[string]any{}
		if c.Metadata != nil {
			for k, v := range c.Metadata {
				meta[k] = v
			}
		}
		items = append(items, rpc.ChunkItem{
			ID:       c.ID,
			ParentID: c.ParentID,
			DocID:    c.DocID,
			MIMEType: c.MIMEType,
			Content:  c.Content,
			Metadata: meta,
			ChunkMeta: rpc.ChunkMetaItem{
				Index:        c.ChunkMeta.Index,
				StartPos:     c.ChunkMeta.StartPos,
				EndPos:       c.ChunkMeta.EndPos,
				HeadingLevel: c.ChunkMeta.HeadingLevel,
				HeadingPath:  c.ChunkMeta.HeadingPath,
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

	count, err := indexer.Count(context.Background())
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

	// 分页遍历所有 chunk，按 session_id 过滤
	var matched []rpc.MemoryChunkItem
	const pageSize = 200
	offset := 0
	for {
		hits, err := indexer.List(context.Background(), offset, pageSize)
		if err != nil {
			return nil, fmt.Errorf("list chunks failed: %w", err)
		}
		if len(hits) == 0 {
			break
		}
		for _, hit := range hits {
			sessionID, _ := hit.Metadata["session_id"].(string)
			if sessionID != p.SessionID {
				continue
			}
			chunk := chunkHitToMemoryItem(hit)
			if chunk != nil {
				matched = append(matched, *chunk)
			}
		}
		if len(hits) < pageSize {
			break
		}
		offset += pageSize
	}

	// 按时间倒序（最新在前）
	// The indexer may return hits in insertion order;
	// we sort by timestamp descending.
	// Since we don't import sort in this file, we'll
	// do a simple slice sort inline.
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

// chunkHitToMemoryItem converts a gorag Hit to a MemoryChunkItem.
func chunkHitToMemoryItem(hit goragcore.Hit) *rpc.MemoryChunkItem {
	item := &rpc.MemoryChunkItem{
		ID:      hit.ID,
		Content: hit.Content,
	}

	if hit.Metadata != nil {
		if a, ok := hit.Metadata["agent_name"].(string); ok {
			item.AgentName = a
		}
		if s, ok := hit.Metadata["session_id"].(string); ok {
			item.SessionID = s
		}
		if s, ok := hit.Metadata["summary"].(string); ok && s != "" {
			item.Summary = s
		} else {
			item.Summary = hit.Title
		}
		if t, ok := hit.Metadata["tags"]; ok {
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
		if ts, ok := hit.Metadata["timestamp"]; ok {
			switch v := ts.(type) {
			case float64:
				item.Timestamp = int64(v)
			case int64:
				item.Timestamp = v
			}
		}
	}

	if item.Summary == "" {
		item.Summary = hit.Title
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

	// We need the existing chunk to preserve fields we aren't updating.
	// Use a zero-offset list with a generous page size to find the specific chunk.
	hits, err := indexer.List(context.Background(), 0, 1_000_000)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	var existing *goharnessmemory.MemoryChunk
	for _, hit := range hits {
		if hit.ID == p.ID {
			chunk := hitToMemoryChunk(hit)
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

// hitToMemoryChunk converts a gorag Hit to a goharness MemoryChunk for update purposes.
func hitToMemoryChunk(hit goragcore.Hit) *goharnessmemory.MemoryChunk {
	chunk := &goharnessmemory.MemoryChunk{
		ID:      hit.ID,
		Content: hit.Content,
	}

	if hit.Metadata != nil {
		if a, ok := hit.Metadata["agent_name"].(string); ok && a != "" {
			chunk.AgentName = a
		}
		if s, ok := hit.Metadata["session_id"].(string); ok {
			chunk.SessionID = s
		}
		if p, ok := hit.Metadata["project_dir"].(string); ok {
			chunk.ProjectDir = p
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
