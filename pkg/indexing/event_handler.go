package indexing

import (
	"os"
	"path/filepath"
	"time"

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
			if (s.IndexEventCallback != nil || s.indexState != nil) && (result.Indexed > 0 || result.Updated > 0) {
				for _, relPath := range toIndex {
					absPath := filepath.Join(absDir, relPath)
					if s.IndexEventCallback != nil {
						s.IndexEventCallback(absPath, relPath, absDir, "indexed")
					}
					if s.indexState != nil {
						s.indexState.IncrementIndexedFiles(absDir)
					}
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
