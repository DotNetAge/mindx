package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/goharness/hooks/action"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/channel"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/devgate"
	"github.com/DotNetAge/mindx/internal/discovery"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/mcp"
	"github.com/DotNetAge/mindx/internal/update"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"go.etcd.io/bbolt"
)

var (
	atAgentRegex = regexp.MustCompile(`^@([\w-]+)(?:\s+(.+))?$`)
	ulidRegex    = regexp.MustCompile(`^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$`)
)

type Daemon struct {
	app         *core.App
	gw          *gateway.Server
	scheduler   *scheduler.Scheduler
	schedulerDB *scheduler.FileSchedulerStore

	// token usage recording
	usageStore    goharnesssession.TokenUsageStore
	addr          string
	wsPath        string
	logger        logging.Logger
	clientCancels sync.Map

	// sessionQueues 按 sessionID 维护同一会话内串行执行的 FIFO 队列，
	// 取代「新消息取消旧执行」的粗粒度取消机制（详见 session_queue.go）。
	sessionQueues sync.Map

	// sessionBlocks 按 sessionID 登记挂起等待用户输入的会话（值 = "ask" |
	// "permission"）。蜂群侧栏气球数据源：主会话挂起时本轮执行已结束（不在
	// sessionQueues 的 running 集合），运行态轮询单独感知不到；登记点为 ask /
	// permission 事件 emit 处与子代理旁路 sink，解除点见 clearSessionBlock。
	// 进程内存态，重启即清——挂起登记本身已持久化在会话目录，重启后气球缺失
	// 属已知限制（打开会话仍可见挂起现场）。
	sessionBlocks sync.Map

	// subBlockParents 记录子会话挂起归属的父会话（子会话 ID → 父会话 ID）：
	// 子代理提问的回答经 dispatchAskAnswer 按子会话 ID 送达，需据此反查清除
	// 父会话的挂起登记（气球呈现在父会话行，子会话不进侧栏列表）。
	subBlockParents sync.Map

	// activeSessions tracks live sessions by sessionID for FileModifyHook
	// to look up the session's TrackModify function.
	activeSessions sync.Map

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
	// and automatically reload registries.
	hotReload *HotReloadWatcher

	// mdns 是局域网 mDNS 服务广播（_mindx._tcp），供 iOS App 零配置发现 daemon。
	// 在 gateway 启动成功后注册，daemon 退出时注销。
	mdns *discovery.Broadcaster

	// chMgr 是 AgentHub 中继管理器（手机连接）：连接中继、配对同意闸、
	// 网关桥接。在 gateway 启动成功后开启，停机时关闭。
	chMgr *channel.Manager

	// devGate 是设备接入闸（Mac 间互连）：允许接入开关、接入请求同意闸、
	// 已授权设备表与已验证连接登记。RPC 注册层强制闸与其共用。
	devGate *devgate.Manager

	// oauthRegistry OAuth 客户端提供商注册表（client-provider.yml，启动期加载）
	oauthRegistry *core.OAuthProviderRegistry

	// oauthCredStore OAuth 凭据库（client_id/client_secret 覆盖值与令牌落点）
	oauthCredStore core.CredentialStore

	// oauthFlows 进行中的授权流程登记（每 provider 至多一条）
	oauthFlows *oauthFlowManager

	// marketDevice 市场设备授权登录编排状态（device flow，单飞）
	marketDevice marketDeviceLoginState

	// marketLoggedOut 用户已主动退出标记（内存态，重启即失效）：退出后本次
	// 运行期静默登录通道整体静默，重新登录成功即解除（2026-10-20 退出语义定案）
	marketLoggedOut bool

	// marketIdentity 社区账号绑定编排状态（单飞；2026-10-20 多社区身份定案）
	marketIdentity marketIdentityLinkState
}

