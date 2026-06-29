package indexing

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/logging"
)

// maxFileIndexTimeout is the maximum wall-clock time spent indexing a single
// file, including all LLM retries. Files exceeding this limit are counted as
// errors and the sync continues with the next file — preventing one oversized
// or slow file from blocking the entire directory sync.
const maxFileIndexTimeout = 15 * time.Minute

// IndexService synchronizes a project directory into a core.Indexer.
// It maintains a file mtime cache to only re-index changed files.
//
// Usage:
//
//	svc := NewIndexService(indexer, cachePath, nil)
//	result := svc.Sync(ctx, projectDir)
//	if result.Err != nil { ... }
type IndexService struct {
	indexer    goragcore.Indexer
	cache      *fileCache
	cacheDir   string
	ignore     *IgnoreRules
	logger     logging.Logger
	usageStore session.TokenUsageStore
	modelName  string

	// SyncStepCallback is called for each file during Sync.
	// Set by FileWatchService.SyncDir to broadcast per-file progress to WebUI.
	SyncStepCallback func(absPath, relPath, state string) // "indexing" | "indexed"

	// maxConcurrency limits the number of files indexed simultaneously
	// during a full directory scan. Defaults to DefaultConcurrency (3).
	maxConcurrency int
}

// IndexServiceOption configures an IndexService.
type IndexServiceOption func(*IndexService)

// WithTokenUsageStore sets the TokenUsageStore for recording LLM token usage
// after each file indexing operation. modelName is the LLM model identifier
// to record in the usage records.
func WithTokenUsageStore(store session.TokenUsageStore, modelName string) IndexServiceOption {
	return func(p *IndexService) {
		p.usageStore = store
		p.modelName = modelName
	}
}

// WithMaxConcurrency sets the maximum number of files to index concurrently
// during a full directory scan. Values ≤ 1 disable concurrent indexing.
func WithMaxConcurrency(n int) IndexServiceOption {
	return func(p *IndexService) {
		if n > 0 {
			p.maxConcurrency = n
		}
	}
}

