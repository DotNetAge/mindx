package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/logging"
)

const (
	maxFileIndexTimeout = 15 * time.Minute

	// minPrintableRatio is the minimum ratio of printable characters required
	// to consider file content valid for indexing.
	minPrintableRatio = 0.5

	// minReadableContentChars is the minimum count of meaningful characters
	// (letters + digits) required to consider file content valid for indexing.
	minReadableContentChars = 20
)

// Indexer is the single public entry point for project file indexing.
// Each instance is bound to one project directory.
type Indexer struct {
	projectDir string
	baseDir    string // data directory for boltDB storage
	indexer    goragcore.Indexer
	manifest   *manifestStore
	ignore     *ignoreRules
	logger     logging.Logger
	usageStore session.TokenUsageStore
	modelName  string
	callbacks  *IndexerCallbacks

	// FIFO queue worker state
	mu       sync.Mutex
	running  bool
	notify   chan struct{}
	stop     chan struct{}
	stopOnce sync.Once

	// current processing state (for Status)
	processing string
}

// NewIndexer creates a new Indexer bound to a project directory.
// Returns an error if the manifest store cannot be opened.
func NewIndexer(projectDir string, indexer goragcore.Indexer, baseDir string, logger logging.Logger, opts ...IndexerOption) (*Indexer, error) {
	ix := &Indexer{
		projectDir: projectDir,
		baseDir:    baseDir,
		indexer:    indexer,
		logger:     logger,
		notify:     make(chan struct{}, 1),
		stop:       make(chan struct{}),
		callbacks:  &IndexerCallbacks{},
	}

	for _, opt := range opts {
		opt(ix)
	}

	// Open boltDB manifest store
	manifest, err := openManifest(projectDir, baseDir)
	if err != nil {
		return nil, fmt.Errorf("open manifest store: %w", err)
	}
	ix.manifest = manifest

	// Load ignore rules
	ix.ignore = loadIgnoreRules(projectDir)

	// Auto-start the worker loop — runs continuously, picks up enqueued files
	ix.running = true
	go ix.workerLoop(context.Background())

	return ix, nil
}

// WithTokenUsageStore configures the token usage store for recording LLM costs.
func WithTokenUsageStore(store session.TokenUsageStore, modelName string) IndexerOption {
	return func(ix *Indexer) {
		ix.usageStore = store
		ix.modelName = modelName
	}
}

// ── Lifecycle ──

// Start launches the internal FIFO queue worker goroutine.
func (ix *Indexer) Start(ctx context.Context) {
	ix.mu.Lock()
	if ix.running {
		ix.mu.Unlock()
		return
	}
	ix.running = true
	ix.mu.Unlock()

	go ix.workerLoop(ctx)
}

// Stop signals the worker to stop gracefully.
func (ix *Indexer) Stop() {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if !ix.running {
		return
	}
	ix.running = false
	close(ix.stop)
}

// Close releases all resources (boltDB connection).
func (ix *Indexer) Close() error {
	ix.Stop()
	if ix.manifest != nil {
		return ix.manifest.close()
	}
	return nil
}

// ── Manifest Operations ──

