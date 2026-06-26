package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/fsnotify/fsnotify"
)

// debounceWindow is the time window for coalescing repeated fsnotify events.
// Rapid writes (e.g., editor auto-save) are batched within this window.
const debounceWindow = 500 * time.Millisecond

// pendingChange tracks a single file change event for batch processing.
type pendingChange struct {
	relPath string
	deleted bool
}

// FileWatchService monitors watched directories for file changes and
// incrementally updates a LongTerm RAG index. It is designed to run as a
// long-lived background service within the MindX Daemon.
//
// Architecture:
//
//	TUI  → WatchListStore (add/remove dirs)
//	         ↓
//	FileWatchService (reads watchlist, registers fsnotify watchers)
//	         ↓
//
// IndexService.SyncFiles() (incremental index)
//
//	         ↓
//	LongTerm RAG Index (shared knowledge base)
type FileWatchService struct {
	indexer    goragcore.Indexer
	store      *WatchListStore
	watcher    *fsnotify.Watcher
	indexers   map[string]*IndexService // keyed by abs dir
	cacheBase  string                   // base directory for per-dir indexing caches
	logger     logging.Logger
	usageStore session.TokenUsageStore
	modelName  string

	// indexState persists per-directory full-scan state (pending/indexing/completed).
	indexState *IndexStateStore

	// indexingGuard prevents concurrent SyncDir goroutines for the same directory.
	indexingGuard sync.Map // map[absDir]struct{}

	// VersionRecorder is called for each changed file to persist version snapshots.
	// Set by Daemon to integrate with FileVersionStore.
	VersionRecorder func(absPath string)

	// IndexEventCallback is called for each file before and after indexing.
	// eventType is "indexing" before and "indexed" after.
	// Set by Daemon to broadcast to WebUI clients.
	IndexEventCallback func(absPath, relPath, absDir, eventType string)

	// Debounce state: coalesce rapid events for the same file
	debounce   map[string]time.Time // absPath → last event time
	debounceMu sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup

	// isRunning is set to true when eventLoop starts and false when it exits.
	isRunning atomic.Bool
}

// NewFileWatchService creates a FileWatchService.
//
//   - indexer: the shared LongTerm RAG core.Indexer to write into.
//   - store: the WatchListStore containing directories to monitor.
//   - indexState: persistent per-directory index state tracker.
//   - cacheBaseDir: directory for per-watched-dir indexing caches
//     (e.g., ~/.mindx/data/memory-cache/).
//   - logger: optional logger.
func NewFileWatchService(
	indexer goragcore.Indexer,
	store *WatchListStore,
	indexState *IndexStateStore,
	cacheBaseDir string,
	logger logging.Logger,
	usageStore session.TokenUsageStore,
	modelName string,
) *FileWatchService {
	return &FileWatchService{
		indexer:    indexer,
		store:      store,
		indexState: indexState,
		indexers:   make(map[string]*IndexService),
		cacheBase:  cacheBaseDir,
		logger:     logger,
		usageStore: usageStore,
		modelName:  modelName,
		debounce:   make(map[string]time.Time),
	}
}

