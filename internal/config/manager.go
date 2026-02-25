package config

import (
	"fmt"
	"sync"
)

var (
	modelsManager *ModelsManager
	modelsOnce    sync.Once
)

type ModelsManager struct {
	modelsConfig *ModelsConfig
	globalConfig *GlobalConfig
	modelMap     map[string]*ModelConfig
}

func NewModelsManager(modelsCfg *ModelsConfig, globalCfg *GlobalConfig) *ModelsManager {
	m := &ModelsManager{
		modelsConfig: modelsCfg,
		globalConfig: globalCfg,
		modelMap:     make(map[string]*ModelConfig),
	}
	for i := range modelsCfg.Models {
		m.modelMap[modelsCfg.Models[i].Name] = &modelsCfg.Models[i]
	}
	return m
}

func SetModelsManager(mgr *ModelsManager) {
	modelsOnce.Do(func() {
		modelsManager = mgr
	})
}

func GetModelsManager() *ModelsManager {
	return modelsManager
}

// OverrideModelsManager 强制覆盖 ModelsManager（仅用于测试）
func OverrideModelsManager(mgr *ModelsManager) {
	modelsOnce = sync.Once{}
	modelsManager = mgr
}

func (m *ModelsManager) GetModel(name string) (*ModelConfig, error) {
	if model, ok := m.modelMap[name]; ok {
		return model, nil
	}
	return nil, fmt.Errorf("model not found: %s", name)
}

func (m *ModelsManager) MustGetModel(name string) *ModelConfig {
	model, err := m.GetModel(name)
	if err != nil {
		panic(err)
	}
	return model
}

func (m *ModelsManager) ListModels() []ModelConfig {
	return m.modelsConfig.Models
}

func (m *ModelsManager) GetBrainModels() BrainModelsConfig {
	return BrainModelsConfig{
		SubconsciousLeftModel:   m.globalConfig.Subconscious.Left,
		SubconsciousRightModel:  m.globalConfig.Subconscious.Right,
		ConsciousnessLeftModel:  m.globalConfig.Consciousness.Left,
		ConsciousnessRightModel: m.globalConfig.Consciousness.Right,
	}
}

func (m *ModelsManager) GetEmbeddingModel() string {
	return m.globalConfig.EmbeddingModel
}

func (m *ModelsManager) GetDefaultModel() string {
	return m.globalConfig.DefaultModel
}
