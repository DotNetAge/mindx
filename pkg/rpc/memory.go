package rpc

import "encoding/json"

// MemoryQueryParams are the params for memory.query.
type MemoryQueryParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

// MemoryStoreParams are the params for memory.store.
type MemoryStoreParams struct {
	Content     string `json:"content"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
}

// MemoryDeleteParams are the params for memory.delete.
type MemoryDeleteParams struct {
	ID string `json:"id"`
}

// MemoryChunksParams are the params for memory.chunks.
type MemoryChunksParams struct {
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
	DocID    string `json:"doc_id,omitempty"`
}

// MemoryGetChunksParams are the params for memory.get_chunks.
type MemoryGetChunksParams struct {
	DocID string `json:"doc_id"`
}

func (c *Client) MemoryQuery(query string, limit int, minScore float64) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.query", MemoryQueryParams{
		Query: query, Limit: limit, MinScore: minScore,
	})
}

func (c *Client) MemoryStore(content, title, description, source string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.store", MemoryStoreParams{
		Content: content, Title: title, Description: description, Source: source,
	})
}

func (c *Client) MemoryDelete(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.delete", MemoryDeleteParams{ID: id})
}

func (c *Client) MemoryChunks(page, pageSize int, docID string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.chunks", MemoryChunksParams{
		Page: page, PageSize: pageSize, DocID: docID,
	})
}

func (c *Client) MemoryGetChunks(docID string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.get_chunks", MemoryGetChunksParams{DocID: docID})
}

// MemoryChunksResult is the result for memory.chunks.
type MemoryChunksResult struct {
	Chunks   []ChunkItem `json:"chunks"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	HasMore  bool        `json:"has_more"`
}

// ChunkItem is a single chunk in the result set.
type ChunkItem struct {
	ID        string         `json:"id"`
	ParentID  string         `json:"parent_id,omitempty"`
	DocID     string         `json:"doc_id,omitempty"`
	MIMEType  string         `json:"mime_type,omitempty"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	ChunkMeta ChunkMetaItem  `json:"chunk_meta,omitempty"`
}

// ChunkMetaItem holds chunk-level metadata.
type ChunkMetaItem struct {
	Index        int      `json:"index"`
	StartPos     int      `json:"start_pos"`
	EndPos       int      `json:"end_pos"`
	HeadingLevel int      `json:"heading_level"`
	HeadingPath  []string `json:"heading_path,omitempty"`
}

// MemoryCountResult is the result for memory.count.
type MemoryCountResult struct {
	Count int `json:"count"`
}

func (c *Client) MemoryCount() (json.RawMessage, error) {
	return c.CallWithTimeout("memory.count", nil)
}
