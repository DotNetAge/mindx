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
	IDs []string `json:"ids"`
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

// MemorySyncProjectParams are the params for memory.sync_project.
type MemorySyncProjectParams struct {
	ProjectDir string `json:"project_dir"`
}

// MemoryFileStatesParams are the params for memory.file_states.
type MemoryFileStatesParams struct {
	ProjectDir string `json:"project_dir"`
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

func (c *Client) MemoryDelete(ids []string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.delete", MemoryDeleteParams{IDs: ids})
}

func (c *Client) MemoryStats() (json.RawMessage, error) {
	return c.CallWithTimeout("memory.stats", nil)
}

func (c *Client) MemoryChunks(page, pageSize int, docID string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.chunks", MemoryChunksParams{
		Page: page, PageSize: pageSize, DocID: docID,
	})
}

func (c *Client) MemoryGetChunks(docID string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.get_chunks", MemoryGetChunksParams{DocID: docID})
}

func (c *Client) MemoryCount() (json.RawMessage, error) {
	return c.CallWithTimeout("memory.count", nil)
}

func (c *Client) MemorySyncProject(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.sync_project", MemorySyncProjectParams{ProjectDir: projectDir})
}

func (c *Client) MemoryFileStates(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("memory.file_states", MemoryFileStatesParams{ProjectDir: projectDir})
}
