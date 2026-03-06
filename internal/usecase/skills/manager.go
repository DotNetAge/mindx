package skills

import (
	"context"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// SkillManager 技能管理器（Phase 4 重构版）
// 职责：只负责 UI 管理功能（列表、安装、启用/禁用等）
// 搜索和执行功能由 HybridSearcher 和 ToolAssembler 负责
type SkillManager struct {
	skillsDir      string
	skills         map[string]*entity.Skill
	skillInfos     map[string]*entity.SkillInfo
	parser         *SkillParser
	indexer        *SkillIndexer
	hybridSearcher *HybridSearcher // Phase 4: 混合检索器
	isReIndexing   bool
	reIndexError   error
	mu             sync.RWMutex
	logger         logging.Logger
}

// NewSkillManager 创建技能管理器
func NewSkillManager(skillsDir string, indexer *SkillIndexer) *SkillManager {
	return &SkillManager{
		skillsDir:  skillsDir,
		skills:     make(map[string]*entity.Skill),
		skillInfos: make(map[string]*entity.SkillInfo),
		parser:     NewSkillParser(),
		indexer:    indexer,
		logger:     logging.GetSystemLogger().Named("skill_manager"),
	}
}

// NewSkillMgrWithStore 创建技能管理器（兼容旧接口）
func NewSkillMgrWithStore(
	skillsPath string,
	workspace string,
	embeddingSvc *embedding.EmbeddingService,
	llamaSvc *infraLlama.OllamaService,
	store core.Store,
	logger logging.Logger,
) (*SkillManager, error) {
	// 创建索引器
	indexer := NewSkillIndexer(embeddingSvc, llamaSvc, store, logger)

	// 创建管理器
	manager := NewSkillManager(skillsPath, indexer)

	// Phase 4 Step 3: 创建 HybridSearcher
	// 1. 尝试获取 BadgerDB 实例（需要类型断言）
	type badgerDBGetter interface {
		GetDB() interface{}
	}

	hybridSearcherCreated := false
	if badgerStore, ok := store.(badgerDBGetter); ok {
		if dbInterface := badgerStore.GetDB(); dbInterface != nil {
			// 类型断言为 *badger.DB
			if db, ok := dbInterface.(*badger.DB); ok {
				// 2. 创建 VectorIndex 和 KeywordIndex
				vectorIndex := NewVectorIndex(db, embeddingSvc)
				keywordIndex := NewKeywordIndex()

				// 3. 创建 HybridSearcher
				manager.hybridSearcher = NewHybridSearcher(vectorIndex, keywordIndex, nil)
				logger.Info("HybridSearcher created successfully")
				hybridSearcherCreated = true
			} else {
				logger.Error("GetDB() returned non-BadgerDB type - HybridSearcher will not be available")
			}
		} else {
			logger.Error("GetDB() returned nil - HybridSearcher will not be available")
		}
	} else {
		logger.Error("Store does not support GetDB() - HybridSearcher will not be available")
	}

	// 如果 HybridSearcher 创建失败，记录警告
	if !hybridSearcherCreated {
		logger.Warn("HybridSearcher not created - skill search will use fallback method")
	}

	// 加载技能
	if err := manager.LoadSkills(); err != nil {
		return nil, fmt.Errorf("failed to load skills: %w", err)
	}

	return manager, nil
}

// LoadSkills 加载所有技能
func (sm *SkillManager) LoadSkills() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.logger.Info("loading skills", logging.String("dir", sm.skillsDir))

	// 检查目录是否存在
	if _, err := os.Stat(sm.skillsDir); os.IsNotExist(err) {
		sm.logger.Warn("skills directory not found", logging.String("dir", sm.skillsDir))
		return nil
	}

	// 扫描技能目录
	entries, err := os.ReadDir(sm.skillsDir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			sm.logger.Debug("skipping non-directory entry", logging.String("name", entry.Name()))
			continue
		}

		skillName := entry.Name()
		skillDir := filepath.Join(sm.skillsDir, skillName)

		sm.logger.Debug("loading skill", logging.String("name", skillName), logging.String("dir", skillDir))

		// 加载技能
		skill, err := sm.loadSkill(skillDir)
		if err != nil {
			sm.logger.Warn("failed to load skill",
				logging.String("skill", skillName),
				logging.String("dir", skillDir),
				logging.Err(err),
			)
			continue
		}

		sm.skills[skill.Name] = skill
		sm.skillInfos[skill.Name] = sm.skillToInfo(skill)
		loadedCount++

		// Phase 4: 自动索引到 KeywordIndex
		if sm.hybridSearcher != nil && sm.hybridSearcher.keywordIndex != nil {
			sm.hybridSearcher.keywordIndex.IndexSkill(sm.skillInfos[skill.Name].Def)
		}

		sm.logger.Info("skill loaded",
			logging.String("name", skill.Name),
			logging.String("version", skill.Version),
		)
	}

	sm.logger.Info("skills loaded", logging.Int("count", loadedCount))

	return nil
}

