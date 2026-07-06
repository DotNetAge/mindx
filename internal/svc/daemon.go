package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	graphapi "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/goharness/hooks/action"
	goharnesssession "github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	goraggograph "github.com/DotNetAge/gorag/v2/store/graph/gograph"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/appicon"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/update"
	"github.com/DotNetAge/mindx/pkg/indexing"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
	"go.etcd.io/bbolt"
)

var (
	atAgentRegex = regexp.MustCompile(`^@([\w-]+)(?:\s+(.+))?$`)
	ulidRegex    = regexp.MustCompile(`^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$`)
)

type pendingInteraction struct {
	replyFn   func(answers map[string]string)
	grantFn   func(params map[string]any)
	denyFn    func(reason string)
	createdAt time.Time
}

type Daemon struct {
	app         *core.App
	gw          *gateway.Server
	scheduler   *scheduler.Scheduler
	schedulerDB *scheduler.FileSchedulerStore
	kbWatch     *indexing.FileWatchService
	// watchListStore and indexStateStore are persisted stores for filewatch
	// entries. They are created unconditionally (independent of GraphIndexer
	// or FileWatchService) so that session creation can always persist a
	// project directory to the watch list. The FileWatchService, when
	// initialized, reuses the same store instances.
	watchListStore  *indexing.WatchListStore
	indexStateStore *indexing.IndexStateStore
	manifestStore   *indexing.ManifestStore
	sharedMemory    *memory.RAGMemory

	// knowledge-graph indexer (GraphIndexer)
	graphIndexer    *goragindexer.GraphIndexer
	graphIndexerErr error // init failure reason, exposed in KB handler errors
	webServer       *WebServer
	addr            string
	wsPath          string
	logger          logging.Logger
	execMu          sync.Mutex
	clientCancels   sync.Map

	// activeSessions tracks live sessions by sessionID for FileModifyHook
	// to look up the session's TrackModify function.
	activeSessions sync.Map

	pendingInteractions map[string]*pendingInteraction
	interactMu          sync.Mutex

	// knowledge-graph database (gograph)
	graphDB    *graphapi.DB
	graphStore *graphapi.GraphStore

	// global key-value store (bbolt)
	kvStore *bbolt.DB

	// watchCancel cancels the currently running filewatch goroutine (if any).
	watchCancel context.CancelFunc

	// startTime records when the daemon started, used for uptime reporting.
	startTime time.Time

	// runtimeFS 是嵌入式文件系统，包含 runtime/ 目录下的资源文件。
	runtimeFS fs.FS

	// updater 负责自动升级检查与安装。
	updater *update.Updater

	// restartCh 接收重启信号；Start() 主循环通过 select 监听。
	restartCh chan struct{}

	// cleanupCancel cancels the stale interaction cleanup loop on shutdown.
	cleanupCancel context.CancelFunc

	// hotReload watches agents/ and skills/ directories for file changes
	// and automatically reloads registries.
	hotReload *HotReloadWatcher

	// manifestWorker fields for FIFO queue processing
	manifestWorkerCancel context.CancelFunc
	manifestNewFileCh    chan struct{}
	manifestResumeCh     chan struct{}
}

