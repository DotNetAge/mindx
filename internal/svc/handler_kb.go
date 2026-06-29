package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragquery "github.com/DotNetAge/gorag/v2/query"
	"github.com/DotNetAge/mindx/pkg/indexing"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// kb.chunks — 分页获取知识库（GraphIndexer）Chunk 列表
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.KBChunksParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize <= 0 || p.PageSize > 200 {
		p.PageSize = 50
	}

	if d.graphIndexer == nil {
		reason := "GraphIndexer not initialized"
		if d.graphIndexerErr != nil {
			reason = d.graphIndexerErr.Error()
		}
		d.logger.Warn("kb.chunks rejected: " + reason)
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	offset := (p.Page - 1) * p.PageSize
	var (
		hits  []goragcore.Hit
		total int
		err   error
	)
	if len(p.Filters) > 0 {
		filters := make([]goragcore.FilterCondition, len(p.Filters))
		for i, f := range p.Filters {
			filters[i] = goragcore.FilterCondition{
				Key:   f.Key,
				Type:  f.Type,
				Value: f.Value,
			}
		}
		hits, total, err = d.graphIndexer.ListFiltered(context.Background(), offset, p.PageSize, filters)
	} else {
		hits, err = d.graphIndexer.List(context.Background(), offset, p.PageSize)
		if err == nil {
			total, err = d.graphIndexer.Count(context.Background())
		}
	}
	if err != nil {
		return nil, fmt.Errorf("kb list chunks failed: %w", err)
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

	d.logger.Info("kb.chunks called", "page", p.Page, "page_size", p.PageSize, "returned", len(chunks), "total", total, "has_more", hasMore)

	return rpc.MemoryChunksResult{
		Chunks:   chunks,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
		HasMore:  hasMore,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.sync_project — 对指定目录执行全量文件扫描和索引
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBSyncProject(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	absDir, absErr := filepath.Abs(p.ProjectDir)
	if absErr != nil {
		return nil, fmt.Errorf("resolve project dir: %w", absErr)
	}

	// Clear the indexing cache for this directory to force full re-index
	cacheBase := filepath.Join(d.app.Settings().DataDir(), "kb-cache")
	cacheDir := filepath.Join(cacheBase, sanitizeDirName(absDir))
	if err := os.RemoveAll(cacheDir); err != nil {
		d.logger.Warn("failed to remove cache dir, re-index may be partial", "cache_dir", cacheDir, "error", err)
	} else {
		d.logger.Info("cache cleared, forcing full re-index", "cache_dir", cacheDir)
	}

	// Perform full sync via FileWatchService.SyncDir
	d.kbWatch.SyncDir(context.Background(), absDir)

	d.logger.Info("kb.sync_project completed",
		"project_dir", p.ProjectDir,
	)

	return map[string]string{"status": "completed", "project_dir": p.ProjectDir}, nil
}

// ---------------------------------------------------------------------------
// kb.stats — 获取知识库文件索引进度统计
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBStats(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	// Get total chunks count from GraphIndexer (KB)
	totalChunks := 0
	if d.graphIndexer != nil {
		if cnt, err := d.graphIndexer.Count(context.Background()); err == nil {
			totalChunks = cnt
		}
	}

	// Get file indexing stats from FileWatchService
	var totalFiles, indexedFiles int
	if d.kbWatch != nil {
		absDir, absErr := filepath.Abs(p.ProjectDir)
		if absErr == nil {
			st := d.kbWatch.Status()
			if dirState, ok := st.IndexStates[absDir]; ok {
				totalFiles = dirState.TotalFiles
				indexedFiles = dirState.IndexedFiles
			}
		}
	}

	d.logger.Info("kb.stats called",
		"project_dir", p.ProjectDir,
		"total_files", totalFiles,
		"indexed_files", indexedFiles,
		"total_chunks", totalChunks,
	)

	return rpc.KBStatsResult{
		TotalFiles:   totalFiles,
		IndexedFiles: indexedFiles,
		TotalChunks:  totalChunks,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.search — 对知识库执行语义搜索（向量检索 + 图融合）
// ---------------------------------------------------------------------------

type kbSearchHit struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	DocID    string         `json:"doc_id"`
	Metadata map[string]any `json:"metadata"`
}

func (d *Daemon) handleKBSearch(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.KBSearchParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if p.Limit <= 0 {
		p.Limit = 10
	}

	if d.graphIndexer == nil {
		reason := "GraphIndexer not initialized"
		if d.graphIndexerErr != nil {
			reason = d.graphIndexerErr.Error()
		}
		d.logger.Warn("kb.search rejected: " + reason)
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	// 构造 GraphQuery，然后清除 TextQuery（跳过 LLM→Cypher），走向量检索+图融合路径
	gq := goragquery.NewGraphQuery(p.Query)
	if graphQ, ok := gq.(*goragquery.GraphQuery); ok {
		graphQ.SetTextQuery("")
		graphQ.SetLimit(p.Limit)
		gq = graphQ
	}
	q := gq

	hits, err := d.graphIndexer.Search(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("kb search failed: %w", err)
	}

	results := make([]kbSearchHit, 0, len(hits))
	for _, h := range hits {
		score := float64(h.Score)
		if p.MinScore > 0 && score < p.MinScore {
			continue
		}
		meta := h.Metadata
		if meta == nil {
			meta = make(map[string]any)
		}
		// 将 Entities/Relations 信息写入 metadata 供前端使用
		if len(h.Entities) > 0 {
			entityNames := make([]string, 0, len(h.Entities))
			for _, e := range h.Entities {
				if name, ok := e.Properties["name"].(string); ok {
					entityNames = append(entityNames, name)
				} else {
					entityNames = append(entityNames, e.ID)
				}
			}
			meta["entity_names"] = entityNames
		}
		results = append(results, kbSearchHit{
			ID:       h.ID,
			Content:  h.Content,
			Score:    score,
			DocID:    h.DocID,
			Metadata: meta,
		})
	}

	d.logger.Info("kb.search completed",
		"query", p.Query,
		"limit", p.Limit,
		"hits", len(results),
	)

	return results, nil
}

// ---------------------------------------------------------------------------
// kb.file_states — 扫描项目目录文件状态（只读，不索引）
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBFileStates(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	// Determine cache dir from FileWatchService
	var cacheDir string
	if d.kbWatch != nil {
		cacheBase := filepath.Join(d.app.Settings().DataDir(), "kb-cache")
		cacheDir = filepath.Join(cacheBase, sanitizeDirName(p.ProjectDir))
	}

	// Prefer GraphIndexer for KB scanning, fall back to memory semantic indexer
	var indexer goragcore.Indexer
	if d.graphIndexer != nil {
		indexer = d.graphIndexer
	} else if d.sharedMemory != nil {
		indexer = d.sharedMemory.Semantic()
	} else {
		reason := "no indexer initialized"
		if d.graphIndexerErr != nil {
			reason = "GraphIndexer not initialized: " + d.graphIndexerErr.Error()
		}
		d.logger.Warn("kb.file_states rejected: " + reason)
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	pi := indexing.NewIndexService(indexer, cacheDir, d.logger)
	states, err := pi.ScanFileStates(context.Background(), p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("file states scan failed: %w", err)
	}

	// Count by state
	counts := map[string]int{
		"indexed": 0,
		"changed": 0,
		"new":     0,
		"removed": 0,
		"skipped": 0,
		"total":   len(states),
	}
	for _, s := range states {
		counts[string(s.State)]++
	}

	d.logger.Info("kb.file_states completed",
		"project_dir", p.ProjectDir,
		"total", len(states),
		"indexed", counts["indexed"],
		"changed", counts["changed"],
		"new", counts["new"],
		"removed", counts["removed"],
		"skipped", counts["skipped"],
	)

	return map[string]any{
		"states": states,
		"counts": counts,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.index — 对单个文件或目录执行索引（--force 强制重索引）
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBIndex(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path  string `json:"path"`
		Force bool   `json:"force"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}
	if d.graphIndexer == nil {
		reason := "GraphIndexer not initialized"
		if d.graphIndexerErr != nil {
			reason = d.graphIndexerErr.Error()
		}
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	absPath, err := filepath.Abs(p.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	cacheBase := filepath.Join(d.app.Settings().DataDir(), "kb-cache")

	if info.IsDir() {
		// ── Directory indexing ──
		cacheDir := filepath.Join(cacheBase, indexing.SanitizeDirName(absPath))
		if p.Force {
			if err := os.RemoveAll(cacheDir); err != nil {
				d.logger.Warn("failed to clear cache for dir", "dir", absPath, "error", err)
			} else {
				d.logger.Info("cache cleared for directory (force)", "dir", absPath)
			}
		}
		d.kbWatch.SyncDir(context.Background(), absPath)
		return map[string]any{"status": "completed", "path": absPath, "type": "directory"}, nil
	}

	// ── Single file indexing ──
	parentDir := filepath.Dir(absPath)
	relPath := filepath.Base(absPath)

	// Broadcast "indexing" event to frontend
	if d.gw != nil {
		d.gw.BroadcastNotification("file_indexing", map[string]any{
			"type": "file_indexing",
			"data": map[string]string{
				"file":      relPath,
				"directory": parentDir,
				"state":     "indexing",
			},
		})
	}

	// Set IndexStateStore to "indexing" BEFORE the actual indexing, so the
	// frontend progress bar shows 0/1 (instead of 0/0) during processing.
	if d.indexStateStore != nil {
		d.indexStateStore.SetIndexing(parentDir, 1)
	}

	pi := indexing.NewIndexService(d.graphIndexer,
		filepath.Join(cacheBase, indexing.SanitizeDirName(parentDir)),
		d.logger,
	)

	if p.Force {
		pi.ClearCacheEntry(relPath)
		d.logger.Info("cache entry cleared (force)", "file", relPath)
	}

	result := pi.SyncFiles(context.Background(), parentDir, []string{relPath}, false)

	if len(result.Errors) > 0 {
		if d.gw != nil {
			d.gw.BroadcastNotification("file_indexing", map[string]any{
				"type": "file_indexing",
				"data": map[string]string{
					"file":      relPath,
					"directory": parentDir,
					"state":     "error",
				},
			})
		}
		if d.indexStateStore != nil {
			d.indexStateStore.SetFailed(parentDir, result.Errors[0])
		}
		return nil, fmt.Errorf("index failed: %s", result.Errors[0])
	}

	status := "skipped"
	if result.Indexed > 0 {
		status = "indexed"
	} else if result.Updated > 0 {
		status = "updated"
	}

	// Update IndexStateStore so the frontend progress bar reflects real data.
	// The entry was already created (SetIndexing above), so we just promote it.
	if d.indexStateStore != nil && (result.Indexed > 0 || result.Updated > 0) {
		d.indexStateStore.IncrementIndexedFiles(parentDir)
		// IncrementIndexedFiles only works when state=="indexing", which we
		// set above. After incrementing, mark as completed with real stats.
		d.indexStateStore.SetCompletedWithStats(parentDir, result.Indexed, result.Skipped, 0, 0, 0,
			[]indexing.CompletedFileRecord{{
				Path:   relPath,
				Chunks: result.Indexed,
			}},
		)
	}

	// Broadcast "indexed" event to frontend (AFTER IndexStateStore update,
	// so the 2s polling timer can pick up the completed state).
	if d.gw != nil && (result.Indexed > 0 || result.Updated > 0) {
		d.gw.BroadcastNotification("file_indexing", map[string]any{
			"type": "file_indexing",
			"data": map[string]string{
				"file":      relPath,
				"directory": parentDir,
				"state":     "indexed",
			},
		})
	}

	d.logger.Info("kb.index completed",
		"path", relPath,
		"status", status,
		"force", p.Force,
	)

	return map[string]any{
		"status": status,
		"path":   absPath,
		"type":   "file",
	}, nil
}
