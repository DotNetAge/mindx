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

	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/config"
	goharnessmemory "github.com/DotNetAge/goharness/memory"
	"github.com/DotNetAge/goharness/sandbox"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/store"
	"github.com/DotNetAge/goharness/tools"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
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
	agents      *agentstore.AgentStore
	models      *config.ModelRegistry
	providerReg config.ProviderRegistry
	versions    *FileVersionStore
	rules       rules.RuleRegistry
	sessDB      *mindxses.RoutedSessionStore

	// Loaded provider configs (for RPC queries)
	providerConfigs []*config.ProviderConfig

	// skills 是技能三级库装载器（全局库 + Agent 级覆盖，见 skillstore 包）
	skills *skillstore.Store

	// promptBuilder 缓存 mindx 侧基础系统提示词组装器（首次调用 PromptBuilder 时创建；
	// ReloadAgents 换入新 AgentStore 后置空重建，保证提示词引用最新注册表）
	promptBuilder   *PromptBuilder
	promptBuilderMu sync.Mutex

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

	// uiBroadcast 是 UI 命令通道的广播回调（Daemon 注入其 broadcastUI），
	// 供 Open/Visit/TerminalRun 三工具把呈现请求广播给各客户端。
	uiBroadcast mindxtools.UIBroadcast

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

	// market 是 COS 静态市场客户端（懒创建，见 Market()）
	market     *bundle.MarketClient
	marketOnce sync.Once
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
	agentsReg, agentLoadErrs, err := agentstore.Load(settings.AgentsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}
	for name, loadErr := range agentLoadErrs {
		logger.Warn("agent 迁移/加载警告", "file", name, "error", loadErr)
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
	skillStore, skillErr := skillstore.NewStore(settings.SkillsDir(), settings.AgentsDir())
	if skillErr != nil {
		logger.Warn("Failed to load skills", "dir", settings.SkillsDir(), "error", skillErr)
	}

	// 会话存储：会话实体落各工作目录 <project_dir>/.sessions（工作目录与会话生命周期
	// 绑定，目录消失会话即消失），工作目录清单持久化在 <DataDir>/session_dirs.json；
	// 旧全局布局（~/.mindx/sessions）在启动时一次性搬迁。
	sessDB, err := mindxses.NewRoutedSessionStore(settings.SessionDirsFile())
	if err != nil {
		logger.Warn("Failed to init session store", "error", err)
	} else if moved, kept, mErr := mindxses.MigrateLegacySessions(settings.SessionsDir(), sessDB.RegisterDir); mErr != nil {
		logger.Warn("旧会话搬迁未完成，下次启动重试", "error", mErr, "moved", moved, "kept", kept)
	} else if moved > 0 {
		logger.Info("旧会话已搬迁至各工作目录 .sessions", "moved", moved, "kept", kept)
	}

	credStore := NewCredentialStore(settings.UserPreferences())

	// 兜底补写 embedder_model：随包语义模型已落盘而配置缺失时写默认文件名。
	// 此前由 App 侧（enableEmbedderIfNeeded）补写，但 daemon 内存中的旧配置在
	// 任意一次 Save() 时会全量重写 mindx.json，抹掉 App 写入的字段（写竞争）。
	// 统一收口到 Go 侧：daemon 是配置唯一写入方，补写一次后随配置持久化。
	// Save 失败不中断启动：内存中已生效，本次记忆可用，下次启动重试补写。
	if mindxConfig != nil && mindxConfig.EmbedderModel == "" {
		defaultModelPath := filepath.Join(settings.UserPreferences(), "data", "models", "model.onnx")
		if _, statErr := os.Stat(defaultModelPath); statErr == nil {
			mindxConfig.EmbedderModel = "model.onnx"
			if saveErr := mindxConfig.Save(); saveErr != nil {
				logger.Warn("补写 embedder_model 默认值后保存配置失败，下次启动重试", "error", saveErr)
			} else {
				logger.Info("embedder_model 未配置，已补写默认值（检测到随包语义模型）", "model", "model.onnx")
			}
		}
	}

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
		skills:              skillStore,
		sessDB:              sessDB,
		runtimeCache:        make(map[string]*agents.Runtime),
		embedder:            emb,
		permissionRuleStore: permStore,
		tokenUsageStore:     mindxses.NewFileTokenUsageStore(settings.DataDir()),
		providerConfigs:     providers,
	}, nil
}

