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
	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goharnesssession "github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/core"
	goragindexer "github.com/DotNetAge/gorag/indexer"
	goraggograph "github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/appicon"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/update"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"gopkg.in/yaml.v3"
)

var (
	atAgentRegex = regexp.MustCompile(`^@([\w-]+)(?:\s+(.+))?$`)
	ulidRegex    = regexp.MustCompile(`^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$`)
)

// fileDiffInfo holds per-file diff data emitted via RespFileModified.
type fileDiffInfo struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsNew     bool   `json:"isNew"`
}

// computeFileDiff reads the current file and its backup (if exists) to compute diff stats.
func computeFileDiff(sess *goharnesssession.Session, filePath string) fileDiffInfo {
	info := fileDiffInfo{Path: filePath}

	current, err := os.ReadFile(filePath)
	if err != nil {
		return info
	}
	newContent := string(current)

	sessionDir := sess.SessionDir()
	if sessionDir == "" {
		// No session dir — can't find backups, treat as new
		lines := strings.Split(newContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		info.IsNew = true
		info.Additions = len(lines)
		info.Diff = buildNewFileDiff(filePath, lines)
		return info
	}

	backupPath := filepath.Join(sessionDir, "backup", filepath.Base(filePath)+".bak")
	oldData, oldErr := os.ReadFile(backupPath)
	if oldErr != nil {
		// No backup — new file
		lines := strings.Split(newContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		info.IsNew = true
		info.Additions = len(lines)
		info.Diff = buildNewFileDiff(filePath, lines)
		return info
	}

	oldContent := string(oldData)
	info.Diff = buildUnifiedDiff(filePath, oldContent, newContent)
	info.Additions, info.Deletions = countDiffLines(oldContent, newContent)
	return info
}

// buildNewFileDiff generates a unified-diff-style string for a newly created file.
func buildNewFileDiff(filePath string, lines []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n", filepath.Base(filePath)))
	b.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
	for _, line := range lines {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// buildUnifiedDiff generates a basic unified diff string for a modified file.
func buildUnifiedDiff(filePath, oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", filepath.Base(filePath), filepath.Base(filePath)))

	// Simple line-by-line diff: scan for changes and emit hunks.
	type diffLine struct {
		kind byte // ' ', '+', '-'
		text string
	}

	var diff []diffLine

	// Build a simple LCS-based diff
	// First pass: mark unchanged, added, removed
	oldUsed := make([]bool, len(oldLines))
	newUsed := make([]bool, len(newLines))

	// Match identical lines in order
	ni := 0
	for oi := 0; oi < len(oldLines); oi++ {
		if ni >= len(newLines) {
			break
		}
		if oldLines[oi] == newLines[ni] {
			oldUsed[oi] = true
			newUsed[ni] = true
			ni++
		} else {
			// Try to find this old line later in new lines
			found := false
			for nj := ni + 1; nj < len(newLines); nj++ {
				if oldLines[oi] == newLines[nj] {
					// Mark skipped new lines as additions
					for nk := ni; nk < nj; nk++ {
						if !newUsed[nk] {
							newUsed[nk] = true
							diff = append(diff, diffLine{kind: '+', text: newLines[nk]})
						}
					}
					oldUsed[oi] = true
					newUsed[nj] = true
					diff = append(diff, diffLine{kind: ' ', text: oldLines[oi]})
					ni = nj + 1
					found = true
					break
				}
			}
			if !found {
				diff = append(diff, diffLine{kind: '-', text: oldLines[oi]})
			}
		}
	}

	// Remaining new lines are additions
	for ; ni < len(newLines); ni++ {
		if !newUsed[ni] {
			diff = append(diff, diffLine{kind: '+', text: newLines[ni]})
		}
	}
	// Remaining old lines are deletions
	for oi := 0; oi < len(oldLines); oi++ {
		if !oldUsed[oi] {
			// Check if already added as deletion
			alreadyAdded := false
			for _, d := range diff {
				if d.kind == '-' && d.text == oldLines[oi] {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				diff = append(diff, diffLine{kind: '-', text: oldLines[oi]})
			}
		}
	}

	if len(diff) == 0 {
		return ""
	}

	// Emit hunks with context
	b.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))
	for _, d := range diff {
		b.WriteByte(d.kind)
		b.WriteString(d.text)
		b.WriteString("\n")
	}

	return b.String()
}

// countDiffLines counts added and removed lines.
func countDiffLines(oldContent, newContent string) (additions, deletions int) {
	oldSet := make(map[string]int)
	for _, l := range strings.Split(oldContent, "\n") {
		oldSet[l]++
	}
	for _, l := range strings.Split(newContent, "\n") {
		if _, exists := oldSet[l]; exists {
			oldSet[l]--
		} else {
			additions++
		}
	}
	for _, count := range oldSet {
		if count > 0 {
			deletions += count
		}
	}
	return additions, deletions
}

// splitLines splits content into lines, dropping the trailing empty line.
func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type pendingInteraction struct {
	replyFn   func(answers map[string]string)
	grantFn   func(params map[string]any)
	denyFn    func(reason string)
	createdAt time.Time
}

type Daemon struct {
	app           *core.App
	gw            *gateway.Server
	scheduler     *scheduler.Scheduler
	schedulerDB   *scheduler.FileSchedulerStore
	memoryWatch   *memory.FileWatchService
	sharedMemory  *memory.RAGMemory
	webServer     *WebServer
	addr          string
	wsPath        string
	logger        logging.Logger
	execMu        sync.Mutex
	clientCancels sync.Map

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

	// hotReload watches agents/ and skills/ directories for file changes
	// and automatically reloads registries.
	hotReload *HotReloadWatcher
}

func NewDaemon(app *core.App, addr, wsPath string, runtimeFS fs.FS) *Daemon {
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

	// ── LLMIndexer 模型配置 ──────────────────────────────────────
	if defaultModel := app.ResolveDefaultModel(); defaultModel != nil {
		lang := "Chinese"
		if c := app.Config(); c != nil {
			switch c.Language {
			case "en", "en-US", "en-GB":
				lang = "English"
			}
		}
		llmModelCfg = &goragindexer.ModelConfig{
			APIKey:    defaultModel.APIKey,
			BaseURL:   defaultModel.BaseURL,
			Model:     defaultModel.Name,
			Language:  lang,
			MaxTokens: int(defaultModel.MaxTokens),
		}
		logger.Info("LLMIndexer model config resolved",
			"model", defaultModel.Name,
			"provider", defaultModel.Provider,
			"lang", lang,
		)
	}

	// ── Shared LongTerm Memory ──────────────────────────────────
	var memoryWatch *memory.FileWatchService
	var sharedMemory *memory.RAGMemory
	if emb := app.Embedder(); emb != nil {
		logger.Info("embedder found, initializing shared memory service")

		// 尝试从 entity_tags.yml 加载用户之前保存的实体标签定义
		var entityDefs []string
		entityTagsPath := filepath.Join(app.Settings().DataDir(), "entity_tags.yml")
		if etData, etErr := os.ReadFile(entityTagsPath); etErr == nil {
			var etFile struct {
				Types []struct {
					Name string `yaml:"name"`
					Desc string `yaml:"desc"`
				} `yaml:"types"`
			}
			if yaml.Unmarshal(etData, &etFile) == nil {
				for _, t := range etFile.Types {
					if t.Name != "" {
						entityDefs = append(entityDefs, "**"+t.Name+"** — "+t.Desc)
					}
				}
				logger.Info("memory: loaded saved entity tags from file", "path", entityTagsPath, "count", len(entityDefs))
			}
		}

		sharedMem, memErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: goharnessmemory.MemoryTypeLongTerm,
			AgentName:  "_shared",
			MemoryDir:  filepath.Join(app.Settings().UserPreferences(), "memory"),
			Embedder:   emb,
			GraphStore: coreGS,
			LLMConfig:  llmModelCfg,
			EntityDefs: entityDefs,
			Logger:     logger,
		})
		if memErr != nil {
			logger.Warn("filewatch: failed to create shared LongTerm indexer, watch disabled", "error", memErr)
		} else {
			sharedMemory = sharedMem
			logger.Info("shared RAG memory initialized")
			watchList, wlErr := memory.NewWatchListStore(app.Settings().DataDir())
			if wlErr != nil {
				logger.Warn("filewatch: failed to create watchlist store, watch disabled", "error", wlErr)
			} else {
				memoryWatch = memory.NewFileWatchService(
					sharedMem.Indexer(),
					watchList,
					filepath.Join(app.Settings().DataDir(), "memory-cache"),
					logger,
				)
				logger.Info("filewatch service configured",
					"cache_dir", filepath.Join(app.Settings().DataDir(), "memory-cache"),
				)
				memoryWatch.VersionRecorder = func(absPath string) {
					if app.SessDB() == nil || app.FileVersions() == nil {
						return
					}
					sessions, listErr := app.SessDB().ListSessions(context.Background())
					if listErr != nil {
						return
					}
					for _, s := range sessions {
						if s.ProjectDir == "" || !strings.HasPrefix(absPath, s.ProjectDir) {
							continue
						}
						if s.SessionDir != "" {
							_ = app.FileVersions().Record(s.SessionDir, absPath)
						}
					}
				}
			}
		}
	} else {
		logger.Info("no embedder configured, filewatch disabled")
	}

	d := &Daemon{
		app:                 app,
		addr:                addr,
		wsPath:              wsPath,
		schedulerDB:         schedulerDB,
		memoryWatch:         memoryWatch,
		sharedMemory:        sharedMemory,
		runtimeFS:           runtimeFS,
		webServer:           NewWebServer(WebDir(app.Settings().UserPreferences()), logger),
		logger:              logger,
		pendingInteractions: make(map[string]*pendingInteraction),
		restartCh:           make(chan struct{}, 1),
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
		"has_memory_watch", d.memoryWatch != nil,
		"has_shared_memory", d.sharedMemory != nil,
		"has_graph_db", d.graphDB != nil,
		"has_kvstore", d.kvStore != nil,
	)

	// 定期清理超时的 pending interactions，防止客户端断线后内存泄漏
	go d.cleanupStaleInteractionsLoop(30*time.Minute, 5*time.Minute)

	return d
}

// cleanupStaleInteractionsLoop 定期清理超时的 pending interactions，
// 防止客户端断线或超时未回复导致内存泄漏。
func (d *Daemon) cleanupStaleInteractionsLoop(timeout, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		d.cleanupStaleInteractions(timeout)
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
				// 自动下载并安装新二进制（但不要重启，只记录日志通知用户）
				if err := d.updater.DownloadAndInstall(ctx); err != nil {
					d.logger.Warn("auto-update: download and install failed", "error", err)
				} else {
					d.logger.Info("auto-update: update installed. User should restart the daemon.")
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

func (d *Daemon) Start(ctx context.Context) error {
	d.startTime = time.Now()
	d.logger.Info("daemon start called", "addr", d.addr, "wsPath", d.wsPath)

	if d.gw == nil {
		d.logger.Info("initializing gateway")
		d.initGateway()
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
		if err := d.hotReload.Start(ctx); err != nil && d.logger != nil {
			d.logger.Warn("hot-reload watcher exited with error", "error", err)
		}
	}()

	if d.memoryWatch != nil {
		d.logger.Info("filewatch service configured but not started (user must call filewatch.start to activate)")
	} else {
		d.logger.Info("no filewatch configured, skipping")
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

func (d *Daemon) stopBackgroundServices() {
	d.logger.Info("stopping background services...")
	if d.hotReload != nil {
		d.logger.Info("stopping hot-reload watcher")
		d.hotReload.Stop()
		d.logger.Info("hot-reload watcher stopped")
	}
	if d.memoryWatch != nil {
		d.logger.Info("stopping filewatch service")
		// Cancel the external watch context first so the Start() goroutine
		// can unblock, then stop the internal eventLoop.
		if d.watchCancel != nil {
			d.watchCancel()
			d.watchCancel = nil
		}
		d.memoryWatch.Stop()
		d.logger.Info("filewatch service stopped")
	}
	if d.scheduler != nil {
		d.logger.Info("stopping scheduler service")
		d.scheduler.Stop()
		d.logger.Info("scheduler service stopped")
	}
	if d.graphDB != nil {
		d.logger.Info("closing knowledge-graph database")
		if err := d.graphDB.Close(); err != nil {
			d.logger.Warn("failed to close knowledge-graph database", "error", err)
		} else {
			d.logger.Info("knowledge-graph database closed")
		}
	}
	if d.kvStore != nil {
		d.logger.Info("closing kvstore")
		if err := d.kvStore.Close(); err != nil {
			d.logger.Warn("failed to close kvstore", "error", err)
		} else {
			d.logger.Info("kvstore closed")
		}
	}
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

	// WebSocket gateway
	if d.gw != nil {
		services["websocket"] = map[string]any{
			"status": "running",
			"addr":   d.addr,
			"path":   d.wsPath,
		}
	} else {
		services["websocket"] = map[string]any{"status": "not initialized"}
	}

	// Memory / RAG
	if d.sharedMemory != nil {
		idx := d.sharedMemory.Indexer()
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
	if d.memoryWatch != nil {
		fwStatus := "stopped"
		if d.memoryWatch.IsRunning() {
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
	for _, svc := range services {
		m, ok := svc.(map[string]any)
		if ok && m["status"] == "not initialized" {
			overall = "degraded"
		}
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

	// Create session backed by the file store (lazy-loading: auto-loads on first access)
	s := goharnesssession.NewSession(sessionID, resolvedAgentName,
		goharnesssession.WithStore(d.app.SessDB()),
	)
	d.activeSessions.Store(sessionID, s)

	clientID := msg.ClientID
	sid := sessionID
	gw := d.gw

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

		_, err := rt.Ask(resolvedAgentName, content, s).
			WithContext(ctx).
			OnThinking(func(chunk string) {
				_ = gw.SendResponse(clientID, gateway.RespThinkingDelta, i18n.T("svc.event.thinking"), chunk, gateway.WithSessionID(sid))
			}).
			OnContent(func(chunk string) {
				d.sendEvent(clientID, sid, gateway.RespMarkdown, i18n.T("svc.event.outputting"), chunk)
			}).
			OnToolUseDelta(func(data events.ToolUseDeltaData) {
_ = gw.SendResponse(clientID, gateway.RespToolUseDelta, i18n.T("svc.event.tool.use.delta"), map[string]any{
					"index": data.Index, "id": data.ID, "name": data.Name, "arguments": data.Arguments,
				}, gateway.WithSessionID(sid))
			}).
			OnThinkingDone(func() {
				d.sendEvent(clientID, sid, gateway.RespThinkingDone, i18n.T("svc.event.thinking.done"), i18n.T("svc.event.thinking.done.detail"))
			}).
			OnToolStart(func(data events.ToolExecStartData) {
_ = gw.SendResponse(clientID, gateway.RespToolExecStart, i18n.T("svc.event.tool.start"), map[string]any{
					"tool_name": data.ToolName, "params": data.Params, "predicted_tokens": data.PredictedTokens,
				}, gateway.WithSessionID(sid))
			}).
			OnToolEnd(func(data events.ToolExecEndData) {
_ = gw.SendResponse(clientID, gateway.RespToolExecEnd, i18n.T("svc.event.tool.end"), map[string]any{
					"tool_name": data.ToolName, "tool_call_id": data.ToolCallID,
					"success": data.Success, "result": data.Result, "error": data.Error,
					"duration_ms": int(data.Duration.Milliseconds()),
				}, gateway.WithSessionID(sid))
				// Broadcast file modification state after tool execution.
				// Compute diff stats (additions/deletions/diff text) for each modified file.
				modFiles := s.GetModifyFiles()
				if len(modFiles) > 0 {
					fileInfos := make([]fileDiffInfo, 0, len(modFiles))
					for _, fp := range modFiles {
						fileInfos = append(fileInfos, computeFileDiff(s, fp))
					}
					gw.SendResponse(clientID, gateway.RespFileModified, i18n.T("svc.event.file.modified"), map[string]any{
						"files":  fileInfos,
						"action": "tracked",
					}, gateway.WithSessionID(sid))
				}
			}).
			OnAnswer(func(answer string) {
				d.sendEvent(clientID, sid, gateway.RespFinalAnswer, i18n.T("svc.event.final.answer"), answer)
			}).
			OnExecutionSummary(func(data events.ExecutionSummaryData) {
				d.logger.Info("[DAEMON] OnExecutionSummary FIRED: total_tokens=" + fmt.Sprint(data.TokensUsed.TotalTokens) +
					" input=" + fmt.Sprint(data.TokensUsed.InputTokens) +
					" output=" + fmt.Sprint(data.TokensUsed.OutputTokens) +
					" iterations=" + fmt.Sprint(data.TotalIterations))
				d.sendExecutionSummary(clientID, sid, data)
			}).
			OnCycleEnd(func(data events.CycleInfo) {
				gw.SendResponse(clientID, gateway.RespCycleEnd, i18n.T("svc.event.cycle.end"), map[string]any{
					"iteration": data.Iteration, "termination_reason": data.TerminationReason, "duration": data.Duration.String(),
				}, gateway.WithSessionID(sid))
			}).
			OnAgentTalkStart(func(data events.AgentTalkInfo) {
				gw.SendResponse(clientID, gateway.RespAgentTalkStart, i18n.T("svc.event.agent.talk.start"), map[string]any{
					"to": data.To, "message": data.Message,
				}, gateway.WithSessionID(sid))
			}).
			OnAgentTalkEnd(func(data events.AgentTalkResult) {
				gw.SendResponse(clientID, gateway.RespAgentTalkEnd, i18n.T("svc.event.agent.talk.end"), map[string]any{
					"to": data.To, "reply": data.Reply, "error": data.Error,
				}, gateway.WithSessionID(sid))
			}).
			OnCompaction(func(data events.CompactionData) {
				gw.SendResponse(clientID, gateway.RespCompaction, i18n.T("svc.event.compaction"), map[string]any{
					"session_id": data.SessionID, "messages_slid": data.MessagesSlid, "remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
				}, gateway.WithSessionID(sid))
			}).
			OnMaxTurnsReached(func(data events.MaxTurnsReachedData) {
				gw.SendResponse(clientID, gateway.RespMaxTurnsReached, i18n.T("svc.event.max.turns.reached"), map[string]any{
					"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
				}, gateway.WithSessionID(sid))
			}).
			OnError(func(errMsg string) {
				d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.error"), errMsg)
			}).
			OnSubtaskSpawned(func(data events.SubtaskInfo) {
				md := buildSubtaskSpawnedMarkdown(data)
				d.sendEvent(clientID, sid, gateway.RespSubtaskSpawned, i18n.T("svc.event.subtask.spawned"), md)
			}).
			OnSubtaskCompleted(func(data events.SubtaskResult) {
				md := buildSubtaskCompletedMarkdown(data)
				d.sendEvent(clientID, sid, gateway.RespSubtaskCompleted, i18n.T("svc.event.subtask.completed"), md)
			}).
			OnAskUser(func(data events.AskUserRequestData) {
				correlationID := uuid.New().String()
				d.interactMu.Lock()
				d.pendingInteractions[correlationID] = &pendingInteraction{
					replyFn:   data.Reply,
					createdAt: time.Now(),
				}
				d.interactMu.Unlock()
				gw.SendResponse(clientID, gateway.RespForm, i18n.T("svc.event.ask.user"), map[string]any{
					"correlation_id": correlationID,
					"questions":      data.Questions,
				}, gateway.WithSessionID(sid))
			}).
			OnPermissionRequest(func(data events.PermissionRequestData) {
				correlationID := uuid.New().String()
				d.interactMu.Lock()
				d.pendingInteractions[correlationID] = &pendingInteraction{
					grantFn:   data.Grant,
					denyFn:    data.Deny,
					createdAt: time.Now(),
				}
				d.interactMu.Unlock()
				gw.SendResponse(clientID, gateway.RespPermissionRequest, i18n.T("svc.event.permission.request"), map[string]any{
					"correlation_id": correlationID,
					"tool_name":      data.ToolName,
					"reason":         data.Reason,
					"security_level": data.SecurityLevel,
					"params":         data.Params,
				}, gateway.WithSessionID(sid))
			}).
			OnPermissionDenied(func(reason string) {
				d.sendEvent(clientID, sid, gateway.RespPermissionDenied, i18n.T("svc.event.permission.denied"), reason)
			}).
			OnTaskSummary(func(data events.TaskSummaryData) {
				md := buildTaskSummaryMarkdown(data)
				gw.SendResponse(clientID, gateway.RespTaskSummary, i18n.T("svc.event.task.summary"), md,
					gateway.WithSessionID(sid),
					gateway.WithResponseMeta(map[string]any{
						"input_tokens":  data.TokenUsage.InputTokens,
						"output_tokens": data.TokenUsage.OutputTokens,
					}))
			}).
			OnLLMTimeout(func(data events.LLMTimeoutData) {
				msg := fmt.Sprintf(i18n.T("svc.event.llm.timeout"), data.Elapsed, data.Error)
				d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.timeout"), msg)
			}).
			OnTokenUsageRecorded(func(record goharnesssession.TokenUsageRecord) {
				gw.SendResponse(clientID, gateway.RespTokenUsageRecorded, i18n.T("svc.event.token.usage"), map[string]any{
					"id":                record.ID,
					"session_id":        record.SessionID,
					"conversation_id":   record.ConversationID,
					"model_name":        record.ModelName,
					"provider_name":     record.ProviderName,
					"agent_name":        record.AgentName,
					"prompt_tokens":     record.PromptTokens,
					"completion_tokens": record.CompletionTokens,
					"cached_tokens":     record.CachedTokens,
					"reasoning_tokens":  record.ReasoningTokens,
					"total_tokens":      record.TotalTokens,
					"timestamp":         record.Timestamp,
				}, gateway.WithSessionID(sid))
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
	if sessionID == "" || sessionID == "new" {
		sessionID = generateSessionID()
		d.logger.Info("scheduled task: created new session",
			"session_id", sessionID,
		)
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

	if targetDir != "" {
		d.execMu.Lock()
		originalCWD, _ := os.Getwd()

		if err := os.Chdir(targetDir); err != nil {
			d.execMu.Unlock()
			d.logger.Warn("scheduled task: failed to chdir to project dir, using current dir",
				"project_dir", targetDir,
				"error", err,
			)
		} else {
			_ = os.Setenv("MINDX_PROJECT_DIR", targetDir)
			_ = os.Setenv("MINDX_SESSION_ID", sessionID)
			d.logger.Info("scheduled task: set execution context",
				"session_id", sessionID,
				"project_dir", targetDir,
				"original_cwd", originalCWD,
			)

			d.logger.Info("scheduled task: calling Runtime.Ask()",
				"session_id", sessionID,
				"agent", agent,
			)

			s := goharnesssession.NewSession(sessionID, agent,
				goharnesssession.WithStore(d.app.SessDB()),
			)
			_, err = rt.Ask(agent, content, s).Run()

			d.logger.Info("scheduled task: Runtime.Ask() returned",
				"session_id", sessionID,
				"error", err,
			)

			if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
				d.logger.Warn("scheduled task: failed to restore cwd after scheduled task",
					"original", originalCWD,
					"error", restoreErr,
				)
			}
			os.Unsetenv("MINDX_PROJECT_DIR")
			os.Unsetenv("MINDX_SESSION_ID")
			d.execMu.Unlock()

			if err != nil {
				return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
			}

			d.logger.Info("scheduled task: execution completed successfully",
				"session_id", sessionID,
				"agent", agent,
			)
			return nil
		}
	}

	d.logger.Info("scheduled task: executing without directory change",
		"session_id", sessionID,
		"agent", agent,
	)

	s := goharnesssession.NewSession(sessionID, agent,
		goharnesssession.WithStore(d.app.SessDB()),
	)
	_, err = rt.Ask(agent, content, s).Run()
	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
	}

	d.logger.Info("scheduled task: execution completed successfully (no dir change)",
		"session_id", sessionID,
		"agent", agent,
	)
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
