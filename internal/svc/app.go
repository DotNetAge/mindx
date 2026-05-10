package svc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
)

// App encapsulates the MindX gateway server lifecycle.
// The server is stateless regarding agent selection — the client is responsible
// for tracking which agent the user is currently talking to, and must prefix
// messages with @<agent-name> to target a specific agent.
type App struct {
	gw       *gateway.Server
	settings *Settings
	logger   logging.Logger
	agents   *goreact.AgentRegistry
	models   *goreact.ModelRegistry
	master   *goreact.Agent
	masterMu sync.RWMutex

	rules  core.RuleRegistry
	sessDB *session.FileSessionStore

	scheduler   *scheduler.Scheduler
	schedulerDB *scheduler.FileSchedulerStore

	agentCache map[string]*goreact.Agent
	agentMu    sync.RWMutex
}

func DefaultApp() (*App, error) {

	logger := logging.DefaultConsoleLogger()

	err := godotenv.Load()
	if err != nil {
		logger.Warn("WARNING: failed to load .env file: %v", err)
	}

	settings := &Settings{
		Workspace:   os.Getenv("MINDX_WORKSPACE"),
		Path:        os.Getenv("MINDX_PWD_PATH"),
		Addr:        os.Getenv("MINDX_WS_ADDR"),
		WSPath:      os.Getenv("MINDX_WS_PATH"),
		MasterAgent: os.Getenv("MINDX_MASTER"),
	}
	core.SYSTEM_INFO_NAME = "MindX"
	core.SYSTEM_INFO_VERSION = "2.0.0"

	logger.Info("loading agents", "dir", settings.AgentsDir())
	logger.Info("Master", "agent", settings.MasterAgent)
	agents, err := goreact.LoadAgentsFrom(settings.AgentsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}

	// If MasterAgent is not configured, default to the first available agent.
	if settings.MasterAgent == "" {
		if list := agents.List(); len(list) > 0 {
			settings.MasterAgent = list[0].Name
			logger.Warn("MINDX_MASTER not set, defaulting to first agent", "name", list[0].Name)
		}
	}

	logger.Info("Loading models", "dir", settings.ModelsFile())
	models, err := goreact.LoadModels(settings.ModelsFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load models: %w", err)
	}

	logger.Info("Loading rules", "file", settings.RulesFile())
	rules, err := core.NewYAMLRuleRegistry(settings.RulesFile())
	if err != nil {
		logger.Warn("Failed to load rules", "file", settings.RulesFile(), "error", err)
	}

	logger.Info("Loading sessions", "dir", settings.SessionsDir())
	sessDB, err := session.NewFileSessionStore(settings.SessionsDir())
	if err != nil {
		logger.Warn("Failed to init session store", "erorr", err)
	}

	logger.Info("Loading schedules", "dir", settings.SchedulesDir())
	schedulerDB, err := scheduler.NewFileSchedulerStore(settings.SchedulesDir())
	if err != nil {
		logger.Warn("Failed to init scheduler store", "error", err)
	}

	app := &App{
		settings:    settings,
		logger:      logger,
		agents:      agents,
		models:      models,
		rules:       rules,
		sessDB:      sessDB,
		schedulerDB: schedulerDB,
		agentCache:  make(map[string]*goreact.Agent),
	}

	app.scheduler = scheduler.NewScheduler(schedulerDB, app.executeScheduleCommand, logger)

	return app, nil
}

// NewApp creates a new App with the given listen address and WebSocket path.
func NewApp(addr, path string) *App {

	return &App{
		settings: &Settings{
			Addr:   addr,
			WSPath: path,
		},
		logger: logging.DefaultConsoleLogger(),
	}
}

func (a *App) Settings() *Settings {
	return a.settings
}

func (a *App) RuleRegistry() core.RuleRegistry {
	return a.rules
}

func (a *App) SessionDB() *session.FileSessionStore {
	return a.sessDB
}

func (a *App) SchedulerDB() *scheduler.FileSchedulerStore {
	return a.schedulerDB
}

func (a *App) Scheduler() *scheduler.Scheduler {
	return a.scheduler
}

func (a *App) executeScheduleCommand(ctx context.Context, agent string, content string) error {
	resolvedAgent, err := a.resolveAgent(agent)
	if err != nil {
		return fmt.Errorf("resolve agent %q: %w", agent, err)
	}
	sessionID := fmt.Sprintf("sched_%s_%s", agent, time.Now().Format("20060102"))
	_, err = resolvedAgent.Ask(sessionID, content)
	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s: %w", agent, err)
	}
	return nil
}

func (a *App) Agents() *goreact.AgentRegistry {
	return a.agents
}

func (a *App) Models() *goreact.ModelRegistry {
	return a.models
}

// GetMaster returns (or creates) the master agent.
func (a *App) GetMaster() (*goreact.Agent, error) {
	return a.getMaster()
}

// SetLogger replaces the default logger.
func (a *App) SetLogger(l logging.Logger) {
	a.logger = l
}

