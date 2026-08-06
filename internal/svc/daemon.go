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
	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/goharness/hooks/action"
	goharnesssession "github.com/DotNetAge/goharness/session"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/appicon"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/mcp"
	"github.com/DotNetAge/mindx/internal/update"
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

type Daemon struct {
	app          *core.App
	gw           *gateway.Server
	scheduler    *scheduler.Scheduler
	schedulerDB  *scheduler.FileSchedulerStore
	sharedMemory *memory.RAGMemory

	// token usage recording
	usageStore    goharnesssession.TokenUsageStore
	webServer     *WebServer
	addr          string
	wsPath        string
	logger        logging.Logger
	clientCancels sync.Map

	// sessionQueues 按 sessionID 维护同一会话内串行执行的 FIFO 队列，
	// 取代「新消息取消旧执行」的粗粒度取消机制（详见 session_queue.go）。
	sessionQueues sync.Map

	// activeSessions tracks live sessions by sessionID for FileModifyHook
	// to look up the session's TrackModify function.
	activeSessions sync.Map

	// knowledge-graph database (gograph)
	graphDB    *graphapi.DB
	graphStore *graphapi.GraphStore

	// global key-value store (bbolt)
	kvStore *bbolt.DB

	// mcp manager for MCP server integration
	mcpMgr *mcp.Manager

	// dataDir is ~/.mindx, passed to Indexer constructors.
	dataDir string

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

func NewDaemon(app *core.App, addr, wsPath string, runtimeFS fs.FS, webFS fs.FS) *Daemon {
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
		MaxSize:    20,
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

	// ── 图数据库 (gograph) — 为 graph.* RPC 提供底层存储 ────────
	var graphDB *graphapi.DB
	var graphStore *graphapi.GraphStore
	var graphErr error

	graphDB, graphStore, graphErr = initGraphDB(app.Settings().DataDir())
	if graphErr != nil {
		logger.Warn("failed to initialize graph database", "error", graphErr)
	} else {
		logger.Info("graph database initialized",
			"path", filepath.Join(app.Settings().DataDir(), "kb.db"),
		)
	}

	// ── Shared Memory（对话记忆）─────────────────────────────────
	// Memory 仅为对话服务，基于 SemanticIndexer，不与知识库耦合
	var sharedMemory *memory.RAGMemory
	if emb := app.Embedder(); emb != nil {
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
		logger.Info("no embedder configured, memory and search disabled")
	}

	d := &Daemon{
		app:          app,
		addr:         addr,
		wsPath:       wsPath,
		schedulerDB:  schedulerDB,
		dataDir:      app.Settings().DataDir(),
		sharedMemory: sharedMemory,
		usageStore:   app.TokenUsageStore(),
		runtimeFS:    runtimeFS,
		webServer:    NewWebServer(webFS, logger),
		logger:       logger,
		restartCh:    make(chan struct{}, 1),
	}

	// Pass shared memory to App for MemorySearch tool registration.
	if sharedMemory != nil {
		app.SetLongTermMemory(sharedMemory)
	}

	// Wire graphDB to daemon fields (deferred because d is needed)
	if graphDB != nil {
		d.graphDB = graphDB
		d.graphStore = graphStore
	} else {
		logger.Warn("graph database unavailable, graph RPC disabled")
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
		d.scheduler = scheduler.NewScheduler(schedulerDB, logger)
		logger.Info("scheduler instance created")

		// Inject scheduler store into App for Cron tool registration.
		app.SetSchedulerStore(schedulerDB)

		// 生命周期回调：started 由 Scheduler 到点触发，Daemon 据此决定
		// 离线跳过（标记 missed）或广播 OnJobStart 交前端前台执行；
		// completed / failed 由对话结束后的 ReportResult 触发并广播。
		d.scheduler.OnLifecycle(func(info scheduler.JobLifecycleInfo) {
			if d.gw == nil {
				return
			}
			switch info.Status {
			case "started":
				if d.gw.ClientCount() == 0 {
					// 离线到点：无客户端可接收，跳过并标记 missed。
					if d.schedulerDB != nil {
						if err := d.schedulerDB.MarkMissed(info.EntryID); err != nil {
							d.logger.Warn("failed to mark job missed", "id", info.EntryID, "error", err)
						}
					}
					d.broadcastJobLifecycle("schedule.job_missed", info)
					return
				}
				if d.schedulerDB != nil {
					if err := d.schedulerDB.MarkStarted(info.EntryID, info.RunID); err != nil {
						d.logger.Warn("failed to mark job started", "id", info.EntryID, "error", err)
					}
				}
				d.broadcastJobLifecycle("schedule.job_started", info)
			default:
				d.broadcastJobLifecycle("schedule.job_"+info.Status, info)
			}
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

		// Initialize MCP Manager with the bbolt-based storage
		credStore := core.NewCredentialStore(app.Settings().UserPreferences())
		d.mcpMgr = mcp.NewManager(logger, mcp.NewStorage(kvDB), credStore)
		app.SetMCPManager(d.mcpMgr)
		logger.Info("mcp manager initialized")
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
		"has_shared_memory", d.sharedMemory != nil,
		"has_graph_db", d.graphDB != nil,
		"has_kvstore", d.kvStore != nil,
	)

	return d
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
		defer func() {
			if r := recover(); r != nil && d.logger != nil {
				d.logger.Error("hot-reload watcher: goroutine panic", fmt.Errorf("%v", r))
			}
		}()
		if err := d.hotReload.Start(ctx); err != nil && d.logger != nil {
			d.logger.Warn("hot-reload watcher exited with error", "error", err)
		}
	}()

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

	d.stopService("hot-reload watcher", func() {
		if d.hotReload != nil {
			d.hotReload.Stop()
		}
	})

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

	d.stopCloseable("mcp", func() error {
		if d.mcpMgr != nil {
			d.mcpMgr.Shutdown()
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
			if admin, ok := idx.(goragindexer.IndexerAdmin); ok {
				if cnt, err := admin.Count(context.Background()); err == nil {
					totalChunks = cnt
				}
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
	services["filewatch"] = map[string]any{"status": "disabled"}

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
			// 断连不再取消执行：对话循环在服务端继续运行，消息持续持久化，
			// 客户端重连后通过会话重载接上（断连恢复机制）。
			d.logger.Debug("client disconnected, execution continues",
				"client_id", clientID,
			)
			// 释放该客户端的取消集合（停止按钮只在连接期间有效）；
			// 在途执行不会因此被取消。
			d.clientCancels.Delete(clientID)
			termMgr.cleanupClient(clientID)
		}),
		gateway.WithConnectHandler(func(clientID string) {
			// 断连恢复补发：客户端重连后，把在途会话中挂起的授权请求重新
			// 发给该客户端，避免断连期间到达的权限请求丢失导致对话停滞。
			if d.gw == nil {
				return
			}
			d.activeSessions.Range(func(k, v any) bool {
				sess, ok := v.(*goharnesssession.Session)
				if !ok {
					return true
				}
				p := sess.PendingPermission()
				if p == nil {
					return true
				}
				sid := k.(string)
				// 主会话授权约定：data.session_id 为空（子会话授权才携带子会话 ID），
				// 前端据此将弹窗渲染到主会话卡片。
				_ = d.gw.SendResponse(clientID, gateway.RespPermissionRequest,
					i18n.T("svc.event.permission.request"), map[string]any{
						"tool_name":      p.ToolName,
						"reason":         p.Reason,
						"security_level": p.SecurityLevel,
						"params":         p.Arguments,
						"session_id":     "",
					}, gateway.WithSessionID(sid))
				return true
			})
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
		Text       string `json:"text"`
		SessionID  string `json:"session_id,omitempty"`
		JobEntryID string `json:"job_entry_id,omitempty"`
		JobRunID   string `json:"job_run_id,omitempty"`
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

	// 调度任务前台执行：客户端把 OnJobStart 携带的 entry_id/run_id 随消息透传，
	// 本轮对话结束后据此标记任务成功/失败（scheduler.ReportResult）。
	jobEntryID := payload.JobEntryID
	jobRunID := payload.JobRunID

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

	// 取消机制调整：不再「新消息取消旧执行」。
	// 同一会话的并发 Ask 通过 sessionQueue 串行排队执行（见 session_queue.go）：
	// 新消息不会打断正在进行的执行（例如 CollectResults 轮询），而是等上一轮
	// 完全结束后自动继续；只有断开连接 / message.cancel 停止按钮才批量取消执行。
	ctx, cancel := context.WithCancel(context.Background())
	cancelSet := d.clientCancelSetFor(msg.ClientID)
	cancelEntry := cancelSet.Add(cancel)

	clientID := msg.ClientID
	sid := sessionID
	gw := d.gw
	currentAgentName := resolvedAgentName

	// 子智能体授权冒泡旁路：子会话（spawn 的子 Agent 执行循环）触发授权请求时，
	// 不依赖父 exec EventBus 的存活（父 exec 结束/被取消后订阅销毁，原 parentEmit
	// 转发链路会静默丢事件），而是经此发送器将授权请求直接广播到所有客户端。
	// 授权请求的 envelope 结构（session_id / meta.agent_name / title / data / type）
	// 与 wireAskEvents 中 emitter.PermissionPending 发送的一致，前端解析无差异；
	// data.session_id 为发起授权的子会话 ID，前端据此精确反查并渲染到对应子任务卡片。
	// 主会话自身的授权请求不走旁路（b.permissionCh == nil），仍经原 EventBus 转发链路。
	ctx = agents.WithPermissionSink(ctx, func(data events.PermissionPendingData) {
		if d.gw == nil {
			return
		}
		d.gw.BroadcastNotification(string(gateway.RespPermissionRequest), map[string]any{
			"session_id": sid,
			"meta":       map[string]any{"agent_name": currentAgentName},
			"title":      i18n.T("svc.event.permission.request"),
			"data": map[string]any{
				"tool_name":      data.ToolName,
				"reason":         data.Reason,
				"security_level": data.SecurityLevel,
				"params":         data.Params,
				"session_id":     data.SessionID,
			},
			"type": gateway.RespPermissionRequest,
		})
	})

	// withAgent returns a ResponseOption that includes the current agent_name in meta.
	// Updated by OnEvent handler when sub-agent events are forwarded to the parent.
	withAgent := func() gateway.ResponseOption {
		return gateway.WithResponseMeta(map[string]any{"agent_name": currentAgentName})
	}

	d.logger.Debug("request: enqueuing async execution via AskBuilder",
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)

	// 同一会话的 Ask 排队串行执行。若上一轮执行尚未结束，返回 true 表示本次
	// 消息进入队列等待；此时通知前端显示「排队中」，避免用户误以为消息丢失。
	// queued 先声明再赋值：任务闭包按引用捕获它，任务真正执行时才能读到结果。
	var queued bool
	queued = d.sessionQueueFor(sessionID).Enqueue(func() {
		defer func() {
			cancelSet.Remove(cancelEntry)
			cancel()
		}()
		defer func() {
			d.activeSessions.Delete(sid)
			if r := recover(); r != nil {
				d.logger.Error("defaultHandler: AskBuilder panic", fmt.Errorf("%v", r),
					"client_id", clientID, "session_id", sid)
				d.sendEvent(clientID, sid, gateway.RespError, i18n.T("svc.event.error"), fmt.Sprintf(i18n.T("svc.event.execution.exception"), r))
			}
		}()

		// 排队等待期间客户端已断开连接或收到停止指令：跳过本次执行。
		if ctx.Err() != nil {
			d.logger.Info("request skipped: cancelled while queued",
				"client_id", clientID, "session_id", sid)
			// 调度任务收尾：跳过执行同样要标记失败，避免任务永久停留在 started
			// （前端待执行队列已出队，后端必须给出终态才能收到 completed/failed 事件）。
			if jobEntryID != "" && jobRunID != "" && d.scheduler != nil {
				d.scheduler.ReportResult(jobEntryID, jobRunID, ctx.Err())
			}
			return
		}

		// 排队中的消息正式开始执行：通知前端从「排队中」切换为「处理中」。
		if queued && gw != nil {
			_ = gw.SendResponse(clientID, gateway.RespMessageProcessing,
				i18n.T("svc.event.message.processing"), map[string]any{
					"timestamp": time.Now().Unix(),
				}, gateway.WithSessionID(sessionID), withAgent())
		}

		// Load existing session from persistent store (verifies it exists).
		//
		// 必须在队列内执行：Session 懒加载后 Append 只更新各自实例的内存，
		// 若在上一轮执行结束前加载会拿到过期快照，丢失对方后续写入的消息。
		//
		// Budget 动态可调：窗口大小通过 modelContextResolver 回调动态查询当前模型的
		// ContextLength，保证用户切换模型后立即生效，不再焊死在 session 上。
		//   - 模型 ContextLength <= 128K：TryCompact 会在 80% 阈值触发全量摘要并清空窗口
		//   - 模型 ContextLength > 128K：resolver 返回大值，ratio 不会达到阈值，
		//     靠 KV 缓存命中兜底——超长上下文模型上清空窗口反而损失有效上下文
		//   - MicroCompact (Dupdu) 已禁用：它修改上下文中间的 tool 消息内容，破坏 KV
		//     缓存，重算成本远大于保留"垃圾"的 attention 成本，属于负优化。
		var sessOpts []goharnesssession.SessionConfig
		// 通用能力（Compactor 压缩引擎 + Sandbox 沙箱）由 Runtime 统一注入，
		// 与子 Agent 会话走同一条装配路径（agents.Runtime.SessionConfigs）。
		sessOpts = append(sessOpts, rt.SessionConfigs()...)
		// 会话特有配置：modelContextResolver 动态查询当前模型窗口大小
		sessOpts = append(sessOpts,
			goharnesssession.WithModelContextResolver(d.resolveSessionModelContextLength),
		)
		// 绑定 RAG 记忆存储，使压缩摘要持久化到 RAG indexer（浏览器可读）
		if d.sharedMemory != nil {
			// 获取会话的 project_dir 用于存储元数据
			projectDir := ""
			if meta, metaErr := d.app.SessDB().GetMeta(context.Background(), sessionID); metaErr == nil && meta != nil {
				projectDir = meta.ProjectDir
			}
			sessOpts = append(sessOpts, goharnesssession.WithMemory(mindxses.NewRAGMemoryAdapter(d.sharedMemory, resolvedAgentName, projectDir)))
		}
		s, err := goharnesssession.Load(context.Background(), sessionID, resolvedAgentName, d.app.SessDB(), d.logger, sessOpts...)
		if err != nil {
			d.logger.Error("failed to load session", err, "session_id", sessionID)
			d.sendEvent(clientID, sessionID, gateway.RespError, "Session Error", err.Error())
			return
		}
		d.activeSessions.Store(sessionID, s)

		// 注册 FileModifyHandler：将文件追踪事件转发为 JSON-RPC 通知，
		// 让前端实时显示 DiffView。
		//
		// 防止重复触发的三重保险：
		//  1. session.TrackModify 内部已用 containsModifyFile 去重，
		//     同一文件被再次追踪时直接 return nil，不调 handler。
		//  2. 这里只对 Action == "tracked" 发送事件；"confirmed" / "rolled_back"
		//     是用户后续操作结果，不应在前端再次弹出 DiffView。
		//  3. 前端 chatStore.handleFileModified 对同路径消息做 upsert，
		//     即使收到重复事件也只会更新而非追加。
		s.SetFileModifyHandler(func(ev goharnesssession.FileModifyEvent) {
			if ev.Action != "tracked" {
				return
			}
			fp := ev.FilePath
			if fp == "" {
				return
			}
			payload, _ := json.Marshal(map[string]any{
				"files":  []string{fp},
				"action": ev.Action,
			})
			d.sendEvent(clientID, sessionID, gateway.RespFileModified,
				i18n.T("svc.event.file_modified"), string(payload))
		})

		// ── Build common event handlers via factory ──
		// eventSubSessionID 记录「当前事件」所属会话 ID：子会话经 parentEmit
		// 转发到父 EventBus 的事件携带子会话 ID，主会话事件为 sid。
		// 由于 EventBus 同步分发，OnEvent 先于类型 handler 执行，handler 读取
		// 该字段即可拿到本事件的会话归属，从而把子会话流式事件精确路由到
		// 前端对应的子任务卡片（见 effectiveEventSession）。
		var eventSubSessionID string
		emitter := newClientAskHandlers(d, gw, clientID, sid, withAgent, s, func() string { return currentAgentName }, func() string { return eventSubSessionID })

		builder := rt.Ask(resolvedAgentName, content, s).
			WithContext(ctx).
			OnEvent(func(ev events.ReactEvent) {
				currentAgentName = ev.AgentName
				eventSubSessionID = ev.SessionID
			}).
			OnEvent(func(ev events.ReactEvent) {
				// Forward Compact / MicroCompact events as JSON-RPC
				// broadcast notifications so all connected clients get real-time
				// context window management visibility.
				// 会话归属用 ev.SessionID：主会话事件经 emit 补齐为 sid，
				// 子会话事件经 parentEmit 转发时即子会话 ID，据此区分广播对象。
				switch ev.Type {
				case events.CompactStart:
					data, _ := ev.Data.(events.CompactStartData)
					sessID := ev.SessionID
					if sessID == "" {
						sessID = sid
					}
					d.logger.Info("[session] compact start",
						"session_id", sessID,
						"window_tokens", data.WindowTokens,
					)
					if gw != nil {
						gw.BroadcastNotification("compact_start", map[string]any{
							"session_id": sessID,
							"data":       data,
						})
					}
				case events.CompactDone:
					data, _ := ev.Data.(events.CompactDoneData)
					sessID := ev.SessionID
					if sessID == "" {
						sessID = sid
					}
					d.logger.Info("[session] compact done",
						"session_id", sessID,
						"messages_slid", data.MessagesSlid,
						"window_tokens", data.WindowTokens,
						"ratio", data.Ratio,
					)
					if gw != nil {
						gw.BroadcastNotification("compact_done", map[string]any{
							"session_id": sessID,
							"data":       data,
						})
					}
				case events.MicroCompactStart:
					data, _ := ev.Data.(events.MicroCompactStartData)
					sessID := ev.SessionID
					if sessID == "" {
						sessID = sid
					}
					d.logger.Info("[session] micro-compact start",
						"session_id", sessID,
						"window_tokens", data.WindowTokens,
					)
					if gw != nil {
						gw.BroadcastNotification("micro_compact_start", map[string]any{
							"session_id": sessID,
							"data":       data,
						})
					}
				case events.MicroCompactDone:
					data, _ := ev.Data.(events.MicroCompactDoneData)
					sessID := ev.SessionID
					if sessID == "" {
						sessID = sid
					}
					d.logger.Info("[session] micro-compact done",
						"session_id", sessID,
						"compressed", data.Compressed,
						"deduped", data.Deduped,
						"window_tokens", data.WindowTokens,
						"ratio", data.Ratio,
					)
					if gw != nil {
						gw.BroadcastNotification("micro_compact_done", map[string]any{
							"session_id": sessID,
							"data":       data,
						})
					}
				}
			})
		builder = wireAskEvents(builder, emitter)

		_, err = builder.
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
		}

		// 调度任务收尾：本消息带有任务标识（OnJobStart 前台执行链路），
		// 对话结束后把结果写回调度存储并广播 completed / failed。
		// 每个入队闭包各自捕获自己的 job 标识，与会话队列的 FIFO 顺序天然一致。
		if jobEntryID != "" && jobRunID != "" && d.scheduler != nil {
			d.scheduler.ReportResult(jobEntryID, jobRunID, err)
		}

		// Broadcast current context window usage after each LLM request.
		// This allows the UI to update the context usage indicator in real time.
		// ContextUsage 通过 modelContextResolver 回调动态获取当前模型的 ContextLength，
		// 无需 fallback。
		if d.gw != nil {
			usage := s.ContextUsage(d.buildSessionPricing())
			d.gw.BroadcastNotification("context_usage", map[string]any{
				"session_id": sid,
				"data": map[string]any{
					"window_tokens":        usage.WindowTokens,
					"max_window_size":      usage.MaxWindowSize,
					"usage_ratio":          usage.UsageRatio,
					"message_count":        usage.MessageCount,
					"cursor":               usage.Cursor,
					"active_message_count": usage.ActiveMessageCount,
					"total_actual_tokens":  usage.TotalActualTokens,
					"total_cost":           usage.TotalCost,
				},
			})
		}

		d.logger.Info("request done",
			"client_id", clientID,
			"session_id", sid,
			"agent", resolvedAgentName,
		)
	})

	// 通知前端：消息已进入该会话的串行队列，正在等待上一轮执行完成。
	if queued && gw != nil {
		d.logger.Info("request queued behind running execution",
			"client_id", clientID, "session_id", sessionID)
		_ = gw.SendResponse(clientID, gateway.RespMessageQueued,
			i18n.T("svc.event.message.queued"), map[string]any{
				"timestamp": time.Now().Unix(),
			}, gateway.WithSessionID(sessionID), withAgent())
	}
}

// cancelClientExecution 批量取消指定客户端的全部执行（运行中与排队中）。
// 仅由客户端断开连接时触发；新消息不再取消旧执行，而是进入会话队列排队。
func (d *Daemon) cancelClientExecution(clientID string) {
	if v, ok := d.clientCancels.Load(clientID); ok {
		v.(*clientCancelSet).CancelAll()
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
			if sid := d.resolveLatestSessionForAgent(currentAgent); sid != "" {
				d.logger.Info("resumed session from store", "agent", currentAgent, "session", sid)
				return sid
			}
		}
	}

	sid := generateSessionID()
	d.logger.Info("created new session", "session", sid)
	return sid
}

// resolveLatestSessionForAgent 返回指定智能体最近活动的会话 ID。
// 供 resolveSessionID（无会话上下文时回退）与调度任务（entry.SessionID 为空时回退）
// 共用。找不到任何会话时返回空字符串。
func (d *Daemon) resolveLatestSessionForAgent(agent string) string {
	if agent == "" || d.app == nil || d.app.SessDB() == nil {
		return ""
	}
	sessions, err := goharnesssession.ListSessions(context.Background(), d.app.SessDB())
	if err != nil {
		return ""
	}
	var best *goharnesssession.SessionInfo
	for i := range sessions {
		if sessions[i].AgentName == agent {
			if best == nil || sessions[i].LastActivityAt.After(best.LastActivityAt) {
				best = &sessions[i]
			}
		}
	}
	if best != nil && best.SessionID != "" {
		return best.SessionID
	}
	return ""
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
