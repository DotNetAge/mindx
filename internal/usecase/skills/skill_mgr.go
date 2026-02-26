package skills

import (
	"context"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SkillMgr struct {
	skillsDir    string
	workspaceDir string
	logger       logging.Logger
	mu           sync.RWMutex

	loader    *SkillLoader
	executor  *SkillExecutor
	searcher  *SkillSearcher
	indexer   *SkillIndexer
	converter *SkillConverter
	installer *Installer
	envMgr    *EnvManager
	mcpMgr    *MCPManager
}

func NewSkillMgr(skillsDir string, workspaceDir string, embeddingSvc *embedding.EmbeddingService, llamaSvc *infraLlama.OllamaService, logger logging.Logger) (*SkillMgr, error) {
	return NewSkillMgrWithStore(skillsDir, workspaceDir, embeddingSvc, llamaSvc, nil, logger)
}

func NewSkillMgrWithStore(skillsDir string, workspaceDir string, embeddingSvc *embedding.EmbeddingService, llamaSvc *infraLlama.OllamaService, store core.Store, logger logging.Logger) (*SkillMgr, error) {
	envMgr := NewEnvManager(workspaceDir, logger)
	installer := NewInstaller(logger)
	loader := NewSkillLoader(skillsDir, logger)
	mcpMgr := NewMCPManager(logger)
	executor := NewSkillExecutor(skillsDir, envMgr, store, mcpMgr, logger)
	searcher := NewSkillSearcher(embeddingSvc, logger)
	indexer := NewSkillIndexer(embeddingSvc, llamaSvc, store, logger)
	converter := NewSkillConverter(skillsDir, logger)

	mgr := &SkillMgr{
		skillsDir:    skillsDir,
		workspaceDir: workspaceDir,
		logger:       logger.Named("SkillMgr"),
		loader:       loader,
		executor:     executor,
		searcher:     searcher,
		indexer:      indexer,
		converter:    converter,
		installer:    installer,
		envMgr:       envMgr,
		mcpMgr:       mcpMgr,
	}

	if err := envMgr.LoadEnv(); err != nil {
		logger.Warn(i18n.T("skill.load_env_failed"), logging.Err(err))
	}

	if err := loader.LoadAll(); err != nil {
		return nil, fmt.Errorf("failed to load skills: %w", err)
	}

	mgr.syncComponents()

	if store != nil {
		if err := indexer.LoadFromStore(); err != nil {
			logger.Warn(i18n.T("skill.load_index_failed"), logging.Err(err))
		}
		mgr.syncComponents()
	}

	indexer.StartWorker()

	logger.Info(i18n.T("skill.init_success"), logging.String(i18n.T("skill.skills_count"), fmt.Sprintf("%d", len(loader.GetSkillInfos()))))
	return mgr, nil
}

func (m *SkillMgr) syncComponents() {
	skills := m.loader.GetSkills()
	skillInfos := m.loader.GetSkillInfos()
	vectors := m.indexer.GetVectors()

	m.executor.SetSkillInfos(skillInfos)
	m.executor.LoadAllStats(skillInfos)
	m.searcher.SetData(skills, skillInfos, vectors)
	m.converter.SetSkillInfos(skillInfos)

	m.updateSkillKeywords(skillInfos)
}

func (m *SkillMgr) updateSkillKeywords(skillInfos map[string]*entity.SkillInfo) {
	uniqueKeywords := make(map[string]bool)
	for _, info := range skillInfos {
		if info.Def != nil {
			for _, tag := range info.Def.Tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					uniqueKeywords[tag] = true
				}
			}
		}
	}

	keywords := make([]string, 0, len(uniqueKeywords))
	for kw := range uniqueKeywords {
		keywords = append(keywords, kw)
	}

	core.SetSkillKeywords(keywords)
	m.logger.Debug("已更新技能关键词到 PromptBuilder", logging.Int("keyword_count", len(keywords)))
}

func (m *SkillMgr) LoadSkills() error {
	if err := m.loader.LoadAll(); err != nil {
		return err
	}
	m.syncComponents()
	return nil
}

func (m *SkillMgr) GetSkills() ([]*core.Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := m.loader.GetSkills()
	result := make([]*core.Skill, 0, len(skills))
	for _, skill := range skills {
		result = append(result, skill)
	}
	return result, nil
}

