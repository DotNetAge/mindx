package capability

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/entity"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/embedding"
	"os"
	"path/filepath"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// CapabilityManager 能力管理器
type CapabilityManager struct {
	capabilities map[string]*entity.Capability
	clients      map[string]*openai.Client
	embeddingSvc *embedding.EmbeddingService
	vectorStore  persistence.Store
	configPath   string
	defaultName  string
	fallback     bool
	mu           sync.RWMutex
	isReIndexing bool
	reIndexError error
}

// NewManager 创建能力管理器
func NewManager(cfg *config.CapabilityConfig, vectorStore persistence.Store, embeddingSvc *embedding.EmbeddingService, workspace string) (*CapabilityManager, error) {
	configPath, err := config.GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}
	configPath = filepath.Join(configPath, "capabilities.yml")

	mgr := &CapabilityManager{
		capabilities: make(map[string]*entity.Capability),
		clients:      make(map[string]*openai.Client),
		embeddingSvc: embeddingSvc,
		vectorStore:  vectorStore,
		configPath:   configPath,
		defaultName:  cfg.DefaultCapability,
		fallback:     cfg.FallbackToLocal,
	}

	// 创建工作区目录（如果不存在）
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("创建工作区目录失败: %w", err)
	}

	// 检查工作区配置文件是否存在
	var finalCfg *config.CapabilityConfig
	if _, err := os.Stat(configPath); err == nil {
		// 工作区配置文件存在，从工作区加载
		workspaceCfg, err := config.LoadCapabilitiesConfig()
		if err != nil {
			return nil, fmt.Errorf("读取工作区配置文件失败: %w", err)
		}

		finalCfg = workspaceCfg
		mgr.defaultName = workspaceCfg.DefaultCapability
		mgr.fallback = workspaceCfg.FallbackToLocal
	} else {
		// 工作区配置文件不存在，使用传入的默认配置
		finalCfg = cfg
		// 保存默认配置到工作区
		mgr.capabilities = make(map[string]*entity.Capability)
		for i := range finalCfg.Capabilities {
			capConfig := &finalCfg.Capabilities[i]
			cap := &entity.Capability{
				Name:         capConfig.Name,
				Title:        capConfig.Title,
				Icon:         capConfig.Icon,
				Description:  capConfig.Description,
				Model:        capConfig.Model,
				SystemPrompt: capConfig.SystemPrompt,
				Tools:        capConfig.Tools,
				Modality:     capConfig.Modality,
				Enabled:      capConfig.Enabled,
				Vector:       capConfig.Vector,
			}
			mgr.capabilities[cap.Name] = cap
		}
		if err := mgr.saveToConfigFile(); err != nil {
			return nil, fmt.Errorf("保存默认配置到工作区失败: %w", err)
		}
	}

	// 验证配置
	if err := validateConfig(finalCfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 重建 capabilities map（如果不是从默认配置初始化的）
	if len(mgr.capabilities) == 0 {
		for i := range finalCfg.Capabilities {
			capConfig := &finalCfg.Capabilities[i]
			cap := &entity.Capability{
				Name:         capConfig.Name,
				Title:        capConfig.Title,
				Icon:         capConfig.Icon,
				Description:  capConfig.Description,
				Model:        capConfig.Model,
				SystemPrompt: capConfig.SystemPrompt,
				Tools:        capConfig.Tools,
				Modality:     capConfig.Modality,
				Enabled:      capConfig.Enabled,
				Vector:       capConfig.Vector,
			}
			mgr.capabilities[cap.Name] = cap
		}
	}

	// 初始化客户端
	mgr.initClients()

	return mgr, nil
}

