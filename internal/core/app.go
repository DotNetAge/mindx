package core

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/goharness/constants"
	"github.com/DotNetAge/goharness/rule"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/skill"
	"github.com/DotNetAge/goharness/store"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	mindxtools "github.com/DotNetAge/mindx/internal/tools"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/rules"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
)

type App struct {
	settings    *Settings
	mindxConfig *MindxConfig
	credStore   CredentialStore
	logger      logging.Logger

	// Registries (shared across all agents)
	agents      *config.AgentRegistry
	models      *config.ModelRegistry
	providerReg config.ProviderRegistry
	costs       *CostRegistry
	versions    *FileVersionStore
	rules       rule.RuleRegistry
	sessDB      *mindxses.FileSessionStore

	// Loaded provider configs (for RPC queries)
	providerConfigs []*config.ProviderConfig

	// Skill registry (loaded from skills directory)
	skillReg skill.SkillRegistry

	// Permission rules
	permissionRuleStore *MindxPermissionRuleStore

	// Optional components
	embedder goragcore.Embedder

	// Knowledge graph indexer (injected by Daemon after initialization)
	graphIndexer *goragindexer.GraphIndexer

	// Embedded app icon filesystem (for favicon / .app bundle)
	iconFS fs.FS

	// Runtime cache (keyed by agent name)
	runtimeCache map[string]*agents.Runtime
	runtimeMu    sync.RWMutex

	// Current session tracking
	currentSessionMeta *session.SessionInfo

	currentMu sync.Mutex

	// TokenUsageStore for persistent LLM token usage records
	tokenUsageStore *mindxses.FileTokenUsageStore

	// skillsPromptOverride, if set, overrides the default skills catalog prompt
	// section in the agent system prompt. Set via SetSkillsPromptOverride().
	skillsPromptOverride func(skills []*skill.Skill) string

	// envsOverride, if set, overrides the default Environment section in system
	// prompts. Set via SetEnvsOverride().
	envsOverride func(params agents.EnvsParams) string
}

func DefaultApp(mindxConfig *MindxConfig) (*App, error) {
	settings := &Settings{}

	logDir := settings.LogsDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile := filepath.Join(logDir, "mindx.log")
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   logFile,
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})

	var err error
	err = godotenv.Load()
	if err != nil {
		logger.Warn("WARNING: failed to load .env file", "error", err)
	}

	constants.SYSTEM_INFO_NAME = "MindX"
	constants.SYSTEM_INFO_VERSION = "2.0.0"

	userPrompt := "\n- User preferences directory: " + settings.UserPreferences()
	userPrompt += "\n- Skills directory: " + settings.SkillsDir()
	userPrompt += "\n- Agents directory: " + settings.AgentsDir()
	userPrompt += "\n- Python virtual environment: " + settings.VenvDir()
	userPrompt += "\n- Now: " + time.Now().Format(time.DateTime)
	constants.SYSTEM_INFO_USERS = userPrompt
	constants.SYSTEM_ADDON_SECTIONS = []string{
		BuildDelegationGuidance(),
	}

	logger.Info("loading agents", "dir", settings.AgentsDir())
	agentsReg, err := config.LoadAgentsFrom(settings.AgentsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}

	logger.Info("Loading models", "dir", settings.ModelsFile())
	models, err := config.LoadModels(settings.ModelsFile())
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

	logger.Info("Loading rules", "file", settings.DataRulesFile())
	rulesReg, err := rules.NewFileRuleRegistry(settings.DataRulesFile())
	if err != nil {
		logger.Warn("Failed to load rules", "file", settings.DataRulesFile(), "error", err)
	}

	logger.Info("Loading skills", "dir", settings.SkillsDir())
	skillReg, err := skill.NewSkillRegistryFromDirectory(settings.SkillsDir())
	if err != nil {
		logger.Warn("Failed to load skills", "dir", settings.SkillsDir(), "error", err)
	}

	logger.Info("Loading sessions", "dir", settings.SessionsDir())
	sessDB, err := mindxses.NewFileSessionStore(settings.SessionsDir())
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
			logger.Warn("Failed to create embedder, memory disabled", "error", embErr, "model", modelPath)
		}
	}

	// Create permission rule store (nil-safe: if mindxConfig is nil, returns no-op store)
	permStore := NewMindxPermissionRuleStore(mindxConfig)

	return &App{
		settings:            settings,
		mindxConfig:         mindxConfig,
		credStore:           credStore,
		logger:              logger,
		agents:              agentsReg,
		models:              models,
		providerReg:         models.ProviderRegistry(),
		costs:               costs,
		versions:            versions,
		rules:               rulesReg,
		skillReg:            skillReg,
		sessDB:              sessDB,
		runtimeCache:        make(map[string]*agents.Runtime),
		embedder:            emb,
		permissionRuleStore: permStore,
		tokenUsageStore:     mindxses.NewFileTokenUsageStore(settings.DataDir()),
		providerConfigs:     providers,
	}, nil
}

