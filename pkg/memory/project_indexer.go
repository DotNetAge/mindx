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
	"unicode"

	"github.com/DotNetAge/gorag"
	"github.com/DotNetAge/gorag/logging"
)

// MaxFileSize is the maximum file size (in bytes) allowed for indexing.
// Files larger than this are skipped with a warning.
const MaxFileSize = 1 << 20 // 1MB

// DefaultIgnoredDirs lists directories excluded by default from project indexing.
var DefaultIgnoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".mindx":       true,
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

// FileState describes the indexing status of a single file.
type FileState string

const (
	FileStateIndexed FileState = "indexed"  // exists on disk and matches cache
	FileStateChanged FileState = "changed"  // exists on disk but mtime/size differs from cache
	FileStateNew     FileState = "new"      // exists on disk but not in cache
	FileStateRemoved FileState = "removed"  // in cache but no longer on disk
	FileStateSkipped FileState = "skipped"  // excluded by ignore rules, size limit, or content check
)

// FileStateInfo holds per-file scanning result from ScanFileStates.
type FileStateInfo struct {
	Path        string `json:"path"`
	State       FileState `json:"state"`
	Size        int64     `json:"size,omitempty"`
	Mtime       int64     `json:"mtime,omitempty"`
	CachedSize  int64     `json:"cached_size,omitempty"`
	CachedMtime int64     `json:"cached_mtime,omitempty"`
	Error       string    `json:"error,omitempty"`
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

	if p.logger != nil {
		p.logger.Info("project_indexer.sync.start", "dir", absDir)
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
	if p.logger != nil {
		p.logger.Info("project_indexer.sync.done",
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

// ScanFileStates performs a read-only scan of projectDir and returns the
// indexing state of each discoverable file without performing any actual
// indexing. This allows the UI to show which files are indexed, changed,
// new, or removed before the user decides to start the indexing service.
func (p *ProjectIndexer) ScanFileStates(ctx context.Context, projectDir string) ([]FileStateInfo, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("project_indexer: resolve project dir: %w", err)
	}

	ignore := LoadMindxIgnore(absDir)

	// Load cache (best-effort — may not exist yet)
	if err := p.loadCache(); err != nil && p.logger != nil {
		p.logger.Warn("project_indexer.scan: failed to load cache", "error", err)
	}

	// Walk project dir collecting current files
	currentFiles := make(map[string]os.FileInfo)
	if walkErr := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		relPath, rErr := filepath.Rel(absDir, path)
		if rErr != nil {
			return nil
		}
		if info.IsDir() {
			if isDirIgnored(relPath, info, ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore.IsIgnored(relPath) {
			return nil
		}
		currentFiles[relPath] = info
		return nil
	}); walkErr != nil {
		return nil, fmt.Errorf("project_indexer: walk project dir: %w", walkErr)
	}

	var states []FileStateInfo

	// Check current files against cache
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

// isValidFileContentForScan performs a quick content check (size + null bytes)
// without reading the entire file. Returns false for binary or empty files.
func isValidFileContentForScan(baseDir, relPath string) bool {
	// Quick check: if the file is likely binary, skip the full read
	fullPath := filepath.Join(baseDir, relPath)
	// Read a small header to detect null bytes (binary indicator)
	header := make([]byte, 512)
	f, err := os.Open(fullPath)
	if err != nil {
		return false
	}
	defer f.Close()
	n, _ := f.Read(header)
	if n == 0 {
		return false
	}
	for _, b := range header[:n] {
		if b == 0 {
			return false
		}
	}
	return true
}

// isDirIgnored checks whether a directory should be skipped during walking.
// This is a package-level helper (no ProjectIndexer receiver needed).
func isDirIgnored(relPath string, info os.FileInfo, ignore *IgnoreRules) bool {
	if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
		if relPath != "." {
			return true
		}
	}
	if DefaultIgnoredDirs[info.Name()] {
		return true
	}
	if ignore.IsIgnored(relPath + "/") {
		return true
	}
	return false
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

// minReadableContentChars is the minimum number of printable text characters
// a file must contain to be considered indexable. Files below this threshold
// (e.g., lock files, one-line logs, editor swaps) are skipped.
const minReadableContentChars = 20

// minPrintableRatio is the minimum ratio of printable Unicode text characters
// (letters, digits, CJK ideographs, etc.) above which file content is
// considered non-binary / non-garbage. Files with too many control characters
// or non-text bytes (e.g., binary blobs) are skipped.
const minPrintableRatio = 0.50

// isValidFileContent performs lightweight content-quality checks on raw file
// content before it enters the indexing pipeline. Returns true when the
// content looks like readable text worth indexing.
//
// Checks performed:
//  1. Binary detection — null bytes indicate a non-text file.
//  2. Printable ratio — the fraction of printable Unicode categories in the
//     decoded string must meet minPrintableRatio.
//  3. Minimum meaningful characters — after stripping whitespace and symbols,
//     the remaining text must be at least minReadableContentChars long.
func isValidFileContent(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	// 1. Binary detection: null byte present → treat as binary
	for _, b := range raw {
		if b == 0 {
			return false
		}
	}

	// 2. Printable ratio check
	s := string(raw)
	totalRunes := 0
	printableRunes := 0

	for _, r := range s {
		totalRunes++
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsPunct(r) ||
			unicode.IsSymbol(r) || r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			printableRunes++
		}
	}

	if totalRunes == 0 {
		return false
	}

	ratio := float64(printableRunes) / float64(totalRunes)
	if ratio < minPrintableRatio {
		return false
	}

	// 3. Minimum meaningful characters (letters + digits)
	meaningful := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			meaningful++
		}
	}

	return meaningful >= minReadableContentChars
}

// indexFile reads and indexes a single file, returning all chunk IDs.
func (p *ProjectIndexer) indexFile(ctx context.Context, absPath string) ([]chunkInfo, error) {
	// Content quality gate: skip binary / garbage files before they reach the
	// chunker & embedder pipeline.
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if !isValidFileContent(raw) {
		if p.logger != nil {
			p.logger.Warn("project_indexer: content quality check failed, skipped",
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
