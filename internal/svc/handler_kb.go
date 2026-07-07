package svc

import (
	"context"
	"crypto/sha256"
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

	hasMore := offset+len(chunks) < total

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
// kb.chunks.get — 按 ID 获取单个 Chunk 详情（JSON）
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBChunksGet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	if d.graphIndexer == nil {
		reason := "GraphIndexer not initialized"
		if d.graphIndexerErr != nil {
			reason = d.graphIndexerErr.Error()
		}
		d.logger.Warn("kb.chunks.get rejected: " + reason)
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	// Iterate through vectorDB pages to find the chunk by ID
	// (vectorDB stores full content; graphDB SearchByChunkIDs does not return content)
	pageSize := 200
	offset := 0
	for {
		hits, count, err := d.graphIndexer.ListFiltered(context.Background(), offset, pageSize, nil)
		if err != nil {
			return nil, fmt.Errorf("list chunks failed: %w", err)
		}
		for _, h := range hits {
			if h.ID == p.ID {
				parentID, _ := h.Metadata["parent_id"].(string)
				mimeType, _ := h.Metadata["mime_type"].(string)
				return rpc.ChunkItem{
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
				}, nil
			}
		}
		if offset+pageSize >= count {
			break
		}
		offset += pageSize
	}

	return nil, fmt.Errorf("chunk not found: %s", p.ID)
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

	// Get file indexing stats from Indexer
	var totalFiles, indexedFiles int
	absDir, absErr := filepath.Abs(p.ProjectDir)
	if absErr == nil {
		pi, piErr := d.getIndexer(absDir)
		if piErr != nil {
			return nil, piErr
		}
		stats, err := pi.Count(context.Background(), absDir)
		if err == nil {
			for _, count := range stats {
				totalFiles += count
			}
			indexedFiles = stats[indexing.FileIndexed]
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

	// Apply region filter (source_file prefix match via region_id)
	if p.Region != "" {
		absDir, err := filepath.Abs(p.Region)
		if err != nil {
			return nil, fmt.Errorf("resolve region path: %w", err)
		}
		regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(absDir))))
		if graphQ, ok := q.(*goragquery.GraphQuery); ok {
			graphQ.AddFilter("region_id", regionID)
		}
	}

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

func (d *Daemon) handleKBFileStates(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	// Get manifest data from Indexer
	absDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	pi, getErr := d.getIndexer(absDir)
	if getErr != nil {
		return nil, getErr
	}
	manifestFiles, _ := pi.ListAllFiles(ctx, "")
	manifestMap := make(map[string]*indexing.FileMeta, len(manifestFiles))
	for _, meta := range manifestFiles {
		manifestMap[meta.Path] = meta
	}

	// Walk filesystem to discover all files on disk
	diskFiles := make(map[string]os.FileInfo)
	filepath.Walk(absDir, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		diskFiles[walkPath] = info
		return nil
	})

	// Merge: for each file on disk, determine state
	type fileStateEntry struct {
		Path  string `json:"path"`
		State string `json:"state"`
	}
	states := make([]fileStateEntry, 0, len(diskFiles))
	counts := map[string]int{
		"indexed": 0, "new": 0, "changed": 0, "skipped": 0, "total": 0,
	}

	for absPath, info := range diskFiles {
		counts["total"]++
		if meta, ok := manifestMap[absPath]; ok {
			switch meta.State {
			case indexing.FileIndexed:
				// Compare mtime/size to detect changes
				if info.ModTime().UnixNano() != meta.Mtime || info.Size() != meta.Size {
					states = append(states, fileStateEntry{Path: absPath, State: "changed"})
					counts["changed"]++
				} else {
					states = append(states, fileStateEntry{Path: absPath, State: "indexed"})
					counts["indexed"]++
				}
			case indexing.FilePending, indexing.FileEnqueued, indexing.FileProcessing:
				states = append(states, fileStateEntry{Path: absPath, State: "indexed"}) // treat as indexed
				counts["indexed"]++
			case indexing.FileFailed:
				states = append(states, fileStateEntry{Path: absPath, State: "error"})
				counts["error"]++
			}
		} else {
			states = append(states, fileStateEntry{Path: absPath, State: "new"})
			counts["new"]++
		}
	}

	d.logger.Info("kb.file_states completed",
		"project_dir", p.ProjectDir,
		"total", counts["total"],
		"indexed", counts["indexed"],
		"new", counts["new"],
		"changed", counts["changed"],
	)

	return map[string]any{
		"states": states,
		"counts": counts,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.count — 返回分片总数（可选按 region 路径前缀过滤）
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBCount(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.KBCountParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	if d.graphIndexer == nil {
		reason := "GraphIndexer not initialized"
		if d.graphIndexerErr != nil {
			reason = d.graphIndexerErr.Error()
		}
		d.logger.Warn("kb.count rejected: " + reason)
		return nil, fmt.Errorf("knowledge base not available: %s", reason)
	}

	if p.Region != "" {
		total, err := d.graphIndexer.CountByRegion(context.Background(), p.Region)
		if err != nil {
			return nil, fmt.Errorf("kb count by region failed: %w", err)
		}
		return map[string]any{"total": total, "region": p.Region}, nil
	}

	total, err := d.graphIndexer.Count(context.Background())
	if err != nil {
		return nil, fmt.Errorf("kb count failed: %w", err)
	}
	return map[string]any{"total": total}, nil
}
