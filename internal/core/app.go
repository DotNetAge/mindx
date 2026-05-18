package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	goragcore "github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type App struct {
	settings    *Settings
	mindxConfig *MindxConfig
	credStore   CredentialStore
	logger      logging.Logger
	agents      *goreact.AgentRegistry
	models      *goreact.ModelRegistry
	current      *goreact.Agent
	currentMu    sync.RWMutex

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

func (a *App) CurrentAgent() (*goreact.Agent, error) {
	return a.currentAgent()
}

func (a *App) currentAgent() (*goreact.Agent, error) {
	a.currentMu.Lock()
	defer a.currentMu.Unlock()

	if a.current != nil {
		return a.current, nil
	}

	currentAgentName := a.CurrentAgentName()
	if currentAgentName == "" {
		return nil, fmt.Errorf("no agent available")
	}

	agent := a.Agents().Get(currentAgentName)
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", currentAgentName)
	}

	var resolvedModel core.ModelConfig
	if agent.Model == "" {
		return nil, fmt.Errorf("agent %q has no model configured", agent.Name)
	}
	modelCfg := a.Models().Get(agent.Model)
	if modelCfg == nil {
		return nil, fmt.Errorf("model %q not found for agent %q", agent.Model, agent.Name)
	}
	resolvedModel = *modelCfg
	resolvedModel.APIKey = a.resolveAPIKey(resolvedModel.APIKey)

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(agent),
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
			AgentName:  agent.Name,
			MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:   a.embedder,
		}
		mem, memErr := memory.NewRAGMemoryFromConfig(memConfig)
		if memErr != nil {
			a.logger.Warn("Failed to create memory for agent %q: %v", agent.Name, memErr)
		} else {
			opts = append(opts, goreact.WithMemory(mem))
		}
	}

	m, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}
	a.current = m
	a.syncProjectMemory(m, currentAgentName)
	return a.current, nil
}

func (a *App) ResolveAgent(name string) (*goreact.Agent, error) {
	if name == "" {
		return a.currentAgent()
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
		a.currentMu.RLock()
		m := a.current
		a.currentMu.RUnlock()
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

func (a *App) SwitchAgentModel(agentName, modelName string) error {
	cfg := a.Agents().Get(agentName)
	if cfg == nil {
		return fmt.Errorf("agent %q not found", agentName)
	}

	model := a.Models().Get(modelName)
	if model == nil || !model.Enabled {
		return fmt.Errorf("model %q not available", modelName)
	}

	oldModel := cfg.Model
	cfg.Model = modelName

	agentFile := filepath.Join(a.settings.AgentsDir(), agentName+".yml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		cfg.Model = oldModel
		return fmt.Errorf("marshal failed: %w", err)
	}
	if err := os.WriteFile(agentFile, data, 0644); err != nil {
		cfg.Model = oldModel
		return fmt.Errorf("write failed: %w", err)
	}

	a.agentMu.Lock()
	delete(a.agentCache, agentName)
	a.agentMu.Unlock()

	if a.CurrentAgentName() == agentName {
		a.currentMu.Lock()
		a.current = nil
		a.currentMu.Unlock()
	}

	a.logger.Info("agent model updated",
		"agent", agentName,
		"old_model", oldModel,
		"new_model", modelName,
	)

	return nil
}

func (a *App) SwitchSession(sessionID string) (*core.SessionInfo, error) {
	ctx := context.Background()
	sessions, err := a.SessDB().ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var target *core.SessionInfo
	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			target = &sessions[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	a.SetCurrentSessionMeta(target)

	if a.mindxConfig != nil {
		a.mindxConfig.LastSessionID = sessionID
		_ = a.mindxConfig.Save()
	}

	a.logger.Info("session switched",
		"session_id", sessionID,
	)

	return target, nil
}

func (a *App) ClearCurrentSession() (*core.SessionInfo, error) {
	currentMeta := a.CurrentSessionMeta()
	var oldSessionID string
	if currentMeta != nil && currentMeta.SessionID != "" {
		oldSessionID = currentMeta.SessionID
		a.logger.Warn("physically deleting session",
			"session_id", currentMeta.SessionID,
			"reason", "user requested /chat clear",
		)

		if err := a.SessDB().Delete(context.Background(), time.Now().Unix(), currentMeta.SessionID); err != nil {
			return nil, fmt.Errorf("delete failed: %w", err)
		}
	}

	newSession, err := a.CreateSession(a.CurrentAgentName())
	if err != nil {
		return nil, fmt.Errorf("create new session failed: %w", err)
	}

	a.logger.Info("session cleared and new one created",
		"old_session_id", oldSessionID,
		"new_session_id", newSession.SessionID,
	)

	return newSession, nil
}
