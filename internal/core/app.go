package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	goragcore "github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/goreact/agents"
	"github.com/DotNetAge/goreact/config"
	"github.com/DotNetAge/goreact/constants"
	goreactmemory "github.com/DotNetAge/goreact/memory"
	"github.com/DotNetAge/goreact/rule"
	"github.com/DotNetAge/goreact/session"
	"github.com/DotNetAge/goreact/skill"
	"github.com/DotNetAge/goreact/store"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
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

	// Skill registry (loaded from skills directory)
	skillReg skill.SkillRegistry

	// Permission rules
	permissionRuleStore *MindxPermissionRuleStore

	// Optional components
	embedder goragcore.Embedder

	// Runtime cache (keyed by agent name)
	runtimeCache map[string]*agents.Runtime
	runtimeMu    sync.RWMutex

	// Current session tracking
	currentSessionMeta *session.SessionInfo

	currentMu sync.Mutex
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

	logger.Info("Loading rules", "file", settings.RulesFile())
	rulesReg, err := rule.NewYAMLRuleRegistry(settings.RulesFile())
	if err != nil {
		logger.Warn("Failed to load rules", "file", settings.RulesFile(), "error", err)
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
	}, nil
}

func resolveCurrentAgentName(cfg *MindxConfig, agents *config.AgentRegistry, logger logging.Logger) string {
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

func (a *App) Embedder() goragcore.Embedder {
	return a.embedder
}

const defaultDaemonAddr = ":1314"

func (a *App) isDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", "localhost"+defaultDaemonAddr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	if a.logger != nil {
		a.logger.Info("daemon detected, opening LongTerm memory in read-only mode")
	}
	return true
}

func (a *App) Config() *MindxConfig {
	return a.mindxConfig
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

// CreateSession creates a new session with metadata including the captured project directory (os.Getwd() at invocation time).
func (a *App) CreateSession(agentName string) (*session.SessionInfo, error) {
	sessionInfo, err := a.sessDB.Create(context.Background(), agentName)
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
	resolvedModel.APIKey = a.resolveAPIKey(resolvedModel.APIKey)
	a.logger.Info("createRuntime: model resolved", "agent", agentName, "model", resolvedModel.Name)

	cacheDir := filepath.Join(a.settings.DataDir(), "cache")
	kvStore, _ := store.NewFileSystemKVStore(cacheDir)

	opts := []agents.RuntimeConfig{
		agents.WithModel(resolvedModel),
		agents.WithAgentRegistry(a.agents),
		agents.WithProviderRegistry(a.providerReg),
		agents.WithRuleRegistry(a.rules),
		agents.WithLogger(a.logger),
	}

	if a.skillReg != nil {
		opts = append(opts, agents.WithSkillRegistry(a.skillReg))
	}

	if kvStore != nil {
		_ = kvStore
	}

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
			opts = append(opts, agents.WithRuleRegistry(permReg))
		}
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
			a.logger.Info("createRuntime: creating long-term memory", "agent", agentName)
			ltMem, ltErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
				MemoryType: goreactmemory.MemoryTypeLongTerm,
				AgentName:  "_shared",
				MemoryDir:  filepath.Join(a.settings.UserPreferences(), "memory"),
				Embedder:   a.embedder,
			})
			if ltErr != nil {
				a.logger.Warn("Failed to create long-term memory for agent %q: %v", agent.Name, ltErr)
			} else {
				opts = append(opts, agents.WithMemory(ltMem))
				a.logger.Info("createRuntime: long-term memory OK", "agent", agentName)
			}
		}
	}

	a.logger.Info("createRuntime: calling agents.NewRuntime", "agent", agentName)
	rt := agents.NewRuntime(opts...)
	a.logger.Info("createRuntime: done", "agent", agentName)
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
func (a *App) EnsureSession() string {
	if a.sessDB == nil {
		panic("FATAL: EnsureSession called but sessDB is nil")
	}
	if a.mindxConfig == nil {
		panic("FATAL: EnsureSession called but mindxConfig is nil")
	}

	agentName := a.CurrentAgentName()
	if agentName == "" {
		panic("FATAL: EnsureSession called but no agent available")
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		panic(fmt.Sprintf("FATAL: os.Getwd failed: %v", cwdErr))
	}

	// If we have a current session meta, check if CWD matches
	if a.currentSessionMeta != nil && a.currentSessionMeta.ProjectDir != "" {
		if sameDirectory(cwd, a.currentSessionMeta.ProjectDir) {
			return a.currentSessionMeta.SessionID
		}

		// CWD changed — find or create a matching session
		a.logger.Warn("working directory changed",
			"old_project_dir", a.currentSessionMeta.ProjectDir,
			"new_cwd", cwd,
		)

		if matched := a.findSessionByProjectDir(cwd); matched != nil {
			a.logger.Info("found matching session for current directory",
				"session_id", matched.SessionID,
				"project_dir", matched.ProjectDir,
			)
			a.currentSessionMeta = matched
			a.mindxConfig.LastSessionID = matched.SessionID
			_ = a.mindxConfig.Save()
			return matched.SessionID
		}

		a.logger.Info("no matching session found, creating new session for current directory",
			"cwd", cwd,
		)
		newSession, createErr := a.CreateSession(agentName)
		if createErr != nil {
			a.logger.Error("failed to create new session", createErr)
			a.currentSessionMeta = nil
			a.mindxConfig.LastSessionID = ""
			return ""
		}
		a.currentSessionMeta = newSession
		a.mindxConfig.LastSessionID = newSession.SessionID
		_ = a.mindxConfig.Save()
		a.logger.Info("new session created",
			"session_id", newSession.SessionID,
			"project_dir", newSession.ProjectDir,
		)
		return newSession.SessionID
	}

	// No current session meta — try to find existing or create new
	a.logger.Info("no current session, searching for existing", "cwd", cwd)

	if matched := a.findSessionByProjectDir(cwd); matched != nil {
		a.logger.Info("found matching session for current directory",
			"session_id", matched.SessionID,
			"project_dir", matched.ProjectDir,
		)
		a.currentSessionMeta = matched
		a.mindxConfig.LastSessionID = matched.SessionID
		_ = a.mindxConfig.Save()
		return matched.SessionID
	}

	a.logger.Info("no existing session found, creating new session", "cwd", cwd)
	newSession, createErr := a.CreateSession(agentName)
	if createErr != nil {
		panic(fmt.Sprintf("FATAL: CreateSession failed for agent=%q cwd=%q: %v", agentName, cwd, createErr))
	}
	a.currentSessionMeta = newSession
	a.mindxConfig.LastSessionID = newSession.SessionID
	_ = a.mindxConfig.Save()
	return newSession.SessionID
}

