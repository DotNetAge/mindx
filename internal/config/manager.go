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
		SubconsciousModel:       m.globalConfig.Subconscious.Default,
		SubconsciousLeftModel:   m.globalConfig.Subconscious.Left,
		SubconsciousRightModel:  m.globalConfig.Subconscious.Right,
		ConsciousnessModel:      m.globalConfig.Consciousness.Default,
		ConsciousnessLeftModel:  m.globalConfig.Consciousness.Left,
		ConsciousnessRightModel: m.globalConfig.Consciousness.Right,
		MemoryModel:             m.globalConfig.MemoryModel,
		IndexModel:              m.globalConfig.IndexModel,
	}
}

func (m *ModelsManager) GetEmbeddingModel() string {
	return m.globalConfig.EmbeddingModel
}

func (m *ModelsManager) GetDefaultModel() string {
	return m.globalConfig.DefaultModel
}
