package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gorag"
	"github.com/DotNetAge/gorag/logging"
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
//	ProjectIndexer.SyncFiles() (incremental index)
//	         ↓
//	LongTerm RAG Index (shared knowledge base)
type FileWatchService struct {
	indexer   *gorag.HybridIndexer
	store     *WatchListStore
	watcher   *fsnotify.Watcher
	indexers  map[string]*ProjectIndexer // keyed by abs dir
	cacheBase string                     // base directory for per-dir indexing caches
	logger    logging.Logger

	// VersionRecorder is called for each changed file to persist version snapshots.
	// Set by Daemon to integrate with FileVersionStore.
	VersionRecorder func(absPath string)

	// Debounce state: coalesce rapid events for the same file
	debounce   map[string]time.Time // absPath → last event time
	debounceMu sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewFileWatchService creates a FileWatchService.
//
//   - indexer: the shared LongTerm RAG HybridIndexer to write into.
//   - store: the WatchListStore containing directories to monitor.
//   - cacheBaseDir: directory for per-watched-dir indexing caches
//     (e.g., ~/.mindx/data/memory-cache/).
//   - logger: optional logger.
func NewFileWatchService(
	indexer *gorag.HybridIndexer,
	store *WatchListStore,
	cacheBaseDir string,
	logger logging.Logger,
) *FileWatchService {
	ctx, cancel := context.WithCancel(context.Background())
	return &FileWatchService{
		indexer:   indexer,
		store:     store,
		indexers:  make(map[string]*ProjectIndexer),
		cacheBase: cacheBaseDir,
		logger:    logger,
		debounce:  make(map[string]time.Time),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// Start begins monitoring all directories in the watchlist.
// It blocks until ctx is cancelled or an error occurs.
// Returns nil immediately if no directories are being watched.
func (s *FileWatchService) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("filewatch: create watcher: %w", err)
	}
	s.watcher = watcher
	defer s.watcher.Close()

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
		s.logger.Info("filewatch: started", "directories", len(added))
	}

	s.wg.Add(1)
	go s.eventLoop()

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the file watch service.
func (s *FileWatchService) Stop() {
	s.cancel()
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	<-s.done
	s.wg.Wait()
}

// AddWatch registers a single directory for monitoring. If the directory is
// already being watched, this is a no-op. Also adds it to the watchlist store.
func (s *FileWatchService) AddWatch(dir, agent string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("filewatch: resolve path: %w", err)
	}

	if err := s.store.Add(absDir, agent); err != nil {
		return fmt.Errorf("filewatch: store add: %w", err)
	}

	if s.watcher != nil {
		return s.watchDir(absDir)
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
		// Skip hidden and ignored directories (same logic as project_indexer)
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

		// Get or create a ProjectIndexer for this directory
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
			result := pi.SyncFiles(s.ctx, absDir, toIndex, false)
			if s.logger != nil && (result.Indexed > 0 || result.Updated > 0 || result.Removed > 0) {
				s.logger.Info("filewatch: indexed files",
					"dir", absDir,
					"indexed", result.Indexed,
					"updated", result.Updated,
					"errors", len(result.Errors),
				)
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

// getIndexer returns a ProjectIndexer for the given directory, creating one if needed.
func (s *FileWatchService) getIndexer(absDir string) *ProjectIndexer {
	if pi, ok := s.indexers[absDir]; ok {
		return pi
	}

	// Each dir gets its own cache directory named by sanitized path
	cacheDir := filepath.Join(s.cacheBase, sanitizeDirName(absDir))
	pi := NewProjectIndexer(s.indexer, cacheDir, s.logger)
	s.indexers[absDir] = pi
	return pi
}

// sanitizeDirName converts a filesystem path to a safe directory name.
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