// NewSessionFromMeta creates a goreact session.Session from the current session metadata.
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
		_ = a.EnsureSession()
		if a.currentSessionMeta == nil {
			return nil
		}
	}

	agentName := a.CurrentAgentName()
	opts := []session.SessionConfig{
		session.WithStore(a.sessDB),
	}

	if a.embedder != nil {
		sessionDir := a.currentSessionMeta.SessionDir
		if sessionDir == "" {
			sessionDir, _ = a.sessDB.ResolveSessionDir(a.currentSessionMeta.SessionID)
		}
		sessRAG, ragErr := memory.NewRAGMemoryFromConfig(memory.MemoryConfig{
			MemoryType: goreactmemory.MemoryTypeSession,
			AgentName:  agentName,
			SessionDir: sessionDir,
			Embedder:   a.embedder,
		})
		if ragErr != nil {
			a.logger.Warn("failed to create session RAG memory, compaction summaries will use in-memory fallback", "error", ragErr)
		} else {
			opts = append(opts, session.WithMemory(mindxses.NewRAGMemoryAdapter(sessRAG)))

			agent := a.Agents().Get(agentName)
			if agent != nil {
				model := a.Models().Get(agent.Model)
				if model != nil && model.Enabled {
					opts = append(opts, session.WithSummarizer(mindxses.NewLLMSummarizer(*model)))
				}
			}
		}
	}

	s := session.NewSession(a.currentSessionMeta.SessionID, agentName, opts...)
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
	return `## Delegation
When a task is outside your expertise, choose one path:

- **Know who handles it** → call **SubAgent** tool directly (agent_name + task), then **CollectResults**
- **Don't know who** → load **find-experts** skill first (discovers experts, then delegates via same workflow)`
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
		_ = a.mindxConfig.Save()
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

func sameDirectory(dir1, dir2 string) bool {
	abs1, err1 := filepath.Abs(dir1)
	abs2, err2 := filepath.Abs(dir2)
	if err1 != nil || err2 != nil {
		return dir1 == dir2
	}
	return abs1 == abs2
}

func (a *App) findSessionByProjectDir(projectDir string) *session.SessionInfo {
	ctx := context.Background()
	sessions, err := a.SessDB().ListSessions(ctx)
	if err != nil {
		return nil
	}

	var bestMatch *session.SessionInfo
	for i := range sessions {
		if sameDirectory(sessions[i].ProjectDir, projectDir) {
			if bestMatch == nil || sessions[i].LastActivityAt.After(bestMatch.LastActivityAt) {
				bestMatch = &sessions[i]
			}
		}
	}
	return bestMatch
}