func resolveCurrentAgentName(cfg *MindxConfig, agents *config.AgentRegistry, logger logging.Logger) string {
	if agents == nil {
		return ""
	}

	if cfg != nil && cfg.LastAgent != "" {
		if agents.Get(cfg.LastAgent) != nil {
			return cfg.LastAgent
		}
		logger.Warn("last_agent not found in registry, will use fallback", "agent", cfg.LastAgent)
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

func (a *App) Embedder() goragcore.Embedder {
	return a.embedder
}

// SetGraphIndexer injects the knowledge graph indexer for LocalSearch tool.
func (a *App) SetGraphIndexer(gi *goragindexer.GraphIndexer) {
	a.graphIndexer = gi
}

// IconFS returns the embedded filesystem containing the app icon, or nil if not set.
func (a *App) IconFS() fs.FS {
	return a.iconFS
}

// SetIconFS sets the embedded app icon filesystem.
func (a *App) SetIconFS(fs fs.FS) {
	a.iconFS = fs
}

const defaultDaemonAddr = ":1314"

func (a *App) isDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", "localhost"+defaultDaemonAddr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	if a.logger != nil {
		a.logger.Info("daemon detected, opening LongTerm memory in read-only mode")
	}
	return true
}

func (a *App) Config() *MindxConfig {
	return a.mindxConfig
}

// ResolveDefaultModel 返回解析后的默认模型配置，包含从 Provider 继承的参数
// 和从 CredentialStore 解析的 API 密钥。优先使用 DefaultModel，为空时 fallback 到 LastModel。
func (a *App) ResolveDefaultModel() *config.ModelConfig {
	if a.mindxConfig == nil {
		return nil
	}
	modelName := a.mindxConfig.DefaultModel
	if modelName == "" {
		modelName = a.mindxConfig.LastModel
	}
	if modelName == "" {
		return nil
	}
	modelCfg := a.Models().Get(modelName)
	if modelCfg == nil {
		return nil
	}
	resolved := modelCfg.ResolveProvider(a.providerReg)
	if resolved.Provider != "" {
		if key, err := a.credStore.Get(resolved.Provider); err == nil && key != "" {
			resolved.APIKey = key
		}
	}
	if resolved.APIKey == "" {
		resolved.APIKey = ResolveAPIKey(a.credStore, resolved.APIKey)
	}
	return resolved
}

func (a *App) CurrentAgentName() string {
	return resolveCurrentAgentName(a.mindxConfig, a.agents, a.logger)
}

func (a *App) RuleRegistry() rule.RuleRegistry {
	return a.rules
}

func (a *App) SessionDB() *mindxses.FileSessionStore {
	return a.sessDB
}

func (a *App) SkillRegistry() skill.SkillRegistry {
	return a.skillReg
}

func (a *App) SetTestDir(tmpDir string) error {
	a.settings.Test = true
	a.settings.testDir = tmpDir
	sessDB, err := mindxses.NewFileSessionStore(filepath.Join(tmpDir, "sessions"))
	if err != nil {
		return err
	}
	a.sessDB = sessDB
	return nil
}

