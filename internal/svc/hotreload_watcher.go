package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/fsnotify/fsnotify"
)

// reloadDebounce is the coalescing window for rapid file changes (e.g., editor auto-save).
// Rapid writes within this window are batched into a single reload.
const reloadDebounce = 500 * time.Millisecond

// HotReloadWatcher monitors the agents/ and skills/ directories for file system
// changes and automatically triggers registry reloads via App.ReloadAgents()/ReloadSkills().
//
// This coexists with FileWatchService (which monitors user project directories for
// RAG knowledge-base indexing) without conflict because they watch completely
// disjoint directory trees:
//   - FileWatchService  → user project working directories (dynamic, via watchlist)
//   - HotReloadWatcher   → ~/.mindx/agents/ and ~/.mindx/skills/ (fixed paths)
// Both use independent fsnotify.Watcher instances.
type HotReloadWatcher struct {
	app    *core.App
	logger logging.Logger

	watcher *fsnotify.Watcher

	agentsDir string
	skillsDir string

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup

	isRunning bool // atomic: set true in eventLoop, false on exit
}

// NewHotReloadWatcher creates a watcher that monitors agents and skills directories.
func NewHotReloadWatcher(app *core.App, logger logging.Logger) *HotReloadWatcher {
	return &HotReloadWatcher{
		app:       app,
		logger:    logger,
		agentsDir: app.Settings().AgentsDir(),
		skillsDir: app.Settings().SkillsDir(),
	}
}

// Start begins watching both directories in a background goroutine.
// Returns immediately (non-blocking). Safe to call multiple times; previous state is reset.
func (w *HotReloadWatcher) Start(ctx context.Context) error {
	// Reset lifecycle state for restart support (same pattern as FileWatchService)
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.done = make(chan struct{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	// Watch both root directories recursively
	added := 0
	if err := w.watchDir(w.agentsDir); err == nil {
		added++
	} else if w.logger != nil {
		w.logger.Warn("hot-reload: failed to watch agents dir", "dir", w.agentsDir, "error", err)
	}
	if err := w.watchDir(w.skillsDir); err == nil {
		added++
	} else if w.logger != nil {
		w.logger.Warn("hot-reload: failed to watch skills dir", "dir", w.skillsDir, "error", err)
	}

	if added == 0 {
		if w.logger != nil {
			w.logger.Warn("hot-reload: no directories could be watched, idle")
		}
		// Not an error — wait for context cancellation
		<-ctx.Done()
		return nil
	}

	w.wg.Add(1)
	go w.eventLoop()

	if w.logger != nil {
		w.logger.Info("hot-reload watcher started",
			"agents_dir", w.agentsDir,
			"skills_dir", w.skillsDir,
		)
	}
	return nil
}

// Stop gracefully shuts down the watcher.
// Safe to call even if the service was never started or already stopped.
func (w *HotReloadWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.watcher != nil {
		_ = w.watcher.Close()
	}
	// Only wait if eventLoop was actually started
	if w.isRunning {
		<-w.done
		w.wg.Wait()
	}
	if w.logger != nil {
		w.logger.Info("hot-reload watcher stopped")
	}
}

// IsRunning returns true if the event loop is active.
func (w *HotReloadWatcher) IsRunning() bool {
	return w.isRunning
}

// watchDir adds a directory and all its subdirectories to the fsnotify watcher.
func (w *HotReloadWatcher) watchDir(dir string) error {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return w.watcher.Add(dir)
	}
	return filepath.Walk(realDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return w.watcher.Add(path)
		}
		return nil
	})
}

// eventLoop reads fsnotify events and triggers reloads with debounce.
func (w *HotReloadWatcher) eventLoop() {
	defer close(w.done)
	defer w.wg.Done()
	defer func() { w.isRunning = false }()
	w.isRunning = true

	ticker := time.NewTicker(reloadDebounce)
	defer ticker.Stop()

	pendingAgents := false
	pendingSkills := false

	flush := func() {
		if pendingAgents {
			if err := w.app.ReloadAgents(); err != nil && w.logger != nil {
				w.logger.Warn("hot-reload: failed to reload agents", "error", err)
			}
			pendingAgents = false
		}
		if pendingSkills {
			if err := w.app.ReloadSkills(); err != nil && w.logger != nil {
				w.logger.Warn("hot-reload: failed to reload skills", "error", err)
			}
			pendingSkills = false
		}
	}

	for {
		select {
		case <-w.ctx.Done():
			flush()
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Name == "" {
				continue
			}

			// Classify which root directory this event belongs to
			relAgents, _ := filepath.Rel(w.agentsDir, event.Name)
			relSkills, _ := filepath.Rel(w.skillsDir, event.Name)

			isAgentEvent := !strings.HasPrefix(relAgents, "..") && relAgents != "."
			isSkillEvent := !strings.HasPrefix(relSkills, "..") && relSkills != "."

			if !isAgentEvent && !isSkillEvent {
				continue
			}

			// Handle new subdirectories (e.g., new skill added)
			if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
				if event.Has(fsnotify.Create) {
					_ = w.watchDir(event.Name)
				}
				continue
			}

			// Agents only care about .md files
			if isAgentEvent && !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
				continue
			}

			if isAgentEvent {
				pendingAgents = true
			}
			if isSkillEvent {
				pendingSkills = true
			}

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

		case <-ticker.C:
			flush()
		}
	}
}
