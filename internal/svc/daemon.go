package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/DotNetAge/mindx/pkg/session"
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
	memoryWatch  *memory.FileWatchService
	sharedMemory *memory.RAGMemory
	addr         string
	wsPath       string
	logger       logging.Logger
	execMu       sync.Mutex
}

func NewDaemon(app *core.App, addr, wsPath string) *Daemon {
	logDir := logging.ResolveLogDir()
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(logDir, "mindx-daemon.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})

	var schedulerDB *scheduler.FileSchedulerStore
	schedDB, err := scheduler.NewFileSchedulerStore(app.Settings().SchedulesDir())
	if err != nil {
		logger.Warn("failed to create scheduler store, scheduled tasks disabled", "error", err)
	} else {
		schedulerDB = schedDB
	}

	var memoryWatch *memory.FileWatchService
	var sharedMemory *memory.RAGMemory
	if emb := app.Embedder(); emb != nil {
		sharedMem, memErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: goreactcore.MemoryTypeLongTerm,
			AgentName:  "_shared",
			MemoryDir:  filepath.Join(app.Settings().UserPreferences(), "memory"),
			Embedder:   emb,
		})
		if memErr != nil {
			logger.Warn("filewatch: failed to create shared LongTerm indexer, watch disabled", "error", memErr)
		} else {
			sharedMemory = sharedMem
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
						app.FileVersions().Record(s.SessionDir, absPath)
					}
				}
			}
			}
		}
	}

	d := &Daemon{
		app:          app,
		addr:         addr,
		wsPath:       wsPath,
		schedulerDB:  schedulerDB,
		memoryWatch:  memoryWatch,
		sharedMemory: sharedMemory,
		logger:       logger,
	}

	if schedulerDB != nil {
		d.scheduler = scheduler.NewScheduler(schedulerDB, d.executeScheduleCommand, logger)
	}
	return d
}

func (d *Daemon) Start(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("Scheduler failed to start", "error", err)
		}
	}

	if d.memoryWatch != nil {
		go func() {
			if err := d.memoryWatch.Start(ctx); err != nil {
				d.logger.Warn("FileWatch service exited with error", "error", err)
			}
		}()
	}

	d.logger.Info("MindX daemon starting", "addr", fmt.Sprintf("ws://localhost%s%s", d.addr, d.wsPath))

	if err := d.gw.Start(); err != nil {
		d.stopBackgroundServices()
		return fmt.Errorf("gateway start failed: %w", err)
	}

	<-ctx.Done()
	d.logger.Info("Shutting down")

	d.stopBackgroundServices()

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) TestStart(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("scheduler failed to start", "error", err)
		}
	}

	if err := d.gw.Start(); err != nil {
		d.stopBackgroundServices()
		return fmt.Errorf("gateway start failed: %w", err)
	}

	return nil
}

func (d *Daemon) stopBackgroundServices() {
	if d.memoryWatch != nil {
		d.memoryWatch.Stop()
	}
	if d.scheduler != nil {
		d.scheduler.Stop()
	}
}

func (d *Daemon) TestStop(ctx context.Context) error {
	if d.gw == nil {
		return nil
	}

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) initGateway() {
	d.gw = gateway.New(
		gateway.WithAddr(d.addr),
		gateway.WithPath(d.wsPath),
		gateway.WithHandler(d.defaultHandler),
	)

	registry := NewRPCHandlerRegistry(d)
	registry.RegisterAll(d.gw)
}

// ---------------------------------------------------------------------------
// Message Handler & Session Resolution
// ---------------------------------------------------------------------------

func (d *Daemon) defaultHandler(msg *gateway.Message) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.Text == "" {
		d.logger.Warn("defaultHandler: missing or invalid text field",
			"data", string(msg.Data), "error", err)
		return
	}
	text := payload.Text

	agentName, providedSessionID, content := parseAgentTarget(text)

	agent, err := d.app.ResolveAgent(agentName)
	if err != nil {
		d.sendEvent(msg.ClientID, msg.SessionID, gateway.RespError, "错误", err.Error())
		return
	}

	sessionID := d.resolveSessionID(msg.SessionID, providedSessionID)
	resolvedAgentName := agentName
	if resolvedAgentName == "" {
		resolvedAgentName = agent.Name()
	}

	d.logger.Info("request start",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
		"input_preview", truncate(content, 100),
	)

	eventCh, cancelEvents := agent.EventsFiltered(func(e goreactcore.ReactEvent) bool {
		switch e.Type {
		case goreactcore.ThinkingDelta, goreactcore.ThinkingDone, goreactcore.ActionStart,
			goreactcore.ActionProgress, goreactcore.ToolExecStart, goreactcore.ToolExecEnd,
			goreactcore.ActionEnd, goreactcore.FinalAnswer,
			goreactcore.ExecutionSummary, goreactcore.Error, goreactcore.SubtaskSpawned,
			goreactcore.SubtaskCompleted, goreactcore.ClarifyNeeded, goreactcore.PermissionRequest,
			goreactcore.PermissionDenied, goreactcore.CycleEnd, goreactcore.TaskSummary:
			return true
		default:
			return false
		}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range eventCh {
			d.forwardEvent(msg.ClientID, event)
		}
	}()

	_, err = agent.Ask(sessionID, content)

	cancelEvents()

	if err != nil {
		d.logger.Error("request failed", err,
			"client_id", msg.ClientID,
			"session_id", sessionID,
			"agent", resolvedAgentName,
		)
		d.sendEvent(msg.ClientID, sessionID, gateway.RespError, "错误", "request failed")
	}

	<-done
	d.logger.Info("request done",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)
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
	resolvedAgent, err := d.app.ResolveAgent(agent)
	if err != nil {
		return fmt.Errorf("resolve agent %q: %w", agent, err)
	}
	if sessionID == "" || sessionID == "new" {
		sessionID = generateSessionID()
	}

	targetDir := projectDir
	if targetDir == "" {
		meta := d.restoreSessionEnvironment(sessionID)
		if meta != nil {
			targetDir = meta.ProjectWorkingDir
		}
	}

	if targetDir != "" {
		d.execMu.Lock()
		originalCWD, _ := os.Getwd()

		if err := os.Chdir(targetDir); err != nil {
			d.execMu.Unlock()
			d.logger.Warn("failed to chdir to project dir, using current dir",
				"project_dir", targetDir,
				"error", err,
			)
		} else {
			os.Setenv("MINDX_PROJECT_DIR", targetDir)
			os.Setenv("MINDX_SESSION_ID", sessionID)
			d.logger.Info("set execution context for scheduled task",
				"session_id", sessionID,
				"project_dir", targetDir,
				"original_cwd", originalCWD,
			)

			_, err = resolvedAgent.Ask(sessionID, content)

			if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
				d.logger.Warn("failed to restore cwd after scheduled task",
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
			return nil
		}
	}

	_, err = resolvedAgent.Ask(sessionID, content)
	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
	}
	return nil
}

func (d *Daemon) restoreSessionEnvironment(sessionID string) *session.SessionMeta {
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