func (a *App) Agents() *config.AgentRegistry {
	return a.agents
}

func (a *App) SetAgentsRegistry(registry *config.AgentRegistry) {
	a.agents = registry
}

// SetSkillsPromptOverride sets an optional function to override the default
// skills catalog prompt section in the agent system prompt.
// When set, it is applied via agents.WithSkillsPrompt in createRuntime.
func (a *App) SetSkillsPromptOverride(fn func(skills []*skill.Skill) string) {
	a.skillsPromptOverride = fn
}

// SetEnvsOverride sets an optional function to override the default
// Environment section in the agent system prompt.
// When set, it is applied via agents.WithEnvs in createRuntime.
func (a *App) SetEnvsOverride(fn func(params agents.EnvsParams) string) {
	a.envsOverride = fn
}

// ReloadAgents re-scans the agents directory and atomically swaps the in-memory registry.
// All cached runtimes for affected agents are invalidated so they pick up the new config
// on next ResolveRuntime() call.
func (a *App) ReloadAgents() error {
	newReg, err := config.LoadAgentsFrom(a.settings.AgentsDir())
	if err != nil {
		return fmt.Errorf("reload agents: %w", err)
	}
	a.agents = newReg

	// Invalidate runtime caches — stale runtimes hold old agent configs + skill refs
	a.runtimeMu.Lock()
	a.runtimeCache = make(map[string]*agents.Runtime)
	a.runtimeMu.Unlock()

	a.logger.Info("agents reloaded", "dir", a.settings.AgentsDir())
	return nil
}

// ReloadSkills re-scans the skills directory and atomically swaps the in-memory registry.
func (a *App) ReloadSkills() error {
	newReg, err := skill.NewSkillRegistryFromDirectory(a.settings.SkillsDir())
	if err != nil {
		return fmt.Errorf("reload skills: %w", err)
	}
	a.skillReg = newReg

	// Invalidate runtime caches — runtimes hold references to the old skill registry
	a.runtimeMu.Lock()
	a.runtimeCache = make(map[string]*agents.Runtime)
	a.runtimeMu.Unlock()

	a.logger.Info("skills reloaded", "dir", a.settings.SkillsDir())
	return nil
}