func NewDaemon(app *core.App, addr, wsPath string, runtimeFS fs.FS) *Daemon {
	// Inject custom skills prompt: list only names, with a tip to use
	// "mindx skill list -f" for detailed descriptions.
	app.SetSkillsPromptOverride(NewSkillsPrompt())

	// Inject custom environment prompt: enrich with SessionID, local time,
	// user prefs, and venv path.
	app.SetEnvsOverride(NewEnvironmentPrompt(
		app.Settings().UserPreferences(),
		app.Settings().VenvDir(),
	))
	app.SetSearchStrategyOverride(NewSearchStrategyPrompt())

	logDir := logging.ResolveLogDir()
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(logDir, "mindx.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})
	logger.Info("=== Daemon initialization starting ===", "addr", addr, "wsPath", wsPath)
	logger.Info("logger initialized", "log_file", filepath.Join(logDir, "mindx.log"))

	var schedulerDB *scheduler.FileSchedulerStore
	schedDB, err := scheduler.NewFileSchedulerStore(app.Settings().SchedulesDir())
	if err != nil {
		logger.Warn("failed to create scheduler store, scheduled tasks disabled", "error", err)
	} else {
		schedulerDB = schedDB
		logger.Info("scheduler store created", "dir", app.Settings().SchedulesDir())
	}

	// ── 知识图谱数据库 (gograph) ──────────────────────────────────
	// 必须在 memory 初始化之前创建，以便共享同一个 GraphDB 实例。
	var graphDB *graphapi.DB
	var graphStore *graphapi.GraphStore
	var graphErr error
	var coreGS goragcore.GraphStore
	var llmModelCfg *goragindexer.ModelConfig

	graphDB, graphStore, graphErr = initGraphDB(app.Settings().DataDir())
	if graphErr != nil {
		logger.Warn("failed to initialize knowledge-graph database", "error", graphErr)
	} else {
		coreGS = goraggograph.WrapGraphStore(graphDB, graphStore)
		logger.Info("knowledge-graph database initialized",
			"path", filepath.Join(app.Settings().DataDir(), "kb.db"),
		)
	}

	// ── GraphIndexer 模型配置 ────────────────────────────────────
	if defaultModel := app.ResolveDefaultModel(); defaultModel != nil {
		lang := resolveIndexerLang(app.Config())
		llmModelCfg = &goragindexer.ModelConfig{
			APIKey:        defaultModel.APIKey,
			BaseURL:       defaultModel.BaseURL,
			Model:         defaultModel.Name,
			Language:      lang,
			MaxTokens:     int(defaultModel.MaxTokens),
			ContextLength: int(defaultModel.ContextLength),
		}
		logger.Info("GraphIndexer model config resolved",
			"model", defaultModel.Name,
			"provider", defaultModel.Provider,
			"lang", lang,
		)
	}

	// ── WatchListStore / IndexStateStore（持久化存储）────────────
	// 与 FileWatchService 解耦，无条件创建，确保会话创建时始终能持久化
	// 监控目录条目。当 FileWatchService 初始化后会自动恢复这些条目。
	watchListStore, wlErr := indexing.NewWatchListStore(app.Settings().DataDir())
	if wlErr != nil {
		logger.Warn("filewatch: failed to create watchlist store", "error", wlErr)
	}
	indexStateStore, isErr := indexing.NewIndexStateStore(app.Settings().DataDir())
	if isErr != nil {
		logger.Warn("filewatch: failed to create index state store", "error", isErr)
	}
	manifestStore := indexing.NewManifestStore(app.Settings().DataDir())

	// ── GraphIndexer（知识库）─────────────────────────────────────
	// 文件监控服务为 KB 服务，memory 仅为对话服务
	var graphIndexer *goragindexer.GraphIndexer
	var graphIndexerErr error
	var sharedMemory *memory.RAGMemory
	var kbWatch *indexing.FileWatchService

	if emb := app.Embedder(); emb != nil {
		logger.Info("embedder found, initializing knowledge base and memory services")

		// ── KB Stack: GraphIndexer + RegionIndexer + FileWatchService ─
		// 需要 coreGS（graph store）、LLM 模型配置、以及持久化存储。
		// 任一条件缺失时跳过 KB 服务，记录具体原因到 graphIndexerErr。
		if coreGS != nil && llmModelCfg != nil {
			gi, kw, kbErr := newKBStack(
				emb, coreGS, llmModelCfg,
				app.Settings().DataDir(),
				logger,
				app.TokenUsageStore(),
				watchListStore, indexStateStore,
				app,
			)
			if kbErr != nil {
				logger.Warn("knowledge base init failed, KB service disabled", "error", kbErr)
				graphIndexerErr = kbErr
			} else {
				graphIndexer = gi
				kbWatch = kw
			}
		} else {
			if coreGS == nil {
				logger.Warn("graph store unavailable, GraphIndexer disabled")
				graphIndexerErr = fmt.Errorf("graph store unavailable (no vector/graph DB configured)")
			}
			if llmModelCfg == nil {
				logger.Warn("no LLM model configured, GraphIndexer disabled")
				graphIndexerErr = fmt.Errorf("no LLM model configured for knowledge base")
			}
		}

		// ── Shared Memory（对话记忆）─────────────────────────────
		// Memory 仅为对话服务，基于 SemanticIndexer
		sharedMem, memErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			AgentName: "_shared",
			MemoryDir: filepath.Join(app.Settings().UserPreferences(), "memory"),
			Embedder:  emb,
			Logger:    logger,
		})
		if memErr != nil {
			logger.Warn("failed to create shared RAG memory", "error", memErr)
		} else {
			sharedMemory = sharedMem
			logger.Info("shared RAG memory initialized for conversation")
		}
	} else {
		logger.Info("no embedder configured, knowledge base and memory disabled")
	}

	d := &Daemon{
		app:                 app,
		addr:                addr,
		wsPath:              wsPath,
		schedulerDB:         schedulerDB,
		kbWatch:             kbWatch,
		watchListStore:      watchListStore,
		indexStateStore:     indexStateStore,
		manifestStore:       manifestStore,
		sharedMemory:        sharedMemory,
		graphIndexer:        graphIndexer,
		graphIndexerErr:     graphIndexerErr,
		runtimeFS:           runtimeFS,
		webServer:           NewWebServer(WebDir(app.Settings().UserPreferences()), logger),
		logger:              logger,
		pendingInteractions: make(map[string]*pendingInteraction),
		restartCh:           make(chan struct{}, 1),
		manifestNewFileCh:   make(chan struct{}, 1),
		manifestResumeCh:    make(chan struct{}, 1),
	}

	// Pass GraphIndexer to App for use in LocalSearch tool
	if graphIndexer != nil {
		app.SetGraphIndexer(graphIndexer)
	}

	// Wire graphDB to daemon fields (deferred because d is needed)
	if graphDB != nil {
		d.graphDB = graphDB
		d.graphStore = graphStore
	} else {
		logger.Warn("knowledge-graph database unavailable, graph RPC disabled")
	}

	// Extract embedded app icon for favicon
	if iconFS := app.IconFS(); iconFS != nil {
		iconDest := filepath.Join(app.Settings().DataDir(), "mindx.png")
		if err := appicon.Write(iconFS, iconDest); err == nil {
			d.webServer.SetFavicon(iconDest)
			logger.Info("app icon extracted", "path", iconDest)
		}
	}

	if schedulerDB != nil {
		d.scheduler = scheduler.NewScheduler(schedulerDB, d.executeScheduleCommand, logger)
		logger.Info("scheduler instance created")

		// Wire lifecycle callback to broadcast job events to all connected clients.
		d.scheduler.OnLifecycle(func(info scheduler.JobLifecycleInfo) {
			if d.gw == nil {
				return
			}
			var method string
			switch info.Status {
			case "started":
				method = "schedule.job_started"
			case "completed":
				method = "schedule.job_completed"
			case "failed":
				method = "schedule.job_failed"
			default:
				method = "schedule.job_" + info.Status
			}
			d.gw.BroadcastNotification(method, info)
		})
	}

	// Initialize global KV store (bbolt)
	kvDB, kvErr := initKVStore(app.Settings().DataDir())
	if kvErr != nil {
		logger.Warn("failed to initialize kvstore", "error", kvErr)
	} else {
		d.kvStore = kvDB
		logger.Info("kvstore initialized",
			"path", filepath.Join(app.Settings().DataDir(), "kvstore.db"),
		)
	}

	// ── 自动升级 ──────────────────────────────────────────────
	// 确保 config.InstalledVersion 不为空（首次启动时设置）
	cfg := app.Config()
	if cfg.InstalledVersion == "" && core.Version != "" {
		cfg.InstalledVersion = core.Version
		if err := cfg.Save(); err != nil {
			logger.Warn("failed to save initial installed version", "error", err)
		}
	}
	d.updater = update.NewUpdater(
		core.Version,
		cfg.InstalledVersion,
		app.Settings().UserPreferences(),
		func(version string) error {
			cfg.InstalledVersion = version
			return cfg.Save()
		},
		func(msg string, args ...any) { logger.Info(fmt.Sprintf("updater: "+msg, args...)) },
	)

	d.logger.Info("=== Daemon initialization complete ===",
		"has_scheduler", d.scheduler != nil,
		"has_kb_watch", d.kbWatch != nil,
		"has_shared_memory", d.sharedMemory != nil,
		"has_graph_db", d.graphDB != nil,
		"has_kvstore", d.kvStore != nil,
	)

	// 定期清理超时的 pending interactions，防止客户端断线后内存泄漏
	{
		ctx, cancel := context.WithCancel(context.Background())
		d.cleanupCancel = cancel
		go d.cleanupStaleInteractionsLoop(ctx, 30*time.Minute, 5*time.Minute)
	}

	return d
}

