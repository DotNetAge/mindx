package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goreactmemory "github.com/DotNetAge/goreact/memory"
)

type memoryQueryParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	Type     string  `json:"type,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

func (d *Daemon) handleMemoryQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryQueryParams
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

	opts := []goreactmemory.RetrieveOption{}
	if p.Limit > 0 {
		opts = append(opts, goreactmemory.WithMemoryLimit(p.Limit))
	}
	if p.MinScore > 0 {
		opts = append(opts, goreactmemory.WithMinScore(p.MinScore))
	}
	if p.Type != "" {
		switch p.Type {
		case "longterm":
			opts = append(opts, goreactmemory.WithMemoryTypes(goreactmemory.MemoryTypeLongTerm))
		case "session":
			opts = append(opts, goreactmemory.WithMemoryTypes(goreactmemory.MemoryTypeSession))
		}
	}

	records, err := mem.Retrieve(context.Background(), p.Query, opts...)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	if records == nil {
		return []goreactmemory.MemoryRecord{}, nil
	}
	return records, nil
}

type memoryStoreParams struct {
	Title   string   `json:"title,omitempty"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Type    string   `json:"type,omitempty"`
}

func (d *Daemon) handleMemoryStore(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryStoreParams
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

	record := goreactmemory.MemoryRecord{
		Title:     p.Title,
		Content:   p.Content,
		Tags:      p.Tags,
		CreatedAt: time.Now(),
	}
	if p.Type == "session" {
		record.Type = goreactmemory.MemoryTypeSession
	} else {
		record.Type = goreactmemory.MemoryTypeLongTerm
	}

	id, err := mem.Store(context.Background(), record)
	if err != nil {
		return nil, fmt.Errorf("memory store failed: %w", err)
	}

	return map[string]string{"id": id}, nil
}

type memoryDeleteParams struct {
	ID string `json:"id"`
}

func (d *Daemon) handleMemoryDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryDeleteParams
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

type memoryChunksParams struct {
	Page     int    `json:"page,omitempty"`      // 页码，从 1 开始
	PageSize int    `json:"page_size,omitempty"` // 每页条数，默认 50
	DocID    string `json:"doc_id,omitempty"`    // 按文档过滤，"all" 表示全部
}

type memoryChunksResult struct {
	Chunks   []chunkItem `json:"chunks"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	HasMore  bool        `json:"has_more"`
}

type chunkItem struct {
	ID        string         `json:"id"`
	ParentID  string         `json:"parent_id,omitempty"`
	DocID     string         `json:"doc_id,omitempty"`
	MIMEType  string         `json:"mime_type,omitempty"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	ChunkMeta chunkMetaItem  `json:"chunk_meta,omitempty"`
}

type chunkMetaItem struct {
	Index        int      `json:"index"`
	StartPos     int      `json:"start_pos"`
	EndPos       int      `json:"end_pos"`
	HeadingLevel int      `json:"heading_level"`
	HeadingPath  []string `json:"heading_path,omitempty"`
}

func (d *Daemon) handleMemoryChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryChunksParams
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

	indexer := mem.Indexer()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	offset := (p.Page - 1) * p.PageSize
	hits, err := indexer.List(context.Background(), offset, p.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list chunks failed: %w", err)
	}

	chunks := make([]chunkItem, 0, len(hits))
	for _, h := range hits {
		parentID, _ := h.Metadata["parent_id"].(string)
		mimeType, _ := h.Metadata["mime_type"].(string)
		chunks = append(chunks, chunkItem{
			ID:       h.ID,
			ParentID: parentID,
			DocID:    h.DocID,
			MIMEType: mimeType,
			Content:  h.Content,
			Metadata: h.Metadata,
			ChunkMeta: chunkMetaItem{
				Index:        h.ChunkMeta.Index,
				StartPos:     h.ChunkMeta.StartPos,
				EndPos:       h.ChunkMeta.EndPos,
				HeadingLevel: h.ChunkMeta.HeadingLevel,
				HeadingPath:  h.ChunkMeta.HeadingPath,
			},
		})
	}

	hasMore := len(chunks) == p.PageSize

	d.logger.Info("memory.chunks called", "page", p.Page, "page_size", p.PageSize, "returned", len(chunks), "has_more", hasMore)

	return memoryChunksResult{
		Chunks:   chunks,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    offset + len(chunks), // 近似值，List 接口不返回总数
		HasMore:  hasMore,
	}, nil
}

// ---------------------------------------------------------------------------
// memory.get_chunks — 按文档ID获取全部分块（一次性拉取单文档所有Chunk）
// ---------------------------------------------------------------------------

type memoryGetChunksParams struct {
	DocID string `json:"doc_id"` // 必填：文档ID
}

func (d *Daemon) handleMemoryGetChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryGetChunksParams
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

	indexer := mem.Indexer()
	if indexer == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}

	chunks, err := indexer.GetChunks(context.Background(), p.DocID)
	if err != nil {
		return nil, fmt.Errorf("get chunks by doc_id failed: %w", err)
	}

	items := make([]chunkItem, 0, len(chunks))
	for _, c := range chunks {
		meta := map[string]any{}
		if c.Metadata != nil {
			for k, v := range c.Metadata {
				meta[k] = v
			}
		}
		items = append(items, chunkItem{
			ID:       c.ID,
			ParentID: c.ParentID,
			DocID:    c.DocID,
			MIMEType: c.MIMEType,
			Content:  c.Content,
			Metadata: meta,
			ChunkMeta: chunkMetaItem{
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
		DocID  string      `json:"doc_id"`
		Chunks []chunkItem `json:"chunks"`
		Count  int         `json:"count"`
	}{
		DocID:  p.DocID,
		Chunks: items,
		Count:  len(items),
	}, nil
}