func (a *App) Models() *config.ModelRegistry {
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

func (a *App) CurrentSessionMeta() *session.SessionInfo {
	return a.currentSessionMeta
}

func (a *App) SetCurrentSessionMeta(meta *session.SessionInfo) {
	a.currentSessionMeta = meta
}

func (a *App) SessDB() *mindxses.FileSessionStore {
	return a.sessDB
}

func (a *App) TokenUsageStore() *mindxses.FileTokenUsageStore {
	return a.tokenUsageStore
}

func (a *App) ProviderConfigs() []*config.ProviderConfig {
	return a.providerConfigs
}

// CreateSession creates a new session with metadata including the captured project directory (os.Getwd() at invocation time).
func (a *App) CreateSession(agentName, projectDir string) (*session.SessionInfo, error) {
	var opts []session.SessionOption
	if projectDir != "" {
		opts = append(opts, session.WithProjectDirOption(projectDir))
	}

	sessionInfo, err := a.sessDB.Create(context.Background(), agentName, opts...)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	a.currentSessionMeta = sessionInfo

	a.logger.Info("session created",
		"session_id", sessionInfo.SessionID,
		"agent", agentName,
		"project_dir", sessionInfo.ProjectDir,
		"session_dir", sessionInfo.SessionDir,
	)

	return sessionInfo, nil
}

// resolveModelName resolves the model config for a given agent model name.
func (a *App) resolveModelName(agentModelName string) (string, *config.ModelConfig, error) {
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

// createRuntime builds an agents.Runtime for the given agent name with all registries and services.
func (a *App) createRuntime(agentName string) (*agents.Runtime, error) {
	a.logger.Info("createRuntime: start", "agent", agentName)

	agent := a.Agents().Get(agentName)
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", agentName)
	}

	_, modelCfg, err := a.resolveModelName(agent.Model)
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", agent.Name, err)
	}
	resolvedModel := *modelCfg

	// 规则5: 优先以 model.provider 为键从 CredentialStore 中读取 APIKey。
	// 这是 APIKey 的主要来源（TUI/Daemon/WebUI 均以此键存储）。
	if resolvedModel.Provider != "" {
		if key, err := a.credStore.Get(resolvedModel.Provider); err == nil && key != "" {
			resolvedModel.APIKey = key
		} else {
			resolvedModel.APIKey = a.resolveAPIKey(resolvedModel.APIKey)
		}
	} else {
		resolvedModel.APIKey = a.resolveAPIKey(resolvedModel.APIKey)
	}

	// 单会话最大思考/交互轮次：覆盖 ModelConfig 中可能存在的 max_turns，
	// 引擎在 [goharness/agents/runtime.go] 中以 <=0 兜底为 20，这里显式抬到 100。
	if resolvedModel.MaxTurns <= 0 || resolvedModel.MaxTurns < 100 {
		resolvedModel.MaxTurns = 100
	}

	a.logger.Info("createRuntime: model resolved", "agent", agentName, "model", resolvedModel.Name, "max_turns", resolvedModel.MaxTurns)

	cacheDir := filepath.Join(a.settings.DataDir(), "cache")
	kvStore, kvErr := store.NewFileSystemKVStore(cacheDir)
	if kvErr != nil {
		a.logger.Warn("createRuntime: failed to init KVStore, task tools will be unavailable", "agent", agentName, "error", kvErr)
	} else {
		a.logger.Info("createRuntime: KVStore ready", "agent", agentName, "dir", cacheDir)
	}
	resultStore := store.NewResultStore()

	opts := []agents.RuntimeConfig{
		agents.WithModel(resolvedModel),
		agents.WithAgentRegistry(a.agents),
		agents.WithProviderRegistry(a.providerReg),
		agents.WithRuleRegistry(a.rules),
		agents.WithLogger(a.logger),
		agents.WithTokenUsageStore(a.tokenUsageStore),
		agents.WithResultStore(resultStore),
	}

	if kvStore != nil {
		opts = append(opts, agents.WithKVStore(kvStore))
	}

	if a.skillReg != nil {
		opts = append(opts, agents.WithSkillRegistry(a.skillReg))
	}

	if a.skillsPromptOverride != nil {
		opts = append(opts, agents.WithSkillsPrompt(a.skillsPromptOverride))
	}

	if a.envsOverride != nil {
		opts = append(opts, agents.WithEnvs(a.envsOverride))
	}

	agentDiscoveryIntro := "Agent discovery: when you need to find or list available agents, run 'mindx agent list' (or 'mindx agent list --json' for structured output). The list shows agent names, roles, descriptions, and their skills. Use this to find the right agent for delegation via SubAgent."

	if a.permissionRuleStore != nil {
		rules, loadErr := a.permissionRuleStore.Load()
		if loadErr == nil && rules != nil {
			permReg := &rule.YAMLRuleRegistry{}
			for _, pr := range rules.AlwaysAllow {
				_ = permReg.Register(rule.Rule{
					ID:       "perm-allow-" + pr.ToolName,
					Intro:    "Always allow " + pr.Description,
					Scope:    rule.ScopeGlobal,
					Priority: 50,
					Enabled:  true,
				})
			}
			for _, pr := range rules.AlwaysDeny {
				_ = permReg.Register(rule.Rule{
					ID:       "perm-deny-" + pr.ToolName,
					Intro:    "Always deny " + pr.Description,
					Scope:    rule.ScopeGlobal,
					Priority: 50,
					Enabled:  true,
				})
			}
			for _, pr := range rules.AlwaysAsk {
				_ = permReg.Register(rule.Rule{
					ID:       "perm-ask-" + pr.ToolName,
					Intro:    "Ask before " + pr.Description,
					Scope:    rule.ScopeGlobal,
					Priority: 50,
					Enabled:  true,
				})
			}
			_ = permReg.Register(rule.Rule{
				ID:       "agent-discovery",
				Intro:    agentDiscoveryIntro,
				Scope:    rule.ScopeGlobal,
				Priority: 40,
				Enabled:  true,
			})
			opts = append(opts, agents.WithRuleRegistry(permReg))
		}
	} else {
		_ = a.rules.Register(rule.Rule{
			ID:       "agent-discovery",
			Intro:    agentDiscoveryIntro,
			Scope:    rule.ScopeGlobal,
			Priority: 40,
			Enabled:  true,
		})
	}
	// Dual memory: LongTerm (project knowledge) + SessionRAG (conversation recall)
	if a.embedder != nil {
		if a.isDaemonRunning() {
			// When Daemon is running, it manages the shared bbolt database with an
			// exclusive file lock (LOCK_EX). Opening the same .db file from the TUI
			// in read-only mode would block indefinitely trying to acquire a shared
			// lock (LOCK_SH), preventing messages from ever reaching the LLM.
			// The TUI delegates all memory operations to the Daemon via RPC instead.
			a.logger.Info("daemon detected: skipping local LongTerm memory init (delegated to daemon)")
		} else {
			a.logger.Info("createRuntime: creating shared memory", "agent", agentName)
			ltMem, ltErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
				AgentName: "_shared",
				MemoryDir: filepath.Join(a.settings.UserPreferences(), "memory"),
				Embedder:  a.embedder,
				Logger:    a.logger,
			})
			if ltErr != nil {
				a.logger.Warn("Failed to create long-term memory", "agent", agent.Name, "error", ltErr)
			} else {
				opts = append(opts, agents.WithMemory(ltMem))
				a.logger.Info("createRuntime: long-term memory OK", "agent", agentName)
			}
		}
	}

	a.logger.Info("createRuntime: calling agents.NewRuntime", "agent", agentName)
	rt := agents.NewRuntime(opts...)
	a.logger.Info("createRuntime: done", "agent", agentName)

	// Register LocalSearch if GraphIndexer is available
	if a.graphIndexer != nil {
		ls := mindxtools.NewLocalSearch(a.graphIndexer)
		if err := rt.RegisterTool(ls); err != nil {
			a.logger.Warn("createRuntime: failed to register LocalSearch", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: LocalSearch registered", "agent", agentName)
		}
	}

	return rt, nil
}