// cleanupStaleInteractionsLoop 定期清理超时的 pending interactions，
// 防止客户端断线或超时未回复导致内存泄漏。
func (d *Daemon) cleanupStaleInteractionsLoop(ctx context.Context, timeout, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.cleanupStaleInteractions(timeout)
		case <-ctx.Done():
			return
		}
	}
}

// cleanupStaleInteractions 清理所有超过 timeout 时间的 pending interactions。
func (d *Daemon) cleanupStaleInteractions(timeout time.Duration) {
	d.interactMu.Lock()
	defer d.interactMu.Unlock()

	now := time.Now()
	var staleKeys []string
	for key, pi := range d.pendingInteractions {
		if now.Sub(pi.createdAt) > timeout {
			staleKeys = append(staleKeys, key)
		}
	}
	for _, key := range staleKeys {
		delete(d.pendingInteractions, key)
	}
	if len(staleKeys) > 0 {
		d.logger.Info("cleaned up stale pending interactions", "count", len(staleKeys))
	}
}

// autoUpdateLoop 在启动时进行一次检查，之后每 24 小时检查一次。
func (d *Daemon) autoUpdateLoop(ctx context.Context) {
	// 启动后稍等 10 秒再检查，避免启动流程堵塞
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			d.logger.Info("auto-update: checking for updates...")
			info := d.updater.Check(true)
			if info.Error != "" {
				d.logger.Warn("auto-update: check failed", "error", info.Error)
			} else if info.UpdateAvailable {
				d.logger.Info("auto-update: update available!",
					"current", info.CurrentVersion,
					"latest", info.LatestVersion,
				)
				// 通知客户端更新即将开始
				if d.gw != nil {
					d.gw.BroadcastNotification("update_started", map[string]any{
						"type": "update_started",
						"data": map[string]string{
							"version": info.LatestVersion,
						},
					})
				}
				// 自动下载并安装新二进制（但不要重启，只记录日志通知用户）
				if err := d.updater.DownloadAndInstall(ctx); err != nil {
					d.logger.Warn("auto-update: download and install failed", "error", err)
				} else {
					d.logger.Info("auto-update: update installed. User should restart the daemon.")
					if d.gw != nil {
						d.gw.BroadcastNotification("update_installed", map[string]any{
							"type": "update_installed",
							"data": map[string]string{
								"version": info.LatestVersion,
							},
						})
					}
				}
			} else {
				d.logger.Info("auto-update: already up-to-date", "version", info.CurrentVersion)
			}
			// 检查完毕后，每 24 小时检查一次
			timer.Reset(24 * time.Hour)

		case <-ctx.Done():
			d.logger.Info("auto-update: stopping")
			return
		}
	}
}

// addWatchEntry persists a directory to the file watch list store
// unconditionally (independent of FileWatchService status). When the
// FileWatchService is running, it also registers the fsnotify watcher
// for real-time monitoring. The store is idempotent — repeated calls
// with the same (dir, agent) pair are safe (dedup by WatchListStore).
func (d *Daemon) addWatchEntry(dir, agent string) error {
	if d.watchListStore == nil {
		return fmt.Errorf("addWatchEntry: watchListStore not initialized")
	}

	if err := d.watchListStore.Add(dir, agent); err != nil {
		return fmt.Errorf("addWatchEntry: failed to persist watch entry: %w", err)
	}
	d.logger.Info("addWatchEntry: watch entry persisted",
		"dir", dir,
		"agent", agent,
		"filewatch_active", d.kbWatch != nil,
	)
	if d.indexStateStore != nil {
		absDir, _ := filepath.Abs(dir)
		d.indexStateStore.SetPending(absDir)
	}
	// If FileWatchService is running, also register fsnotify watcher.
	// AddWatch's internal store.Add is a no-op (dedup), so this is safe.
	if d.kbWatch != nil {
		if err := d.kbWatch.AddWatch(dir, agent); err != nil {
			d.logger.Warn("addWatchEntry: failed to register fsnotify watcher",
				"dir", dir,
				"agent", agent,
				"error", err,
			)
		}
	}
	return nil
}

// resolveIndexerLang returns the language string used by GraphIndexer's
// ModelConfig based on the application language setting.
func resolveIndexerLang(cfg *core.MindxConfig) string {
	if cfg == nil {
		return "Chinese"
	}
	switch cfg.Language {
	case "en", "en-US", "en-GB":
		return "English"
	}
	return "Chinese"
}

// wireFileIndexCallback sets the file indexing broadcast callback on a
// FileWatchService. The callback is idempotent — safe to call multiple times
// (e.g., from both Start and ensureGraphIndexer).
func (d *Daemon) wireFileIndexCallback(kw *indexing.FileWatchService) {
	if kw == nil || d.gw == nil {
		return
	}
	kw.IndexEventCallback = func(absPath, relPath, absDir, eventType string) {
		d.gw.BroadcastNotification("file_indexing", map[string]any{
			"type": "file_indexing",
			"data": map[string]string{
				"file":      relPath,
				"directory": absDir,
				"state":     eventType,
			},
		})
	}
}