func (m *SkillMgr) GetSkillInfo(name string) (*entity.SkillInfo, bool) {
	_, info, exists := m.loader.GetSkill(name)
	return info, exists
}

func (m *SkillMgr) GetSkillInfos() map[string]*entity.SkillInfo {
	return m.loader.GetSkillInfos()
}

func (m *SkillMgr) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return fmt.Errorf("skill not found: %s", name)
	}

	info.Def.Enabled = true
	m.loader.UpdateSkillInfo(name, info)
	m.syncComponents()

	m.logger.Info(i18n.T("skill.enabled"), logging.String(i18n.T("skill.name"), name))
	return nil
}

func (m *SkillMgr) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return fmt.Errorf("skill not found: %s", name)
	}

	info.Def.Enabled = false
	m.loader.UpdateSkillInfo(name, info)
	m.syncComponents()

	m.logger.Info(i18n.T("skill.disabled"), logging.String(i18n.T("skill.name"), name))
	return nil
}

func (m *SkillMgr) InstallDependency(name string, method entity.InstallMethod) error {
	return m.installer.InstallDependency(method)
}

func (m *SkillMgr) Execute(skill *core.Skill, params map[string]any) error {
	if skill == nil || skill.Execute == nil {
		return fmt.Errorf("skill or execute is empty")
	}

	name := skill.GetName()
	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return fmt.Errorf("skill not found: %s", name)
	}

	_, err := m.executor.Execute(name, info.Def, params)
	return err
}

func (m *SkillMgr) ExecuteByName(name string, params map[string]any) (string, error) {
	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	return m.executor.Execute(name, info.Def, params)
}

func (m *SkillMgr) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	m.logger.Info(i18n.T("skill.exec_func"),
		logging.String(i18n.T("skill.function"), function.Name),
		logging.Any(i18n.T("skill.arguments"), function.Arguments))

	result, err := m.executor.ExecuteFunc(function)
	if err != nil {
		m.logger.Error(i18n.T("skill.exec_func_failed"), logging.Err(err))
		return "", err
	}

	m.logger.Info(i18n.T("skill.exec_func_success"), logging.String(i18n.T("skill.output"), result))
	return result, nil
}

func (m *SkillMgr) SearchSkills(keywords ...string) ([]*core.Skill, error) {
	return m.searcher.Search(keywords...)
}

func (m *SkillMgr) ReIndex() error {
	skillInfos := m.loader.GetSkillInfos()
	if err := m.indexer.ReIndex(skillInfos); err != nil {
		return err
	}
	// 等待异步索引队列处理完成，再同步向量到 searcher
	m.indexer.WaitForCompletion(5 * time.Minute)
	m.syncComponents()
	return nil
}

// indexMCPSkills 将 MCP 工具增量送入索引队列
// 只索引新注册的 MCP skill，不触发全量 ReIndex
func (m *SkillMgr) indexMCPSkills(defs []*entity.SkillDef) {
	allInfos := m.loader.GetSkillInfos()
	mcpInfos := make(map[string]*entity.SkillInfo, len(defs))
	for _, def := range defs {
		if info, ok := allInfos[def.Name]; ok {
			mcpInfos[def.Name] = info
		}
	}
	if len(mcpInfos) == 0 {
		return
	}
	// 送入 indexer 队列（异步处理，不阻塞连接流程）
	if err := m.indexer.ReIndex(mcpInfos); err != nil {
		m.logger.Warn("MCP 工具索引失败", logging.Err(err))
	}
}

func (m *SkillMgr) IsReIndexing() bool {
	return m.indexer.IsReIndexing()
}

func (m *SkillMgr) GetReIndexError() error {
	return m.indexer.GetReIndexError()
}

func (m *SkillMgr) StartReIndexInBackground() {
	go func() {
		if err := m.ReIndex(); err != nil {
			m.logger.Error(i18n.T("skill.bg_reindex_failed"), logging.Err(err))
		}
	}()
}

func (m *SkillMgr) IsVectorTableEmpty() bool {
	return m.searcher.IsVectorTableEmpty()
}

