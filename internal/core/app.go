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
)

type App struct {
	settings    *Settings
	mindxConfig *MindxConfig
	credStore   CredentialStore
	logger      logging.Logger
	agents      *goreact.AgentRegistry
	models      *goreact.ModelRegistry
	costs       *CostRegistry
	versions    *FileVersionStore
	current     *goreact.Agent
	currentMu   sync.RWMutex

	rules  core.RuleRegistry
	sessDB *session.FileSessionStore

	agentCache         map[string]*goreact.Agent
	agentMu            sync.RWMutex
	currentSessionMeta *core.SessionInfo

	embedder goragcore.Embedder

	permissionRuleStore *MindxPermissionRuleStore
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
	userPrompt += "\n- Now: " + time.Now().Format(time.DateTime)
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

	logger.Info("Loading providers", "dir", settings.ProvidersFile())
	providers, err := LoadProvidersFile(settings.ProvidersFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load providers: %w", err)
	}
	for _, p := range providers {
		models.RegisterProvider(p.Name, p)
		logger.Info("Registered provider", "name", p.Name)
	}

	logger.Info("Loading model costs", "dir", settings.ModelsFile())
	costs, err := LoadCostsFromModelsFile(settings.ModelsFile())

	versions := NewFileVersionStore()
	if err != nil {
		return nil, fmt.Errorf("failed to load model costs: %w", err)
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

	// Create permission rule store (nil-safe: if mindxConfig is nil, returns no-op store)
	permStore := NewMindxPermissionRuleStore(mindxConfig)

	return &App{
		settings:            settings,
		mindxConfig:         mindxConfig,
		credStore:           credStore,
		logger:              logger,
		agents:              agents,
		models:              models,
		costs:               costs,
		versions:            versions,
		rules:               rules,
		sessDB:              sessDB,
		agentCache:          make(map[string]*goreact.Agent),
		embedder:            emb,
		permissionRuleStore: permStore,
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

// Embedder returns the semantic embedder for memory indexing, or nil if not configured.
func (a *App) Embedder() goragcore.Embedder {
	return a.embedder
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

func (a *App) SetTestDir(tmpDir string) error {
	a.settings.Test = true
	sessDB, err := session.NewFileSessionStore(filepath.Join(tmpDir, "sessions"))
	if err != nil {
		return err
	}
	a.sessDB = sessDB
	return nil
}

func (a *App) Agents() *goreact.AgentRegistry {
	return a.agents
}

func (a *App) SetAgentsRegistry(registry *goreact.AgentRegistry) {
	a.agents = registry
}

func (a *App) Models() *goreact.ModelRegistry {
	return a.models
}

func (a *App) Costs() *CostRegistry {
	return a.costs
}

func (a *App) ModelCost(name string) (ModelCost, bool) {
	return a.costs.Get(name)
}

func (a *App) FileVersions() *FileVersionStore {
	return a.versions
}

func (a *App) resolveAPIKey(ref string) string {
	return ResolveAPIKey(a.credStore, ref)
}

func (a *App) SetLogger(l logging.Logger) {
	a.logger = l
}

func (a *App) Logger() logging.Logger {
	return a.logger
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

func (a *App) resolveModelName(agentModelName string) (string, *core.ModelConfig, error) {
	// Priority: last_model > default_model > agent YAML model
	modelName := agentModelName
	if a.mindxConfig != nil {
		if a.mindxConfig.LastModel != "" {
			modelName = a.mindxConfig.LastModel
		} else if a.mindxConfig.DefaultModel != "" {
			modelName = a.mindxConfig.DefaultModel
		}
	}
	if modelName == "" {
		return "", nil, fmt.Errorf("no model configured")
	}
	modelCfg := a.Models().Get(modelName)
	if modelCfg == nil {
		return "", nil, fmt.Errorf("model %q not found", modelName)
	}
	return modelName, modelCfg, nil
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
	_, modelCfg, err := a.resolveModelName(agent.Model)
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", agent.Name, err)
	}
	resolvedModel = *modelCfg
	resolvedModel.APIKey = a.resolveAPIKey(resolvedModel.APIKey)

	cacheDir := filepath.Join(a.settings.DataDir(), "cache")
	kvStore, _ := core.NewFileSystemKVStore(cacheDir)

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(agent),
		goreact.WithModel(&resolvedModel),
		goreact.WithLogger(a.logger),
	}

	if kvStore != nil {
		opts = append(opts, goreact.WithKVStore(kvStore))
	}

	if a.mindxConfig != nil && a.mindxConfig.LastSessionID != "" {
		opts = append(opts, goreact.WithSession(a.mindxConfig.LastSessionID))
	}

	if a.rules != nil {
		opts = append(opts, goreact.WithRuleRegistry(a.rules))
	}

	if a.sessDB != nil {
		opts = append(opts, goreact.WithSessionStore(a.sessDB))
	}

	if a.currentSessionMeta == nil && a.sessDB != nil && a.mindxConfig != nil && a.mindxConfig.LastSessionID != "" {
		if si, err := a.sessDB.GetMeta(context.Background(), a.mindxConfig.LastSessionID); err == nil {
			a.currentSessionMeta = si
		}
	}

	// Smart session matching: if current working directory differs from the restored session's ProjectDir,
	// try to find a session that matches the current directory, or create a new one.
	if a.sessDB != nil && a.mindxConfig != nil {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil && a.currentSessionMeta != nil && a.currentSessionMeta.GetProjectDir() != "" {
			if !sameDirectory(cwd, a.currentSessionMeta.GetProjectDir()) {
				a.logger.Warn("working directory changed",
					"old_project_dir", a.currentSessionMeta.GetProjectDir(),
					"new_cwd", cwd,
				)
				// Try to find an existing session for this working directory
				if matched := a.findSessionByProjectDir(cwd); matched != nil {
					a.logger.Info("found matching session for current directory",
						"session_id", matched.SessionID,
						"project_dir", matched.GetProjectDir(),
					)
					a.currentSessionMeta = matched
					a.mindxConfig.LastSessionID = matched.SessionID
					_ = a.mindxConfig.Save()
				} else {
					a.logger.Info("no matching session found, will create new session on first interaction",
						"cwd", cwd,
					)
					// Clear currentSessionMeta so that CreateSession will be called with correct cwd
					a.currentSessionMeta = nil
					a.mindxConfig.LastSessionID = ""
					_ = a.mindxConfig.Save()
				}
			}
		}
	}

	if a.currentSessionMeta != nil {
		if a.currentSessionMeta.GetProjectDir() != "" {
			opts = append(opts, goreact.WithProjectDir(a.currentSessionMeta.GetProjectDir()))
		}
		if a.currentSessionMeta.GetSessionDir() != "" {
			opts = append(opts, goreact.WithSessionDir(
				filepath.Join(a.currentSessionMeta.GetSessionDir(), agent.Name)))
		}
	}

	if a.permissionRuleStore != nil {
		opts = append(opts, goreact.WithPermissionRuleStore(a.permissionRuleStore))
	}

	if a.embedder != nil {
		// LongTerm — shared unified knowledge base (indexed by Daemon memoryWatch)
		ltMem, ltErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: core.MemoryTypeLongTerm,
			AgentName:  "_shared",
			MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:   a.embedder,
		})
		if ltErr != nil {
			a.logger.Warn("Failed to create long-term memory for agent %q: %v", agent.Name, ltErr)
		} else {
			opts = append(opts, goreact.WithMemory(ltMem))
		}

		// SessionRAG — slid-context recall (ephemeral, bound to SessionDir)
		var sessionDir string
		if a.currentSessionMeta != nil {
			sessionDir = filepath.Join(a.currentSessionMeta.GetSessionDir(), agent.Name)
		}
		sMem, sErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: core.MemoryTypeSession,
			AgentName:  agent.Name,
			SessionDir: sessionDir,
			Embedder:   a.embedder,
		})
		if sErr != nil {
			a.logger.Warn("Failed to create session memory for agent %q: %v", agent.Name, sErr)
		} else {
			opts = append(opts, goreact.WithSessionMemory(sMem))
		}
	}

	opts = append(opts, goreact.WithProviderRegistry(a.models.ProviderRegistry()))
	m, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}

	if a.currentSessionMeta != nil && a.currentSessionMeta.SessionID != "" {
		m.NewSession(a.currentSessionMeta.SessionID)
	}
	a.current = m
	return a.current, nil
}