// restoreSessionWatches is a no-op in manual indexing mode.
// Sessions no longer auto-add directories to the file watchlist.
// Users add files to the index manifest manually via the File Explorer.
func (d *Daemon) restoreSessionWatches() {
	if d.logger != nil {
		d.logger.Info("restoreSessionWatches: auto-indexing disabled (manual mode)")
	}
}

// ensureGraphIndexer initializes GraphIndexer and FileWatchService at runtime
// if they were not created during startup (e.g., because no LLM model was configured).
// This allows model.switch to dynamically enable filewatch/auto-indexing.
func (d *Daemon) ensureGraphIndexer() error {
	if d.kbWatch != nil {
		d.logger.Info("ensureGraphIndexer: kbWatch already initialized")
		return nil
	}

	if d.graphStore == nil || d.graphDB == nil {
		return fmt.Errorf("graph store not available")
	}
	coreGS := goraggograph.WrapGraphStore(d.graphDB, d.graphStore)

	emb := d.app.Embedder()
	if emb == nil {
		return fmt.Errorf("embedder not available")
	}

	defaultModel := d.app.ResolveDefaultModel()
	if defaultModel == nil {
		return fmt.Errorf("no LLM model configured")
	}

	llmModelCfg := &goragindexer.ModelConfig{
		APIKey:        defaultModel.APIKey,
		BaseURL:       defaultModel.BaseURL,
		Model:         defaultModel.Name,
		Language:      resolveIndexerLang(d.app.Config()),
		MaxTokens:     int(defaultModel.MaxTokens),
		ContextLength: int(defaultModel.ContextLength),
	}

	// Ensure stores exist before building KB stack
	if d.watchListStore == nil {
		ws, wlErr := indexing.NewWatchListStore(d.app.Settings().DataDir())
		if wlErr != nil {
			return fmt.Errorf("create watchlist store: %w", wlErr)
		}
		d.watchListStore = ws
	}
	if d.indexStateStore == nil {
		is, isErr := indexing.NewIndexStateStore(d.app.Settings().DataDir())
		if isErr != nil {
			return fmt.Errorf("create index state store: %w", isErr)
		}
		d.indexStateStore = is
	}

	gi, kw, kbErr := newKBStack(
		emb, coreGS, llmModelCfg,
		d.app.Settings().DataDir(),
		d.logger,
		d.app.TokenUsageStore(),
		d.watchListStore, d.indexStateStore,
		d.app,
	)
	if kbErr != nil {
		d.graphIndexerErr = kbErr
		return kbErr
	}

	d.graphIndexer = gi
	d.app.SetGraphIndexer(gi)
	d.kbWatch = kw

	// Wire IndexEventCallback and restore existing watches if gateway already running.
	if d.gw != nil {
		d.wireFileIndexCallback(d.kbWatch)
		d.restoreSessionWatches()
	}

	return nil
}

func (d *Daemon) Start(ctx context.Context) error {
	d.startTime = time.Now()
	d.logger.Info("daemon start called", "addr", d.addr, "wsPath", d.wsPath)

	if d.gw == nil {
		d.logger.Info("initializing gateway")
		d.initGateway()
	}

	// Wire filewatch indexing events to WebUI broadcast
	if d.kbWatch != nil {
		d.wireFileIndexCallback(d.kbWatch)
		// Restore watches for all existing sessions with a project_dir.
		d.restoreSessionWatches()
	}

	if d.scheduler != nil {
		d.logger.Info("starting scheduler service")
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("Scheduler failed to start", "error", err)
		} else {
			d.logger.Info("scheduler started successfully")
		}
	} else {
		d.logger.Info("no scheduler configured, skipping")
	}

	// ── 自动升级检查（启动时 + 每日一次） ─────────────────
	go d.autoUpdateLoop(ctx)

	// ── Hot-reload: watch agents/skills directories for file changes ──
	d.hotReload = NewHotReloadWatcher(d.app, d.logger)
	go func() {
		defer func() {
			if r := recover(); r != nil && d.logger != nil {
				d.logger.Error("hot-reload watcher: goroutine panic", fmt.Errorf("%v", r))
			}
		}()
		if err := d.hotReload.Start(ctx); err != nil && d.logger != nil {
			d.logger.Warn("hot-reload watcher exited with error", "error", err)
		}
	}()

	if d.kbWatch != nil {
		// Note: Auto-indexing via FileWatchService is disabled in manual mode.
		// Users add files to the index manifest manually via the File Explorer,
		// and indexing is started/stopped per-session via kb.index.start/stop.
		// The FileWatchService is still available as an IndexService provider
		// for on-demand file indexing (via kbWatch.GetIndexer()).
		d.logger.Info("filewatch service configured but not auto-started (manual indexing mode)")
	} else {
		d.logger.Info("no filewatch configured, skipping")
	}

	// ── Start FIFO manifest worker ──────────────────────────────
	if d.kbWatch != nil && d.graphIndexer != nil {
		d.startManifestWorker(ctx)
		d.logger.Info("manifest FIFO worker started")
	} else {
		d.logger.Info("manifest FIFO worker not started (kbWatch or graphIndexer unavailable)")
	}

	// Register system health / diagnostics endpoint.
	d.webServer.HandleFunc("/api/health", d.handleHealth)
	// Register file download handler for binary file access.
	d.webServer.HandleFunc("/api/fs/download", d.handleFSDownload)

	if err := d.webServer.Start(ctx); err != nil {
		d.logger.Warn("WebUI server failed to start", "error", err)
	}

	addr := fmt.Sprintf("ws://localhost%s%s", d.addr, d.wsPath)
	d.logger.Info("MindX daemon starting", "addr", addr)
	d.logger.Info("gateway starting, waiting for connections...")

	// gw.Start() 启动 HTTP server（后台 goroutine）+ TCP 探测，探测成功后即返回。
	// 服务端在后台持续运行。如果启动失败则返回 error。
	if err := d.gw.Start(); err != nil {
		d.logger.Error("gateway start failed", err)
		d.stopBackgroundServices()
		return fmt.Errorf("gateway start failed: %w", err)
	}

	d.logger.Info("gateway started successfully, daemon is now running")
	d.logger.Info("daemon running, waiting for shutdown signal...")

	// 监听 shutdown 或 restart 信号
	var restart bool
	select {
	case <-ctx.Done():
		d.logger.Info("received shutdown signal, cleaning up...")
	case <-d.restartCh:
		d.logger.Info("restart requested, cleaning up...")
		restart = true
	}

	d.stopBackgroundServices()

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	d.logger.Info("shutting down gateway")

	if restart {
		d.logger.Info("starting new daemon process...")
		execPath, err := os.Executable()
		if err != nil {
			d.logger.Error("failed to get executable path", err)
			return fmt.Errorf("get executable: %w", err)
		}

		proc, err := os.StartProcess(execPath, os.Args, &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		})
		if err != nil {
			d.logger.Error("failed to start new daemon process", err)
			// The daemon is already shut down, so we return an error
			// (the caller will handle it via the original ctx.Done())
			return fmt.Errorf("restart: start new process: %w", err)
		}
		d.logger.Info("new daemon process started", "pid", proc.Pid)
		os.Exit(0)
	}

	return d.gw.Shutdown(ctx)
}