// loadSkill 加载单个技能
func (sm *SkillManager) loadSkill(skillDir string) (*entity.Skill, error) {
	// 读取 SKILL.md
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// 使用解析器解析文件
	skill, err := sm.parser.Parse(skillPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SKILL.md: %w", err)
	}

	return skill, nil
}

// skillToInfo 将 Skill 转换为 SkillInfo
func (sm *SkillManager) skillToInfo(skill *entity.Skill) *entity.SkillInfo {
	return &entity.SkillInfo{
		Def: &entity.SkillDef{
			Name:        skill.Name,
			Description: skill.Description,
			Version:     skill.Version,
			Tags:        skill.Tags,
			Enabled:     true, // 默认启用
			IsInternal:  false,
		},
		CanRun: true,
		Format: "markdown",
		Status: "ready",
	}
}

// GetSkillInfos 获取所有技能信息
func (sm *SkillManager) GetSkillInfos() []*entity.SkillInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]*entity.SkillInfo, 0, len(sm.skillInfos))
	for _, info := range sm.skillInfos {
		infos = append(infos, info)
	}

	return infos
}

// GetSkillInfo 获取单个技能信息
func (sm *SkillManager) GetSkillInfo(name string) (*entity.SkillInfo, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	info, ok := sm.skillInfos[name]
	return info, ok
}

// GetSkill 获取技能
func (sm *SkillManager) GetSkill(name string) (*entity.Skill, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	skill, ok := sm.skills[name]
	return skill, ok
}

// ReIndex 重建索引
func (sm *SkillManager) ReIndex() error {
	sm.mu.Lock()
	sm.isReIndexing = true
	sm.reIndexError = nil
	sm.mu.Unlock()

	defer func() {
		sm.mu.Lock()
		sm.isReIndexing = false
		sm.mu.Unlock()
	}()

	sm.logger.Info("reindexing skills")

	// 重新加载技能
	if err := sm.LoadSkills(); err != nil {
		sm.mu.Lock()
		sm.reIndexError = err
		sm.mu.Unlock()
		return err
	}

	// 重建向量索引
	if sm.indexer != nil {
		sm.mu.RLock()
		skillInfos := make(map[string]*entity.SkillInfo)
		for name, info := range sm.skillInfos {
			skillInfos[name] = info
		}
		sm.mu.RUnlock()

		if err := sm.indexer.ReIndex(skillInfos); err != nil {
			sm.mu.Lock()
			sm.reIndexError = err
			sm.mu.Unlock()
			return err
		}
	}

	sm.logger.Info("reindexing completed")

	return nil
}

// IsReIndexing 是否正在重建索引
func (sm *SkillManager) IsReIndexing() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.isReIndexing
}

// GetReIndexError 获取重建索引错误
func (sm *SkillManager) GetReIndexError() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.reIndexError
}

