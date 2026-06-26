package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	"github.com/DotNetAge/mindx/pkg/indexing"
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
// filewatch.start — 启动文件监控服务
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchStart(_ context.Context, params json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	if d.kbWatch.IsRunning() {
		return map[string]string{"status": "already_running"}, nil
	}

	// Persist enabled state to config so it survives daemon restart.
	if c := d.app.Config(); c != nil && !c.AutoIndexing {
		c.AutoIndexing = true
		if err := c.Save(); err != nil {
			d.logger.Warn("filewatch.start: failed to persist auto_indexing config", "error", err)
		}
	}

	// Before starting, restore watches for all existing sessions that have a
	// project_dir. This ensures directories are registered even if sessions were
	// created before the filewatch service was available (e.g. no LLM model at
	// session creation time) or if the watchlist was cleared.
	d.restoreSessionWatches()

	// Create a cancellable context for this watch session.
	ctx, cancel := context.WithCancel(context.Background())
	d.watchCancel = cancel

	d.logger.Info("filewatch.start: starting filewatch service")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("filewatch.start: goroutine panic", fmt.Errorf("%v", r))
			}
		}()
		if err := d.kbWatch.Start(ctx); err != nil {
			d.logger.Warn("filewatch.start: service exited with error", "error", err)
		}
	}()

	// Wait briefly for the eventLoop goroutine to set isRunning=true.
	for i := 0; i < 50; i++ {
		if d.kbWatch.IsRunning() {
			return map[string]string{"status": "started"}, nil
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Timed out — the goroutine might still be starting, but report success anyway
	// since Start() is non-blocking. The frontend should refresh status again.
	d.logger.Warn("filewatch.start: took longer than expected, returning started optimistically")
	return map[string]string{"status": "started"}, nil
}

// ---------------------------------------------------------------------------
// filewatch.stop — 停止文件监控服务
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchStop(_ context.Context, _ json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	if !d.kbWatch.IsRunning() {
		return map[string]string{"status": "already_stopped"}, nil
	}

	d.logger.Info("filewatch.stop: stopping filewatch service")

	// Cancel the watch context to unblock Start().
	if d.watchCancel != nil {
		d.watchCancel()
		d.watchCancel = nil
	}
	d.kbWatch.Stop()

	// Persist disabled state to config so it survives daemon restart.
	if c := d.app.Config(); c != nil && c.AutoIndexing {
		c.AutoIndexing = false
		if err := c.Save(); err != nil {
			d.logger.Warn("filewatch.stop: failed to persist auto_indexing config", "error", err)
		}
	}

	d.logger.Info("filewatch.stop: filewatch service stopped")

	return map[string]string{"status": "stopped"}, nil
}

// ---------------------------------------------------------------------------
// filewatch.remove — 从监控列表中移除指定目录
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchRemove(_ context.Context, params json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	var p rpc.FilewatchRemoveParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Dir == "" {
		return nil, fmt.Errorf("dir is required")
	}

	d.logger.Info("filewatch.remove: removing directory", "dir", p.Dir)

	if err := d.kbWatch.RemoveWatchByDir(p.Dir); err != nil {
		return nil, fmt.Errorf("filewatch.remove: %w", err)
	}

	d.logger.Info("filewatch.remove: directory removed", "dir", p.Dir)

	return map[string]string{"status": "removed", "dir": p.Dir}, nil
}

// ---------------------------------------------------------------------------
// filewatch.status — 查询文件监控服务状态
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchStatus(_ context.Context, _ json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return map[string]any{
			"available": false,
			"running":   false,
		}, nil
	}

	status := d.kbWatch.Status()

	d.logger.Info("filewatch.status query",
		"running", status.Running,
		"watched_dirs", status.Watched,
	)
	for dir, st := range status.IndexStates {
		d.logger.Debug("filewatch.status state",
			"dir", dir,
			"state", st.State,
			"indexed", st.IndexedFiles,
			"total", st.TotalFiles,
		)
	}

	// Reconcile stale index_state with actual indexer cache data.
	// When auto-indexing was never started, or crashed before completing,
	// index_state stays at "pending / 0/0" even though files may have been
	// indexed (e.g. during a previous successful run).  Cross-reference each
	// stale entry against its on-disk index_cache.json so the dialog always
	// shows real data.
	if status.CacheBase != "" {
		for dir, st := range status.IndexStates {
			if st == nil {
				continue
			}
			// Only reconcile entries that look like they were never synced.
			if st.State != "pending" && (st.State != "indexing" || st.TotalFiles != 0 || st.IndexedFiles != 0) {
				continue
			}
			cacheDir := filepath.Join(status.CacheBase, indexing.SanitizeDirName(dir))
			cache := indexing.NewProjectFileCache()
			if err := cache.LoadFromFile(cacheDir); err != nil || len(cache.Files) == 0 {
				continue
			}
			// Cache has real data — promote the state to completed so the
			// frontend displays meaningful file counts and chunk info.
			completed := make([]indexing.CompletedFileRecord, 0, len(cache.Files))
			for _, entry := range cache.Files {
				completed = append(completed, indexing.CompletedFileRecord{
					Path:   entry.Path,
					Chunks: len(entry.Chunks),
				})
			}
			st.State = "completed"
			st.IndexedFiles = len(cache.Files)
			st.TotalFiles = len(cache.Files)
			st.CompletedFiles = completed
			st.CompletedAt = time.Now().Unix()
			d.logger.Info("filewatch.status: reconciled stale index_state from cache",
				"dir", dir,
				"cached_files", len(cache.Files),
			)
		}
	}

	// Filter out ignored files from the per-directory failed lists before
	// returning the status to the frontend.
	for dir, st := range status.IndexStates {
		if st == nil || len(st.IgnoredFiles) == 0 || len(st.FailedFiles) == 0 {
			continue
		}
		ignored := make(map[string]bool, len(st.IgnoredFiles))
		for _, f := range st.IgnoredFiles {
			ignored[f] = true
		}
		filtered := make([]indexing.FailedFileRecord, 0, len(st.FailedFiles))
		for _, rec := range st.FailedFiles {
			if !ignored[rec.Path] {
				filtered = append(filtered, rec)
			}
		}
		status.IndexStates[dir].FailedFiles = filtered
	}

	return map[string]any{
		"available":   true,
		"running":     status.Running,
		"watched":     status.Watched,
		"cache_dir":   status.CacheBase,
		"index_state": status.IndexStates,
	}, nil
}