// Restart 触发 daemon 优雅重启：关闭服务 → 启动新进程 → os.Exit(0)
func (d *Daemon) Restart() {
	d.logger.Info("restart signal sent")
	select {
	case d.restartCh <- struct{}{}:
	default:
		d.logger.Warn("restart already requested")
	}
}

// startManifestWorker launches the FIFO queue worker goroutine for a given
// project directory. It processes pending files one at a time (serial, FIFO).
func (d *Daemon) startManifestWorker(ctx context.Context) {
	d.logger.Info("manifest worker: starting FIFO queue worker")

	ctx, cancel := context.WithCancel(ctx)
	d.manifestWorkerCancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("manifest worker: panic recovered", fmt.Errorf("%v", r))
			}
		}()
		defer d.logger.Info("manifest worker: stopped")

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check if we have any active session/project dir with pending work
			absDir, relFile := d.nextPendingFile()
			if relFile == "" {
				// Nothing to do — wait for a wake signal
				select {
				case <-d.manifestNewFileCh:
					continue
				case <-d.manifestResumeCh:
					continue
				case <-ctx.Done():
					return
				}
			}

			// Check if worker is paused
			if !d.isManifestProcessing(absDir) {
				select {
				case <-d.manifestResumeCh:
					continue
				case <-d.manifestNewFileCh:
					continue
				case <-ctx.Done():
					return
				}
			}

			// Process one file
			d.processManifestFile(ctx, absDir, relFile)
		}
	}()
}

// stopManifestWorker cancels the FIFO queue worker goroutine.
func (d *Daemon) stopManifestWorker() {
	if d.manifestWorkerCancel != nil {
		d.manifestWorkerCancel()
		d.manifestWorkerCancel = nil
	}
}

// wakeManifestWorker signals the worker to check for new pending files.
func (d *Daemon) wakeManifestWorker() {
	select {
	case d.manifestNewFileCh <- struct{}{}:
	default:
	}
}

// resumeManifestWorker signals the worker to resume processing after pause.
func (d *Daemon) resumeManifestWorker() {
	select {
	case d.manifestResumeCh <- struct{}{}:
	default:
	}
}

// nextPendingFile iterates all manifests and returns the next pending file.
func (d *Daemon) nextPendingFile() (absDir, relFile string) {
	if d.manifestStore == nil {
		return "", ""
	}
	for _, m := range d.manifestStore.All() {
		if !m.Processing {
			continue
		}
		p := m.PeekNext()
		if p != "" {
			return m.ProjectDir, p
		}
	}
	return "", ""
}

// isManifestProcessing checks whether worker processing is active for a dir.
func (d *Daemon) isManifestProcessing(absDir string) bool {
	if d.manifestStore == nil {
		return false
	}
	m := d.manifestStore.Get(absDir)
	return m != nil && m.Processing
}

// processManifestFile indexes a single file from the manifest queue.
func (d *Daemon) processManifestFile(ctx context.Context, absDir, relFile string) {
	m := d.manifestStore.LoadOrCreate(absDir)
	dequeued := m.DequeueNext()
	if dequeued == "" {
		return // already consumed
	}
	_ = d.manifestStore.Save(absDir)
	d.logger.Info("manifest worker: processing file", "dir", absDir, "file", relFile)

	// Broadcast indexing started
	if d.gw != nil {
		d.gw.BroadcastNotification("file_indexing", map[string]any{
			"type": "file_indexing",
			"data": map[string]string{
				"file":      relFile,
				"directory": absDir,
				"state":     "processing",
			},
		})
	}

	if d.graphIndexer == nil || d.kbWatch == nil {
		m.SetError(relFile, "indexer not available")
		_ = d.manifestStore.Save(absDir)
		d.logger.Warn("manifest worker: indexer not available, file failed", "file", relFile)
		return
	}

	// Get the indexer for this directory
	pi := d.kbWatch.GetIndexer(absDir)
	if pi == nil {
		m.SetError(relFile, "no indexer for this directory")
		_ = d.manifestStore.Save(absDir)
		d.logger.Warn("manifest worker: no indexer for dir", "dir", absDir)
		return
	}

	// Sync the single file
	result := pi.SyncFiles(ctx, absDir, []string{relFile}, false)

	// Capture LLM token usage
	tokens := &indexing.TokenUsage{}
	if d.graphIndexer != nil {
		tu := d.graphIndexer.LastTokenUsage()
		if tu != nil {
			tokens.InputTokens = tu.PromptTokens
			tokens.OutputTokens = tu.CompletionTokens
			tokens.Cost = calculateIndexCost(tu.PromptTokens, tu.CompletionTokens)
		}
	}

	if len(result.FailedFiles) > 0 {
		failedRec := result.FailedFiles[0]
		m.SetError(relFile, failedRec.Error)
		_ = d.manifestStore.Save(absDir)

		if d.gw != nil {
			d.gw.BroadcastNotification("file_indexing", map[string]any{
				"type": "file_indexing",
				"data": map[string]string{
					"file":      relFile,
					"directory": absDir,
					"state":     "error",
				},
			})
		}
		d.logger.Warn("manifest worker: file indexing failed", "file", relFile, "error", failedRec.Error)
	} else {
		// Record completion
		elapsedMs := int64(0)
		chunks := result.Indexed
		if len(result.CompletedFiles) > 0 {
			cf := result.CompletedFiles[0]
			elapsedMs = cf.Elapsed.Milliseconds()
			if cf.Chunks > 0 {
				chunks = cf.Chunks
			}
		}
		tokens.ElapsedMs = elapsedMs
		m.SetDone(relFile, tokens, elapsedMs, chunks)
		_ = d.manifestStore.Save(absDir)

		if d.gw != nil {
			d.gw.BroadcastNotification("file_indexing", map[string]any{
				"type": "file_indexing",
				"data": map[string]string{
					"file":      relFile,
					"directory": absDir,
					"state":     "done",
				},
			})
		}
		d.logger.Info("manifest worker: file indexed successfully", "file", relFile)
	}
}