// Add adds files to the manifest. Returns the number of files actually added.
// Skips files that match ignore rules, don't exist on disk, or are already
// indexed with matching mtime/size.
func (ix *Indexer) Add(ctx context.Context, files ...string) int {
	if ix.manifest == nil {
		return 0
	}

	added := 0
	for _, absPath := range files {
		absPath = filepath.Clean(absPath)

		// Generate relative path for ignore checking
		relPath, err := filepath.Rel(ix.projectDir, absPath)
		if err != nil {
			if ix.logger != nil {
				ix.logger.Error("indexer: failed to compute relative path", fmt.Errorf("%w", err), "path", absPath, "projectDir", ix.projectDir)
			}
			continue
		}

		// Skip ignored files
		if ix.ignore != nil && ix.ignore.isIgnored(relPath) {
			continue
		}

		// Stat the file on disk
		info, statErr := os.Stat(absPath)
		if statErr != nil || info.IsDir() {
			if statErr != nil && ix.logger != nil {
				ix.logger.Error("indexer: failed to stat file", fmt.Errorf("%w", statErr), "path", absPath)
			}
			continue
		}

		// Size limit check
		if info.Size() > MaxFileSize {
			if ix.logger != nil {
				ix.logger.Info("indexer: file too large, skipped", "path", absPath, "size", info.Size())
			}
			continue
		}

		// Check if already in manifest with matching mtime/size
		existing, getErr := ix.manifest.get(absPath)
		if getErr == nil && existing != nil &&
			existing.Mtime == info.ModTime().UnixNano() &&
			existing.Size == info.Size() &&
			existing.State != FilePending &&
			existing.State != FileEnqueued &&
			existing.State != FileProcessing {
			// Already indexed and unchanged
			continue
		}

		// Write or update as pending
		now := time.Now().Unix()
		if existing != nil {
			// Clear old chunks if file content changed
			if existing.Mtime != info.ModTime().UnixNano() || existing.Size != info.Size() {
				if len(existing.ChunkIDs) > 0 {
					ix.removeChunks(ctx, existing.ChunkIDs)
				}
			}
			existing.State = FilePending
			existing.Mtime = info.ModTime().UnixNano()
			existing.Size = info.Size()
			existing.Error = ""
			existing.UpdatedAt = now
			if err := ix.manifest.put(existing); err != nil && ix.logger != nil {
				ix.logger.Error("indexer: failed to update file in manifest", fmt.Errorf("%w", err), "path", absPath)
			}
		} else {
			meta := &FileMeta{
				Path:      absPath,
				State:     FilePending,
				Mtime:     info.ModTime().UnixNano(),
				Size:      info.Size(),
				UpdatedAt: now,
			}
			if err := ix.manifest.put(meta); err != nil && ix.logger != nil {
				ix.logger.Error("indexer: failed to add file to manifest", fmt.Errorf("%w", err), "path", absPath)
			}
		}
		added++

		// Trigger callback
		if ix.callbacks.OnFileAdded != nil {
			ix.callbacks.OnFileAdded(ctx, absPath)
		}
	}
	return added
}

// Enqueue moves files from Pending to Enqueued state and wakes the worker.
// If files is empty, all Pending files are enqueued.
func (ix *Indexer) Enqueue(ctx context.Context, files ...string) int {
	if ix.manifest == nil {
		return 0
	}

	var count int
	if len(files) == 0 {
		var err error
		count, err = ix.manifest.movePendingToEnqueued()
		if err != nil && ix.logger != nil {
			ix.logger.Error("indexer: enqueue all failed", fmt.Errorf("%w", err))
		}
	} else {
		var err error
		count, err = ix.manifest.moveToEnqueuedByPaths(files)
		if err != nil && ix.logger != nil {
			ix.logger.Error("indexer: enqueue files failed", fmt.Errorf("%w", err), "files", files)
		}
	}

	if count > 0 {
		// Non-blocking send to wake worker
		select {
		case ix.notify <- struct{}{}:
		default:
		}
	}
	return count
}

// ── Other Operations ──

// Summarize generates a Region summary for the given directory.
// Currently this sets the project_dir metadata in the manifest.
// RegionIndexer integration will be added when the daemon is refactored.
func (ix *Indexer) Summarize(ctx context.Context, dir string) error {
	if ix.manifest == nil {
		return fmt.Errorf("manifest not available")
	}

	// Set region metadata
	if err := ix.manifest.setMeta("project_dir", ix.projectDir); err != nil && ix.logger != nil {
		ix.logger.Error("indexer: failed to set project_dir metadata", fmt.Errorf("%w", err))
	}

	return nil
}

// RemoveFile removes a file from the manifest and cleans up its chunks.
func (ix *Indexer) RemoveFile(ctx context.Context, path string) error {
	if ix.manifest == nil {
		return fmt.Errorf("manifest not available")
	}

	path = filepath.Clean(path)

	// If it's Processing, skip (design rule)
	existing, err := ix.manifest.get(path)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	if existing.State == FileProcessing {
		return fmt.Errorf("cannot remove file while indexing: %s", path)
	}

	// Remove chunks if any
	if len(existing.ChunkIDs) > 0 {
		ix.removeChunks(ctx, existing.ChunkIDs)
	}

	// Delete from manifest
	removed, err := ix.manifest.delete(path)
	if err != nil {
		return err
	}

	if removed != nil && ix.callbacks.OnFileRemoved != nil {
		ix.callbacks.OnFileRemoved(ctx, path)
	}

	return nil
}