// Start begins monitoring all directories in the watchlist.
// It blocks until ctx is cancelled or an error occurs.
// Returns nil immediately if no directories are being watched.
// Safe to call multiple times; previous internal state is reset.
func (s *FileWatchService) Start(ctx context.Context) error {
	// Recreate internal context and done channel for restart support.
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.done = make(chan struct{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("filewatch: create watcher: %w", err)
	}
	s.watcher = watcher

	// Register all directories from the watchlist
	entries := s.store.List()
	if len(entries) == 0 {
		if s.logger != nil {
			s.logger.Info("filewatch: no directories in watchlist, idle")
		}
		// Not an error — wait for context cancellation
		<-ctx.Done()
		return nil
	}

	added := make(map[string]bool)
	for _, e := range entries {
		if added[e.Dir] {
			continue
		}
		if err := s.watchDir(e.Dir); err != nil && s.logger != nil {
			s.logger.Warn("filewatch: failed to watch directory", "dir", e.Dir, "error", err)
			continue
		}
		added[e.Dir] = true
	}
	if len(added) == 0 {
		if s.logger != nil {
			s.logger.Warn("filewatch: no directories could be watched")
		}
		<-ctx.Done()
		return nil
	}

	if s.logger != nil {
		dirList := make([]string, 0, len(added))
		for d := range added {
			dirList = append(dirList, d)
		}
		s.logger.Info("filewatch: started",
			"directories", len(added),
			"watch_list", dirList,
		)
	}

	s.wg.Add(1)
	go s.eventLoop()

	// Resume incomplete indexing for registered directories.
	// SyncDir → Sync() loads the persisted file cache (mtime/size),
	// so already-indexed files are skipped — work continues from
	// where it left off, not restarted from zero.
	if s.indexState != nil {
		for dir := range added {
			st := s.indexState.Get(dir)
			if st != nil && st.State == "completed" {
				continue
			}
			if s.logger != nil {
				s.logger.Info("filewatch: resuming indexing",
					"dir", dir,
					"state", func() string {
						if st != nil {
							return st.State
						}
						return "pending (new)"
					}(),
				)
			}
			go s.SyncDir(s.ctx, dir)
		}
	}

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the file watch service.
// Safe to call even if the service was never started.
func (s *FileWatchService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	// Only wait for eventLoop to finish if it was actually started.
	if s.isRunning.Load() {
		<-s.done
		s.wg.Wait()
	}
}

// IsRunning returns true if the file watch event loop is active.
func (s *FileWatchService) IsRunning() bool {
	return s.isRunning.Load()
}

// FileWatchStatus summarizes the current state of the FileWatchService.
type FileWatchStatus struct {
	Running     bool                      `json:"running"`
	Watched     []string                  `json:"watched,omitempty"`
	CacheBase   string                    `json:"cache_base,omitempty"`
	IndexStates map[string]*DirIndexState `json:"index_states,omitempty"`
}

// Status returns the current running state and list of watched directories.
func (s *FileWatchService) Status() FileWatchStatus {
	status := FileWatchStatus{
		Running:   s.isRunning.Load(),
		CacheBase: s.cacheBase,
	}
	if s.store != nil {
		for _, e := range s.store.List() {
			status.Watched = append(status.Watched, e.Dir)
		}
	}
	if s.indexState != nil {
		status.IndexStates = s.indexState.All()
	}
	return status
}

// systemDirPrefixes lists path prefixes that should never be added to the file
// watchlist. Watching system-level directories can result in indexing garbage
// or transient data (temp files, logs, kernel pseudo-filesystems, …).
var systemDirPrefixes = []string{
	"/tmp",
	"/private/tmp",
	"/var/tmp",
	"/dev",
	"/proc",
	"/sys",
	"/etc",
	"/var/log",
}

// isSystemDir returns true when absPath is a system directory that should not
// be watched for file indexing. The check is case-sensitive (macOS /tmp is
// the only common form); Linux paths are already lowercase.
func isSystemDir(absPath string) bool {
	clean := filepath.Clean(absPath)
	for _, prefix := range systemDirPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

// AddWatch registers a single directory for monitoring. If the directory is
// already being watched, this is a no-op. Also adds it to the watchlist store.
func (s *FileWatchService) AddWatch(dir, agent string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("filewatch: resolve path: %w", err)
	}

	// Reject system directories
	if isSystemDir(absDir) {
		return fmt.Errorf("filewatch: refusing to watch system directory: %s", absDir)
	}

	// Check if a parent directory is already being watched — reject.
	if ancestor, ok := s.store.CoveredByAncestor(absDir); ok {
		return fmt.Errorf("filewatch: %s is already covered by %s", absDir, ancestor)
	}

	// If adding a broader parent, remove child watches from fsnotify first.
	if removed := s.store.RemoveDescendants(absDir); len(removed) > 0 {
		if s.watcher != nil {
			for _, child := range removed {
				_ = s.watcher.Remove(child)
			}
		}
	}

	if err := s.store.Add(absDir, agent); err != nil {
		return fmt.Errorf("filewatch: store add: %w", err)
	}

	// Create index state entry so Start()'s resume loop picks up
	// the directory for full scanning even if the service isn't running yet.
	if s.indexState != nil {
		s.indexState.SetPending(absDir)
	}

	if s.watcher != nil {
		if err := s.watchDir(absDir); err != nil {
			return err
		}
		// Trigger full scan in background if service is running
		if s.isRunning.Load() {
			go s.SyncDir(s.ctx, absDir)
		}
		return nil
	}
	return nil
}

// RemoveWatch stops monitoring a directory and removes it from the store.
func (s *FileWatchService) RemoveWatch(dir, agent string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("filewatch: resolve path: %w", err)
	}

	if err := s.store.Remove(absDir, agent); err != nil {
		return fmt.Errorf("filewatch: store remove: %w", err)
	}

	if s.watcher != nil {
		_ = s.watcher.Remove(absDir)
	}

	// Clean up index state and indexer
	if s.indexState != nil {
		s.indexState.Remove(absDir)
	}
	delete(s.indexers, absDir)
	return nil
}

// SyncDir performs a full scan and index of the given directory.
// It updates indexState to reflect progress (pending → indexing → completed).
// Runs synchronously; call in a goroutine if you need non-blocking behavior.
func (s *FileWatchService) SyncDir(ctx context.Context, absDir string) {
	if s.indexState == nil {
		return
	}

	s.indexState.SetPending(absDir)

	// No indexer configured: keep state as "pending" so the directory will
	// be picked up when an indexer becomes available (e.g. after model.switch).
	if s.indexer == nil {
		if s.logger != nil {
			s.logger.Info("filewatch.sync: indexer not available, leaving state as pending",
				"dir", absDir)
		}
		return
	}

	// Guard: skip if another goroutine is already indexing this directory.
	if _, loaded := s.indexingGuard.LoadOrStore(absDir, struct{}{}); loaded {
		if s.logger != nil {
			s.logger.Info("filewatch.sync: skip (already indexing)", "dir", absDir)
		}
		return
	}
	defer s.indexingGuard.Delete(absDir)

	// Lightweight file count for progress (no stat calls, no ignore rules).
	totalFiles, err := countFilesRecursive(absDir)
	if err != nil {
		s.indexState.SetFailed(absDir, err.Error())
		if s.logger != nil {
			s.logger.Error("filewatch.sync: count failed", err, "dir", absDir)
		}
		return
	}

	s.indexState.SetIndexing(absDir, totalFiles)
	if s.logger != nil {
		s.logger.Info("filewatch.sync: starting full scan",
			"dir", absDir, "total_files", totalFiles)
	}

	// Perform the full sync (IndexService internally walks with ignore rules).
	pi := s.getIndexer(absDir)
	if s.IndexEventCallback != nil || s.indexState != nil {
		cbAbsDir := absDir // capture for closure
		pi.SyncStepCallback = func(absPath, relPath, state string) {
			// Fire a "pre-index" event so the frontend can show which file is
			// currently being processed (the indexer calls this AFTER the file
			// has been processed, so we fire "indexing" proactively here).
			if state == "indexed" {
				if s.IndexEventCallback != nil {
					s.IndexEventCallback(absPath, relPath, cbAbsDir, "indexing")
				}
				if s.indexState != nil {
					s.indexState.IncrementIndexedFiles(cbAbsDir)
				}
			}
			// Broadcast the actual result state to WebUI.
			if s.IndexEventCallback != nil {
				s.IndexEventCallback(absPath, relPath, cbAbsDir, state)
			}
		}
	}
	result := pi.Sync(ctx, absDir)
	pi.SyncStepCallback = nil // clean up after sync completes

	if result.Err != nil {
		s.indexState.SetFailed(absDir, result.Err.Error())
		if s.logger != nil {
			s.logger.Error("filewatch.sync: failed", result.Err, "dir", absDir)
		}
		return
	}

	// Record actual indexing stats from the Sync result, including per-file errors.
	indexedCount := result.Indexed + result.Updated

	// Build completed files list with timing info
	completedRecs := make([]CompletedFileRecord, 0, len(result.CompletedFiles))
	for _, cf := range result.CompletedFiles {
		completedRecs = append(completedRecs, CompletedFileRecord{
			Path:      cf.Path,
			Chunks:    cf.Chunks,
			ElapsedMs: cf.Elapsed.Milliseconds(),
			Timestamp: cf.Timestamp.Unix(),
		})
	}

	// When all files were cache hits (skipped), CompletedFiles is empty but the
	// files ARE indexed. Use ScanFileStates to retrieve them so the UI can
	// display the full history of indexed files.
	if len(completedRecs) == 0 && result.Skipped > 0 {
		if scanResult, scanErr := pi.ScanFileStates(s.ctx, absDir); scanErr == nil {
			for _, fs := range scanResult {
				if fs.State == FileStateIndexed || fs.State == FileStateChanged {
					completedRecs = append(completedRecs, CompletedFileRecord{
						Path: fs.Path,
					})
				}
			}
			if s.logger != nil {
				s.logger.Info("filewatch.sync: populated completed_files from ScanFileStates",
					"dir", absDir, "count", len(completedRecs))
			}
		} else if s.logger != nil {
			s.logger.Warn("filewatch.sync: ScanFileStates failed, completed_files will be empty",
				"dir", absDir, "error", scanErr)
		}
	}

	if len(result.FailedFiles) > 0 {
		failedRecs := make([]FailedFileRecord, len(result.FailedFiles))
		for i, fe := range result.FailedFiles {
			failedRecs[i] = FailedFileRecord{
				Path:      fe.Path,
				Error:     fe.Error,
				Timestamp: fe.Timestamp.Unix(),
				ElapsedMs: fe.Elapsed.Milliseconds(),
			}
		}
		s.indexState.SetCompletedWithFailedFiles(absDir, indexedCount, result.Skipped, result.EntitiesCreated, result.RelsCreated, result.Elapsed.Milliseconds(), completedRecs, failedRecs)
	} else {
		s.indexState.SetCompletedWithStats(absDir, indexedCount, result.Skipped, result.EntitiesCreated, result.RelsCreated, result.Elapsed.Milliseconds(), completedRecs)
	}
	if s.logger != nil {
		s.logger.Info("filewatch.sync: completed",
			"dir", absDir,
			"indexed", result.Indexed,
			"updated", result.Updated,
			"skipped", result.Skipped,
			"removed", result.Removed,
			"errors", len(result.Errors),
			"failed_files", len(result.FailedFiles),
			"completed_files", len(result.CompletedFiles),
			"entities", result.EntitiesCreated,
			"rels", result.RelsCreated,
			"elapsed_ms", result.Elapsed.Milliseconds(),
		)
	}
}

// RemoveWatchByDir stops monitoring all entries for a directory and removes
// them from the store (regardless of agent name). This is the method to use
// when the frontend removes a watched directory by path alone.
//
// Concurrency safety:
//   - WatchListStore.RemoveByDir() acquires a write lock, so concurrent
//     List() calls (used by eventLoop.findRootDir()) will see the directory
//     as removed immediately after this returns.
//   - fsnotify.Watcher.Remove() stops OS-level events for this directory.
//   - Any events already queued in the channel will be processed by eventLoop,
//     but findRootDir() returns "" for removed directories → events dropped.
//   - The indexing operation is idempotent, so even if a small number of
//     pre-removal events get processed, no data inconsistency occurs.
func (s *FileWatchService) RemoveWatchByDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("filewatch: resolve path: %w", err)
	}

	if err := s.store.RemoveByDir(absDir); err != nil {
		return fmt.Errorf("filewatch: store remove: %w", err)
	}

	if s.watcher != nil {
		_ = s.watcher.Remove(absDir)
	}

	// Clean up index state and indexer
	if s.indexState != nil {
		s.indexState.Remove(absDir)
	}
	delete(s.indexers, absDir)
	return nil
}

