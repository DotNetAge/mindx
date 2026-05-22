package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gorag"
	"github.com/DotNetAge/gorag/logging"
)

// MaxFileSize is the maximum file size (in bytes) allowed for indexing.
// Files larger than this are skipped with a warning.
const MaxFileSize = 1 << 20 // 1MB

// DefaultIgnoredDirs lists directories excluded by default from project indexing.
var DefaultIgnoredDirs = map[string]bool{
	".git":       true,
	"node_modules": true,
	".venv":      true,
	"venv":       true,
	"__pycache__": true,
	"vendor":     true,
	"dist":       true,
	"build":      true,
	".mindx":     true,
}

// projectFileEntry tracks a single indexed file's metadata.
type projectFileEntry struct {
	Path   string      `json:"path"`
	Mtime  int64       `json:"mtime"`
	Size   int64       `json:"size"`
	Chunks []chunkInfo `json:"chunks"`
}

type chunkInfo struct {
	ID string `json:"id"`
}

// projectFileCache persists file indexing metadata to disk.
type projectFileCache struct {
	Files map[string]*projectFileEntry `json:"files"`
	mu    sync.Mutex
}

func newProjectFileCache() *projectFileCache {
	return &projectFileCache{
		Files: make(map[string]*projectFileEntry),
	}
}

func (c *projectFileCache) Get(path string) *projectFileEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Files[path]
}

func (c *projectFileCache) Set(entry *projectFileEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Files[entry.Path] = entry
}

func (c *projectFileCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Files, path)
}

// ProjectIndexer synchronizes a project directory into a HybridIndexer.
// It maintains a file mtime cache to only re-index changed files.
//
// Usage:
//
//	pi := NewProjectIndexer(indexer, cachePath, nil)
//	result := pi.Sync(ctx, projectDir)
//	if result.Err != nil { ... }
type ProjectIndexer struct {
	indexer  *gorag.HybridIndexer
	cache    *projectFileCache
	cacheDir string
	ignore   *IgnoreRules
	logger   logging.Logger
}

// ProjectSyncResult summarizes a Sync operation.
type ProjectSyncResult struct {
	Indexed int      // files newly indexed
	Updated int      // files re-indexed due to change
	Skipped int      // files unchanged (cache hit)
	Removed int      // chunks cleaned up from deleted files
	Errors  []string // non-fatal errors grouped by file
	Err     error    // fatal error (operation aborted)
	Elapsed time.Duration
}

// NewProjectIndexer creates a ProjectIndexer.
// If logger is nil, no logging is performed.
func NewProjectIndexer(idx *gorag.HybridIndexer, cacheDir string, logger logging.Logger) *ProjectIndexer {
	return &ProjectIndexer{
		indexer:  idx,
		cache:    newProjectFileCache(),
		cacheDir: cacheDir,
		ignore:   nil, // set per Sync call
		logger:   logger,
	}
}

// Sync scans projectDir and indexes new/changed files.
// Returns the sync result with per-file error details.
func (p *ProjectIndexer) Sync(ctx context.Context, projectDir string) *ProjectSyncResult {
	start := time.Now()
	result := &ProjectSyncResult{}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("project_indexer: resolve project dir: %w", err)
		return result
	}

	// Load rules and cache
	p.ignore = LoadMindxIgnore(absDir)
	if err := p.loadCache(); err != nil && p.logger != nil {
		p.logger.Warn("project_indexer: failed to load cache, starting fresh", "error", err)
	}

	// Walk project dir, collect files
	currentFiles := make(map[string]os.FileInfo)
	walkErr := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get relative path for rule matching
		relPath, err := filepath.Rel(absDir, path)
		if err != nil {
			return nil
		}

		// Skip ignored directories early to avoid walking into them
		if info.IsDir() {
			if p.isDirIgnored(relPath, info) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored files
		if p.ignore.IsIgnored(relPath) {
			return nil
		}

		currentFiles[relPath] = info
		return nil
	})
	if walkErr != nil {
		result.Err = fmt.Errorf("project_indexer: walk project dir: %w", walkErr)
		return result
	}

	// Process each current file
	for relPath, info := range currentFiles {
		// File size limit
		if info.Size() > MaxFileSize {
			result.Skipped++
			if p.logger != nil {
				p.logger.Warn("project_indexer: file too large, skipped", "path", relPath, "size", info.Size())
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
				p.logger.Warn("project_indexer: index failed", "path", relPath, "error", idxErr)
			}
			continue
		}

		// Remove old chunks if file was previously indexed
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
	if saveErr := p.saveCache(); saveErr != nil && p.logger != nil {
		p.logger.Warn("project_indexer: failed to save cache", "error", saveErr)
	}

	result.Elapsed = time.Since(start)
	return result
}