// initClients 初始化 OpenAI 客户端
// 注意：调用者必须持有 m.mu 锁
func (m *CapabilityManager) initClients() {
	modelsMgr := config.GetModelsManager()
	for name, cap := range m.capabilities {
		if !cap.Enabled {
			continue
		}

		modelConfig, err := modelsMgr.GetModel(cap.Model)
		if err != nil {
			continue
		}

		var client *openai.Client
		config := openai.DefaultConfig(modelConfig.APIKey)
		config.BaseURL = modelConfig.BaseURL
		client = openai.NewClientWithConfig(config)

		m.clients[name] = client
	}
}

// saveToConfigFile 保存配置到文件
// 注意：调用者必须持有 m.mu 锁
func (m *CapabilityManager) saveToConfigFile() error {
	// 构建配置
	caps := make([]config.Capability, 0, len(m.capabilities))
	for _, cap := range m.capabilities {
		caps = append(caps, config.Capability{
			Name:         cap.Name,
			Title:        cap.Title,
			Icon:         cap.Icon,
			Description:  cap.Description,
			Model:        cap.Model,
			SystemPrompt: cap.SystemPrompt,
			Tools:        cap.Tools,
			Modality:     cap.Modality,
			Enabled:      cap.Enabled,
			Vector:       cap.Vector,
		})
	}

	cfg := &config.CapabilityConfig{
		Capabilities:      caps,
		DefaultCapability: m.defaultName,
		FallbackToLocal:   m.fallback,
	}

	return config.SaveCapabilitiesConfig(cfg)
}

// GetCapability 获取能力配置
func (m *CapabilityManager) GetCapability(name string) (*entity.Capability, bool) {
	cap, exists := m.capabilities[name]
	return cap, exists
}

// GetClient 获取能力的客户端
func (m *CapabilityManager) GetClient(name string) (*openai.Client, error) {
	cap, exists := m.capabilities[name]
	if !exists {
		return nil, fmt.Errorf("能力 '%s' 不存在", name)
	}

	if !cap.Enabled {
		return nil, fmt.Errorf("能力 '%s' 已禁用", name)
	}

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("能力 '%s' 的客户端未初始化", name)
	}

	return client, nil
}

// ListCapabilities 列出所有能力
func (m *CapabilityManager) ListCapabilities() []entity.Capability {
	var result []entity.Capability
	for _, cap := range m.capabilities {
		result = append(result, *cap)
	}
	return result
}

// ListEnabledCapabilities 列出启用的能力
func (m *CapabilityManager) ListEnabledCapabilities() []entity.Capability {
	var result []entity.Capability
	for _, cap := range m.capabilities {
		if cap.Enabled {
			result = append(result, *cap)
		}
	}
	return result
}

// GetDefaultCapability 获取默认能力
func (m *CapabilityManager) GetDefaultCapability(defaultName string) (*entity.Capability, error) {
	if defaultName == "" {
		return nil, fmt.Errorf("未指定默认能力名称")
	}

	cap, exists := m.capabilities[defaultName]
	if !exists {
		return nil, fmt.Errorf("默认能力 '%s' 不存在", defaultName)
	}

	if !cap.Enabled {
		return nil, fmt.Errorf("默认能力 '%s' 已禁用", defaultName)
	}

	return cap, nil
}

// ShouldFallbackToLocal 是否应该回退到本地
func (m *CapabilityManager) ShouldFallbackToLocal(fallback bool) bool {
	return fallback
}

// EnableCapability 启用能力
func (m *CapabilityManager) EnableCapability(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cap, exists := m.capabilities[name]
	if !exists {
		return fmt.Errorf("能力 '%s' 不存在", name)
	}

	cap.Enabled = true
	m.initClients()

	return m.saveToConfigFile()
}

// DisableCapability 禁用能力
func (m *CapabilityManager) DisableCapability(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cap, exists := m.capabilities[name]
	if !exists {
		return fmt.Errorf("能力 '%s' 不存在", name)
	}

	cap.Enabled = false
	delete(m.clients, name)

	return m.saveToConfigFile()
}