// CurrentRuntime returns the cached Runtime for the current agent, creating it if needed.
func (a *App) CurrentRuntime() (*agents.Runtime, error) {
	a.currentMu.Lock()
	defer a.currentMu.Unlock()

	agentName := a.CurrentAgentName()
	if agentName == "" {
		return nil, fmt.Errorf("no agent available")
	}

	return a.ResolveRuntime(agentName)
}

// ResolveRuntime returns (or creates and caches) a Runtime for the given agent name.
func (a *App) ResolveRuntime(name string) (*agents.Runtime, error) {
	if name == "" {
		return a.CurrentRuntime()
	}

	a.runtimeMu.RLock()
	if cached, ok := a.runtimeCache[name]; ok {
		a.runtimeMu.RUnlock()
		return cached, nil
	}
	a.runtimeMu.RUnlock()

	rt, err := a.createRuntime(name)
	if err != nil {
		return nil, err
	}

	a.runtimeMu.Lock()
	a.runtimeCache[name] = rt
	a.runtimeMu.Unlock()
	return rt, nil
}

// EnsureSession ensures a valid session exists for the current agent and returns its ID.
// This handles smart session matching (CWD changes) and auto-creates sessions.
func (a *App) EnsureSession() (string, error) {
	if a.sessDB == nil {
		return "", fmt.Errorf("EnsureSession called but sessDB is nil")
	}
	if a.mindxConfig == nil {
		return "", fmt.Errorf("EnsureSession called but mindxConfig is nil")
	}

	agentName := a.CurrentAgentName()
	if agentName == "" {
		return "", fmt.Errorf("EnsureSession called but no agent available")
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", fmt.Errorf("os.Getwd failed: %w", cwdErr)
	}

	// If we have a current session meta, check if CWD matches
	if a.currentSessionMeta != nil && a.currentSessionMeta.ProjectDir != "" {
		if sameDirectory(cwd, a.currentSessionMeta.ProjectDir) {
			return a.currentSessionMeta.SessionID, nil
		}

		// CWD changed — find or create a matching session
		a.logger.Warn("working directory changed",
			"old_project_dir", a.currentSessionMeta.ProjectDir,
			"new_cwd", cwd,
		)

		if matched := a.findSessionByProjectDir(cwd, agentName); matched != nil {
			a.logger.Info("found matching session for current directory",
				"session_id", matched.SessionID,
				"project_dir", matched.ProjectDir,
				"agent", agentName,
			)
			a.currentSessionMeta = matched
			a.mindxConfig.LastSessionID = matched.SessionID
			if saveErr := a.mindxConfig.Save(); saveErr != nil {
				a.logger.Warn("failed to save config after session match", "error", saveErr)
			}
			return matched.SessionID, nil
		}

		a.logger.Info("no matching session found, creating new session for current directory",
			"cwd", cwd,
			"agent", agentName,
		)
		newSession, createErr := a.CreateSession(agentName, cwd)
		if createErr != nil {
			a.logger.Error("failed to create new session", createErr)
			a.currentSessionMeta = nil
			a.mindxConfig.LastSessionID = ""
			return "", createErr
		}
		a.currentSessionMeta = newSession
		a.mindxConfig.LastSessionID = newSession.SessionID
		if saveErr := a.mindxConfig.Save(); saveErr != nil {
			a.logger.Warn("failed to save config after session create (cwd changed)", "error", saveErr)
		}
		a.logger.Info("new session created",
			"session_id", newSession.SessionID,
			"project_dir", newSession.ProjectDir,
		)
		return newSession.SessionID, nil
	}

	// No current session meta — try to find existing or create new
	a.logger.Info("no current session, searching for existing", "cwd", cwd)

	if matched := a.findSessionByProjectDir(cwd, agentName); matched != nil {
		a.logger.Info("found matching session for current directory",
			"session_id", matched.SessionID,
			"project_dir", matched.ProjectDir,
			"agent", agentName,
		)
		a.currentSessionMeta = matched
		a.mindxConfig.LastSessionID = matched.SessionID
		if saveErr := a.mindxConfig.Save(); saveErr != nil {
			a.logger.Warn("failed to save config after session match (no current meta)", "error", saveErr)
		}
		return matched.SessionID, nil
	}

	a.logger.Info("no existing session found, creating new session", "cwd", cwd)
	newSession, createErr := a.CreateSession(agentName, cwd)
	if createErr != nil {
		return "", fmt.Errorf("CreateSession failed for agent=%q cwd=%q: %w", agentName, cwd, createErr)
	}
	a.currentSessionMeta = newSession
	a.mindxConfig.LastSessionID = newSession.SessionID
	if saveErr := a.mindxConfig.Save(); saveErr != nil {
		a.logger.Warn("failed to save config after new session creation", "error", saveErr)
	}
	return newSession.SessionID, nil
}

