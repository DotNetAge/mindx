package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gorag"
	goragindexer "github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
)

// IndexService synchronizes a project directory into a HybridIndexer.
// It maintains a file mtime cache to only re-index changed files.
//
// Usage:
//
//	svc := NewIndexService(indexer, cachePath, nil)
//	result := svc.Sync(ctx, projectDir)
//	if result.Err != nil { ... }
type IndexService struct {
	indexer    *gorag.HybridIndexer
	cache      *fileCache
	cacheDir   string
	ignore     *IgnoreRules
	logger     logging.Logger
	usageStore session.TokenUsageStore
	modelName  string
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

// NewIndexService creates an IndexService.
// If logger is nil, no logging is performed.
// Options can be provided to configure token usage recording, etc.
func NewIndexService(idx *gorag.HybridIndexer, cacheDir string, logger logging.Logger, opts ...IndexServiceOption) *IndexService {
	p := &IndexService{
		indexer:  idx,
		cache:    newProjectFileCache(),
		cacheDir: cacheDir,
		ignore:   nil, // set per Sync call
		logger:   logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Sync scans projectDir and indexes new/changed files.
// Returns the sync result with per-file error details.
func (p *IndexService) Sync(ctx context.Context, projectDir string) *ProjectSyncResult {
	start := time.Now()
	result := &ProjectSyncResult{}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("index-service: resolve project dir: %w", err)
		return result
	}

	if p.logger != nil {
		p.logger.Info("index-service.sync.start", "dir", absDir)
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

	// Process each current file
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

		absPath := filepath.Join(absDir, relPath)
		chunks, idxErr := p.indexFile(ctx, absPath)
		if idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relPath, idxErr))
			if p.logger != nil {
				p.logger.Warn("index-service: index failed", "path", relPath, "error", idxErr)
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

	// Handle deleted files: remove their chunks
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
	if p.logger != nil {
		p.logger.Info("index-service.sync.done",
			"dir", absDir,
			"indexed", result.Indexed,
			"updated", result.Updated,
			"skipped", result.Skipped,
			"removed", result.Removed,
			"errors", len(result.Errors),
			"elapsed_ms", result.Elapsed.Milliseconds(),
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

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("index-service: resolve project dir: %w", err)
		return result
	}

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

		chunks, idxErr := p.indexFile(ctx, absPath)
		if idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relPath, idxErr))
			if p.logger != nil {
				p.logger.Warn("index-service: index failed", "path", relPath, "error", idxErr)
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

// indexFile reads and indexes a single file, returning all chunk IDs.
func (p *IndexService) indexFile(ctx context.Context, absPath string) ([]chunkInfo, error) {
	// Content quality gate: skip binary / garbage files before they reach the
	// chunker & embedder pipeline.
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if !isValidFileContent(raw) {
		if p.logger != nil {
			p.logger.Warn("index-service: content quality check failed, skipped",
				"path", absPath,
				"bytes", len(raw),
			)
		}
		return nil, nil
	}

	chunks, err := p.indexer.AddFile(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("add file: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	// Record LLM token usage if a TokenUsageStore is configured
	p.recordTokenUsage(ctx)

	infos := make([]chunkInfo, len(chunks))
	for i, c := range chunks {
		infos[i] = chunkInfo{ID: c.ID}
	}
	return infos, nil
}

// recordTokenUsage extracts LLM token usage from the LLMIndexer (if available)
// and writes it to the configured TokenUsageStore.
func (p *IndexService) recordTokenUsage(ctx context.Context) {
	if p.usageStore == nil {
		return
	}
	raw, ok := p.indexer.GetIndexer("llm")
	if !ok {
		return
	}
	llm, ok := raw.(*goragindexer.LLMIndexer)
	if !ok {
		return
	}
	tu := llm.LastTokenUsage()
	if tu == nil {
		return
	}

	record := session.TokenUsageRecord{
		ID:               session.NewRecordID(),
		ModelName:        p.modelName,
		PromptTokens:     tu.PromptTokens,
		CompletionTokens: tu.CompletionTokens,
		TotalTokens:      tu.TotalTokens,
		Timestamp:        time.Now(),
	}
	if err := p.usageStore.Append(ctx, record); err != nil && p.logger != nil {
		p.logger.Warn("index-service: failed to record token usage", "error", err)
	}
}

// removeChunks removes all tracked chunks for a previously indexed file.
func (p *IndexService) removeChunks(ctx context.Context, chunks []chunkInfo) {
	for _, ci := range chunks {
		if err := p.indexer.Remove(ctx, ci.ID); err != nil && p.logger != nil {
			p.logger.Warn("index-service: failed to remove chunk", "id", ci.ID, "error", err)
		}
	}
}

// isDirIgnored checks whether a directory should be skipped entirely during walking.
func (p *IndexService) _isDirIgnored(relPath string, info os.FileInfo) bool {
	// Skip hidden directories
	if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
		if relPath != "." {
			return true
		}
	}

	if DefaultIgnoredDirs[info.Name()] {
		return true
	}

	if p.ignore.IsIgnored(relPath + "/") {
		return true
	}

	return false
}
