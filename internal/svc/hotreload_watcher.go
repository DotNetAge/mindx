package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/fsnotify/fsnotify"
)

// reloadDebounce is the coalescing window for rapid file changes (e.g., editor auto-save).
// Rapid writes within this window are batched into a single reload.
const reloadDebounce = 500 * time.Millisecond

// hotReloadStopTimeout 是停机时等待事件循环退出的上限。
// 超过即放弃等待并记录告警——停机不能无限期挂着。
const hotReloadStopTimeout = 3 * time.Second

// HotReloadWatcher monitors the agents/ and skills/ directories for file system
// changes and automatically triggers registry reloads via App.ReloadAgents()/ReloadSkills().
//
// This coexists with FileWatchService (which monitors user project directories for
// RAG knowledge-base indexing) without conflict because they watch completely
// disjoint directory trees:
//   - FileWatchService  → user project working directories (dynamic, via watchlist)
//   - HotReloadWatcher   → ~/.mindx/agents/ and ~/.mindx/skills/ (fixed paths)
//
// Both use independent fsnotify.Watcher instances.
type HotReloadWatcher struct {
	app    *core.App
	logger logging.Logger

	// notify 注册表重装完成后的对外通知钩子（广播 WebSocket 通知，前端收到后
	// 自动重拉列表）。为 nil 时静默跳过，不影响重载本身。
	notify func(method string, params any)

	watcher *fsnotify.Watcher

	agentsDir string
	skillsDir string
	teamsDir  string

	// Lifecycle
	//
	// mu 保护 Start/Stop 对下面四个字段的读写：Start 可能在 daemon 启动
	// goroutine 里跑，Stop 在停机路径上跑，两者并发是常态。
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup

	// running 由 eventLoop 写、IsRunning/Stop 读——必须是原子量，
	// 裸 bool 在这里是数据竞争（go test -race 会直接报）。
	running atomic.Bool
}

// NewHotReloadWatcher creates a watcher that monitors agents and skills directories.
// notify 为注册表重装后的广播钩子（可为 nil），签名对齐 gort gateway 的
// BroadcastNotification，由 daemon 侧传入。
func NewHotReloadWatcher(app *core.App, logger logging.Logger, notify func(method string, params any)) *HotReloadWatcher {
	return &HotReloadWatcher{
		app:       app,
		logger:    logger,
		notify:    notify,
		agentsDir: app.Settings().AgentsDir(),
		skillsDir: app.Settings().SkillsDir(),
		teamsDir:  app.Settings().TeamsDir(),
	}
}

// Start begins watching both directories in a background goroutine.
// Returns immediately (non-blocking). Safe to call multiple times (幂等)。
//
// ctx 必须来自调用方（daemon 的停机 ctx）：以前这里用
// context.WithCancel(context.Background()) 丢弃了调用方 ctx，daemon 停机时
// watcher 感知不到，只能靠 Stop 兜。
func (w *HotReloadWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running.Load() {
		w.mu.Unlock()
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.done = make(chan struct{})
	done := w.done

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.mu.Unlock()
		return err
	}
	w.watcher = watcher
	w.mu.Unlock()

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
	if err := w.watchDir(w.teamsDir); err == nil {
		added++
	} else if w.logger != nil {
		w.logger.Warn("hot-reload: failed to watch teams dir", "dir", w.teamsDir, "error", err)
	}

	if added == 0 {
		if w.logger != nil {
			w.logger.Warn("hot-reload: no directories could be watched, idle")
		}
		// 未启动事件循环也要给出完成信号，否则 Stop 会白等到超时。
		close(done)
		// Not an error — wait for context cancellation
		<-ctx.Done()
		return nil
	}

	w.running.Store(true)
	w.wg.Add(1)
	go w.eventLoop(w.ctx, done)

	if w.logger != nil {
		w.logger.Info("hot-reload watcher started",
			"agents_dir", w.agentsDir,
			"skills_dir", w.skillsDir,
			"teams_dir", w.teamsDir,
		)
	}
	return nil
}

