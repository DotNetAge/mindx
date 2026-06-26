package rpc

import "encoding/json"

// KBSearchParams are the params for kb.search.
type KBSearchParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

// KBChunksParams are the params for kb.chunks.
type KBChunksParams struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

// KBStatsParams are the params for kb.stats.
type KBStatsParams struct {
	ProjectDir string `json:"project_dir"`
}

// KBSyncProjectParams are the params for kb.sync_project.
type KBSyncProjectParams struct {
	ProjectDir string `json:"project_dir"`
}

// KBFileStatesParams are the params for kb.file_states.
type KBFileStatesParams struct {
	ProjectDir string `json:"project_dir"`
}

func (c *Client) KBSearch(query string, limit int, minScore float64) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.search", KBSearchParams{
		Query: query, Limit: limit, MinScore: minScore,
	})
}

func (c *Client) KBChunks(page, pageSize int) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.chunks", KBChunksParams{
		Page: page, PageSize: pageSize,
	})
}

func (c *Client) KBStats(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.stats", KBStatsParams{ProjectDir: projectDir})
}

func (c *Client) KBSyncProject(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.sync_project", KBSyncProjectParams{ProjectDir: projectDir})
}

// KBStatsResult is the result for kb.stats.
type KBStatsResult struct {
	TotalFiles   int `json:"total_files"`
	IndexedFiles int `json:"indexed_files"`
	TotalChunks  int `json:"total_chunks"`
}

func (c *Client) KBFileStates(projectDir string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.file_states", KBFileStatesParams{ProjectDir: projectDir})
}