func (m *SkillMgr) ConvertSkill(name string) error {
	if err := m.converter.Convert(name); err != nil {
		return err
	}
	m.syncComponents()
	return nil
}

func (m *SkillMgr) InstallRuntime(name string) error {
	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return fmt.Errorf("skill not found: %s", name)
	}

	if info.Def == nil || len(info.Def.Install) == 0 {
		return fmt.Errorf("no install method for skill")
	}

	m.logger.Info(i18n.T("skill.start_install"), logging.String(i18n.T("skill.name"), name))

	var lastErr error
	for _, method := range info.Def.Install {
		if err := m.installer.InstallDependency(method); err != nil {
			m.logger.Warn(i18n.T("skill.install_method_failed"),
				logging.String(i18n.T("skill.name"), name),
				logging.String(i18n.T("skill.method"), method.ID),
				logging.Err(err))
			lastErr = err
			continue
		}

		m.logger.Info(i18n.T("skill.install_success"),
			logging.String(i18n.T("skill.name"), name),
			logging.String(i18n.T("skill.method"), method.ID))
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("all install methods failed: %w", lastErr)
	}

	return nil
}

func (m *SkillMgr) BatchConvert(names []string) (success []string, failed map[string]string) {
	success, failed = m.converter.BatchConvert(names)
	m.syncComponents()
	return success, failed
}

func (m *SkillMgr) BatchInstall(names []string) (success []string, failed map[string]string) {
	success = make([]string, 0)
	failed = make(map[string]string)

	for _, name := range names {
		if err := m.InstallRuntime(name); err != nil {
			failed[name] = err.Error()
			m.logger.Warn(i18n.T("skill.batch_install_failed"), logging.String(i18n.T("skill.name"), name), logging.Err(err))
		} else {
			success = append(success, name)
		}
	}

	return success, failed
}

func (m *SkillMgr) GetMissingDependencies(name string) ([]string, []string, error) {
	_, info, exists := m.loader.GetSkill(name)
	if !exists {
		return nil, nil, fmt.Errorf("skill not found: %s", name)
	}

	return info.MissingBins, info.MissingEnv, nil
}

func (m *SkillMgr) RegisterInternalSkill(name string, fn func(params map[string]any) (string, error)) {
	m.executor.RegisterInternalSkill(name, fn)
}

func (m *SkillMgr) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.indexer.StopWorker()
	if m.mcpMgr != nil {
		_ = m.mcpMgr.Close()
	}

	return nil
}

// InitMCPServers 初始化所有配置的 MCP server（并发，每个 server 有独立超时）
func (m *SkillMgr) InitMCPServers(ctx context.Context, cfg *config.MCPServersConfig) {
	if cfg == nil || len(cfg.MCPServers) == 0 {
		return
	}

	var wg sync.WaitGroup
	for name, entry := range cfg.MCPServers {
		if !entry.Enabled {
			m.logger.Info("MCP server 已禁用，跳过", logging.String("server", name))
			continue
		}

		wg.Add(1)
		go func(n string, e config.MCPServerEntry) {
			defer wg.Done()
			m.initMCPServerWithRetry(ctx, n, e)
		}(name, entry)
	}
	wg.Wait()
}

// mcpConnectTimeout 根据传输类型返回连接超时时间
// stdio 类型使用 npx，冷启动需要下载+安装+启动，需要更长超时
func mcpConnectTimeout(entry config.MCPServerEntry) time.Duration {
	if entry.GetType() == "sse" {
		return 30 * time.Second
	}
	return 120 * time.Second // stdio: npx 冷启动可能很慢
}