func (a *App) getMaster() (*goreact.Agent, error) {
	a.masterMu.Lock()
	defer a.masterMu.Unlock()

	if a.master != nil {
		return a.master, nil
	}

	masterAgent := a.Agents().Get(a.settings.MasterAgent)
	if masterAgent == nil {
		return nil, fmt.Errorf("Master agent not defined")
	}

	if masterAgent.Model == "" {
		return nil, fmt.Errorf("agent %q has no model configured", masterAgent.Name)
	}
	masterModel := a.Models().Get(masterAgent.Model)
	if masterModel == nil {
		return nil, fmt.Errorf("model %q not found for agent %q", masterAgent.Model, masterAgent.Name)
	}

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(masterAgent),
		goreact.WithModel(masterModel),
	}

	if a.rules != nil {
		opts = append(opts, goreact.WithRuleRegistry(a.rules))
	}

	if a.sessDB != nil {
		opts = append(opts, goreact.WithSessionStore(a.sessDB))
	}

	m, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}
	a.master = m
	return a.master, nil
}

func (a *App) resolveAgent(name string) (*goreact.Agent, error) {
	if name == "" {
		return a.getMaster()
	}

	a.agentMu.RLock()
	if cached, ok := a.agentCache[name]; ok {
		a.agentMu.RUnlock()
		return cached, nil
	}
	a.agentMu.RUnlock()

	cfg := a.agents.Get(name)
	if cfg == nil {
		return nil, fmt.Errorf("agent %q not found in registry", name)
	}

	if cfg.Model == "" {
		return nil, fmt.Errorf("agent %q has no model configured", name)
	}
	model := a.Models().Get(cfg.Model)
	if model == nil {
		return nil, fmt.Errorf("model %q not found for agent %q", cfg.Model, name)
	}

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(cfg),
		goreact.WithModel(model),
	}

	if a.rules != nil {
		opts = append(opts, goreact.WithRuleRegistry(a.rules))
	}

	if a.sessDB != nil {
		opts = append(opts, goreact.WithSessionStore(a.sessDB))
	}

	agent, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}

	a.agentMu.Lock()
	a.agentCache[name] = agent
	a.agentMu.Unlock()
	return agent, nil
}

func (a *App) IsModelAvailable(name ...string) bool {
	n := ""
	if len(name) == 0 {
		a.masterMu.RLock()
		m := a.master
		a.masterMu.RUnlock()
		if m == nil || m.Model() == nil {
			return false
		}
		n = m.Model().Name
	} else {
		n = name[0]
	}

	if n == "" {
		return false
	}

	m := a.Models().Get(n)
	if m == nil {
		return false
	}

	client := gochat.Client().Config(
		gochat.WithBaseURL(m.BaseURL),
		gochat.WithAPIKey(m.APIKey),
		gochat.WithModel(m.Name),
		gochat.WithAuthToken(m.AuthToken),
		gochat.WithTimeout(10*time.Second),
	)

	llm, err := client.UserMessage("Hello").GetResponse()
	if err != nil {
		return false
	}
	return llm.Content != ""
}

// RegisterCommand adds a slash command to the gateway.
func (a *App) RegisterCommand(meta gateway.CommandMeta, handler gateway.CommandHandler) {
	if a.gw == nil {
		a.initGateway()
	}
	a.gw.RegisterCommand(meta, handler)
}

// RegisterBuiltinCommands registers the default /help, /agents, /skills commands.
func (a *App) RegisterBuiltinCommands() {
	if a.gw == nil {
		a.initGateway()
	}
	RegisterBuiltinCommands(a.gw, a)
}

// Server returns the underlying gateway server for advanced usage.
func (a *App) Server() *gateway.Server {
	if a.gw == nil {
		a.initGateway()
	}
	return a.gw
}

// Start initializes the gateway (if not yet) and starts listening.
// The gateway runs in a background goroutine. This method blocks until
// the caller cancels ctx, then performs a graceful shutdown.
func (a *App) Start(ctx context.Context) error {
	if a.gw == nil {
		a.initGateway()
	}

	if a.scheduler != nil {
		if err := a.scheduler.Start(ctx); err != nil {
			a.logger.Warn("Scheduler failed to start", "error", err)
		}
	}

	a.logger.Info("MindX gateway starting", "addr", fmt.Sprintf("ws://localhost%s%s", a.settings.Addr, a.settings.WSPath))

	if err := a.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	<-ctx.Done()
	a.logger.Info("Shutting down")

	if err := a.gw.StopAllChannels(ctx); err != nil {
		a.logger.Warn("failed to stop channels", "error", err)
	}

	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	return a.gw.Shutdown(ctx)
}

// TestStart initializes and starts the gateway server for testing purposes.
// Unlike Start, this method is non-blocking and returns immediately after
// the server is ready to accept connections.
func (a *App) TestStart(ctx context.Context) error {
	if a.gw == nil {
		a.initGateway()
	}

	if a.scheduler != nil {
		if err := a.scheduler.Start(ctx); err != nil {
			a.logger.Warn("scheduler failed to start", "error", err)
		}
	}

	if err := a.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	return nil
}

// TestStop gracefully shuts down the gateway server for testing purposes.
func (a *App) TestStop(ctx context.Context) error {
	if a.gw == nil {
		return nil
	}

	if err := a.gw.StopAllChannels(ctx); err != nil {
		a.logger.Warn("failed to stop channels", "error", err)
	}

	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	return a.gw.Shutdown(ctx)
}

// initGateway lazily creates the gateway server.
func (a *App) initGateway() {
	a.gw = gateway.New(
		gateway.WithAddr(a.settings.Addr),
		gateway.WithPath(a.settings.WSPath),
		gateway.WithHandler(a.defaultHandler),
	)
}