// watchDir adds a directory to the fsnotify watcher.
// Subdirectories are also added to capture deep file changes.
func (s *FileWatchService) watchDir(absDir string) error {
	// Resolve symlinks so that filepath.Walk sees the real directory.
	// On macOS, /tmp is a symlink to /private/tmp; without this resolution
	// Walk treats the root as a non-directory (symlink) and skips everything.
	realDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		// Fallback: try adding the path directly (fsnotify can handle some symlinks)
		return s.watcher.Add(absDir)
	}
	return filepath.Walk(realDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if !info.IsDir() {
			return nil
		}
		// Skip hidden and ignored directories (same logic as index_service)
		if path != realDir {
			base := info.Name()
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			if DefaultIgnoredDirs[base] {
				return filepath.SkipDir
			}
			// Also check .mindxignore for this directory
			// We use a lightweight check: only load ignore rules for the root dir
		}
		return s.watcher.Add(path)
	})
}

// eventLoop reads events from the fsnotify watcher and processes them.
func (s *FileWatchService) eventLoop() {
	defer close(s.done)
	defer s.isRunning.Store(false)
	s.isRunning.Store(true)

	// Timer for debounce flushing
	ticker := time.NewTicker(debounceWindow)
	defer ticker.Stop()

	// Pending file changes, grouped by root watched directory
	pending := make(map[string][]pendingChange) // absDir → changes

	flush := func() {
		if len(pending) == 0 {
			return
		}
		s.processChanges(pending)
		pending = make(map[string][]pendingChange)
	}

	for {
		select {
		case <-s.ctx.Done():
			flush()
			return

		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Skip if path is empty
			if event.Name == "" {
				continue
			}

			// Skip directories (events on the dir itself, not files in it)
			if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
				// If a new directory was created, watch it recursively
				if event.Has(fsnotify.Create) {
					if watchErr := s.watchNewDir(event.Name); watchErr != nil && s.logger != nil {
						s.logger.Warn("filewatch: failed to watch new subdirectory",
							"dir", event.Name, "error", watchErr)
					}
				}
				continue
			}

			// Debounce: coalesce rapid events for the same file
			s.debounceMu.Lock()
			last, exists := s.debounce[event.Name]
			now := time.Now()
			if exists && now.Sub(last) < debounceWindow {
				// Extend the debounce timer — coalesce
				s.debounce[event.Name] = now
				s.debounceMu.Unlock()
				continue
			}
			s.debounce[event.Name] = now
			s.debounceMu.Unlock()

			// Map absolute path to one of our watched root directories
			rootDir := s.findRootDir(event.Name)
			if rootDir == "" {
				continue
			}

			// Compute relative path using resolved paths to handle symlinks correctly.
			// e.g. event.Name=/private/tmp/test.md, rootDir=/tmp → must resolve both first.
			resolvedEvent, _ := filepath.EvalSymlinks(event.Name)
			resolvedRoot, _ := filepath.EvalSymlinks(rootDir)
			if resolvedRoot == "" {
				resolvedRoot = rootDir
			}
			if resolvedEvent == "" {
				resolvedEvent = event.Name
			}
			relPath, err := filepath.Rel(resolvedRoot, resolvedEvent)
			if err != nil {
				continue
			}

			// Determine if this is a delete or a modify/create
			isDelete := event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)
			isCreate := event.Has(fsnotify.Create) || event.Has(fsnotify.Write)

			if isDelete {
				pending[rootDir] = append(pending[rootDir], pendingChange{relPath: relPath, deleted: true})
			} else if isCreate {
				pending[rootDir] = append(pending[rootDir], pendingChange{relPath: relPath, deleted: false})
			}

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			if s.logger != nil {
				s.logger.Warn("filewatch: watcher error", "error", err)
			}

		case <-ticker.C:
			flush()
		}
	}
}

