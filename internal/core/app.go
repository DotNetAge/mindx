package core

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gochat"
	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/config"
	goharnessmemory "github.com/DotNetAge/goharness/memory"
	"github.com/DotNetAge/goharness/rule"
	"github.com/DotNetAge/goharness/sandbox"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/skill"
	"github.com/DotNetAge/goharness/store"
	"github.com/DotNetAge/goharness/tools"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	mindxtools "github.com/DotNetAge/mindx/internal/tools"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/memory"
	"github.com/DotNetAge/mindx/pkg/rules"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
	"github.com/joho/godotenv"
)

// MCPToolProvider provides MCP tools for registration in goharness.
// The concrete implementation lives in internal/mcp and is injected by the Daemon.
type MCPToolProvider interface {
	EnabledTools() []tools.FuncTool
}

type App struct {
	settings    *Settings
	mindxConfig *MindxConfig
	credStore   CredentialStore
	logger      logging.Logger

	// Registries (shared across all agents)
	agents      *config.AgentRegistry
	models      *config.ModelRegistry
	providerReg config.ProviderRegistry
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

	// Long-term memory store (injected by Daemon; TUI mode creates locally)
	longTermMemory goharnessmemory.Memory

	// MCP manager (injected by Daemon after initialization)
	mcpMgr MCPToolProvider

	// Scheduler store (injected by Daemon after initialization)
	schedulerStore *scheduler.FileSchedulerStore

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

	// searchStrategyOverride, if set, overrides the default Search Strategy
	// section in system prompts. Set via SetSearchStrategyOverride().
	searchStrategyOverride func() string
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
		MaxSize:    20,
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

	versions := NewFileVersionStore()

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
		if last := agents.Get(cfg.LastAgent); last != nil {
			if AgentIsHired(last) {
				return cfg.LastAgent
			}
			logger.Warn("last_agent 未雇佣，回退到雇佣视图中的首个 Agent", "agent", cfg.LastAgent)
		} else {
			logger.Warn("last_agent not found in registry, will use fallback", "agent", cfg.LastAgent)
		}
	}

	// 回退基于雇佣视图：默认 Agent 必须是会话可用的已雇佣 Agent
	for _, hired := range HiredAgentsOf(agents) {
		logger.Info("using first hired agent as current", "name", hired.Name)
		return hired.Name
	}
	logger.Warn("雇佣视图中没有任何 Agent，将使用空默认值")

	return ""
}

func (a *App) Settings() *Settings {
	return a.settings
}

func (a *App) Embedder() goragcore.Embedder {
	return a.embedder
}

// SetLongTermMemory injects the long-term memory store for MemorySearch tool registration.
// Called by Daemon after shared memory is initialized; TUI mode sets it in createRuntime.
func (a *App) SetLongTermMemory(mem goharnessmemory.Memory) {
	a.longTermMemory = mem
}

// LongTermMemory returns the long-term memory store, or nil if not configured.
func (a *App) LongTermMemory() goharnessmemory.Memory {
	return a.longTermMemory
}

// SetMCPManager injects the MCP manager for MCP tool registration.
func (a *App) SetMCPManager(mgr MCPToolProvider) {
	a.mcpMgr = mgr
}

// SetSchedulerStore injects the scheduler store for Cron tool registration.
func (a *App) SetSchedulerStore(store *scheduler.FileSchedulerStore) {
	a.schedulerStore = store
}

