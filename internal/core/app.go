package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
)

type App struct {
	settings *Settings
	logger   logging.Logger
	agents   *goreact.AgentRegistry
	models   *goreact.ModelRegistry
	master   *goreact.Agent
	masterMu sync.RWMutex

	rules  core.RuleRegistry
	sessDB *session.FileSessionStore

	agentCache         map[string]*goreact.Agent
	agentMu            sync.RWMutex
	currentSessionMeta *session.SessionMeta
}

func DefaultApp() (*App, error) {
	logger := logging.DefaultConsoleLogger()

	err := godotenv.Load()
	if err != nil {
		logger.Warn("WARNING: failed to load .env file: %v", err)
	}

	settings := &Settings{
		// Path:        os.Getenv("MINDX_PWD_PATH"),
		MasterAgent: os.Getenv("MINDX_MASTER"),
	}
	core.SYSTEM_INFO_NAME = "MindX"
	core.SYSTEM_INFO_VERSION = "2.0.0"
	// userPrompt := "- Documents directory : " + filepath.Join(settings.Workspace, "documents")
	// userPrompt += "\n- Programs directory : " + filepath.Join(settings.Workspace, "programs")
	userPrompt := "\n- Skills directory: " + settings.SkillsDir()
	userPrompt += "\n- Agents directory: " + settings.AgentsDir()

	core.SYSTEM_INFO_USERS = userPrompt
	logger.Info("loading agents", "dir", settings.AgentsDir())
	agents, err := goreact.LoadAgentsFrom(settings.AgentsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}

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
		logger.Warn("Failed to init session store", "error", err)
	}

	return &App{
		settings:   settings,
		logger:     logger,
		agents:     agents,
		models:     models,
		rules:      rules,
		sessDB:     sessDB,
		agentCache: make(map[string]*goreact.Agent),
	}, nil
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

func (a *App) Agents() *goreact.AgentRegistry {
	return a.agents
}

func (a *App) Models() *goreact.ModelRegistry {
	return a.models
}

func (a *App) SetLogger(l logging.Logger) {
	a.logger = l
}

// CurrentSessionMeta returns the metadata for the current active session.
func (a *App) CurrentSessionMeta() *session.SessionMeta {
	return a.currentSessionMeta
}

// CreateSession creates a new session with metadata including the captured working directory.
// This captures os.Getwd() at creation time to bind the session to a project directory.
func (a *App) CreateSession(agentName string) (*session.SessionMeta, error) {
	projectCWD, err := os.Getwd()
	if err != nil {
		a.logger.Warn("failed to get cwd, using home dir as fallback", "error", err)
		projectCWD = a.settings.UserPreferences()
	}

	sessionID := generateSessionID()

	meta, err := session.NewSessionMeta(sessionID, agentName, projectCWD)
	if err != nil {
		return nil, fmt.Errorf("create session meta: %w", err)
	}

	sessionBaseDir := a.settings.SessionsDir()
	sessionDir := filepath.Join(sessionBaseDir, agentName, sessionID)
	tmpDir := filepath.Join(sessionDir, "tmp")

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	if err := meta.Save(sessionDir); err != nil {
		return nil, fmt.Errorf("save session meta: %w", err)
	}

	a.currentSessionMeta = meta

	a.logger.Info("session created",
		"session_id", sessionID,
		"agent", agentName,
		"project_dir", projectCWD,
		"session_dir", sessionDir,
	)

	return meta, nil
}

func (a *App) GetMaster() (*goreact.Agent, error) {
	return a.getMaster()
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
		goreact.WithLogger(a.logger), // 注入统一日志接口（ZapLogger）
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

func (a *App) ResolveAgent(name string) (*goreact.Agent, error) {
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
		goreact.WithLogger(a.logger), // 注入统一日志接口（ZapLogger）
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

// generateSessionID generates a unique session identifier.
// This is shared across the application to ensure consistent ID format (sess_xxxxxxxx).
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano()%100000000)
}