// calculateIndexCost estimates the USD cost for indexing a single file
// based on LLM token consumption. Uses GPT-4o-mini pricing as default:
//
//	Input:  $0.15  / 1M tokens
//	Output: $0.60  / 1M tokens
func calculateIndexCost(inputTokens, outputTokens int) float64 {
	const inputPricePerM = 0.15
	const outputPricePerM = 0.60
	inputCost := float64(inputTokens) / 1_000_000 * inputPricePerM
	outputCost := float64(outputTokens) / 1_000_000 * outputPricePerM
	return inputCost + outputCost
}

// stopService stops a service whose Stop method returns no error.
func (d *Daemon) stopService(name string, stopper func()) {
	if stopper == nil {
		return
	}
	d.logger.Info("stopping " + name)
	stopper()
	d.logger.Info(name + " stopped")
}

// stopCloseable stops a service whose Close method returns an error.
func (d *Daemon) stopCloseable(name string, closer func() error) {
	if closer == nil {
		return
	}
	d.logger.Info("closing " + name)
	if err := closer(); err != nil {
		d.logger.Warn("failed to close "+name, "error", err)
	} else {
		d.logger.Info(name + " closed")
	}
}

func (d *Daemon) stopBackgroundServices() {
	d.logger.Info("stopping background services...")

	// Cancel stale interaction cleanup loop first (no I/O, no blocking).
	if d.cleanupCancel != nil {
		d.cleanupCancel()
		d.cleanupCancel = nil
	}

	d.stopService("manifest FIFO worker", func() {
		d.stopManifestWorker()
	})

	d.stopService("hot-reload watcher", func() {
		if d.hotReload != nil {
			d.hotReload.Stop()
		}
	})

	if d.kbWatch != nil {
		d.logger.Info("stopping filewatch service")
		// Cancel the external watch context first so the Start() goroutine
		// can unblock, then stop the internal eventLoop.
		if d.watchCancel != nil {
			d.watchCancel()
			d.watchCancel = nil
		}
		d.kbWatch.Stop()
		d.logger.Info("filewatch service stopped")
	}

	d.stopService("scheduler service", func() {
		if d.scheduler != nil {
			d.scheduler.Stop()
		}
	})

	d.stopCloseable("knowledge-graph database", func() error {
		if d.graphDB != nil {
			return d.graphDB.Close()
		}
		return nil
	})

	d.stopCloseable("kvstore", func() error {
		if d.kvStore != nil {
			return d.kvStore.Close()
		}
		return nil
	})

	d.logger.Info("all background services stopped")
}

// ---------------------------------------------------------------------------
// Health / Diagnostics — GET /api/health
// ---------------------------------------------------------------------------