// NewIndexService creates an IndexService.
// If logger is nil, no logging is performed.
// Options can be provided to configure token usage recording, etc.
func NewIndexService(idx goragcore.Indexer, cacheDir string, logger logging.Logger, opts ...IndexServiceOption) *IndexService {
	p := &IndexService{
		indexer:        idx,
		cache:          NewProjectFileCache(),
		cacheDir:       cacheDir,
		ignore:         nil, // set per Sync call
		logger:         logger,
		maxConcurrency: DefaultConcurrency,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Sync scans projectDir and indexes new/changed files.
// Returns the sync result with per-file error details.
// Files are indexed concurrently (up to maxConcurrency) and per-file
// failures are isolated — a single slow or failing file does not block
// the rest of the directory scan.
func (p *IndexService) Sync(ctx context.Context, projectDir string) *ProjectSyncResult {
	start := time.Now()
	result := &ProjectSyncResult{}

	// No indexer configured — skip the entire scan. The file cache is not
	// updated, so the next Sync (when an indexer becomes available) will
	// pick up all files as new/changed.
	if p.indexer == nil {
		return result
	}

	// Reset entity/rel counters on GraphIndexer for this sync cycle.
	if gi, ok := p.indexer.(*goragindexer.GraphIndexer); ok && gi != nil {
		gi.ResetEntityStats()
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("index-service: resolve project dir: %w", err)
		return result
	}

	// Same regionID computation as Sync().
	regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(absDir)))

	// Compute a stable regionID for this directory.
	// All files in this directory share the same regionID, which is stored
	// as metadata on each chunk. This decouples the region identifier from
	// individual file paths — if a file moves within the same directory,
	// only source_file changes, not region_id.

	if p.logger != nil {
		p.logger.Info("index-service.sync.start",
			"dir", absDir,
			"concurrency", p.maxConcurrency,
		)
	}

	// Load rules and cache
	p.ignore = LoadMindxIgnore(absDir)
	if err := p.cache.LoadFromFile(p.cacheDir); err != nil && p.logger != nil {
		p.logger.Warn("index-service: failed to load cache, starting fresh", "error", err)
	}

	// Walk project dir, collect files
	currentFiles, walkErr := walkProjectDir(ctx, absDir, p.ignore)
	if walkErr != nil {
		result.Err = fmt.Errorf("index-service: walk project dir: %w", walkErr)
		return result
	}

	// ── Phase 1: Collect job list ────────────────────────────────────────────
	// Filter files for indexing by size and cache hit. This is sequential,
	// single-threaded, and fast (no LLM calls).

	type fileJob struct {
		relPath string
		info    os.FileInfo
		absPath string
		cached  *projectFileEntry // nil = new file
	}

	var jobs []fileJob
	for relPath, info := range currentFiles {
		if info.Size() > MaxFileSize {
			result.Skipped++
			if p.logger != nil {
				p.logger.Warn("index-service: file too large, skipped", "path", relPath, "size", info.Size())
			}
			continue
		}

		entry := p.cache.Get(relPath)
		if entry != nil && entry.Mtime == info.ModTime().UnixNano() && entry.Size == info.Size() {
			result.Skipped++
			continue // unchanged
		}

		jobs = append(jobs, fileJob{
			relPath: relPath,
			info:    info,
			absPath: filepath.Join(absDir, relPath),
			cached:  entry,
		})
	}

	// ── Phase 2: Concurrently index files ────────────────────────────────────
	// Each job fires its own pre/post events through SyncStepCallback.
	// A semaphore bounds concurrency; a mutex protects shared result/cache.

	type fileOutcome struct {
		relPath string
		chunks  []chunkInfo
		idxErr  error
		elapsed time.Duration
		skipped bool // true when chunks were empty (counts as skipped)
		updated bool // true when the file already existed in cache
	}

	outcomes := make(chan fileOutcome, len(jobs))
	sem := make(chan struct{}, p.maxConcurrency)
	var jobsWg sync.WaitGroup

	for _, job := range jobs {
		jobsWg.Add(1)

		go func(j fileJob) {
			sem <- struct{}{}        // acquire — blocks this goroutine when at capacity
			defer func() { <-sem }() // release (runs first on exit, LIFO)
			defer jobsWg.Done()

			fileStart := time.Now()

			// Fire pre-index event (before LLM call)
			if p.SyncStepCallback != nil {
				p.SyncStepCallback(j.absPath, j.relPath, "indexing")
			}

			// Per-file timeout so a single large/slow file cannot block the
			// entire directory sync. The context timeout bounds the total time
			// including LLM retries inside indexFile → AddFile.
			fileCtx, fileCancel := context.WithTimeout(ctx, maxFileIndexTimeout)
			fileCtx = goragindexer.WithRegionID(fileCtx, regionID)
			chunks, idxErr := p.indexFile(fileCtx, j.absPath)
			fileCancel()

			elapsed := time.Since(fileStart)

			// Log per-file timing + outcome
			if p.logger != nil {
				if idxErr != nil {
					errType := classifyError(idxErr)
					p.logger.Warn("index-service.sync.file",
						"path", j.relPath,
						"elapsed_ms", elapsed.Milliseconds(),
						"error_type", errType,
						"error", idxErr,
					)
				} else if len(chunks) == 0 {
					p.logger.Warn("index-service.sync.file",
						"path", j.relPath,
						"elapsed_ms", elapsed.Milliseconds(),
						"result", "skipped (no chunks)",
					)
				} else {
					p.logger.Info("index-service.sync.file",
						"path", j.relPath,
						"elapsed_ms", elapsed.Milliseconds(),
						"chunks", len(chunks),
					)
				}
			}

			// Check if old chunks need removal (file was updated, not new)
			updated := false
			if idxErr == nil && len(chunks) > 0 && j.cached != nil && len(j.cached.Chunks) > 0 {
				// Remove old chunks outside the mutex — this is an I/O
				// operation that should not block other goroutines.
				p.removeChunks(ctx, j.cached.Chunks)
				updated = true
			}

			outcomes <- fileOutcome{
				relPath: j.relPath,
				chunks:  chunks,
				idxErr:  idxErr,
				elapsed: elapsed,
				skipped: len(chunks) == 0,
				updated: updated,
			}
		}(job)
	}

	// Close outcomes channel when ALL workers finish
	go func() {
		jobsWg.Wait()
		close(outcomes)
	}()

	// ── Phase 3: Collect outcomes (sequential, post-LLM work) ────────────────
	// Update cache and result counters. Since the LLM-heavy work is done in
	// Phase 2, this phase is lightweight and runs sequentially.
	for oc := range outcomes {
		if oc.idxErr != nil {
			errType := classifyError(oc.idxErr)
			errMsg := fmt.Sprintf("%s: [%s] %v", oc.relPath, errType, oc.idxErr)
			result.Errors = append(result.Errors, errMsg)
			result.FailedFiles = append(result.FailedFiles, FileIndexError{
				Path:      oc.relPath,
				Error:     errMsg,
				Elapsed:   oc.elapsed,
				Timestamp: time.Now(),
			})
			// Notify the frontend about the failure
			if p.SyncStepCallback != nil {
				absPath := filepath.Join(absDir, oc.relPath)
				p.SyncStepCallback(absPath, oc.relPath, "error")
			}
			continue
		}

		if oc.skipped {
			result.Skipped++
			continue
		}

		if oc.updated {
			result.Updated++
		} else {
			result.Indexed++
		}

		// Record successfully indexed file with timing info
		result.CompletedFiles = append(result.CompletedFiles, CompletedFileInfo{
			Path:      oc.relPath,
			Chunks:    len(oc.chunks),
			Elapsed:   oc.elapsed,
			Timestamp: time.Now(),
		})

		p.cache.Set(&projectFileEntry{
			Path:   oc.relPath,
			Mtime:  currentFiles[oc.relPath].ModTime().UnixNano(),
			Size:   currentFiles[oc.relPath].Size(),
			Chunks: oc.chunks,
		})

		if p.SyncStepCallback != nil {
			absPath := filepath.Join(absDir, oc.relPath)
			p.SyncStepCallback(absPath, oc.relPath, "indexed")
		}
	}

	// ── Phase 4: Handle deleted files ────────────────────────────────────────
	for relPath, entry := range p.cache.Files {
		if _, exists := currentFiles[relPath]; !exists && len(entry.Chunks) > 0 {
			p.removeChunks(ctx, entry.Chunks)
			p.cache.Delete(relPath)
			result.Removed++
		}
	}

	// Persist cache
	if saveErr := p.cache.SaveToFile(p.cacheDir); saveErr != nil && p.logger != nil {
		p.logger.Warn("index-service: failed to save cache", "error", saveErr)
	}

	result.Elapsed = time.Since(start)

	// Collect entity/rel stats from GraphIndexer.
	if gi, ok := p.indexer.(*goragindexer.GraphIndexer); ok {
		result.EntitiesCreated, result.RelsCreated = gi.EntityStats()
	}

	if p.logger != nil {
		p.logger.Info("index-service.sync.done",
			"dir", absDir,
			"indexed", result.Indexed,
			"updated", result.Updated,
			"skipped", result.Skipped,
			"removed", result.Removed,
			"errors", len(result.Errors),
			"elapsed_ms", result.Elapsed.Milliseconds(),
			"concurrency", p.maxConcurrency,
		)
	}
	return result
}