// initMCPServerWithRetry 带重试的 MCP server 初始化
// 仅对超时类错误重试，协议错误/进程崩溃等不可恢复错误直接放弃
func (m *SkillMgr) initMCPServerWithRetry(ctx context.Context, name string, entry config.MCPServerEntry) {
	const maxAttempts = 3
	timeout := mcpConnectTimeout(entry)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		connCtx, cancel := context.WithTimeout(ctx, timeout)
		err := m.connectAndRegisterMCP(connCtx, name, entry)
		cancel()

		if err == nil {
			return
		}

		// 判断是否值得重试：只有超时/临时网络错误才重试
		if !isMCPRetryableError(err) {
			m.logger.Warn("MCP server 初始化失败（不可重试，跳过）",
				logging.String("server", name),
				logging.Err(err))
			return
		}

		if attempt < maxAttempts {
			delay := time.Duration(attempt) * 5 * time.Second
			m.logger.Warn("MCP server 连接超时，准备重试",
				logging.String("server", name),
				logging.Int("attempt", attempt),
				logging.Int("max_attempts", maxAttempts),
				logging.Err(err))

			select {
			case <-ctx.Done():
				m.logger.Warn("MCP server 初始化被取消",
					logging.String("server", name))
				return
			case <-time.After(delay):
			}
		} else {
			m.logger.Warn("MCP server 初始化失败（已达最大重试次数，跳过）",
				logging.String("server", name),
				logging.Int("attempts", maxAttempts),
				logging.Err(err))
		}
	}
}

// isMCPRetryableError 判断 MCP 连接错误是否值得重试
// 超时、临时网络错误 → 重试
// EOF（进程崩溃）、405（协议不兼容）等 → 不重试
func isMCPRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 超时：值得重试（npx 冷启动慢）
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "connection refused") {
		return true
	}
	// EOF：子进程启动后立刻崩溃，重试无意义
	// Method Not Allowed：协议不兼容，重试无意义
	return false
}

// connectAndRegisterMCP 连接 MCP server 并注册发现的工具
func (m *SkillMgr) connectAndRegisterMCP(ctx context.Context, name string, entry config.MCPServerEntry) error {
	if err := m.mcpMgr.ConnectServer(ctx, name, entry); err != nil {
		return err
	}

	tools, err := m.mcpMgr.GetDiscoveredTools(name)
	if err != nil {
		return err
	}

	// 从 catalog 获取中文工具描述，用于覆盖 MCP server 返回的英文描述
	zhDescriptions := config.GetCatalogToolDescriptions(name, "zh")
	// 从 catalog 获取 tags，注入到 skill keywords 中提升意图识别准确性
	catalogTags := config.GetCatalogTags(name)

	defs := make([]*entity.SkillDef, 0, len(tools))
	for _, tool := range tools {
		def := MCPToolToSkillDef(name, tool, catalogTags)
		// 如果 catalog 中有中文描述，用中文覆盖（提升向量索引的中文匹配能力）
		if zhDesc, ok := config.MatchCatalogToolDescription(zhDescriptions, tool.Name); ok && zhDesc != "" {
			def.Description = zhDesc
		}
		defs = append(defs, def)
	}

	m.loader.RegisterMCPSkills(name, defs)
	m.syncComponents()

	// 将新注册的 MCP 工具送入索引队列，生成向量索引
	m.indexMCPSkills(defs)

	m.logger.Info("MCP server 初始化完成",
		logging.String("server", name),
		logging.Int("tools", len(defs)))
	return nil
}

// AddMCPServer 运行时添加 MCP server
func (m *SkillMgr) AddMCPServer(ctx context.Context, name string, entry config.MCPServerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.connectAndRegisterMCP(ctx, name, entry)
}

// RemoveMCPServer 运行时移除 MCP server
func (m *SkillMgr) RemoveMCPServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loader.UnregisterMCPSkills(name)
	if err := m.mcpMgr.RemoveServer(name); err != nil {
		return err
	}
	m.syncComponents()
	return nil
}

// RestartMCPServer 重启 MCP server
func (m *SkillMgr) RestartMCPServer(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.mcpMgr.GetServerState(name)
	if !ok {
		return fmt.Errorf("MCP server not found: %s", name)
	}

	// 先注销旧工具
	m.loader.UnregisterMCPSkills(name)

	// 重新连接并注册
	if err := m.connectAndRegisterMCP(ctx, name, state.Config); err != nil {
		return err
	}
	return nil
}

// GetMCPServers 获取所有 MCP server 状态
func (m *SkillMgr) GetMCPServers() []*MCPServerState {
	return m.mcpMgr.ListServers()
}

// GetMCPServerTools 获取某 MCP server 的工具列表
func (m *SkillMgr) GetMCPServerTools(name string) ([]*mcp.Tool, error) {
	return m.mcpMgr.GetDiscoveredTools(name)
}