// healthResponse is the JSON payload returned by /api/health.
type healthResponse struct {
	Status   string         `json:"status"`
	Version  string         `json:"version"`
	Commit   string         `json:"commit"`
	Build    string         `json:"build"`
	Dirty    string         `json:"dirty"`
	Uptime   string         `json:"uptime"`
	Services map[string]any `json:"services"`
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(d.startTime).Truncate(time.Second).String()
	if d.startTime.IsZero() {
		uptime = "starting…"
	}

	services := map[string]any{}
	degraded := false

	// WebSocket gateway
	if d.gw != nil {
		services["websocket"] = map[string]any{
			"status": "running",
			"addr":   d.addr,
			"path":   d.wsPath,
		}
	} else {
		services["websocket"] = map[string]any{"status": "not initialized"}
		degraded = true
	}

	// Memory / RAG
	if d.sharedMemory != nil {
		idx := d.sharedMemory.Semantic()
		var totalChunks int
		if idx != nil {
			if cnt, err := idx.Count(context.Background()); err == nil {
				totalChunks = cnt
			}
		}
		services["memory"] = map[string]any{
			"status":       "running",
			"total_chunks": totalChunks,
			"agent":        "_shared",
		}
	} else {
		services["memory"] = map[string]any{"status": "not configured"}
	}

	// FileWatch
	if d.kbWatch != nil {
		fwStatus := "stopped"
		if d.kbWatch.IsRunning() {
			fwStatus = "running"
		}
		services["filewatch"] = map[string]any{"status": fwStatus}
	} else {
		services["filewatch"] = map[string]any{"status": "disabled"}
	}

	// Scheduler
	if d.scheduler != nil {
		services["scheduler"] = map[string]any{"status": "running"}
	} else {
		services["scheduler"] = map[string]any{"status": "disabled"}
	}

	// Knowledge graph
	if d.graphDB != nil {
		services["knowledge_graph"] = map[string]any{"status": "running"}
	} else {
		services["knowledge_graph"] = map[string]any{"status": "disabled"}
	}

	overall := "ok"
	if degraded {
		overall = "degraded"
	}

	resp := healthResponse{
		Status:   overall,
		Version:  core.Version,
		Commit:   core.Commit,
		Build:    core.BuildTime,
		Dirty:    core.Dirty,
		Uptime:   uptime,
		Services: services,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (d *Daemon) initGateway() {
	d.logger.Info("initializing WebSocket gateway",
		"addr", d.addr,
		"wsPath", d.wsPath,
	)
	d.gw = gateway.New(
		gateway.WithAddr(d.addr),
		gateway.WithPath(d.wsPath),
		gateway.WithHandler(d.defaultHandler),
		gateway.WithDisconnectHandler(func(clientID string) {
			d.logger.Debug("client disconnected, cancelling running execution",
				"client_id", clientID,
			)
			d.cancelClientExecution(clientID)
			termMgr.cleanupClient(clientID)
		}),
	)
	d.logger.Info("gateway instance created")

	registry := NewRPCHandlerRegistry(d)
	registry.RegisterAll(d.gw)
	d.logger.Info("RPC handlers registered successfully")
}

// ---------------------------------------------------------------------------
// Message Handler & Session Resolution
// ---------------------------------------------------------------------------

func (d *Daemon) defaultHandler(msg *gateway.Message) {
	d.logger.Debug("defaultHandler: received message",
		"client_id", msg.ClientID,
		"session_id", msg.SessionID,
		"data_size", len(msg.Data),
	)

	var payload struct {
		Text      string `json:"text"`
		SessionID string `json:"session_id,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.Text == "" {
		d.logger.Warn("defaultHandler: missing or invalid text field",
			"data", string(msg.Data), "error", err)
		return
	}
	if payload.SessionID != "" {
		msg.SessionID = payload.SessionID
	}
	text := payload.Text

	d.logger.Info("defaultHandler: parsing agent target",
		"client_id", msg.ClientID,
		"text_preview", truncate(text, 100),
	)

	agentName, providedSessionID, content := parseAgentTarget(text)

	rt, err := d.app.ResolveRuntime(agentName)
	if err != nil {
		d.logger.Error("defaultHandler: failed to resolve runtime", err,
			"client_id", msg.ClientID,
			"requested_agent", agentName,
		)
		d.sendEvent(msg.ClientID, msg.SessionID, gateway.RespError, i18n.T("svc.event.error"), err.Error())
		return
	}

	// Wire FileModifyHook to look up sessions from the active sessions map.
	// This enables file backup before Write/FileEdit tools execute.
	rt.WithFileModifyTracker(func(sessionID string) (action.TrackFunc, bool) {
		val, ok := d.activeSessions.Load(sessionID)
		if !ok {
			return nil, false
		}
		sess := val.(*goharnesssession.Session)
		return sess.TrackModify, true
	})

	// NOTE: Old GrantCache / execution.resume non-blocking permission flow has
	// been removed. Permission resumption now flows through the
	// PermissionAllow / PermissionDeny magic words (see runtime.resolvePermissionMagicWord),
	// which the UI sends as a regular user message. The runtime intercepts
	// the magic word, drains session.PendingPermission, and either runs the
	// tool (Allow) or appends a "Permission Denied" result (Deny).

	sessionID := d.resolveSessionID(msg.SessionID, providedSessionID)
	resolvedAgentName := agentName
	if resolvedAgentName == "" {
		resolvedAgentName = d.app.CurrentAgentName()
	}

	d.logger.Info("request start",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
		"input_preview", truncate(content, 100),
	)

	// Cancel any existing execution for this client
	d.cancelClientExecution(msg.ClientID)

	// Create cancellable context for interrupt/cancel support
	ctx, cancel := context.WithCancel(context.Background())
	d.clientCancels.Store(msg.ClientID, cancel)

	// Load existing session from persistent store (verifies it exists).
	s, err := goharnesssession.Load(sessionID, resolvedAgentName, d.app.SessDB())
	if err != nil {
		d.logger.Error("failed to load session", err, "session_id", sessionID)
		d.sendEvent(msg.ClientID, sessionID, gateway.RespError, "Session Error", err.Error())
		return
	}
	d.activeSessions.Store(sessionID, s)

	clientID := msg.ClientID
	sid := sessionID
	gw := d.gw
	currentAgentName := resolvedAgentName

	// withAgent returns a ResponseOption that includes the current agent_name in meta.
	// Updated by OnEvent handler when sub-agent events are forwarded to the parent.
	withAgent := func() gateway.ResponseOption {
		return gateway.WithResponseMeta(map[string]any{"agent_name": currentAgentName})
	}

	d.logger.Debug("request: starting async execution via AskBuilder",
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)

	go func() {
		defer func() {
			d.activeSessions.Delete(sid)
			if r := recover(); r != nil {
				d.logger.Error("defaultHandler: AskBuilder panic", fmt.Errorf("%v", r),
					"client_id", clientID, "session_id", sid)
				d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.error"), fmt.Sprintf(i18n.T("svc.event.execution.exception"), r))
			}
		}()
		defer func() {
			cancel()
			d.clientCancels.Delete(clientID)
		}()

		// ── Build common event handlers via factory ──
		emitter := newClientAskHandlers(d, gw, clientID, sid, withAgent, s, func() string { return currentAgentName })

		builder := rt.Ask(resolvedAgentName, content, s).
			WithContext(ctx).
			OnEvent(func(ev events.ReactEvent) {
				currentAgentName = ev.AgentName
			})
		builder = wireAskEvents(builder, emitter)

		_, err := builder.
			OnPermissionDenied(func(reason string) {
				d.sendEvent(clientID, sid, gateway.RespPermissionDenied, i18n.T("svc.event.permission.denied"), reason, withAgent())
			}).
			Run()

		if err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("request failed", err,
				"client_id", clientID,
				"session_id", sid,
				"agent", resolvedAgentName,
			)
			d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.error"), i18n.T("svc.event.request.failed"))
		}

		d.logger.Info("request done",
			"client_id", clientID,
			"session_id", sid,
			"agent", resolvedAgentName,
		)
	}()
}

// cancelClientExecution cancels any running execution for the given client.
func (d *Daemon) cancelClientExecution(clientID string) {
	if v, ok := d.clientCancels.Load(clientID); ok {
		v.(context.CancelFunc)()
		d.clientCancels.Delete(clientID)
	}
}

func parseAgentTarget(text string) (agentName string, sessionID string, content string) {
	matches := atAgentRegex.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", "", text
	}
	agentName = matches[1]
	if len(matches) < 3 || matches[2] == "" {
		return agentName, "", ""
	}

	rest := matches[2]
	parts := strings.SplitN(rest, " ", 2)
	if ulidRegex.MatchString(parts[0]) {
		sessionID = parts[0]
		if len(parts) > 1 {
			content = parts[1]
		}
		return agentName, sessionID, content
	}

	return agentName, "", rest
}

func (d *Daemon) resolveSessionID(clientProvided string, commandProvided string) string {
	if commandProvided != "" && commandProvided != "new" {
		d.logger.Info("using session_id from command", "session", commandProvided)
		return commandProvided
	}
	if clientProvided != "" {
		return clientProvided
	}

	if d.app.SessionDB() != nil {
		currentAgent := d.app.CurrentAgentName()
		if currentAgent != "" {
			sid, err := d.app.SessionDB().GetByRole(context.Background(), currentAgent)
			if err == nil && sid != nil && sid.SessionID != "" {
				d.logger.Info("resumed session from store", "agent", currentAgent, "session", sid.SessionID)
				return sid.SessionID
			}
		}
	}

	sid := generateSessionID()
	d.logger.Info("created new session", "session", sid)
	return sid
}

// ---------------------------------------------------------------------------
// Scheduler Command Execution
// ---------------------------------------------------------------------------

func (d *Daemon) executeScheduleCommand(ctx context.Context, agent string, sessionID string, content string, projectDir string) error {
	d.logger.Info("scheduled task: execution started",
		"agent", agent,
		"session_id", sessionID,
		"project_dir", projectDir,
		"content_preview", truncate(content, 100),
	)

	rt, err := d.app.ResolveRuntime(agent)
	if err != nil {
		d.logger.Error("scheduled task: failed to resolve runtime", err,
			"agent", agent,
		)
		return fmt.Errorf("resolve runtime for %q: %w", agent, err)
	}

	targetDir := projectDir
	if targetDir == "" {
		meta := d.restoreSessionEnvironment(sessionID)
		if meta != nil {
			targetDir = meta.ProjectWorkingDir
			d.logger.Info("scheduled task: restored project dir from session meta",
				"target_dir", targetDir,
			)
		}
	}

	s, err := goharnesssession.Load(sessionID, agent, d.app.SessDB())
	if err != nil {
		return fmt.Errorf("scheduled task: load session %q: %w", sessionID, err)
	}

	// Build AskBuilder with common event handlers (via factory).
	emitter := newBroadcastAskHandlers(d, sessionID, agent)
	ask := wireAskEvents(rt.Ask(agent, content, s).WithContext(ctx), emitter)

	// ── Optional chdir for project directory ──
	if targetDir != "" {
		d.execMu.Lock()
		originalCWD, _ := os.Getwd()

		if err := os.Chdir(targetDir); err != nil {
			d.execMu.Unlock()
			d.logger.Warn("scheduled task: failed to chdir to project dir, using current dir",
				"project_dir", targetDir, "error", err)
		} else {
			_ = os.Setenv("MINDX_PROJECT_DIR", targetDir)
			_ = os.Setenv("MINDX_SESSION_ID", sessionID)
			defer func() {
				if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
					d.logger.Warn("scheduled task: failed to restore cwd", "original", originalCWD, "error", restoreErr)
				}
				_ = os.Unsetenv("MINDX_PROJECT_DIR")
				_ = os.Unsetenv("MINDX_SESSION_ID")
				d.execMu.Unlock()
			}()
		}
	}

	d.logger.Info("scheduled task: calling Runtime.Ask()",
		"session_id", sessionID, "agent", agent)
	_, err = ask.Run()
	d.logger.Info("scheduled task: Runtime.Ask() returned",
		"session_id", sessionID, "error", err)

	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
	}

	d.logger.Info("scheduled task: execution completed successfully",
		"session_id", sessionID, "agent", agent)
	return nil
}

func (d *Daemon) restoreSessionEnvironment(sessionID string) *mindxses.SessionMeta {
	if d.app == nil || d.app.SessDB() == nil {
		return nil
	}
	meta, err := d.app.SessDB().GetSessionMeta(sessionID)
	if err != nil {
		d.logger.Debug("could not load session meta for scheduled task",
			"session_id", sessionID,
			"error", err,
		)
		return nil
	}
	return meta
}

// ---------------------------------------------------------------------------
// Public Accessors
// ---------------------------------------------------------------------------

func (d *Daemon) Gateway() *gateway.Server {
	return d.gw
}

func (d *Daemon) App() *core.App {
	return d.app
}

func (d *Daemon) Scheduler() *scheduler.Scheduler {
	return d.scheduler
}

func (d *Daemon) SchedulerDB() *scheduler.FileSchedulerStore {
	return d.schedulerDB
}

func (d *Daemon) Addr() string {
	return d.addr
}

func (d *Daemon) WSPath() string {
	return d.wsPath
}

func (d *Daemon) GraphDB() *graphapi.DB {
	return d.graphDB
}

func (d *Daemon) GraphStore() *graphapi.GraphStore {
	return d.graphStore
}

func (d *Daemon) KVStore() *bbolt.DB {
	return d.kvStore
}
