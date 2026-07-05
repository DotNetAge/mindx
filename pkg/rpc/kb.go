package rpc

import "encoding/json"

// KBSearchParams are the params for kb.search.
type KBSearchParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

// FilterCondition mirrors gorag/core.FilterCondition for JSON parsing.
type FilterCondition struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// KBChunksParams are the params for kb.chunks.
type KBChunksParams struct {
	Page     int               `json:"page,omitempty"`
	PageSize int               `json:"page_size,omitempty"`
	Filters  []FilterCondition `json:"filters,omitempty"`
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

func (c *Client) KBChunks(page, pageSize int, filters ...FilterCondition) (json.RawMessage, error) {
	params := KBChunksParams{
		Page: page, PageSize: pageSize,
	}
	if len(filters) > 0 {
		params.Filters = filters
	}
	return c.CallWithTimeout("kb.chunks", params)
}

// KBChunksGetParams are the params for kb.chunks.get.
type KBChunksGetParams struct {
	ID string `json:"id"`
}

func (c *Client) KBChunksGet(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.chunks.get", KBChunksGetParams{ID: id})
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

// KBIndexParams are the params for kb.index.
type KBIndexParams struct {
	Path  string `json:"path"`
	Force bool   `json:"force"`
}

func (c *Client) KBIndex(path string, force bool) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.index", KBIndexParams{Path: path, Force: force})
}

// ── kb.manifest.* ──

// KBManifestGetParams are the params for kb.manifest.get.
type KBManifestGetParams struct {
	SessionID string `json:"session_id"`
}

// KBManifestAddParams are the params for kb.manifest.add.
type KBManifestAddParams struct {
	SessionID string   `json:"session_id"`
	Files     []string `json:"files"`
}

// KBManifestRemoveParams are the params for kb.manifest.remove.
type KBManifestRemoveParams struct {
	SessionID string   `json:"session_id"`
	Files     []string `json:"files"`
}

// KBManifestStartParams are the params for kb.manifest.start.
type KBManifestStartParams struct {
	SessionID string `json:"session_id"`
}

// KBManifestStopParams are the params for kb.manifest.stop.
type KBManifestStopParams struct {
	SessionID string `json:"session_id"`
}

func (c *Client) KBManifestGet(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.manifest.get", KBManifestGetParams{SessionID: sessionID})
}

func (c *Client) KBManifestAdd(sessionID string, files []string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.manifest.add", KBManifestAddParams{SessionID: sessionID, Files: files})
}

func (c *Client) KBManifestRemove(sessionID string, files []string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.manifest.remove", KBManifestRemoveParams{SessionID: sessionID, Files: files})
}

func (c *Client) KBManifestStart(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.manifest.start", KBManifestStartParams{SessionID: sessionID})
}

func (c *Client) KBManifestStop(sessionID string) (json.RawMessage, error) {
	return c.CallWithTimeout("kb.manifest.stop", KBManifestStopParams{SessionID: sessionID})
}