func NewDaemon(app *core.App, addr, wsPath string, runtimeFS fs.FS) *Daemon {
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

	// ── Shared Memory（对话记忆）─────────────────────────────────
	// 2026-10-06 记忆剥离重构：RAGMemory 退役，压缩摘要改经 FileMemorySink
	// 落盘为 .agents/memory/ markdown 文件（App 内置），daemon 不再持有共享记忆。

	d := &Daemon{
		app:         app,
		addr:        addr,
		wsPath:      wsPath,
		schedulerDB: schedulerDB,
		dataDir:     app.Settings().DataDir(),
		usageStore:  app.TokenUsageStore(),
		runtimeFS:   runtimeFS,
		logger:      logger,
		restartCh:   make(chan struct{}, 1),
	}

	// UI 命令通道：把 daemon 广播回调注入 App，createRuntime 据此注册
	// Open/Visit/TerminalRun 三工具（未走 daemon 的 TUI 场景不注册）。
	app.SetUIBroadcast(d.broadcastUI)

	if schedulerDB != nil {
		d.scheduler = scheduler.NewScheduler(schedulerDB, logger)
		logger.Info("scheduler instance created")

		// Inject scheduler store into App for Cron tool registration.
		app.SetSchedulerStore(schedulerDB)

		// 任务库增删改回调：watchLoop 感知注册集变化后广播前端刷新（派工/删改
		// 即时可见，不必等任务到点触发才有动静）。
		d.scheduler.OnEntriesChanged(func() {
			d.broadcastScheduleChanged()
		})

		// 生命周期回调：started 由 Scheduler 到点触发，Daemon 直接拉起 Runtime
		// 在后台执行（2026-10-21 定案回归，不依赖客户端在线，离线照常执行）；
		// completed / failed / missed 由 ReportResult 触发并广播，前端仅作通知。
		d.scheduler.OnLifecycle(func(info scheduler.JobLifecycleInfo) {
			if d.gw == nil {
				return
			}
			switch info.Status {
			case "started":
				if d.schedulerDB != nil {
					if err := d.schedulerDB.MarkStarted(info.EntryID, info.RunID); err != nil {
						d.logger.Warn("failed to mark job started", "id", info.EntryID, "error", err)
					}
				}
				d.broadcastJobLifecycle("schedule.job_started", info)
				// 后台执行：goroutine 中拉起 Runtime Ask，不阻塞 cron goroutine；
				// panic 与终态回写均由 runScheduledJob 收敛。
				go d.runScheduledJob(info)
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
			"path", filepath.Join(app.Settings().DataDir(), "kv.db"),
		)
		// 决策计量混杂登记（白皮书 2.1）：daemon 版本变化自动登记——
		// compare 归因解读时区分"路由修订效果"与"daemon 更新混杂"。
		d.checkMetricVersionSnapshot(core.Version)
		// Initialize MCP Manager
		// Server 配置由 ~/.mindx/settings/mcp.json 主存（文件），
		// 工具清单运行时从 MCP server tools/list 动态拉取。零 bbolt 依赖。
		credStore := core.NewCredentialStore(app.Settings().UserPreferences())
		d.mcpMgr = mcp.NewManager(logger, credStore, app.Settings().McpFile(), mcp.DefaultMarketMCPURL)
		app.SetMCPManager(d.mcpMgr)
		// 热更新：MCP 配置变更（enabled / add / remove / sync）后清空 Runtime 缓存，
		// 下一轮对话 createRuntime 会重新 EnabledTools() 装配新的工具列表。
		d.mcpMgr.SetOnConfigChanged(func() { app.InvalidateRuntimes() })
		logger.Info("mcp manager initialized（Runtime 热更新已启用）")

		// 后台异步同步远程 market mcp.json（懒操作，不阻塞启动）
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result := d.mcpMgr.Sync(ctx)
			if len(result.Added) > 0 {
				logger.Info("mcp: 远程同步新增 server", "count", len(result.Added), "servers", strings.Join(result.Added, ","))
			} else if result.Error != "" {
				logger.Debug("mcp: 远程同步跳过", "reason", result.Error)
			}
		}()
	}

	// ── OAuth 提供商注册表：client-provider.yml 加载失败降级为空注册表 ──
	// （RPC 层报「暂无可用提供商」，不阻断 daemon 启动——文件可随版本修复）
	if oauthProviders, oauthErr := core.LoadClientProviders(app.Settings().ClientProvidersFile()); oauthErr != nil {
		logger.Warn("failed to load client providers, oauth providers disabled", "error", oauthErr)
		d.oauthRegistry = core.NewOAuthProviderRegistry(nil)
	} else {
		d.oauthRegistry = core.NewOAuthProviderRegistry(oauthProviders)
	}
	d.oauthCredStore = core.NewCredentialStore(app.Settings().UserPreferences())
	d.oauthFlows = newOAuthFlowManager()

	// ── 设备接入闸（Mac 间互连）：凭据存储与本机令牌就绪即可装配 ──
	// 通知回调内判 gw 判空（Notify 时点可能早于 gateway 启动）。
	d.initDeviceGate()

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
		"has_kvstore", d.kvStore != nil,
	)

	return d
}