func (a *App) ResolveAgent(name string) (*goreact.Agent, error) {
	if name == "" {
		return a.currentAgent()
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

	_, modelCfg, err := a.resolveModelName(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", name, err)
	}

	resolvedModel := *modelCfg
	resolvedModel.APIKey = a.resolveAPIKey(modelCfg.APIKey)

	cacheDir := filepath.Join(a.settings.DataDir(), "cache")
	kvStore, _ := core.NewFileSystemKVStore(cacheDir)

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(cfg),
		goreact.WithModel(&resolvedModel),
		goreact.WithLogger(a.logger),
	}

	if kvStore != nil {
		opts = append(opts, goreact.WithKVStore(kvStore))
	}

	if a.mindxConfig != nil && a.mindxConfig.LastSessionID != "" {
		opts = append(opts, goreact.WithSession(a.mindxConfig.LastSessionID))
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
			opts = append(opts, goreact.WithSessionDir(
				filepath.Join(a.currentSessionMeta.GetSessionDir(), name)))
		}
	}

	if a.permissionRuleStore != nil {
		opts = append(opts, goreact.WithPermissionRuleStore(a.permissionRuleStore))
	}

	// Dual memory: LongTerm (project knowledge) + SessionRAG (conversation recall)
	if a.embedder != nil {
		// LongTerm — shared unified knowledge base (indexed by Daemon memoryWatch)
		ltMem, ltErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: core.MemoryTypeLongTerm,
			AgentName:  "_shared",
			MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:   a.embedder,
		})
		if ltErr != nil {
			a.logger.Warn("Failed to create long-term memory for agent %q: %v", cfg.Name, ltErr)
		} else {
			opts = append(opts, goreact.WithMemory(ltMem))
		}

		// SessionRAG — slid-context recall (ephemeral, bound to SessionDir)
		var sessionDir string
		if a.currentSessionMeta != nil {
			sessionDir = filepath.Join(a.currentSessionMeta.GetSessionDir(), name)
		}
		sMem, sErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: core.MemoryTypeSession,
			AgentName:  cfg.Name,
			SessionDir: sessionDir,
			Embedder:   a.embedder,
		})
		if sErr != nil {
			a.logger.Warn("Failed to create session memory for agent %q: %v", cfg.Name, sErr)
		} else {
			opts = append(opts, goreact.WithSessionMemory(sMem))
		}
	}

	opts = append(opts, goreact.WithProviderRegistry(a.models.ProviderRegistry()))
	agent, err := goreact.NewAgent(opts...)
	if err != nil {
		return nil, err
	}

	if a.currentSessionMeta != nil && a.currentSessionMeta.SessionID != "" {
		agent.NewSession(a.currentSessionMeta.SessionID)
	}

	a.agentMu.Lock()
	a.agentCache[name] = agent
	a.agentMu.Unlock()
	return agent, nil
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

- **Know who handles it** → call **SubAgent** tool directly (agent_name + task), then **CollectResults**
- **Don't know who** → load **find-experts** skill first (discovers experts, then delegates via same workflow)`
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

// sameDirectory checks if two paths refer to the same directory (handles path normalization).
func sameDirectory(dir1, dir2 string) bool {
	abs1, err1 := filepath.Abs(dir1)
	abs2, err2 := filepath.Abs(dir2)
	if err1 != nil || err2 != nil {
		return dir1 == dir2
	}
	return abs1 == abs2
}

// findSessionByProjectDir searches for the most recent session that matches the given project directory.
func (a *App) findSessionByProjectDir(projectDir string) *core.SessionInfo {
	ctx := context.Background()
	sessions, err := a.SessDB().ListSessions(ctx)
	if err != nil {
		return nil
	}

	var bestMatch *core.SessionInfo
	for i := range sessions {
		if sameDirectory(sessions[i].GetProjectDir(), projectDir) {
			if bestMatch == nil || sessions[i].LastActivityAt.After(bestMatch.LastActivityAt) {
				bestMatch = &sessions[i]
			}
		}
	}
	return bestMatch
}
