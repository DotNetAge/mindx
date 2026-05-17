package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/goreact/core"
	goragcore "github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
)

type App struct {
	settings   *Settings
	mindxConfig *MindxConfig
	credStore  CredentialStore
	logger     logging.Logger
	agents     *goreact.AgentRegistry
	models     *goreact.ModelRegistry
	master     *goreact.Agent
	masterMu   sync.RWMutex

	rules  core.RuleRegistry
	sessDB *session.FileSessionStore

	agentCache         map[string]*goreact.Agent
	agentMu            sync.RWMutex
	currentSessionMeta *core.SessionInfo // Changed from session.SessionMeta to core.SessionInfo (framework-level)

	embedder goragcore.Embedder
}

func DefaultApp(mindxConfig *MindxConfig) (*App, error) {
	settings := &Settings{}

	logDir := settings.LogsDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile := filepath.Join(logDir, "mindx.log")
	logger, err := logging.DefaultFileLogger(logFile)
	if err != nil {
		logger = logging.DefaultNoopLogger()
	}

	err = godotenv.Load()
	if err != nil {
		logger.Warn("WARNING: failed to load .env file: %v", err)
	}
	core.SYSTEM_INFO_NAME = "MindX"
	core.SYSTEM_INFO_VERSION = "2.0.0"

	userPrompt := "\n- User preferences directory: " + settings.UserPreferences()
	userPrompt += "\n- Skills directory: " + settings.SkillsDir()
	userPrompt += "\n- Agents directory: " + settings.AgentsDir()
	userPrompt += "\n- Python virtual environment: " + settings.VenvDir()
	core.SYSTEM_INFO_USERS = userPrompt
	core.SYSTEM_ADDON_SECTIONS = []string{
		BuildDelegationGuidance(),
	}

	logger.Info("loading agents", "dir", settings.AgentsDir())
	agents, err := goreact.LoadAgentsFrom(settings.AgentsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
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

	credStore := NewCredentialStore(settings.UserPreferences())

	// Create embedder if configured for semantic memory support
	var emb goragcore.Embedder
	if mindxConfig != nil && mindxConfig.HasEmbedder() {
		modelPath := mindxConfig.EmbedderModelPath(settings.UserPreferences())
		var embErr error
		emb, embErr = memory.NewEmbedderFromConfig(modelPath)
		if embErr != nil {
			logger.Warn("Failed to create embedder, memory disabled: %v", embErr)
		}
	}

	return &App{
		settings:    settings,
		mindxConfig: mindxConfig,
		credStore:   credStore,
		logger:      logger,
		agents:      agents,
		models:      models,
		rules:       rules,
		sessDB:      sessDB,
		agentCache:  make(map[string]*goreact.Agent),
		embedder:    emb,
	}, nil
}

func resolveCurrentAgentName(cfg *MindxConfig, agents *goreact.AgentRegistry, logger logging.Logger) string {
	if cfg != nil && cfg.LastAgent != "" {
		if agents.Get(cfg.LastAgent) != nil {
			return cfg.LastAgent
		}
		logger.Warn("last_agent %q not found in registry, will use fallback", cfg.LastAgent)
	}

	if list := agents.List(); len(list) > 0 {
		logger.Info("using first agent as current", "name", list[0].Name)
		return list[0].Name
	}

	return ""
}

func (a *App) Settings() *Settings {
	return a.settings
}

func (a *App) Config() *MindxConfig {
	return a.mindxConfig
}

func (a *App) CurrentAgentName() string {
	return resolveCurrentAgentName(a.mindxConfig, a.agents, a.logger)
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

func (a *App) resolveAPIKey(ref string) string {
	return ResolveAPIKey(a.credStore, ref)
}

func (a *App) SetLogger(l logging.Logger) {
	a.logger = l
}

// CurrentSessionMeta returns the metadata for the current active session.
func (a *App) CurrentSessionMeta() *core.SessionInfo {
	return a.currentSessionMeta
}

// SetCurrentSessionMeta sets the current session metadata (used when loading existing sessions).
func (a *App) SetCurrentSessionMeta(meta *core.SessionInfo) {
	a.currentSessionMeta = meta
}

// SessDB returns the session store for accessing session data.
func (a *App) SessDB() *session.FileSessionStore {
	return a.sessDB
}

// CreateSession creates a new session with metadata including the captured project directory (os.Getwd() at invocation time).
// This captures os.Getwd() at creation time to bind the session to a project directory.
// Delegates to SessionStore.Create() which handles directory creation and ID generation.
func (a *App) CreateSession(agentName string) (*core.SessionInfo, error) {
	sessionInfo, err := a.sessDB.Create(context.Background(), agentName)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	a.currentSessionMeta = sessionInfo

	a.logger.Info("session created",
		"session_id", sessionInfo.SessionID,
		"agent", agentName,
		"project_dir", sessionInfo.GetProjectDir(),
		"session_dir", sessionInfo.GetSessionDir(),
	)

	return sessionInfo, nil
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

	currentAgentName := a.CurrentAgentName()
	if currentAgentName == "" {
		return nil, fmt.Errorf("no agent available")
	}

	masterAgent := a.Agents().Get(currentAgentName)
	if masterAgent == nil {
		return nil, fmt.Errorf("agent %q not found", currentAgentName)
	}

	if masterAgent.Model == "" {
		return nil, fmt.Errorf("agent %q has no model configured", masterAgent.Name)
	}
	masterModel := a.Models().Get(masterAgent.Model)
	if masterModel == nil {
		return nil, fmt.Errorf("model %q not found for agent %q", masterAgent.Model, masterAgent.Name)
	}

	resolvedModel := *masterModel
	resolvedModel.APIKey = a.resolveAPIKey(masterModel.APIKey)

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(masterAgent),
		goreact.WithModel(&resolvedModel),
		goreact.WithLogger(a.logger),
	}

	if a.rules != nil {
		opts = append(opts, goreact.WithRuleRegistry(a.rules))
	}

	if a.sessDB != nil {
		opts = append(opts, goreact.WithSessionStore(a.sessDB))
	}

	if a.currentSessionMeta != nil {
		if a.currentSessionMeta.GetProjectDir() != "" {
			opts = append(opts, goreact.WithProjectDir(a.currentSessionMeta.GetProjectDir()))
		}
		if a.currentSessionMeta.GetSessionDir() != "" {
			opts = append(opts, goreact.WithSessionDir(a.currentSessionMeta.GetSessionDir()))
		}
	}

	sessionBaseDir := a.settings.SessionsDir()
	if sessionBaseDir != "" {
		opts = append(opts, goreact.WithSessionBaseDir(sessionBaseDir))
	}

	if a.embedder != nil {
		memConfig := memory.MemoryConfig{
			MemoryType: core.MemoryTypeLongTerm,
			AgentName:  masterAgent.Name,
			MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:   a.embedder,
		}
		mem, memErr := memory.NewRAGMemoryFromConfig(memConfig)
		if memErr != nil {
			a.logger.Warn("Failed to create memory for agent %q: %v", masterAgent.Name, memErr)
		} else {
			opts = append(opts, goreact.WithMemory(mem))
		}
	}

	m, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}
	a.master = m
	a.syncProjectMemory(m, currentAgentName)
	return a.master, nil
}