// SyncFiles indexes only the specified files (relative paths) under projectDir.
// This is the incremental counterpart to Sync() — instead of walking the entire
// directory tree, it processes only the files that have changed (e.g., from
// fsnotify events). Deleted files are detected and removed from the cache/index.
//
// relFiles: list of file paths relative to projectDir that have changed.
// deleted: if true, all relFiles are treated as deletions.
func (p *ProjectIndexer) SyncFiles(ctx context.Context, projectDir string, relFiles []string, deleted bool) *ProjectSyncResult {
	start := time.Now()
	result := &ProjectSyncResult{}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		result.Err = fmt.Errorf("project_indexer: resolve project dir: %w", err)
		return result
	}

	// Load rules and cache
	p.ignore = LoadMindxIgnore(absDir)
	if err := p.loadCache(); err != nil && p.logger != nil {
		p.logger.Warn("project_indexer: failed to load cache, starting fresh", "error", err)
	}

	for _, relPath := range relFiles {
		// Clean and normalize
		relPath = filepath.ToSlash(filepath.Clean(relPath))
		if relPath == "." || relPath == "" {
			continue
		}

		// Check if this is a directory (skip — dirs aren't indexed directly)
		absPath := filepath.Join(absDir, relPath)
		info, statErr := os.Stat(absPath)
		if statErr == nil && info.IsDir() {
			continue
		}

		if deleted || os.IsNotExist(statErr) {
			// File deleted: remove from cache and index
			p.removeCachedFile(ctx, relPath, result)
			continue
		}
		if statErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: stat error: %v", relPath, statErr))
			continue
		}

		// Skip ignored files
		if p.ignore.IsIgnored(relPath) {
			result.Skipped++
			continue
		}

		// File size limit
		if info.Size() > MaxFileSize {
			result.Skipped++
			if p.logger != nil {
				p.logger.Warn("project_indexer: file too large, skipped", "path", relPath, "size", info.Size())
			}
			continue
		}

		// Check cache: skip if unchanged
		entry := p.cache.Get(relPath)
		if entry != nil && entry.Mtime == info.ModTime().UnixNano() && entry.Size == info.Size() {
			result.Skipped++
			continue
		}

		// Index the file
		chunks, idxErr := p.indexFile(ctx, absPath)
		if idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relPath, idxErr))
			if p.logger != nil {
				p.logger.Warn("project_indexer: index failed", "path", relPath, "error", idxErr)
			}
			continue
		}

		// Remove old chunks if re-indexing
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

	// Persist cache
	if saveErr := p.saveCache(); saveErr != nil && p.logger != nil {
		p.logger.Warn("project_indexer: failed to save cache", "error", saveErr)
	}

	result.Elapsed = time.Since(start)
	return result
}

// removeCachedFile removes a file's chunks from the index and cache.
func (p *ProjectIndexer) removeCachedFile(ctx context.Context, relPath string, result *ProjectSyncResult) {
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
func (p *ProjectIndexer) indexFile(ctx context.Context, absPath string) ([]chunkInfo, error) {
	chunks, err := p.indexer.AddFile(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("add file: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	infos := make([]chunkInfo, len(chunks))
	for i, c := range chunks {
		infos[i] = chunkInfo{ID: c.ID}
	}
	return infos, nil
}

// removeChunks removes all tracked chunks for a previously indexed file.
func (p *ProjectIndexer) removeChunks(ctx context.Context, chunks []chunkInfo) {
	for _, ci := range chunks {
		if err := p.indexer.Remove(ctx, ci.ID); err != nil && p.logger != nil {
			p.logger.Warn("project_indexer: failed to remove chunk", "id", ci.ID, "error", err)
		}
	}
}

// isDirIgnored checks whether a directory should be skipped entirely during walking.
func (p *ProjectIndexer) isDirIgnored(relPath string, info os.FileInfo) bool {
	// Skip hidden directories
	if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
		// But don't skip .mindxignore's parent — the project root itself can be "." if walking from root
		if relPath != "." {
			return true
		}
	}

	// Default ignored dirs
	if DefaultIgnoredDirs[info.Name()] {
		return true
	}

	// .mindxignore rules for directories
	if p.ignore.IsIgnored(relPath + "/") {
		return true
	}

	return false
}

// --- Cache persistence ---

func (p *ProjectIndexer) cacheFile() string {
	return filepath.Join(p.cacheDir, "project_cache.json")
}

func (p *ProjectIndexer) loadCache() error {
	data, err := os.ReadFile(p.cacheFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, p.cache)
}

func (p *ProjectIndexer) saveCache() error {
	if err := os.MkdirAll(p.cacheDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p.cache, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically via temp file
	tmpPath := p.cacheFile() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, p.cacheFile())
}