// AddCapability 添加新能力
func (m *CapabilityManager) AddCapability(cap entity.Capability) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.capabilities[cap.Name]; exists {
		return fmt.Errorf("能力 '%s' 已存在", cap.Name)
	}

	m.capabilities[cap.Name] = &cap

	if cap.Enabled {
		m.initClients()
	}

	return m.saveToConfigFile()
}

// UpdateCapability 更新能力
func (m *CapabilityManager) UpdateCapability(name string, updates entity.Capability) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cap, exists := m.capabilities[name]
	if !exists {
		return fmt.Errorf("能力 '%s' 不存在", name)
	}

	// 更新字段
	if updates.Title != "" {
		cap.Title = updates.Title
	}
	if updates.Icon != "" {
		cap.Icon = updates.Icon
	}
	if updates.Description != "" {
		cap.Description = updates.Description
	}
	if updates.Model != "" {
		cap.Model = updates.Model
	}
	if updates.SystemPrompt != "" {
		cap.SystemPrompt = updates.SystemPrompt
	}

	m.initClients()

	return m.saveToConfigFile()
}

// RemoveCapability 移除能力
func (m *CapabilityManager) RemoveCapability(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.capabilities[name]; !exists {
		return fmt.Errorf("能力 '%s' 不存在", name)
	}

	delete(m.capabilities, name)
	delete(m.clients, name)

	return m.saveToConfigFile()
}