func (a *App) ResolveAgent(name string) (*goreact.Agent, error) {
	if name == "" {
		return a.getMaster()
	}

	a.agentMu.RLock()
	if cached, ok := a.agentCache[name]; ok {
		a.agentMu.RUnlock()
		a.syncProjectMemory(cached, name)
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

	resolvedModel := *model
	resolvedModel.APIKey = a.resolveAPIKey(model.APIKey)

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(cfg),
		goreact.WithModel(&resolvedModel),
		goreact.WithLogger(a.logger),
	}

	if a.rules != nil {
		opts = append(opts, goreact.WithRuleRegistry(a.rules))
	}

	if a.sessDB != nil {
		opts = append(opts, goreact.WithSessionStore(a.sessDB))
	}

	// Auto-inject directory context from active session (Design-time safety)
	if a.currentSessionMeta != nil {
		if a.currentSessionMeta.GetProjectDir() != "" {
			opts = append(opts, goreact.WithProjectDir(a.currentSessionMeta.GetProjectDir()))
		}
		if a.currentSessionMeta.GetSessionDir() != "" {
			opts = append(opts, goreact.WithSessionDir(a.currentSessionMeta.GetSessionDir()))
		}
	}

	// Enable Agent Native sandbox design (4-Layer Architecture)
	sessionBaseDir := a.settings.SessionsDir()
	if sessionBaseDir != "" {
		opts = append(opts, goreact.WithSessionBaseDir(sessionBaseDir))
	}

	// Create per-agent memory if embedder is available
	if a.embedder != nil {
		memConfig := memory.MemoryConfig{
			MemoryType: core.MemoryTypeLongTerm,
			AgentName:  cfg.Name,
			MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:   a.embedder,
		}
		mem, memErr := memory.NewRAGMemoryFromConfig(memConfig)
		if memErr != nil {
			a.logger.Warn("Failed to create memory for agent %q: %v", cfg.Name, memErr)
		} else {
			opts = append(opts, goreact.WithMemory(mem))
		}
	}

	agent, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}

	a.agentMu.Lock()
	a.agentCache[name] = agent
	a.agentMu.Unlock()
	a.syncProjectMemory(agent, name)
	return agent, nil
}

// syncProjectMemory triggers incremental project file indexing for the given agent.
// It only runs when a session with a ProjectDir is active.
// The sync is non-blocking for performance; errors are logged.
func (a *App) syncProjectMemory(agent *goreact.Agent, agentName string) {
	if a.currentSessionMeta == nil || a.currentSessionMeta.GetProjectDir() == "" {
		return
	}
	mem := agent.Memory()
	if mem == nil {
		return
	}
	ragMem, ok := mem.(*memory.RAGMemory)
	if !ok {
		return
	}

	projectDir := a.currentSessionMeta.GetProjectDir()
	cacheDir := filepath.Join(a.settings.UserPreferences(), "memory", agentName, "project")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := ragMem.SyncProjectDir(ctx, projectDir, cacheDir)
	if result.Err != nil {
		a.logger.Warn("project sync failed", "agent", agentName, "error", result.Err)
		return
	}
	if result.Indexed > 0 || result.Updated > 0 || result.Removed > 0 {
		a.logger.Info("project sync",
			"agent", agentName,
			"indexed", result.Indexed,
			"updated", result.Updated,
			"skipped", result.Skipped,
			"removed", result.Removed,
		)
	}
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
	if m == nil || !m.Enabled {
		return false
	}

	client := gochat.Client().Config(
		gochat.WithBaseURL(m.BaseURL),
		gochat.WithAPIKey(a.resolveAPIKey(m.APIKey)),
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

// BuildDelegationGuidance returns the MindX-specific delegation strategy addon.
// Injected via SYSTEM_ADDON_SECTIONS — bridges P0 Scope Gate ("delegate") to concrete actions.
// GoReact's DefaultBehavioralRules only says "delegate" — this tells the LLM HOW.
func BuildDelegationGuidance() string {
	return `## Delegation
When a task is outside your expertise, choose one path:

- **Know who handles it** → call **Delegate** tool directly (agent_name + task), then **CollectResults**
- **Don't know who** → load **find-experts** skill first (discovers experts, then delegates via same workflow)`
}