// ── Query Methods ──

// GetFile returns the FileMeta for a single path, or nil if not in manifest.
func (ix *Indexer) GetFile(ctx context.Context, path string) (*FileMeta, error) {
	if ix.manifest == nil {
		return nil, fmt.Errorf("manifest not available")
	}
	return ix.manifest.get(filepath.Clean(path))
}

// ListFiles returns files directly under dir (non-recursive), optionally filtered by state.
func (ix *Indexer) ListFiles(ctx context.Context, dir string, states ...FileState) ([]*FileMeta, error) {
	if ix.manifest == nil {
		return nil, fmt.Errorf("manifest not available")
	}

	dir = filepath.Clean(dir)
	prefix := dir + string(filepath.Separator)

	all, err := ix.manifest.list()
	if err != nil {
		return nil, err
	}

	stateSet := make(map[FileState]bool, len(states))
	for _, s := range states {
		stateSet[s] = true
	}

	result := make([]*FileMeta, 0, len(all))
	for _, m := range all {
		if !strings.HasPrefix(m.Path, prefix) {
			continue
		}
		// Non-recursive: check no further separator after the dir prefix
		rel := strings.TrimPrefix(m.Path, prefix)
		if strings.Contains(rel, string(filepath.Separator)) {
			continue
		}
		if len(states) > 0 && !stateSet[m.State] {
			continue
		}
		result = append(result, m)
	}
	return result, nil
}

// ListAllFiles returns all files under dir (recursive), optionally filtered by state.
func (ix *Indexer) ListAllFiles(ctx context.Context, dir string, states ...FileState) ([]*FileMeta, error) {
	if ix.manifest == nil {
		return nil, fmt.Errorf("manifest not available")
	}

	dir = filepath.Clean(dir)

	all, err := ix.manifest.list()
	if err != nil {
		return nil, err
	}

	stateSet := make(map[FileState]bool, len(states))
	for _, s := range states {
		stateSet[s] = true
	}

	result := make([]*FileMeta, 0, len(all))
	for _, m := range all {
		if !strings.HasPrefix(m.Path, dir) {
			continue
		}
		if len(states) > 0 && !stateSet[m.State] {
			continue
		}
		result = append(result, m)
	}
	return result, nil
}

// Count returns file counts grouped by state for files under the given directory.
func (ix *Indexer) Count(ctx context.Context, dir string) (map[FileState]int, error) {
	if ix.manifest == nil {
		return nil, fmt.Errorf("manifest not available")
	}

	all, err := ix.manifest.list()
	if err != nil {
		return nil, err
	}

	result := map[FileState]int{
		FilePending:    0,
		FileEnqueued:   0,
		FileProcessing: 0,
		FileIndexed:    0,
		FileFailed:     0,
	}

	dir = filepath.Clean(dir)
	for _, m := range all {
		if strings.HasPrefix(m.Path, dir) {
			result[m.State]++
		}
	}
	return result, nil
}

// Status returns the current runtime state.
func (ix *Indexer) Status(ctx context.Context) IndexerStatus {
	ix.mu.Lock()
	running := ix.running
	processing := ix.processing
	ix.mu.Unlock()

	stats := ix.manifest.countByState()

	// Count total chunks
	totalChunks := 0
	if err := ix.manifest.forEach(func(meta *FileMeta) bool {
		if meta.State == FileIndexed {
			totalChunks += meta.Chunks
		}
		return true
	}); err != nil && ix.logger != nil {
		ix.logger.Error("indexer: failed to iterate manifest for chunk count", fmt.Errorf("%w", err))
	}

	return IndexerStatus{
		ProjectDir:   ix.projectDir,
		Running:      running,
		PendingCount: stats[FilePending],
		Enqueued:     stats[FileEnqueued],
		Processing:   processing,
		DoneCount:    stats[FileIndexed],
		ErrorCount:   stats[FileFailed],
		TotalChunks:  totalChunks,
	}
}

// SetCallbacks registers event callbacks. Daemon uses these to broadcast
// JSON-RPC notifications to WebUI clients.
func (ix *Indexer) SetCallbacks(cb *IndexerCallbacks) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if cb != nil {
		ix.callbacks = cb
	}
}