// Enable 启用技能
func (sm *SkillManager) Enable(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	info, ok := sm.skillInfos[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	if info.Def != nil {
		info.Def.Enabled = true
	}

	sm.logger.Info("skill enabled", logging.String("name", name))

	return nil
}

// Disable 禁用技能
func (sm *SkillManager) Disable(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	info, ok := sm.skillInfos[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	if info.Def != nil {
		info.Def.Enabled = false
	}

	sm.logger.Info("skill disabled", logging.String("name", name))

	return nil
}

// ConvertSkill 转换技能格式（Phase 4 中暂不实现，返回成功）
func (sm *SkillManager) ConvertSkill(name string) error {
	sm.logger.Info("convert skill (no-op in Phase 4)", logging.String("name", name))
	return nil
}

// InstallDependency 安装依赖（Phase 4 中暂不实现，返回成功）
func (sm *SkillManager) InstallDependency(name string, method entity.InstallMethod) error {
	sm.logger.Info("install dependency (no-op in Phase 4)", logging.String("name", name))
	return nil
}

// InstallRuntime 安装运行时（Phase 4 中暂不实现，返回成功）
func (sm *SkillManager) InstallRuntime(name string) error {
	sm.logger.Info("install runtime (no-op in Phase 4)", logging.String("name", name))
	return nil
}

// BatchConvert 批量转换技能
func (sm *SkillManager) BatchConvert(names []string) (success []string, failed []string) {
	for _, name := range names {
		if err := sm.ConvertSkill(name); err != nil {
			failed = append(failed, name)
		} else {
			success = append(success, name)
		}
	}
	return
}

// BatchInstall 批量安装依赖
func (sm *SkillManager) BatchInstall(names []string) (success []string, failed []string) {
	for _, name := range names {
		if err := sm.InstallDependency(name, entity.InstallMethod{}); err != nil {
			failed = append(failed, name)
		} else {
			success = append(success, name)
		}
	}
	return
}

// GetSkills 获取所有技能（用于执行）
// 注意：这个方法返回的是旧的 core.Skill 类型，需要适配
func (sm *SkillManager) GetSkills() ([]*entity.Skill, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	skills := make([]*entity.Skill, 0, len(sm.skills))
	for _, skill := range sm.skills {
		skills = append(skills, skill)
	}

	return skills, nil
}

// Execute 执行技能（Phase 4 中暂不实现，返回成功）
// 实际执行应该通过 ToolAssembler 完成
func (sm *SkillManager) Execute(skill *entity.Skill, params map[string]any) error {
	sm.logger.Info("execute skill (delegated to ToolAssembler)",
		logging.String("name", skill.Name),
	)
	// TODO: 实际执行应该通过 ToolAssembler 完成
	return nil
}

// InitMCPServers 初始化 MCP 服务器（兼容旧接口）
func (sm *SkillManager) InitMCPServers(ctx context.Context, mcpCfg *config.MCPServersConfig) {
	sm.logger.Info("init MCP servers (no-op in Phase 4)")
	// TODO: Phase 4 中 MCP 由独立的 MCPManager 管理
}

// IsVectorTableEmpty 检查向量表是否为空（兼容旧接口）
func (sm *SkillManager) IsVectorTableEmpty() bool {
	// 简单实现：检查是否有技能
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.skills) == 0
}

// StartReIndexInBackground 后台启动重建索引（兼容旧接口）
func (sm *SkillManager) StartReIndexInBackground() {
	go func() {
		if err := sm.ReIndex(); err != nil {
			sm.logger.Error("background reindex failed", logging.Err(err))
		}
	}()
}

// SearchSkills 搜索技能（兼容旧接口）
// 返回技能名称列表
func (sm *SkillManager) SearchSkills(keywords ...string) ([]string, error) {
	// 如果没有关键词，返回所有技能
	if len(keywords) == 0 {
		sm.mu.RLock()
		defer sm.mu.RUnlock()

		names := make([]string, 0, len(sm.skills))
		for name := range sm.skills {
			names = append(names, name)
		}
		return names, nil
	}

	// 如果有 HybridSearcher，使用它进行搜索
	if sm.hybridSearcher != nil {
		query := ""
		if len(keywords) > 0 {
			query = keywords[0] // 使用第一个关键词作为查询
		}

		matches, err := sm.hybridSearcher.Search(query, 10) // 返回前 10 个
		if err != nil {
			sm.logger.Warn("hybrid search failed, fallback to simple search", logging.Err(err))
		} else {
			names := make([]string, 0, len(matches))
			for _, match := range matches {
				names = append(names, match.Skill.Name)
			}
			return names, nil
		}
	}

	// 回退：简单的关键词匹配
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0)
	for name, skill := range sm.skills {
		// 检查技能名称或描述是否包含关键词
		for _, keyword := range keywords {
			if containsSubstring(name, keyword) || containsSubstring(skill.Description, keyword) {
				names = append(names, name)
				break
			}
		}
	}

	return names, nil
}

// containsSubstring 检查字符串是否包含子串（简单实现）
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ExecuteFunc 执行工具函数（兼容旧接口）
// 注意：这是一个兼容方法，实际执行应该通过 ToolAssembler 完成
// 由于 SkillManager 不再负责执行，这个方法返回错误提示
func (sm *SkillManager) ExecuteFunc(function core.ToolCallFunction) (string, error) {
	sm.logger.Warn("ExecuteFunc called on SkillManager - this is deprecated",
		logging.String("function", function.Name),
	)

	// SkillManager 不再负责工具执行
	// 调用者应该使用 ToolAssembler 和 ToolExecutor
	return "", fmt.Errorf("SkillManager.ExecuteFunc is deprecated - use ToolAssembler and ToolExecutor instead")
}

// GetHybridSearcher 获取混合检索器
func (sm *SkillManager) GetHybridSearcher() *HybridSearcher {
	return sm.hybridSearcher
}

// SkillMgr 类型别名（向后兼容）
type SkillMgr = SkillManager