// processChanges applies all pending file changes to the index.
func (s *FileWatchService) processChanges(pending map[string][]pendingChange) {
	for absDir, changes := range pending {
		if len(changes) == 0 {
			continue
		}

		// Get or create an IndexService for this directory
		pi := s.getIndexer(absDir)

		// Separate deletes from creates
		var toIndex []string
		var toDelete []string
		for _, c := range changes {
			if c.deleted {
				toDelete = append(toDelete, c.relPath)
			} else {
				toIndex = append(toIndex, c.relPath)
			}
		}

		// Process deletions first
		if len(toDelete) > 0 {
			result := pi.SyncFiles(s.ctx, absDir, toDelete, true)
			if s.logger != nil && (result.Indexed > 0 || result.Updated > 0 || result.Removed > 0) {
				s.logger.Info("filewatch: processed deletions",
					"dir", absDir,
					"removed", result.Removed,
					"errors", len(result.Errors),
				)
			}
		}

		// Process creates/updates
		if len(toIndex) > 0 {
			// Fire index-start events for each file
			for _, relPath := range toIndex {
				if s.IndexEventCallback != nil {
					absPath := filepath.Join(absDir, relPath)
					s.IndexEventCallback(absPath, relPath, absDir, "indexing")
				}
			}

			result := pi.SyncFiles(s.ctx, absDir, toIndex, false)
			if s.logger != nil && (result.Indexed > 0 || result.Updated > 0 || result.Removed > 0) {
				s.logger.Info("filewatch: indexed files",
					"dir", absDir,
					"indexed", result.Indexed,
					"updated", result.Updated,
					"errors", len(result.Errors),
				)
			}

			// Fire index-complete events for each file (only those actually indexed/updated)
			if s.IndexEventCallback != nil && (result.Indexed > 0 || result.Updated > 0) {
				for _, relPath := range toIndex {
					absPath := filepath.Join(absDir, relPath)
					s.IndexEventCallback(absPath, relPath, absDir, "indexed")
				}
			}
			// Record file versions for changed files
			if s.VersionRecorder != nil {
				for _, relPath := range toIndex {
					absPath := filepath.Join(absDir, relPath)
					s.VersionRecorder(absPath)
				}
			}
		}
	}
}