// GetCallbacks returns the current callbacks for external wiring.
func (ix *Indexer) GetCallbacks() *IndexerCallbacks {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.callbacks
}

// ── Internal: Worker Loop ──

func (ix *Indexer) workerLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			if ix.logger != nil {
				ix.logger.Error("indexer: worker panic", fmt.Errorf("%v", r))
			}
		}
	}()
	for {
		// Check for work
		file := ix.manifest.firstEnqueued()
		if file == nil {
			// Queue empty — wait for notify
			if ix.callbacks.OnQueueEmpty != nil {
				ix.callbacks.OnQueueEmpty(ctx)
			}
			select {
			case <-ix.notify:
				continue
			case <-ix.stop:
				return
			case <-ctx.Done():
				return
			}
		}

		if ix.logger != nil {
			ix.logger.Info("indexer: dequeue file", "path", file.Path)
		}

		// Process the file
		if !ix.processNext(ctx, file) {
			// Stop requested
			return
		}
	}
}

// processNext indexes a single enqueued file and updates its state.
func (ix *Indexer) processNext(ctx context.Context, file *FileMeta) bool {
	select {
	case <-ix.stop:
		return false
	case <-ctx.Done():
		return false
	default:
	}

	// Update state to Processing
	ix.mu.Lock()
	ix.processing = file.Path
	ix.mu.Unlock()

	file.State = FileProcessing
	if err := ix.manifest.put(file); err != nil && ix.logger != nil {
		ix.logger.Error("indexer: failed to mark file as processing", fmt.Errorf("%w", err), "path", file.Path)
	}

	if ix.logger != nil {
		ix.logger.Info("indexer: start indexing file", "path", file.Path, "size", file.Size)
	}

	// Trigger OnFileIndexStart
	if ix.callbacks.OnFileIndexStart != nil {
		ix.callbacks.OnFileIndexStart(ctx, file.Path)
	}

	// Index the file
	fileCtx, fileCancel := context.WithTimeout(ctx, maxFileIndexTimeout)
	defer fileCancel()

	// Set region ID from projectDir
	regionID := fmt.Sprintf("%x", len(ix.projectDir))
	fileCtx = goragindexer.WithRegionID(fileCtx, regionID)

	indexStart := time.Now()

	// Snapshot entity count before indexing for per-file node delta
	var entitiesBefore int
	if gi, ok := ix.indexer.(*goragindexer.GraphIndexer); ok {
		entitiesBefore, _ = gi.EntityStats()
	}

	chunks, idxErr := ix.indexFile(fileCtx, file.Path)
	if idxErr != nil {
		file.State = FileFailed
		file.Error = idxErr.Error()
		file.UpdatedAt = time.Now().Unix()
		if err := ix.manifest.put(file); err != nil && ix.logger != nil {
			ix.logger.Error("indexer: failed to mark file as failed", fmt.Errorf("%w", err), "path", file.Path)
		}

		if ix.logger != nil {
			ix.logger.Error("indexer: file indexing failed", fmt.Errorf("%w", idxErr), "path", file.Path, "error", idxErr.Error())
		}

		if ix.callbacks.OnFileIndexFail != nil {
			ix.callbacks.OnFileIndexFail(ctx, file.Path, idxErr.Error())
		}

		ix.mu.Lock()
		ix.processing = ""
		ix.mu.Unlock()
		return true
	}

	// Stat file for mtime/size
	info, statErr := os.Stat(file.Path)
	if statErr == nil {
		file.Mtime = info.ModTime().UnixNano()
		file.Size = info.Size()
	}

	if len(chunks) > 0 {
		// Remove old chunks if re-indexing
		if len(file.ChunkIDs) > 0 {
			ix.removeChunks(ctx, file.ChunkIDs)
		}
		file.ChunkIDs = chunks
		file.Chunks = len(chunks)
	}

	// Record token usage, node count, elapsed time and cost
	file.ElapsedMs = time.Since(indexStart).Milliseconds()
	if gi, ok := ix.indexer.(*goragindexer.GraphIndexer); ok {
		if tu := gi.LastTokenUsage(); tu != nil {
			file.InputTokens = tu.PromptTokens
			file.OutputTokens = tu.CompletionTokens
			// Rough cost estimate using default Haiku pricing:
			//   Input:  $0.25 / 1M tokens → 0.00025 / 1K
			//   Output: $1.25 / 1M tokens → 0.00125 / 1K
			file.Cost = (float64(tu.PromptTokens)*0.00025 + float64(tu.CompletionTokens)*0.00125) / 1000.0
		}
		entitiesAfter, _ := gi.EntityStats()
		file.Nodes = entitiesAfter - entitiesBefore
	}

	file.State = FileIndexed
	file.Error = ""
	file.UpdatedAt = time.Now().Unix()
	if err := ix.manifest.put(file); err != nil && ix.logger != nil {
		ix.logger.Error("indexer: failed to mark file as indexed", fmt.Errorf("%w", err), "path", file.Path)
	}

	if ix.logger != nil {
		ix.logger.Info("indexer: file indexing done", "path", file.Path, "chunks", len(chunks))
	}

	if ix.callbacks.OnFileIndexDone != nil {
		ix.callbacks.OnFileIndexDone(ctx, file.Path)
	}

	ix.mu.Lock()
	ix.processing = ""
	ix.mu.Unlock()

	return true
}