// SyncFiles indexes only the specified files (relative paths) under projectDir.
// This is the incremental counterpart to Sync() — instead of walking the entire
// directory tree, it processes only the files that have changed (e.g., from
// fsnotify events). Deleted files are detected and removed from the cache/index.
//
// relFiles: list of file paths relative to projectDir that have changed.
// deleted: if true, all relFiles are treated as deletions.
func (p *IndexService) SyncFiles(ctx context.Context, projectDir string, relFiles []string, deleted bool) *ProjectSyncResult {
	start := time.Now()
	result := &ProjectSyncResult{}

	// No indexer configured — skip all indexing but don't mark files as
	// processed (cache remains unchanged, so they'll be re-evaluated later).
	if p.indexer == nil {
		result.Skipped = len(relFiles)
		return result
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("index-service: resolve project dir: %w", err)
		return result
	}

	// Stable regionID for this directory (same computation as Sync).
	regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(absDir)))

	// Load rules and cache
	p.ignore = LoadMindxIgnore(absDir)
	if err := p.cache.LoadFromFile(p.cacheDir); err != nil && p.logger != nil {
		p.logger.Warn("index-service: failed to load cache, starting fresh", "error", err)
	}

	for _, relPath := range relFiles {
		relPath = filepath.ToSlash(filepath.Clean(relPath))
		if relPath == "." || relPath == "" {
			continue
		}

		absPath := filepath.Join(absDir, relPath)
		info, statErr := os.Stat(absPath)
		if statErr == nil && info.IsDir() {
			continue
		}

		if deleted || os.IsNotExist(statErr) {
			p.removeCachedFile(ctx, relPath, result)
			continue
		}
		if statErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: stat error: %v", relPath, statErr))
			continue
		}

		if p.ignore.IsIgnored(relPath) {
			result.Skipped++
			continue
		}

		if info.Size() > MaxFileSize {
			result.Skipped++
			if p.logger != nil {
				p.logger.Warn("index-service: file too large, skipped", "path", relPath, "size", info.Size())
			}
			continue
		}

		entry := p.cache.Get(relPath)
		if entry != nil && entry.Mtime == info.ModTime().UnixNano() && entry.Size == info.Size() {
			result.Skipped++
			continue
		}

		// Per-file timeout so a single large/slow file cannot block the
		// entire incremental sync. The context timeout bounds the total time
		// including LLM retries inside indexFile → AddFile.
		fileCtx, fileCancel := context.WithTimeout(ctx, maxFileIndexTimeout)
		fileCtx = goragindexer.WithRegionID(fileCtx, regionID)
		chunks, idxErr := p.indexFile(fileCtx, absPath)
		fileCancel()
		if idxErr != nil {
			if errors.Is(idxErr, context.DeadlineExceeded) {
				if p.logger != nil {
					p.logger.Warn("index-service: file indexing timed out (SyncFiles)", "path", relPath, "timeout", maxFileIndexTimeout)
				}
			}
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relPath, idxErr))
			if p.logger != nil {
				p.logger.Warn("index-service: index failed", "path", relPath, "error", idxErr)
			}
			continue
		}

		if len(chunks) == 0 {
			result.Skipped++
			if p.logger != nil {
				p.logger.Warn("index-service: file produced no chunks", "path", relPath)
			}
			continue
		}

		if entry != nil && len(entry.Chunks) > 0 {
			p.removeChunks(ctx, entry.Chunks)
			result.Updated++
		} else {
			result.Indexed++
		}

		p.cache.Set(&projectFileEntry{
			Path:   relPath,
			Mtime:  info.ModTime().UnixNano(),
			Size:   info.Size(),
			Chunks: chunks,
		})
	}

	if saveErr := p.cache.SaveToFile(p.cacheDir); saveErr != nil && p.logger != nil {
		p.logger.Warn("index-service: failed to save cache", "error", saveErr)
	}

	result.Elapsed = time.Since(start)
	return result
}