// SchedulerStore returns the scheduler store, or nil if not set.
func (a *App) SchedulerStore() *scheduler.FileSchedulerStore {
	return a.schedulerStore
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

// KBAddr 返回配置的知识库（mrag/mindstore）服务地址。
// 返回空字符串表示未配置知识库，此时不装配知识库相关工具。
// 统一规范化：去掉尾部斜杠，缺少协议前缀时补 http://，保证工具拼接 /api/... 路径正确。
func (a *App) KBAddr() string {
	if a.mindxConfig == nil {
		return ""
	}
	return normalizeKBAddr(a.mindxConfig.KBAddr)
}

// normalizeKBAddr 规范化知识库服务地址：
//   - 去除首尾空白与尾部斜杠（避免拼接 /api/query 时出现 // 双斜杠）
//   - 缺少 http(s):// 协议前缀时补 http://
func normalizeKBAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimRight(addr, "/")
	if addr == "" {
		return ""
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return addr
}

// InvalidateRuntimes 清空运行时缓存。
// 知识库地址等工具装配参数变化后调用，使下一次 ResolveRuntime 按新配置重新装配工具。
func (a *App) InvalidateRuntimes() {
	a.runtimeMu.Lock()
	a.runtimeCache = make(map[string]*agents.Runtime)
	a.runtimeMu.Unlock()
	a.logger.Info("runtime cache invalidated（工具装配配置已变更）")
}

// ResolveDefaultModel 返回解析后的默认模型配置，包含从 Provider 继承的参数
// 和从 CredentialStore 解析的 API 密钥。优先使用 DefaultModel，为空时 fallback 到 LastModel。
func (a *App) ResolveDefaultModel() *config.ModelConfig {
	if a.mindxConfig == nil {
		return nil
	}
	modelName := a.mindxConfig.LastModel
	if modelName == "" {
		modelName = a.mindxConfig.DefaultModel
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
		// Ollama 为本地模型，不需要真实 API Key，直接填充占位值以避免 gochat 校验失败
		if strings.EqualFold(resolved.Provider, "ollama") {
			resolved.APIKey = "NONEKey"
			resolved.AuthToken = ""
		} else if key, err := a.credStore.Get(resolved.Provider); err == nil && key != "" {
			resolved.APIKey = key
		}
	}
	if resolved.APIKey == "" {
		resolved.APIKey = ResolveAPIKey(a.credStore, resolved.APIKey)
	}
	return resolved
}

// ModelContextLength 返回当前默认模型的上下文窗口大小，
// 作为 modelContextResolver 回调注入到 session，保证窗口大小动态查询当前模型。
//
// 每次调用都通过 ResolveDefaultModel() 读取最新的全局默认模型配置，
// 保证用户切换模型后窗口大小立即更新——窗口大小是模型能力的函数，
// 不是会话的固定属性。
func (a *App) ModelContextLength() int64 {
	m := a.ResolveDefaultModel()
	if m == nil {
		return 0
	}
	return m.ContextLength
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

// SetSearchStrategyOverride sets an optional function to override the default
// Search Strategy section in the agent system prompt.
// When set, it is applied via agents.WithSearchStrategy in createRuntime.
func (a *App) SetSearchStrategyOverride(fn func() string) {
	a.searchStrategyOverride = fn
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

// InvalidateRuntimeCache 清空已缓存的 Runtime 实例。
// 切换模型、修改 provider 凭证等会影响 Runtime 构建结果的场景必须调用，
// 否则后续请求会复用旧 Runtime（持有过期的模型配置与 LLMClient），
// 出现"切换后仍用旧模型调用"的问题。
func (a *App) InvalidateRuntimeCache() {
	a.runtimeMu.Lock()
	a.runtimeCache = make(map[string]*agents.Runtime)
	a.runtimeMu.Unlock()
	a.logger.Info("runtime cache invalidated")
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

func (a *App) SetProviderConfigs(providers []*config.ProviderConfig) {
	a.providerConfigs = providers
}

// CreateSession creates a new session with metadata including the captured project directory (os.Getwd() at invocation time).
func (a *App) CreateSession(agentName, projectDir string) (*session.SessionInfo, error) {
	var opts []session.SessionOption
	if projectDir != "" {
		opts = append(opts, session.WithProjectDirOption(projectDir))
	}

	sessionInfo, err := session.CreateSession(context.Background(), a.sessDB, agentName, opts...)
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
		// Ollama 为本地模型，不需要真实 API Key，直接填充占位值以避免 gochat 校验失败
		if strings.EqualFold(resolvedModel.Provider, "ollama") {
			resolvedModel.APIKey = "NONEKey"
			resolvedModel.AuthToken = ""
		} else if key, err := a.credStore.Get(resolvedModel.Provider); err == nil && key != "" {
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
	opts := []agents.RuntimeConfig{
		agents.WithModel(resolvedModel),
		agents.WithAgentRegistry(a.agents),
		agents.WithProviderRegistry(a.providerReg),
		agents.WithRuleRegistry(a.rules),
		agents.WithLogger(a.logger),
		agents.WithTokenUsageStore(a.tokenUsageStore),
	}

	if kvStore != nil {
		opts = append(opts, agents.WithKVStore(kvStore))
	}

	// SessionStore 用于 CollectResults 加载子 session 消息
	// a.sessDB (FileSessionStore) 是全局单例，始终可用
	if a.sessDB != nil {
		opts = append(opts, agents.WithSessionStore(a.sessDB))
		a.logger.Info("createRuntime: SessionStore ready", "agent", agentName)
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

	if a.searchStrategyOverride != nil {
		opts = append(opts, agents.WithSearchStrategy(a.searchStrategyOverride))
	}

	agentDiscoveryIntro := "Agent 发现：当需要查找或列出可用 Agent 时，运行 'mindx agent list'（或 'mindx agent list --json' 获取结构化输出）。列表显示 Agent 名称、角色、描述及其技能。用于查找合适的 Agent 并通过 SubAgent 进行委托。"

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
			// The TUI delegates all local memory creation to the Daemon via RPC instead.
			// Memory store itself is still needed for MemoryThoughtHook (取出最近记忆
			// 注入系统提示词)，因此用 daemon 已注入的 longTermMemory。
			if a.longTermMemory != nil {
				opts = append(opts, agents.WithMemory(a.longTermMemory))
				a.logger.Info("createRuntime: using daemon-provided long-term memory for MemoryThoughtHook", "agent", agentName)
			}
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
				a.SetLongTermMemory(ltMem)
				a.logger.Info("createRuntime: long-term memory OK", "agent", agentName)
			}
		}
	}

	// 构造会话级逻辑沙箱：统一文件、命令、URL 安全决策。
	// AllowedDirs 仅包含用户主目录（~/.mindx），projectDir 在运行时由 CheckFile/EnforceFile 传入，
	// isOutsideWorkspace 已修复为始终将 projectDir 视为允许目录。
	homeDir := a.settings.UserPreferences()
	sandboxPolicy := sandbox.SandboxPolicy{
		AllowedDirs:           []string{homeDir},
		DeniedFileGlobs:       sandbox.DefaultDeniedFileGlobs(),
		DeniedDirGlobs:        sandbox.DefaultDeniedDirGlobs(),
		DeniedDevicePaths:     sandbox.DefaultDeniedDevicePaths(),
		NetworkDenySubnets:    sandbox.DefaultDeniedSubnets(),
		AllowedCommands:       sandbox.DefaultAllowedCommands(),
		DeniedCommandPatterns: sandbox.DefaultDeniedCommandPatterns(),
		NetworkCommands:       sandbox.DefaultNetworkCommands(),
	}
	sb, sbErr := sandbox.NewSandbox(&sandboxPolicy, a.logger)
	if sbErr != nil {
		a.logger.Warn("createRuntime: 沙箱创建失败，回退到旧安全逻辑", "agent", agentName, "error", sbErr)
	} else {
		opts = append(opts, agents.WithSandbox(sb))
		a.logger.Info("createRuntime: 沙箱已启用", "agent", agentName, "home_dir", homeDir)
	}

	a.logger.Info("createRuntime: calling agents.NewRuntime", "agent", agentName)
	rt := agents.NewRuntime(opts...)
	a.logger.Info("createRuntime: done", "agent", agentName)

	// Configure RunScript tool: use the mindx-managed Python venv instead
	// of auto-creating per-skill virtual environments.
	if t, ok := rt.ToolRegistry().Get("RunScript"); ok {
		if rs, ok := t.(*tools.RunScript); ok {
			rs.SetPythonVenv(a.settings.VenvDir())
			a.logger.Info("createRuntime: RunScript venv configured",
				"agent", agentName, "venv", a.settings.VenvDir())
		}
	}

	// Register MemorySearch tool whenever long-term memory is available.
	// This gives the LLM a tool to actively recall past conversation summaries.
	if mem := a.LongTermMemory(); mem != nil {
		ms := tools.NewMemorySearch(mem)
		if ms != nil {
			if err := rt.RegisterTool(ms); err != nil {
				a.logger.Warn("createRuntime: 注册 MemorySearch 失败", "agent", agentName, "error", err)
			} else {
				a.logger.Info("createRuntime: MemorySearch 注册成功", "agent", agentName)
			}
		}
	}

	// Register MCP tools from MCPManager (injected by Daemon).
	// Each MCP tool is a separate FuncTool instance registered in goharness.
	if a.mcpMgr != nil {
		for _, tool := range a.mcpMgr.EnabledTools() {
			if err := rt.RegisterTool(tool); err != nil {
				a.logger.Warn("createRuntime: 注册 MCP工具 失败", "agent", agentName, "tool", tool.Info().Name, "error", err)
			} else {
				a.logger.Info("createRuntime: MCP工具 注册成功", "agent", agentName, "tool", tool.Info().Name)
			}
		}
	}

	// Register Cron tool whenever the scheduler store is available.
	if a.schedulerStore != nil {
		cronTool := mindxtools.NewCron(a.schedulerStore)
		if err := rt.RegisterTool(cronTool); err != nil {
			a.logger.Warn("createRuntime: 注册 Cron 失败", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: Cron 注册成功", "agent", agentName)
		}
	}

	// Register SendMessage tool (macOS only).
	if runtime.GOOS == "darwin" {
		msgTool := mindxtools.NewSendMessage()
		if err := rt.RegisterTool(msgTool); err != nil {
			a.logger.Warn("createRuntime: 注册 SendMessage 失败", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: SendMessage 注册成功", "agent", agentName)
		}
	}

	// 知识库工具装配：地址来自配置（kb_addr），未配置时不装配知识库相关工具
	//（LSPro/ReadPro/QuickSearch），保留 goharness 默认的 Ls/Read。
	kbAddr := a.KBAddr()
	if kbAddr == "" {
		a.logger.Info("createRuntime: 未配置知识库地址，跳过知识库相关工具装配", "agent", agentName)
		// 图片读取能力与知识库无关，仍需按模型视觉能力（Visioning）配置到默认 Read 上，
		// 避免未配置知识库时视觉模型无法读取图片。
		if t, ok := rt.ToolRegistry().Get("Read"); ok {
			if r, ok := t.(*tools.Read); ok {
				r.SetImageReading(resolvedModel.Visioning)
			}
		}
	} else {
		// 替换默认 Ls 工具为增强版 LSPro（知识库优先 + 原生回退）.
		_ = rt.ToolRegistry().Remove("Ls") // 先删除 goharness 默认的 Ls
		lsPro := mindxtools.NewLSPro(kbAddr)
		if err := rt.RegisterTool(lsPro); err != nil {
			a.logger.Warn("createRuntime: 注册 LSPro 失败", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: LSPro 注册成功（替代默认 Ls）", "agent", agentName)
			// 配置目录列表白名单：允许列出用户偏好目录（如 ~/.mindx），与 ReadPro 保持一致。
			// 注意必须在注册后配置到 LSPro（原 Ls 已被移除，若配置在替换前会在 Remove 时丢失）。
			if lp, ok := lsPro.(*mindxtools.LSPro); ok {
				lp.AddWhiteList(a.settings.UserPreferences())
			}
		}

		// 替换默认 Read 工具为增强版 ReadPro（大文件自动知识库分块树预览 + 原生回退）.
		_ = rt.ToolRegistry().Remove("Read") // 先删除 goharness 默认的 Read
		readPro := mindxtools.NewReadPro(kbAddr)
		if err := rt.RegisterTool(readPro); err != nil {
			a.logger.Warn("createRuntime: 注册 ReadPro 失败", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: ReadPro 注册成功（替代默认 Read）", "agent", agentName)
			// 配置读取白名单：允许读取用户偏好目录（如 ~/.mindx）下的配置、日志等
			// 项目外文件。注意必须在注册后配置到 ReadPro（原 Read 已被移除，
			// 若配置在替换前会在 Remove 时丢失）。
			if rp, ok := readPro.(*mindxtools.ReadPro); ok {
				rp.AddWhiteList(a.settings.UserPreferences())
				// 图片读取开关按模型视觉能力（Visioning）配置在内嵌的 goharness Read 上。
				// ReadPro 自身不参与图片链路：图片读取的消费（转换为 image_url 消息）
				// 由 goharness 层的 ImageHook 完成，此处只控制 Read 是否返回图片数据。
				rp.Read.SetImageReading(resolvedModel.Visioning)
				a.logger.Info("createRuntime: ReadPro whitelist configured",
					"agent", agentName, "dir", a.settings.UserPreferences(),
					"image_reading", resolvedModel.Visioning)
			}
		}

		// Register QuickSearch tool (知识库语义搜索).
		searchTool := mindxtools.NewQuickSearch(kbAddr)
		if err := rt.RegisterTool(searchTool); err != nil {
			a.logger.Warn("createRuntime: 注册 QuickSearch 失败", "agent", agentName, "error", err)
		} else {
			a.logger.Info("createRuntime: QuickSearch 注册成功", "agent", agentName)
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
		return nil, fmt.Errorf("当前没有可用的智能体")
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
		return "", fmt.Errorf("当前没有可用的会话数据库")
	}
	if a.mindxConfig == nil {
		return "", fmt.Errorf("当前没有可用的配置文件")
	}

	agentName := a.CurrentAgentName()
	if agentName == "" {
		return "", fmt.Errorf("当前没有可用的智能体")
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
			a.logger.Info("找到匹配会话",
				"session_id", matched.SessionID,
				"project_dir", matched.ProjectDir,
				"agent", agentName,
			)
			a.currentSessionMeta = matched
			a.mindxConfig.LastSessionID = matched.SessionID
			if saveErr := a.mindxConfig.Save(); saveErr != nil {
				a.logger.Warn("保存配置文件失败（会话匹配后）", "error", saveErr)
			}
			return matched.SessionID, nil
		}

		a.logger.Info("未找到匹配会话，创建新会话",
			"cwd", cwd,
			"agent", agentName,
		)
		newSession, createErr := a.CreateSession(agentName, cwd)
		if createErr != nil {
			a.logger.Error("创建新会话失败", createErr)
			a.currentSessionMeta = nil
			a.mindxConfig.LastSessionID = ""
			return "", createErr
		}
		a.currentSessionMeta = newSession
		a.mindxConfig.LastSessionID = newSession.SessionID
		if saveErr := a.mindxConfig.Save(); saveErr != nil {
			a.logger.Warn("保存配置文件失败（会话创建后）", "error", saveErr)
		}
		a.logger.Info("新会话创建",
			"session_id", newSession.SessionID,
			"project_dir", newSession.ProjectDir,
		)
		return newSession.SessionID, nil
	}

	// No current session meta — try to find existing or create new
	a.logger.Info("当前没有会话元数据，搜索已存在会话", "cwd", cwd)

	if matched := a.findSessionByProjectDir(cwd, agentName); matched != nil {
		a.logger.Info("找到匹配会话",
			"session_id", matched.SessionID,
			"project_dir", matched.ProjectDir,
			"agent", agentName,
		)
		a.currentSessionMeta = matched
		a.mindxConfig.LastSessionID = matched.SessionID
		if saveErr := a.mindxConfig.Save(); saveErr != nil {
			a.logger.Warn("保存配置文件失败（会话匹配后）", "error", saveErr)
		}
		return matched.SessionID, nil
	}

	a.logger.Info("未找到匹配会话，创建新会话", "cwd", cwd)
	newSession, createErr := a.CreateSession(agentName, cwd)
	if createErr != nil {
		return "", fmt.Errorf("CreateSession failed for agent=%q cwd=%q: %w", agentName, cwd, createErr)
	}
	a.currentSessionMeta = newSession
	a.mindxConfig.LastSessionID = newSession.SessionID
	if saveErr := a.mindxConfig.Save(); saveErr != nil {
		a.logger.Warn("保存配置文件失败（会话创建后）", "error", saveErr)
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

	// 通用能力（Compactor 压缩引擎 + Sandbox 沙箱）由 Runtime 统一注入，
	// 与主会话/子会话走同一条装配路径（agents.Runtime.SessionConfigs）。
	if rt, rtErr := a.ResolveRuntime(agentName); rtErr == nil && rt != nil {
		opts = append(opts, rt.SessionConfigs()...)
	}
	// 注入 modelContextResolver，保证窗口大小动态查询当前模型
	opts = append(opts, session.WithModelContextResolver(a.ModelContextLength))

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
			projectDir := ""
			if a.currentSessionMeta != nil {
				projectDir = a.currentSessionMeta.ProjectDir
			}
			opts = append(opts, session.WithMemory(mindxses.NewRAGMemoryAdapter(sessRAG, agentName, projectDir)))

		}
	}

	s, err := session.Load(context.Background(), a.currentSessionMeta.SessionID, agentName, a.sessDB, a.logger, opts...)
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

	apiKey := m.APIKey
	if strings.EqualFold(m.Provider, "ollama") {
		apiKey = "NONEKey"
	} else {
		apiKey = a.resolveAPIKey(apiKey)
	}
	client := gochat.Client().Config(
		gochat.WithBaseURL(m.BaseURL),
		gochat.WithAPIKey(apiKey),
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
	sessions, err := session.ListSessions(ctx, a.SessDB())
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

		if err := session.DeleteSession(context.Background(), a.SessDB(), currentMeta.SessionID); err != nil {
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
	sessions, err := session.ListSessions(ctx, a.SessDB())
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