func resolveCurrentAgentName(cfg *MindxConfig, agents *agentstore.AgentStore, logger logging.Logger) string {
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
	for _, hired := range agents.Hired() {
		logger.Info("using first hired agent as current", "name", hired.Meta.Name)
		return hired.Meta.Name
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

// SetUIBroadcast 注入 UI 命令通道广播回调（Daemon 的 broadcastUI）。由 Daemon
// 初始化时调用；未注入时 createRuntime 不注册 Open/Visit/TerminalRun 三工具。
func (a *App) SetUIBroadcast(broadcast mindxtools.UIBroadcast) {
	a.uiBroadcast = broadcast
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

func (a *App) RuleRegistry() rules.RuleRegistry {
	return a.rules
}

func (a *App) SessionDB() *mindxses.RoutedSessionStore {
	return a.sessDB
}

// Skills 返回技能三级库装载器（全局库视图供管理 RPC 消费，
// 运行时注册表按 Agent 组装，见 createRuntime）。
func (a *App) Skills() *skillstore.Store {
	return a.skills
}

// Market 返回 COS 静态市场客户端（懒创建；缓存目录位于数据目录 market-cache）。
func (a *App) Market() *bundle.MarketClient {
	a.marketOnce.Do(func() {
		a.market = bundle.NewMarketClient(bundle.DefaultMarketManifestURL,
			filepath.Join(a.settings.DataDir(), "market-cache"))
	})
	return a.market
}

// PromptBuilder 返回 mindx 侧基础系统提示词组装器（懒创建，App 生命周期内缓存；
// ReloadAgents 会置空重建，避免引用被换出的旧 AgentStore）。
func (a *App) PromptBuilder() *PromptBuilder {
	a.promptBuilderMu.Lock()
	defer a.promptBuilderMu.Unlock()
	if a.promptBuilder == nil {
		a.promptBuilder = NewPromptBuilder(a.agents, a.skills,
			a.settings.UserPreferences(), a.settings.VenvDir(), a.BuildRulesSection)
	}
	return a.promptBuilder
}

// BuildRulesSection 渲染 System Prompt 的「扩展规则」段（P4 定案：应用语义规则的
// 拼接收口到 mindx，goharness 不再持有 ruleReg）。内容三部分：
//  1. 权限规则（mindx.json 的 always_allow/deny/ask，启动时确定）；
//  2. Agent 发现引导（原 createRuntime 注册进规则注册表的固定文案，改为直接渲染，
//     不再写入用户规则文件）；
//  3. 用户规则（rules.yml，经 agent.rule RPC 管理，实时渲染使增删改即时生效）。
//
// 原实现经 goharness ruleReg 注入且权限规则与用户规则共用一个槽位互相覆盖
// （app.go 两次 WithRuleRegistry 的缺陷），本方法合并渲染后缺陷自然消解。
// Agent 发现引导为固定常驻条目（与原实现一致），因此本方法永不返回空串。
func (a *App) BuildRulesSection() string {
	var lines []string

	// 1. 权限规则（文案与原 permReg 注册时的 Intro 保持一致）
	if a.permissionRuleStore != nil {
		if permRules, err := a.permissionRuleStore.Load(); err == nil && permRules != nil {
			for _, pr := range permRules.AlwaysAllow {
				lines = append(lines, "Always allow "+pr.Description)
			}
			for _, pr := range permRules.AlwaysDeny {
				lines = append(lines, "Always deny "+pr.Description)
			}
			for _, pr := range permRules.AlwaysAsk {
				lines = append(lines, "Ask before "+pr.Description)
			}
		}
	}

	// 2. Agent 发现引导（固定文案，随应用版本发布）
	lines = append(lines, agentDiscoveryIntro)

	// 3. 用户规则（rules.yml，实时渲染）
	if a.rules != nil {
		if userSection := a.rules.FormatPromptSection(); userSection != "" {
			lines = append(lines, strings.TrimRight(userSection, "\n"))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 扩展规则\n\n")
	for _, line := range lines {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (a *App) SetTestDir(tmpDir string) error {
	a.settings.Test = true
	a.settings.testDir = tmpDir
	// 测试路由器：清单文件隔离在测试目录内，会话实体按 project_dir 落
	// <project_dir>/.sessions（测试中 project_dir 用 t.TempDir，随测试清理）。
	sessDB, err := mindxses.NewRoutedSessionStore(filepath.Join(tmpDir, "data", "session_dirs.json"))
	if err != nil {
		return err
	}
	a.sessDB = sessDB

	// 技能库必须同步重定向：DefaultApp 构造时 Test 尚未置位，skillstore 已指向
	// 真实 ~/.mindx/skills，测试中的晋升/删除等写操作会污染真实全局技能库。
	skills, err := skillstore.NewStore(a.settings.SkillsDir(), a.settings.AgentsDir())
	if err != nil {
		return fmt.Errorf("重定向测试技能库失败: %w", err)
	}
	a.skills = skills

	// 智能体注册表同理重定向：默认构造的 AgentStore 指向真实 ~/.mindx/agents，
	// 测试中的保存/删除/分发包导入会污染真实 Agent 库（分发包安装直接写 store.Dir()）。
	agentsReg, _, err := agentstore.Load(a.settings.AgentsDir())
	if err != nil {
		return fmt.Errorf("重定向测试智能体注册表失败: %w", err)
	}
	a.agents = agentsReg

	// 提示词组装器持有旧注册表引用，一并置空待懒重建
	a.promptBuilderMu.Lock()
	a.promptBuilder = nil
	a.promptBuilderMu.Unlock()
	return nil
}

func (a *App) Agents() *agentstore.AgentStore {
	return a.agents
}

// ReloadAgents re-scans the agents directory and atomically swaps the in-memory registry.
// All cached runtimes for affected agents are invalidated so they pick up the new config
// on next ResolveRuntime() call.
// 注意：从当前 settings 解析的目录重新加载（而非 AgentStore 构造时的旧目录），
// 测试等先建 App 后改目录的场景才能生效。
func (a *App) ReloadAgents() error {
	fresh, _, err := agentstore.Load(a.settings.AgentsDir())
	if err != nil {
		return fmt.Errorf("reload agents: %w", err)
	}
	a.agents = fresh

	// 提示词组装器持有旧 AgentStore 引用，置空待下次懒重建
	a.promptBuilderMu.Lock()
	a.promptBuilder = nil
	a.promptBuilderMu.Unlock()

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

// ReloadSkills 重扫全局技能库并原子替换内存注册表。
// Runtime 持有的是 LiveRegistry 活视图（Agent 级实时读盘、全局库读原子指针），
// 替换后即刻对所有会话生效，无需失效 Runtime 缓存。
func (a *App) ReloadSkills() error {
	if err := a.skills.ReloadGlobal(); err != nil {
		a.logger.Warn("skills reloaded with warnings", "dir", a.settings.SkillsDir(), "error", err)
	}

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

func (a *App) SessDB() *mindxses.RoutedSessionStore {
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
	// 空目录兜底 Getwd：维持旧 FileSessionStore.Create 的语义（工作目录分片存储
	// 要求 project_dir 明确，路由器不接受空值落盘）。
	if projectDir == "" {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			projectDir = wd
		}
	}
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

// resolveModelName 解析当前生效的模型配置。
// 模型选择是用户级语义（LastModel > DefaultModel），Agent 不持有模型属性。
func (a *App) resolveModelName() (string, *config.ModelConfig, error) {
	modelName := ""
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

	_, modelCfg, err := a.resolveModelName()
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", agent.Meta.Name, err)
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
		// Agent 存在性校验与 exclude_tools 解析均由 mindx 侧回调提供，
		// goharness 对 Agent 结构零依赖（PR-PROMPTS 第一节）。
		agents.WithAgentExists(a.agents.Exists),
		agents.WithExcludeTools(a.agents.ExcludeToolsOf),
		agents.WithProviderRegistry(a.providerReg),
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

	// 技能检索 SPI：按 Agent 组装（全局库 + Agent 级同名覆盖）。
	if a.skills != nil {
		// 活检索视图（不快照）：技能增删改经 ReloadGlobal/实时读盘即刻对
		// 全部会话生效，无需重建 Runtime（PR-PROMPTS 第二节热加载语义）。
		opts = append(opts, agents.WithSkillRegistry(a.skills.LiveRegistryFor(agentName)))
		// 动态技能回退解析器：Skill 工具在基础库未命中时按会话项目目录
		// 解析 <ProjectDir>/.agents/skills/<name>（军规：动态技能绝不进入
		// 系统提示词，经 mindx skills discovery 发现后按需加载；
		// 执行期解析不触碰 system prompt，不影响 KV 缓存前缀）。
		opts = append(opts, agents.WithProjectSkillResolver(a.skills.ResolveProject))
	}

	// 基础系统提示词由 mindx 侧组装（IDENTITY → SOUL → Skill 目录 → AGENTS.md → Env → 扩展规则），
	// 扩展规则（权限规则 + Agent 发现引导 + 用户规则）一并由 mindx 渲染（P4 定案），
	// goharness 不追加任何文案段，base 输出即完整 system prompt。
	opts = append(opts, agents.WithBaseSystemPrompt(a.PromptBuilder().Build))

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

	// TeamXXX 默认剥离：组队语义由 mindx 侧 agentstore（agent.yaml 的
	// team/members）承担，goharness 的内存版组队工具不参与 mindx 流程。
	// Agent 在 IDENTITY.md frontmatter 的 include_tools 里声明条目即装配回来
	//（opt-in 白名单，与 exclude_tools 对称）。
	included := make(map[string]bool)
	for _, name := range a.agents.IncludeToolsOf(agentName) {
		included[name] = true
	}
	for _, name := range []string{"TeamCreate", "TeamDelete", "TeamList", "TeamGetTasks"} {
		if included[name] {
			a.logger.Info("createRuntime: Team 工具按声明装配", "agent", agentName, "tool", name)
			continue
		}
		if err := rt.ToolRegistry().Remove(name); err != nil {
			a.logger.Debug("createRuntime: 剥离 Team 工具跳过（不存在）", "agent", agentName, "tool", name)
		} else {
			a.logger.Info("createRuntime: Team 工具已剥离", "agent", agentName, "tool", name)
		}
	}

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
	// 按 Agent 的 allows_tools（云技能清单，条目 "mcp:<server>"）过滤注入：
	//   - allows_tools 为空（nil / 空数组）→ 不做白名单过滤，全部 enabled MCP server 的工具都注册
	//   - allows_tools 非空 → 只注册条目里命中的 server（格式 "mcp:<server>"）
	//   - mcp.json 里 enabled=false 的 server 自然不会命中（EnabledTools 已过滤）
	if a.mcpMgr != nil {
		allowedServers := allowedMCPServers(agent.Meta.AllowsTools)
		tools := a.mcpMgr.EnabledTools()
		if len(tools) == 0 {
			a.logger.Debug("createRuntime: 无 enabled MCP server", "agent", agentName)
		} else if len(allowedServers) == 0 {
			// 空白名单 = 不过滤，全部注册
			for _, tool := range tools {
				if err := rt.RegisterTool(tool); err != nil {
					a.logger.Warn("createRuntime: 注册 MCP工具 失败", "agent", agentName, "tool", tool.Info().Name, "error", err)
				} else {
					a.logger.Info("createRuntime: MCP工具 注册成功", "agent", agentName, "tool", tool.Info().Name)
				}
			}
		} else {
			// 非空白名单 = 按 server 名过滤
			for _, tool := range tools {
				server := mcpServerOfTool(tool.Info().Name)
				if !allowedServers[server] {
					continue
				}
				if err := rt.RegisterTool(tool); err != nil {
					a.logger.Warn("createRuntime: 注册 MCP工具 失败", "agent", agentName, "tool", tool.Info().Name, "error", err)
				} else {
					a.logger.Info("createRuntime: MCP工具 注册成功", "agent", agentName, "tool", tool.Info().Name)
				}
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

	// Register UI 呈现三工具（Agent-Driven UI 命令通道）：广播回调由 Daemon 注入，
	// 未注入（TUI 等无 daemon 场景）不注册，避免 Agent 调用无人执行的命令。
	if a.uiBroadcast != nil {
		uiTools := []tools.FuncTool{
			mindxtools.NewOpen(a.uiBroadcast),
			mindxtools.NewVisit(a.uiBroadcast),
			mindxtools.NewTerminalRun(a.uiBroadcast),
		}
		for _, t := range uiTools {
			if err := rt.RegisterTool(t); err != nil {
				a.logger.Warn("createRuntime: 注册 UI工具 失败", "agent", agentName, "tool", t.Info().Name, "error", err)
			} else {
				a.logger.Info("createRuntime: UI工具 注册成功", "agent", agentName, "tool", t.Info().Name)
			}
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

// ForEachRuntime 对全部缓存 Runtime 执行 fn（读锁保护）。
// Runtime 按 agent 名缓存，而停止请求仅携带 session_id、无法定位执行该
// 会话的具体 Runtime 实例，故遍历全部实例（如强停派生子代理时，
// 仅实际持有登记的 Runtime 会命中）。fn 内禁止调用会再次获取
// runtimeMu 写锁的 App 方法（如 ResolveRuntime），避免锁重入死锁。
func (a *App) ForEachRuntime(fn func(*agents.Runtime)) {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	for _, rt := range a.runtimeCache {
		fn(rt)
	}
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

// allowedMCPServers 把 allows_tools 条目解析为允许的 MCP server 名集合。
// 条目格式 "mcp:<server>"；缺前缀或格式不符的条目忽略（容忍手写）。
func allowedMCPServers(entries []string) map[string]bool {
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if server, ok := strings.CutPrefix(e, "mcp:"); ok && server != "" {
			set[server] = true
		}
	}
	return set
}

// mcpServerOfTool 从 goharness 工具名中提取 MCP server 段。
// 工具名格式 "mcp:<server>:<tool>"（见 internal/mcp BuildTools）；非此格式返回空串。
func mcpServerOfTool(toolName string) string {
	rest, ok := strings.CutPrefix(toolName, "mcp:")
	if !ok {
		return ""
	}
	server, _, _ := strings.Cut(rest, ":")
	return server
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