// watchNewDir adds a newly created directory and its subdirectories to the watcher.
func (s *FileWatchService) watchNewDir(absPath string) error {
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return s.watcher.Add(absPath)
	}
	return filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		base := info.Name()
		if strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		if DefaultIgnoredDirs[base] {
			return filepath.SkipDir
		}
		return s.watcher.Add(path)
	})
}

// findRootDir finds which watched root directory contains the given absolute path.
// It handles symlinks by resolving both the input path and watchlist entries.
func (s *FileWatchService) findRootDir(absPath string) string {
	// Resolve the event path so we can match against resolved watchlist entries.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolvedPath = absPath
	}
	for _, entry := range s.store.List() {
		// Resolve the watchlist entry for comparison (handles /tmp → /private/tmp)
		resolvedEntry, err := filepath.EvalSymlinks(entry.Dir)
		if err != nil {
			resolvedEntry = entry.Dir
		}
		if strings.HasPrefix(resolvedPath, resolvedEntry+string(filepath.Separator)) || resolvedPath == resolvedEntry {
			return entry.Dir // return ORIGINAL dir (used as key in pending map)
		}
		// Also check unresolved in case EvalSymlinks changes things unexpectedly
		if strings.HasPrefix(absPath, entry.Dir+string(filepath.Separator)) || absPath == entry.Dir {
			return entry.Dir
		}
	}
	return ""
}

