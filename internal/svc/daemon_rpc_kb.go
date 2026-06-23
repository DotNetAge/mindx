package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	goragcore "github.com/DotNetAge/gorag/core"
	goragquery "github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/mindx/pkg/kbwatch"
)

// ---------------------------------------------------------------------------
// kb.chunks — 分页获取知识库（GraphIndexer）Chunk 列表
// ---------------------------------------------------------------------------

type kbChunksParams struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

type kbChunksResult struct {
	Chunks   []chunkItem `json:"chunks"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	HasMore  bool        `json:"has_more"`
}

func (d *Daemon) handleKBChunks(_ context.Context, params json.RawMessage) (any, error) {
	var p kbChunksParams
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
		return nil, fmt.Errorf("knowledge base not available (GraphIndexer not initialized)")
	}

	offset := (p.Page - 1) * p.PageSize
	hits, err := d.graphIndexer.List(context.Background(), offset, p.PageSize)
	if err != nil {
		return nil, fmt.Errorf("kb list chunks failed: %w", err)
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

	total, err := d.graphIndexer.Count(context.Background())
	if err != nil {
		total = offset + len(chunks)
	}

	d.logger.Info("kb.chunks called", "page", p.Page, "page_size", p.PageSize, "returned", len(chunks), "total", total, "has_more", hasMore)

	return kbChunksResult{
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

type kbStatsResult struct {
	TotalFiles   int `json:"total_files"`
	IndexedFiles int `json:"indexed_files"`
	TotalChunks  int `json:"total_chunks"`
}

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

	return kbStatsResult{
		TotalFiles:   totalFiles,
		IndexedFiles: indexedFiles,
		TotalChunks:  totalChunks,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.search — 对知识库执行语义搜索（向量检索 + 图融合）
// ---------------------------------------------------------------------------

type kbSearchParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

type kbSearchHit struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	DocID    string         `json:"doc_id"`
	Metadata map[string]any `json:"metadata"`
}

func (d *Daemon) handleKBSearch(_ context.Context, params json.RawMessage) (any, error) {
	var p kbSearchParams
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
		return nil, fmt.Errorf("knowledge base not available (GraphIndexer not initialized)")
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
		return nil, fmt.Errorf("knowledge base not available (no indexer initialized)")
	}

	pi := kbwatch.NewIndexService(indexer, cacheDir, d.logger)
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