// indexFile reads and indexes a single file, returning chunk IDs.
func (ix *Indexer) indexFile(ctx context.Context, absPath string) ([]string, error) {
	if ix.indexer == nil {
		if ix.logger != nil {
			ix.logger.Error("indexer: graph indexer is nil, cannot index file", fmt.Errorf("nil indexer"), "path", absPath)
		}
		return nil, nil
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if !isValidFileContent(raw) {
		if ix.logger != nil {
			ix.logger.Warn("indexer: content quality check failed, skipped",
				"path", absPath,
				"bytes", len(raw),
			)
		}
		return nil, nil
	}

	chunks, err := ix.indexer.AddFile(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("add file: %w", err)
	}
	if len(chunks) == 0 {
		if ix.logger != nil {
			ix.logger.Info("indexer: file yielded no chunks, skipped", "path", absPath)
		}
		return nil, nil
	}

	ix.recordTokenUsage(ctx)

	ids := make([]string, len(chunks))
	for i, c := range chunks {
		ids[i] = c.ID
	}
	return ids, nil
}

// removeChunks removes all tracked chunks for a previously indexed file.
func (ix *Indexer) removeChunks(ctx context.Context, chunkIDs []string) {
	if ix.indexer == nil {
		if ix.logger != nil {
			ix.logger.Error("indexer: graph indexer is nil, cannot remove chunks", fmt.Errorf("nil indexer"), "chunkIDs", chunkIDs)
		}
		return
	}
	for _, id := range chunkIDs {
		if err := ix.indexer.Remove(ctx, id); err != nil && ix.logger != nil {
			ix.logger.Error("indexer: failed to remove chunk", fmt.Errorf("%w", err), "id", id)
		}
	}
}

// recordTokenUsage extracts LLM token usage from the GraphIndexer.
func (ix *Indexer) recordTokenUsage(ctx context.Context) {
	if ix.usageStore == nil {
		return
	}
	gi, ok := ix.indexer.(*goragindexer.GraphIndexer)
	if !ok {
		return
	}
	tu := gi.LastTokenUsage()
	if tu == nil {
		return
	}

	record := session.TokenUsageRecord{
		ID:               session.NewRecordID(),
		ModelName:        ix.modelName,
		PromptTokens:     tu.PromptTokens,
		CompletionTokens: tu.CompletionTokens,
		TotalTokens:      tu.TotalTokens,
		Timestamp:        time.Now(),
	}
	if sws, ok := ix.usageStore.(interface {
		AppendWithSource(context.Context, session.TokenUsageRecord, string) error
	}); ok {
		if err := sws.AppendWithSource(ctx, record, "indexing"); err != nil && ix.logger != nil {
			ix.logger.Warn("indexer: failed to record token usage", "error", err)
		}
	} else {
		if err := ix.usageStore.Append(ctx, record); err != nil && ix.logger != nil {
			ix.logger.Warn("indexer: failed to record token usage", "error", err)
		}
	}
}

// isValidFileContent checks if file content is worth indexing (not binary,
// has sufficient printable characters, etc.).
func isValidFileContent(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	for _, b := range raw {
		if b == 0 {
			return false
		}
	}
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
	meaningful := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			meaningful++
		}
	}
	return meaningful >= minReadableContentChars
}