// NewSessionFromMeta creates a goharness session.Session from the current session metadata.
// The session uses lazy-loading: historical messages are automatically loaded
// from the persistent store on first access (Current() or Append()), so there's
// no need for an explicit Restore() call here.
//
// Dual-Store Architecture:
//   - SessionStore (sessDB): Persists raw messages to disk for history recovery
//   - MemoryStore: Stores compaction summaries for semantic recall via MemoryThoughtHook
//   - When external RAG is available (embedder configured): summaries → RAG (priority path)
//   - When no external RAG: summaries → in-memory fallback (lost on exit)
func (a *App) NewSessionFromMeta() *session.Session {
	if a.currentSessionMeta == nil {
		agentName := a.CurrentAgentName()
		if agentName == "" {
			return nil
		}
		_, err := a.EnsureSession()
		if err != nil || a.currentSessionMeta == nil {
			return nil
		}
	}

	agentName := a.CurrentAgentName()
	var opts []session.SessionConfig

	if a.embedder != nil {
		sessRAG, ragErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			AgentName: agentName,
			MemoryDir: filepath.Join(a.settings.UserPreferences(), "memory"),
			Embedder:  a.embedder,
			Logger:    a.logger,
		})
		if ragErr != nil {
			a.logger.Warn("failed to create session RAG memory, compaction summaries will use in-memory fallback", "error", ragErr)
		} else {
			opts = append(opts, session.WithMemory(mindxses.NewRAGMemoryAdapter(sessRAG)))

			agent := a.Agents().Get(agentName)
			if agent != nil {
				model := a.Models().Get(agent.Model)
				if model != nil && model.Enabled {
					opts = append(opts, session.WithSummarizer(session.NewLLMSummarizer(*model)))
				}
			}
		}
	}

	s, err := session.Load(a.currentSessionMeta.SessionID, agentName, a.sessDB, opts...)
	if err != nil {
		a.logger.Error("failed to load session from store", err, "session_id", a.currentSessionMeta.SessionID)
		return nil
	}
	return s
}