// ---------------------------------------------------------------------------
// filewatch.retry-failed — 重新索引指定失败文件
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchRetryFailed(_ context.Context, params json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	var p rpc.FilewatchRetryFailedParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Dir == "" || len(p.Files) == 0 {
		return nil, fmt.Errorf("dir and files are required")
	}

	d.logger.Info("filewatch.retry-failed: retrying files",
		"dir", p.Dir,
		"files", p.Files,
		"count", len(p.Files),
	)

	// Clear cache entries for the failed files so they are re-indexed.
	pi := d.kbWatch.GetIndexer(p.Dir)
	if pi != nil {
		for _, f := range p.Files {
			pi.ClearCacheEntry(f)
		}
	}

	// Remove from the failed list (retry will either succeed or fail again).
	if d.kbWatch.IndexState() != nil {
		d.kbWatch.IndexState().RemoveFailedFiles(p.Dir, p.Files)
	}

	// Re-index the files. SyncFiles handles the actual LLM indexing.
	if pi != nil {
		result := pi.SyncFiles(context.Background(), p.Dir, p.Files, false)
		d.logger.Info("filewatch.retry-failed: sync result",
			"dir", p.Dir,
			"indexed", result.Indexed,
			"updated", result.Updated,
			"errors", len(result.Errors),
		)
		if len(result.Errors) > 0 && d.kbWatch.IndexState() != nil {
			// Re-record failures with new timestamps from result.Errors.
			st := d.kbWatch.IndexState().Get(p.Dir)
			if st != nil {
				now := time.Now().Unix()
				for _, errStr := range result.Errors {
					// errStr format: "filepath: error message"
					parts := strings.SplitN(errStr, ": ", 2)
					fpath := parts[0]
					st.FailedFiles = append(st.FailedFiles, indexing.FailedFileRecord{
						Path:      fpath,
						Error:     errStr,
						Timestamp: now,
					})
				}
			}
			return map[string]any{"status": "partial", "errors": len(result.Errors)}, nil
		}
		return map[string]any{"status": "ok", "indexed": result.Indexed}, nil
	}

	return map[string]any{"status": "ok"}, nil
}

// ---------------------------------------------------------------------------
// filewatch.ignore-failed — 将失败文件标记为忽略
// ---------------------------------------------------------------------------

func (d *Daemon) handleFilewatchIgnoreFailed(_ context.Context, params json.RawMessage) (any, error) {
	if d.kbWatch == nil {
		return nil, fmt.Errorf("filewatch service not available")
	}

	var p rpc.FilewatchIgnoreFailedParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Dir == "" || len(p.Files) == 0 {
		return nil, fmt.Errorf("dir and files are required")
	}

	d.logger.Info("filewatch.ignore-failed: ignoring files",
		"dir", p.Dir,
		"files", p.Files,
	)

	if d.kbWatch.IndexState() != nil {
		d.kbWatch.IndexState().IgnoreFailedFiles(p.Dir, p.Files)
	}

	return map[string]any{"status": "ok"}, nil
}