// ScanFileStates performs a read-only scan of projectDir and returns the
// indexing state of each discoverable file without performing any actual
// indexing. This allows the UI to show which files are indexed, changed,
// new, or removed before the user decides to start the indexing service.
func (p *IndexService) ScanFileStates(ctx context.Context, projectDir string) ([]FileStateInfo, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("index-service: resolve project dir: %w", err)
	}

	ignore := LoadMindxIgnore(absDir)

	// Load cache (best-effort — may not exist yet)
	if err := p.cache.LoadFromFile(p.cacheDir); err != nil && p.logger != nil {
		p.logger.Warn("index-service.scan: failed to load cache", "error", err)
	}

	// Walk project dir collecting current files
	currentFiles, walkErr := walkProjectDir(ctx, absDir, ignore)
	if walkErr != nil {
		return nil, fmt.Errorf("index-service: walk project dir: %w", walkErr)
	}

	var states []FileStateInfo

	for relPath, info := range currentFiles {
		entry := p.cache.Get(relPath)
		state := FileStateInfo{
			Path:  relPath,
			Size:  info.Size(),
			Mtime: info.ModTime().UnixNano(),
		}

		if info.Size() > MaxFileSize || !isValidFileContentForScan(absDir, relPath) {
			state.State = FileStateSkipped
			if info.Size() > MaxFileSize {
				state.Error = fmt.Sprintf("file exceeds max size (%d > %d bytes)", info.Size(), MaxFileSize)
			} else {
				state.Error = "content quality check failed (binary or too short)"
			}
		} else if entry == nil {
			state.State = FileStateNew
		} else if entry.Mtime == info.ModTime().UnixNano() && entry.Size == info.Size() {
			state.State = FileStateIndexed
		} else {
			state.State = FileStateChanged
			state.CachedSize = entry.Size
			state.CachedMtime = entry.Mtime
		}
		states = append(states, state)
	}

	// Check for removed files (in cache but not on disk)
	for relPath, entry := range p.cache.Files {
		if _, exists := currentFiles[relPath]; !exists {
			states = append(states, FileStateInfo{
				Path:        relPath,
				State:       FileStateRemoved,
				CachedSize:  entry.Size,
				CachedMtime: entry.Mtime,
			})
		}
	}

	return states, nil
}

// removeCachedFile removes a file's chunks from the index and cache.
func (p *IndexService) removeCachedFile(ctx context.Context, relPath string, result *ProjectSyncResult) {
	entry := p.cache.Get(relPath)
	if entry != nil {
		if len(entry.Chunks) > 0 {
			p.removeChunks(ctx, entry.Chunks)
		}
		p.cache.Delete(relPath)
		result.Removed++
	}
}

// ClearCacheEntry removes the cached indexing state for a single file,
// causing the next Sync or SyncFiles call to re-index it. This is used
// by the retry-failed mechanism to retry specific files.
func (p *IndexService) ClearCacheEntry(relPath string) {
	p.cache.Delete(relPath)
}