// validateConfig 验证配置
func validateConfig(config *config.CapabilityConfig) error {
	// 检查是否有能力配置
	if len(config.Capabilities) == 0 {
		return fmt.Errorf("配置中未定义任何能力")
	}

	// 检查能力名称是否唯一
	names := make(map[string]bool)
	for _, cap := range config.Capabilities {
		if cap.Name == "" {
			return fmt.Errorf("能力名称为空")
		}
		if names[cap.Name] {
			return fmt.Errorf("能力名称重复: %s", cap.Name)
		}
		names[cap.Name] = true

		// 验证必需字段
		if cap.Model == "" {
			return fmt.Errorf("能力 %s: 模型名称为必需字段", cap.Name)
		}
		if cap.SystemPrompt == "" {
			return fmt.Errorf("能力 %s: 系统提示词为必需字段", cap.Name)
		}
	}

	// 检查默认能力是否存在
	if config.DefaultCapability != "" {
		found := false
		for _, cap := range config.Capabilities {
			if cap.Name == config.DefaultCapability {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("默认能力 '%s' 不在能力列表中", config.DefaultCapability)
		}
	}

	return nil
}

// Close 关闭能力管理器
func (m *CapabilityManager) Close() {
	m.clients = make(map[string]*openai.Client)
}

// PrecomputeVectors 预计算所有能力的向量
// 从 system_prompt 提取语义，因为 system_prompt 包含更丰富的能力描述
func (m *CapabilityManager) PrecomputeVectors() error {
	if m.embeddingSvc == nil {
		return fmt.Errorf("向量化服务未设置")
	}

	entries := make([]entity.VectorEntry, 0, len(m.capabilities))
	for _, cap := range m.capabilities {
		if len(cap.Vector) == 0 {
			searchText := fmt.Sprintf("%s %s", cap.Description, cap.SystemPrompt)
			vec, err := m.embeddingSvc.GenerateEmbedding(searchText)
			if err != nil {
				return fmt.Errorf("生成能力 %s 的向量失败: %w", cap.Name, err)
			}
			cap.Vector = vec
		}

		entries = append(entries, entity.VectorEntry{
			Key:    "capability:" + cap.Name,
			Vector: cap.Vector,
		})
	}

	if len(entries) > 0 {
		if err := m.vectorStore.BatchPut(entries); err != nil {
			return fmt.Errorf("存储向量失败: %w", err)
		}
	}

	return nil
}

// RouteCapability 基于查询文本路由到最匹配的能力
func (m *CapabilityManager) RouteCapability(query string, topN int) ([]*entity.Capability, error) {
	if m.embeddingSvc == nil {
		return nil, nil
	}

	queryVec, err := m.embeddingSvc.GenerateEmbedding(query)
	if err != nil {
		return nil, nil
	}

	entries, err := m.vectorStore.Search(queryVec, topN)
	if err != nil {
		return nil, nil
	}

	if len(entries) == 0 {
		return nil, nil
	}

	caps := make([]*entity.Capability, 0, len(entries))
	for _, entry := range entries {
		capName := entry.Key
		if len(capName) > len("capability:") && capName[:len("capability:")] == "capability:" {
			capName = capName[len("capability:"):]
		}
		if cap, exists := m.capabilities[capName]; exists && cap.Enabled {
			caps = append(caps, cap)
		}
	}

	if len(caps) == 0 {
		return nil, nil
	}

	return caps, nil
}

// routeByDefault 默认路由（返回启用的能力）
func (m *CapabilityManager) routeByDefault(topN int) []*entity.Capability {
	var enabledCaps []*entity.Capability
	for _, cap := range m.capabilities {
		if cap.Enabled {
			enabledCaps = append(enabledCaps, cap)
		}
	}

	if topN > len(enabledCaps) {
		topN = len(enabledCaps)
	}

	return enabledCaps[:topN]
}

// ReIndex 重新计算所有能力的向量
func (m *CapabilityManager) ReIndex() error {
	m.mu.Lock()
	m.isReIndexing = true
	m.reIndexError = nil
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isReIndexing = false
		m.mu.Unlock()
	}()

	if m.embeddingSvc == nil {
		m.mu.Lock()
		m.reIndexError = fmt.Errorf("向量化服务未设置")
		m.mu.Unlock()
		return m.reIndexError
	}

	entries := make([]entity.VectorEntry, 0, len(m.capabilities))
	for _, cap := range m.capabilities {
		searchText := fmt.Sprintf("%s %s", cap.Description, cap.SystemPrompt)
		vec, err := m.embeddingSvc.GenerateEmbedding(searchText)
		if err != nil {
			m.mu.Lock()
			m.reIndexError = fmt.Errorf("生成能力 %s 的向量失败: %w", cap.Name, err)
			m.mu.Unlock()
			return m.reIndexError
		}
		cap.Vector = vec

		entries = append(entries, entity.VectorEntry{
			Key:    "capability:" + cap.Name,
			Vector: cap.Vector,
		})
	}

	if len(entries) > 0 {
		if err := m.vectorStore.BatchPut(entries); err != nil {
			m.mu.Lock()
			m.reIndexError = fmt.Errorf("存储向量失败: %w", err)
			m.mu.Unlock()
			return m.reIndexError
		}
	}

	return nil
}

// IsReIndexing 返回是否正在重新索引
func (m *CapabilityManager) IsReIndexing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isReIndexing
}

// GetReIndexError 返回重新索引的错误
func (m *CapabilityManager) GetReIndexError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reIndexError
}

// StartReIndexInBackground 在后台启动重新索引
func (m *CapabilityManager) StartReIndexInBackground() {
	go func() {
		if err := m.ReIndex(); err != nil {
			// 错误已在 ReIndex 中记录
		}
	}()
}

func (m *CapabilityManager) LoadVectorsFromStore() error {
	if m.vectorStore == nil {
		return nil
	}

	entries, err := m.vectorStore.Scan("capability:")
	if err != nil {
		return fmt.Errorf("加载能力向量失败: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	loadedCount := 0
	for _, entry := range entries {
		capName := entry.Key[len("capability:"):]
		cap, exists := m.capabilities[capName]
		if !exists {
			continue
		}

		if len(entry.Vector) > 0 {
			cap.Vector = entry.Vector
			loadedCount++
		}
	}

	return nil
}

func (m *CapabilityManager) HasVectors() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cap := range m.capabilities {
		if len(cap.Vector) > 0 {
			return true
		}
	}
	return false
}