func (a *App) IsModelAvailable(name ...string) bool {
	n := ""
	if len(name) == 0 {
		agentName := a.CurrentAgentName()
		agent := a.Agents().Get(agentName)
		if agent == nil {
			return false
		}
		mc := a.Models().Get(agent.Model)
		if mc == nil {
			return false
		}
		n = mc.Name
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

func BuildDelegationGuidance() string {
	return `## Execution
Pick one path:

- **Within your remit, multiple steps** → decompose with task tools
- **Outside your remit, single expert** → delegate to the right expert
- **Cross-domain collaboration** → form a team and delegate to an expert panel`
}

func (a *App) SwitchSession(sessionID string) (*session.SessionInfo, error) {
	ctx := context.Background()
	sessions, err := a.SessDB().ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var target *session.SessionInfo
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
		if saveErr := a.mindxConfig.Save(); saveErr != nil {
			a.logger.Warn("failed to save config after session switch", "error", saveErr)
		}
	}

	a.logger.Info("session switched",
		"session_id", sessionID,
	)

	return target, nil
}

func (a *App) ClearCurrentSession() (*session.SessionInfo, error) {
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

	// Use the old session's project_dir if available; otherwise fall back to CWD.
	projectDir := ""
	if currentMeta != nil && currentMeta.ProjectDir != "" {
		projectDir = currentMeta.ProjectDir
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd failed and no previous project_dir: %w", err)
		}
		projectDir = cwd
	}
	newSession, err := a.CreateSession(a.CurrentAgentName(), projectDir)
	if err != nil {
		return nil, fmt.Errorf("create new session failed: %w", err)
	}

	a.logger.Info("session cleared and new one created",
		"old_session_id", oldSessionID,
		"new_session_id", newSession.SessionID,
	)

	return newSession, nil
}

func sameDirectory(dir1, dir2 string) bool {
	abs1, err1 := filepath.Abs(dir1)
	abs2, err2 := filepath.Abs(dir2)
	if err1 != nil || err2 != nil {
		return dir1 == dir2
	}
	return abs1 == abs2
}

func (a *App) findSessionByProjectDir(projectDir, agentName string) *session.SessionInfo {
	ctx := context.Background()
	sessions, err := a.SessDB().ListSessions(ctx)
	if err != nil {
		return nil
	}

	var bestMatch *session.SessionInfo
	for i := range sessions {
		if sessions[i].AgentName == agentName && sameDirectory(sessions[i].ProjectDir, projectDir) {
			if bestMatch == nil || sessions[i].LastActivityAt.After(bestMatch.LastActivityAt) {
				bestMatch = &sessions[i]
			}
		}
	}
	return bestMatch
}