// getIndexer returns an IndexService for the given directory, creating one if needed.
func (s *FileWatchService) getIndexer(absDir string) *IndexService {
	if pi, ok := s.indexers[absDir]; ok {
		return pi
	}

	// Each dir gets its own cache directory named by sanitized path
	cacheDir := filepath.Join(s.cacheBase, SanitizeDirName(absDir))
	opts := []IndexServiceOption{}
	if s.usageStore != nil && s.modelName != "" {
		opts = append(opts, WithTokenUsageStore(s.usageStore, s.modelName))
	}
	pi := NewIndexService(s.indexer, cacheDir, s.logger, opts...)
	s.indexers[absDir] = pi
	return pi
}

// GetIndexer returns the IndexService for the given directory, or nil if
// no indexer has been created yet.
func (s *FileWatchService) GetIndexer(absDir string) *IndexService {
	s.indexingGuard.Load(absDir) // ensure the directory has been seen
	return s.indexers[absDir]
}

// IndexState returns the persistent per-directory index state store.
func (s *FileWatchService) IndexState() *IndexStateStore {
	return s.indexState
}

// sanitizeDirName converts a filesystem path to a safe directory name.
func SanitizeDirName(absPath string) string {
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

// countFilesRecursive counts non-directory entries under root using os.ReadDir.
// It is much lighter than walkProjectDir (no stat calls, no ignore rules) and
// only serves to estimate total file count for progress reporting.
func countFilesRecursive(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}