// Stop gracefully shuts down the watcher.
// Safe to call even if the service was never started or already stopped.
func (w *HotReloadWatcher) Stop() {
	w.mu.Lock()
	cancel, watcher, done := w.cancel, w.watcher, w.done
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if watcher != nil {
		_ = watcher.Close()
	}
	// 等待事件循环退出，但必须有上限：停机路径上不能因为一次 fsnotify
	// 异常或状态残留就永久阻塞（以前 <-w.done 无超时，且用裸 bool 判断
	// 是否需要等待，两者叠加会把 daemon 卡死）。
	if done != nil {
		select {
		case <-done:
		case <-time.After(hotReloadStopTimeout):
			if w.logger != nil {
				w.logger.Warn("hot-reload: 事件循环未在超时内退出，继续停机")
			}
			return
		}
	}
	w.wg.Wait()
	if w.logger != nil {
		w.logger.Info("hot-reload watcher stopped")
	}
}

// IsRunning returns true if the event loop is active.
func (w *HotReloadWatcher) IsRunning() bool {
	return w.running.Load()
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
func (w *HotReloadWatcher) eventLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer w.wg.Done()
	defer w.running.Store(false)

	ticker := time.NewTicker(reloadDebounce)
	defer ticker.Stop()

	pendingAgents := false
	pendingSkills := false
	pendingTeams := false

	flush := func() {
		if pendingAgents {
			if err := w.app.ReloadAgents(); err != nil && w.logger != nil {
				w.logger.Warn("hot-reload: failed to reload agents", "error", err)
			}
			// 无论重载是否报错都广播：注册表已换入（部分装载失败仅记日志跳过），
			// 前端收到 agents_changed 后自动重拉列表，保持 UI 与注册表最终一致
			if w.notify != nil {
				w.notify("agents_changed", map[string]any{})
			}
			pendingAgents = false
		}
		if pendingSkills {
			if err := w.app.ReloadSkills(); err != nil && w.logger != nil {
				w.logger.Warn("hot-reload: failed to reload skills", "error", err)
			}
			if w.notify != nil {
				w.notify("skills_changed", map[string]any{})
			}
			pendingSkills = false
		}
		if pendingTeams {
			if err := w.app.ReloadTeams(); err != nil && w.logger != nil {
				w.logger.Warn("hot-reload: failed to reload teams", "error", err)
			}
			if w.notify != nil {
				w.notify("teams_changed", map[string]any{})
			}
			pendingTeams = false
		}
	}

	for {
		select {
		case <-ctx.Done():
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
			relTeams, _ := filepath.Rel(w.teamsDir, event.Name)

			isAgentEvent := !strings.HasPrefix(relAgents, "..") && relAgents != "."
			isSkillEvent := !strings.HasPrefix(relSkills, "..") && relSkills != "."
			isTeamEvent := !strings.HasPrefix(relTeams, "..") && relTeams != "."

			if !isAgentEvent && !isSkillEvent && !isTeamEvent {
				continue
			}

			// Handle new subdirectories (e.g., a new agent/skill/team directory was created).
			// 目录化存储后 Agent 是 agents/<name>/ 目录、技能是 skills/<name>/ 目录、
			// 团队是 teams/<name>/ 目录：除递归加入 watch 外，还必须把目录创建本身计入
			// pending，否则目录内文件先于 watch 生效写入时会漏掉这次变更。
			if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
				if event.Has(fsnotify.Create) {
					_ = w.watchDir(event.Name)
					if isAgentEvent {
						pendingAgents = true
					}
					if isSkillEvent {
						pendingSkills = true
					}
					if isTeamEvent {
						pendingTeams = true
					}
				}
				continue
			}

			// Agent 目录的变更载体是 IDENTITY.md / SOUL.md（.md 文件）
			if isAgentEvent && !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
				continue
			}

			if isAgentEvent {
				pendingAgents = true
			}
			if isSkillEvent {
				pendingSkills = true
			}
			if isTeamEvent {
				pendingTeams = true
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
