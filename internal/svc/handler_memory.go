package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
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