// sessionProjectDir 查询会话的项目目录（用于记忆文件落盘定位）。
// 查询失败返回空串（回退到 ~/.mindx/.agents/memory/）。
func (d *Daemon) sessionProjectDir(sessionID string) string {
	meta, err := d.app.SessDB().GetMeta(context.Background(), sessionID)
	if err != nil || meta == nil {
		return ""
	}
	return meta.ProjectDir
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

	// ── MindX 账号静默登录（账号机制定案 2026-10-10） ─────────────
	// 重装恢复：密钥存储存有 username+password 时静默重登换 token 并绑定设备；
	// 失败只记日志不打扰（匿名态可用，下次启动再试）。全程无 UI。
	go d.silentMarketRelogin()

	// ── Hot-reload: watch agents/skills directories for file changes ──
	// 注册表重装完成后经 gateway 广播 agents_changed/skills_changed 通知，
	// 前端收到后自动重拉列表（目录即注册表：手动放置的 Agent/技能即时可见）。
	d.hotReload = NewHotReloadWatcher(d.app, d.logger, func(method string, params any) {
		if d.gw != nil {
			d.gw.BroadcastNotification(method, params)
		}
	})
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

	// ── 手机连接：中继管理器随网关就绪后开启（依赖网关自拨建桥） ──
	d.initChannelManager()
	if d.chMgr != nil {
		d.chMgr.Start(ctx)
		d.logger.Info("channel manager started")
	}

	// ── mDNS 局域网广播：服务就绪后开启，iOS App 同一 WiFi 下零配置发现 daemon ──
	d.startMDNS()

	// ── 信件派发：地址上报 + 收件箱轮询（跨机器派工与汇报的直达/兜底通路） ──
	d.startMailLoop(ctx)

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

// startMDNS 注册局域网 mDNS 服务广播（_mindx._tcp）。
// 广播失败仅告警不阻断 daemon 启动；监听端口为 0（测试场景）时跳过。
func (d *Daemon) startMDNS() {
	_, portStr, err := net.SplitHostPort(d.addr)
	if err != nil {
		d.logger.Warn("mDNS 广播跳过：无法解析监听地址", "addr", d.addr)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		d.logger.Warn("mDNS 广播跳过：监听端口无效", "addr", d.addr, "port", portStr)
		return
	}
	b, err := discovery.Start(port, "")
	if err != nil {
		d.logger.Warn("mDNS 广播启动失败，局域网发现不可用", "error", err)
		return
	}
	d.mdns = b
	d.logger.Info("mDNS 广播已开启", "service", discovery.ServiceType, "port", port)
}

// stopBackgroundServices 停止所有后台辅助服务。
func (d *Daemon) stopBackgroundServices() {
	d.logger.Info("stopping background services...")

	// 停机安全网：先于 gateway 关闭级联强停全部运行中的子代理，并留出
	// 短暂宽限让 spawn 的 defer 写入 [sub-agent-terminated] 终止标记。
	// 否则子代理被进程死亡杀死且无终止标记，重启后 CollectResults 对其
	// 轮询将死等到上限（30 分钟）才判定失败。
	if n := d.cancelAllSubAgents(); n > 0 {
		d.logger.Info("停机级联强停运行中子代理", "count", n)
		time.Sleep(2 * time.Second)
	}

	d.stopService("hot-reload watcher", func() {
		if d.hotReload != nil {
			d.hotReload.Stop()
		}
	})

	// 关闭中继管理器：断开中继连接与网关桥
	d.stopService("channel 中继管理器", func() {
		if d.chMgr != nil {
			d.chMgr.Stop()
		}
	})

	// 注销 mDNS 广播，避免退出后 iOS 端仍发现已下线的 daemon。
	d.stopService("mDNS 广播", func() {
		if d.mdns != nil {
			d.mdns.Shutdown()
			d.mdns = nil
		}
	})

	d.stopService("scheduler service", func() {
		if d.scheduler != nil {
			d.scheduler.Stop()
		}
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

func (d *Daemon) initGateway() {
	d.logger.Info("initializing WebSocket gateway",
		"addr", d.addr,
		"wsPath", d.wsPath,
	)
	d.gw = gateway.New(
		gateway.WithAddr(d.addr),
		gateway.WithPath(d.wsPath),
		gateway.WithHandler(d.defaultHandler),
		// Origin 白名单：gort 默认列表只放行 http/localhost/内网段/Wails 壳，
		// Electron 打包壳从 file:// 加载页面，浏览器 WebSocket Origin 恒为
		// file://，会被默认白名单 403 拒绝（mindx-work 打包版连接全灭实证）。
		// 此处整体覆盖为 gort v0.1.9 默认列表 + file://——列表手抄自
		// gort defaultWSConfig，升级 gort 版本时必须对齐漂移。
		gateway.WithWSConfig(&gateway.WSConfig{
			AllowedOrigins: []string{
				"http://localhost:*",
				"http://127.0.0.1:*",
				"ws://localhost:*",
				"ws://127.0.0.1:*",
				// Wails 壳（mindx-app）的 WebView 页面 origin：dev 带端口、生产无端口
				"wails://wails.localhost:*",
				"wails://wails.localhost",
				// Electron 打包壳（mindx-work）：file:// 页面发起的 WebSocket
				"file://",
				// LAN subnets
				"http://192.168.*:*", "ws://192.168.*:*",
				"http://10.*:*", "ws://10.*:*",
				"http://172.16.*:*", "ws://172.16.*:*",
				"http://172.17.*:*", "ws://172.17.*:*",
				"http://172.18.*:*", "ws://172.18.*:*",
				"http://172.19.*:*", "ws://172.19.*:*",
				"http://172.20.*:*", "ws://172.20.*:*",
				"http://172.21.*:*", "ws://172.21.*:*",
				"http://172.22.*:*", "ws://172.22.*:*",
				"http://172.23.*:*", "ws://172.23.*:*",
				"http://172.24.*:*", "ws://172.24.*:*",
				"http://172.25.*:*", "ws://172.25.*:*",
				"http://172.26.*:*", "ws://172.26.*:*",
				"http://172.27.*:*", "ws://172.27.*:*",
				"http://172.28.*:*", "ws://172.28.*:*",
				"http://172.29.*:*", "ws://172.29.*:*",
				"http://172.30.*:*", "ws://172.30.*:*",
				"http://172.31.*:*", "ws://172.31.*:*",
			},
			AllowAllInDev: true,
		}),
		// 握手层网络准入：私网放行；公网必须凭本机令牌、已授权设备令牌
		// 或有效配对码（networkAuthenticator 三分支，见 handler_network.go）
		gateway.WithAuthenticator(d.networkAuthenticator()),
		// 首条消息认证（升级闸的通道内等价物）：公网连接未凭升级请求过闸时，
		// 凭连接后首条消息 device.auth 过闸（浏览器端无法自定义请求头）；
		// 认证通过前不进入常规消息分发，超时断连防连接洪泛
		gateway.WithMessageAuthenticator(d.firstMessageAuthenticator(), 10*time.Second),
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
			// 设备接入验证登记随连接失效（重连需重新握手）。
			if d.devGate != nil {
				d.devGate.Unbind(clientID)
			}
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
	// 设备接入强制闸（user.message 通道）：未完成接入验证的客户端一律拒绝。
	// 与 RPC 注册层闸同源（deviceGateAuthorized），白名单方法仅 device.handshake。
	if !d.deviceGateAuthorized(msg.ClientID) {
		d.logger.Warn("拒绝未接入设备的消息", "client_id", msg.ClientID)
		if d.gw != nil {
			_ = d.gw.SendResponse(msg.ClientID, gateway.RespError,
				"对方还没有就绪", map[string]any{
					"message": "对方设备还没有连接就绪，暂时联系不上。请对方打开 MindX 后再试。",
				})
		}
		return
	}

	d.logger.Debug("defaultHandler: received message",
		"client_id", msg.ClientID,
		"session_id", msg.SessionID,
		"data_size", len(msg.Data),
	)

	var payload struct {
		Text      string `json:"text"`
		SessionID string `json:"session_id,omitempty"`
		Images    []struct {
			Path      string `json:"path"`
			MediaType string `json:"media_type"`
		} `json:"images,omitempty"`
	}
	// 附加图片：前端把粘贴/上传的图片落盘到会话临时目录后，
	// 以 {path, media_type} 引用随 user.message 一起送达。
	// 此处仅做路径引用收集（读文件转 base64 延迟到组装 LLM 消息时）。
	userImages := make([]goharnesssession.ImageBlock, 0, len(payload.Images))
	for _, img := range payload.Images {
		if img.Path == "" {
			continue
		}
		userImages = append(userImages, goharnesssession.ImageBlock{
			Path:      img.Path,
			MediaType: img.MediaType,
		})
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || (payload.Text == "" && len(userImages) == 0) {
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

	// 子代理提问回答分派：目标会话存在挂起等待回答的子智能体提问时，
	// 本条消息即用户对提问的回答，注入挂起的子 exec（askCh）恢复其执行
	// 循环后直接返回，不再进入正常 Ask 流程。未命中（无挂起提问）按普通
	// 消息处理，保留旧的子会话 Tab 直接作答路径。
	askTarget := msg.SessionID
	if providedSessionID != "" {
		askTarget = providedSessionID
	}
	if content != "" && d.dispatchAskAnswer(askTarget, content) {
		return
	}

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
	// 登记取消函数所属会话：停止按钮（message.cancel 带 session_id）
	// 按会话精确取消，不波及同一客户端其它 Tab 的执行。
	cancelEntry := cancelSet.Add(sessionID, cancel)

	clientID := msg.ClientID
	sid := sessionID
	gw := d.gw
	currentAgentName := resolvedAgentName

	// 子智能体授权/提问冒泡旁路：注入到 exec ctx（前台与后台执行共用实现，
	// 详见 scheduled_exec.go 的 withAskBypassSinks）。
	ctx = d.withAskBypassSinks(ctx, sessionID, func() string { return currentAgentName })

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
			return
		}

		// 排队中的消息正式开始执行：通知前端从「排队中」切换为「处理中」。
		if queued && gw != nil {
			_ = gw.SendResponse(clientID, gateway.RespMessageProcessing,
				i18n.T("svc.event.message.processing"), map[string]any{
					"timestamp": time.Now().Unix(),
				}, gateway.WithSessionID(sessionID), withAgent())
		}

		// 新任务开工即解除挂起登记：用户回应（回答提问 / 授权魔法词 / 普通消息）
		// 是挂起状态的消费点，本轮 Ask 开始时挂起登记随之消化（loop 内
		// TakePendingAskUser / resolvePermissionMagicWord）。若本轮再次挂起，
		// emit 点会在其之后重新登记，时序安全。
		d.clearSessionBlock(sid)

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
		// 文件记忆：压缩摘要经 SummarySink 落盘到会话项目目录 .agents/memory/
		//（2026-10-06 记忆剥离重构，替代 RAGMemoryAdapter）
		memoryDir := d.app.FileMemory().MemoryDirFor(d.sessionProjectDir(sessionID))
		sessOpts = append(sessOpts, goharnesssession.WithSummarySink(func(ctx context.Context, summary string) error {
			return d.app.FileMemory().Write(ctx, memoryDir, resolvedAgentName, sessionID, summary)
		}))
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
		// 防止重复触发的保险：
		//  1. session.TrackModify 对已追踪文件不重复备份，但再次被工具触碰时
		//     仍发事件（文件内容相对旧快照可能已变化，前端需知道）。
		//  2. 这里只对 Action == "tracked" 发送事件；"confirmed" / "rolled_back"
		//     是用户后续操作结果，不应在前端再次弹出 DiffView。
		//  3. 前端 chatStore.handleFileModified 对同路径消息做幂等 upsert
		//     （已有条目保留原 diff，不降级为空），重复事件无副作用。
		//  注意：事件在工具执行「前」发出（备份时机），事件内的文件内容尚未更新，
		//  前端展示的最新 diff 由 openNativeDiff 失败时拉取 session.get 兜底保证。
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

		// 轮次日志（日报事实流）：记录本轮起点快照与开始时间，Run() 返回后
		// 把当轮「用户问题 + 最终回复」原文落盘到项目目录 .agents/logs/
		//（详见 turnlog.go；旁路采集，失败不影响对话流程）。
		turnLogLenBefore := len(s.All())
		turnLogStart := time.Now()

		builder := rt.Ask(resolvedAgentName, content, s).
			WithImages(userImages).
			WithContext(ctx).
			OnEvent(func(ev events.ReactEvent) {
				currentAgentName = ev.AgentName
				eventSubSessionID = ev.SessionID
			})
		// 决策计量埋点：在 UI emitter 之外独立注册（OnXxx 追加语义），
		// 每次任务执行结束落一条 round 事件（白皮书第 4 章采集层）。
		builder = d.withMetricHook(builder, sid, resolvedAgentName, func() string { return eventSubSessionID }, content).
			OnEvent(func(ev events.ReactEvent) {
				// Forward Compact events as JSON-RPC broadcast notifications
				// so all connected clients get real-time context window
				// management visibility.
				// 会话归属用 ev.SessionID：主会话事件经 emit 补齐为 sid，
				// 子会话事件经 parentEmit 转发时即子会话 ID，据此区分广播对象。
				// 注：MicroCompact 已拆除（负优化，破坏 KV 缓存），仅保留全量压缩事件。
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

		// 轮次日志落盘：本轮结束后把用户问题与最终回复原文写入 .agents/logs/
		//（mindx report collect 跨目录收割，daily-report 技能消费生成日报）。
		d.appendTurnLog(s, resolvedAgentName, content, turnLogLenBefore, turnLogStart)

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

// cancelClientExecution 批量取消指定客户端的全部执行（运行中与排队中），
// 并对每个被执行会话级联强停其派生的运行中子代理。
// 仅由客户端断开连接时触发；新消息不再取消旧执行，而是进入会话队列排队。
func (d *Daemon) cancelClientExecution(clientID string) {
	if v, ok := d.clientCancels.Load(clientID); ok {
		sessionIDs := v.(*clientCancelSet).CancelAll()
		d.clientCancels.Delete(clientID)
		for _, sid := range sessionIDs {
			// 执行树被强停：挂起等待随执行消亡，解除挂起登记（气球熄灭）
			d.clearSessionBlock(sid)
			if n := d.cancelSubAgents(sid); n > 0 {
				d.logger.Info("断连级联强停子代理", "session_id", sid, "count", n)
			}
		}
	}
}

// cancelSubAgents 强停指定主会话派生的全部运行中子代理。
// Runtime 按 agent 名缓存（runtimeCache），停止请求仅携带 session_id、
// 无法定位执行该会话的具体 Runtime 实例，故遍历全部缓存 Runtime——
// 仅实际持有该 sponsor 强停登记的 Runtime 会命中（见 App.ForEachRuntime）。
// 返回被强停的子代理数。
func (d *Daemon) cancelSubAgents(sessionID string) int {
	if sessionID == "" || d.app == nil {
		return 0
	}
	total := 0
	d.app.ForEachRuntime(func(rt *agents.Runtime) {
		total += rt.CancelSubAgents(sessionID)
	})
	return total
}

// cancelAllSubAgents 强停本进程全部 Runtime 派生的运行中子代理（不限 sponsor）。
// 供停机安全网使用：停机没有「活跃 sponsor 清单」可遍历，直接全量强停更完备，
// 无需宿主追踪哪些主会话派生过子代理。返回被强停的子代理总数。
func (d *Daemon) cancelAllSubAgents() int {
	if d.app == nil {
		return 0
	}
	total := 0
	d.app.ForEachRuntime(func(rt *agents.Runtime) {
		total += rt.CancelAllSubAgents()
	})
	return total
}

// dispatchAskAnswer 将用户对子代理提问的回答路由到挂起等待的子会话。
// Runtime 按 agent 名缓存（runtimeCache），回答仅携带目标子会话 ID、
// 无法定位持有该会话的 Runtime 实例，故遍历全部缓存 Runtime 逐一尝试——
// 仅实际持有挂起提问登记的 Runtime 会命中（见 App.ForEachRuntime）。
// 返回是否命中并已送达；false 时调用方按普通消息走正常 Ask 流程。
func (d *Daemon) dispatchAskAnswer(target, answer string) bool {
	if target == "" || answer == "" || d.app == nil {
		return false
	}
	dispatched := false
	d.app.ForEachRuntime(func(rt *agents.Runtime) {
		if dispatched {
			return
		}
		dispatched = rt.DispatchAskAnswer(target, answer)
	})
	if dispatched {
		// 子会话提问已回答：反查归属父会话并解除其挂起登记（气球熄灭）
		d.resolveSubBlockAnswer(target)
	}
	return dispatched
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

// ensureScheduleSession 为调度条目落实可执行的绑定会话：空 ID 与 CLI 约定的
// "new" 哨兵都视为未绑定——先回退该智能体最近活动的会话，仍无则新建
// （project_dir 非空随 meta 落库，文件记忆随之落到项目目录）。返回绑定后的
// 会话 ID；调用方决定是否回写调度存储。
func (d *Daemon) ensureScheduleSession(agent, sessionID, projectDir string) (string, error) {
	if sessionID != "" && sessionID != "new" {
		return sessionID, nil
	}
	if d.app == nil || d.app.SessDB() == nil {
		return "", fmt.Errorf("session store not available")
	}
	if sid := d.resolveLatestSessionForAgent(agent); sid != "" {
		return sid, nil
	}
	var opts []goharnesssession.SessionOption
	if projectDir != "" {
		opts = append(opts, goharnesssession.WithProjectDirOption(projectDir))
	}
	info, err := goharnesssession.CreateSession(context.Background(), d.app.SessDB(), agent, opts...)
	if err != nil {
		return "", fmt.Errorf("create schedule session failed: %w", err)
	}
	d.logger.Info("schedule entry: session created", "agent", agent, "session_id", info.SessionID, "project_dir", projectDir)
	return info.SessionID, nil
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

func (d *Daemon) KVStore() *bbolt.DB {
	return d.kvStore
}
