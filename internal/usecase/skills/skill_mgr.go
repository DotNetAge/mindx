package skills

import (
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"sync"
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

func NewSkillMgrWithStore(skillsDir string, workspaceDir string, embeddingSvc *embedding.EmbeddingService, llamaSvc *infraLlama.OllamaService, store persistence.Store, logger logging.Logger) (*SkillMgr, error) {
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
	m.syncComponents()
	return nil
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

func (m *SkillMgr) RegisterMCPServer(config MCPServerConfig) error {
	return m.mcpMgr.RegisterServer(config)
}

func (m *SkillMgr) ListMCPServers() []string {
	return m.mcpMgr.ListServers()
}
