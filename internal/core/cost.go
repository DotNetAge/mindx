package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ModelCost struct {
	CostPer1MIn        float64 `yaml:"cost_per_1m_in"`
	CostPer1MOut       float64 `yaml:"cost_per_1m_out"`
	CostPer1MInCached  float64 `yaml:"cost_per_1m_in_cached"`
	CostPer1MOutCached float64 `yaml:"cost_per_1m_out_cached"`
}

type CostRegistry struct {
	costs map[string]ModelCost
}

func NewCostRegistry() *CostRegistry {
	return &CostRegistry{
		costs: make(map[string]ModelCost),
	}
}

func (r *CostRegistry) Set(modelName string, cost ModelCost) {
	r.costs[modelName] = cost
}

func (r *CostRegistry) Get(modelName string) (ModelCost, bool) {
	c, ok := r.costs[modelName]
	return c, ok
}

type NamedCost struct {
	Name string
	ModelCost
}

func (r *CostRegistry) List() []NamedCost {
	result := make([]NamedCost, 0, len(r.costs))
	for k, v := range r.costs {
		result = append(result, NamedCost{Name: k, ModelCost: v})
	}
	return result
}

type costModelEntry struct {
	Name string `yaml:"name"`
	ModelCost    `yaml:",inline"`
}

type costFile struct {
	Models []costModelEntry `yaml:"models"`
}

func LoadCostsFromModelsFile(path string) (*CostRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewCostRegistry(), nil
		}
		return nil, fmt.Errorf("failed to read models file for costs: %w", err)
	}

	var parsed costFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse models costs: %w", err)
	}

	reg := NewCostRegistry()
	for _, m := range parsed.Models {
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 || m.CostPer1MInCached != 0 || m.CostPer1MOutCached != 0 {
			reg.Set(m.Name, m.ModelCost)
		}
	}
	return reg, nil
}

func CalculateCost(modelCost ModelCost, inputTokens, outputTokens, cachedInputTokens int64) float64 {
	cost := 0.0
	if modelCost.CostPer1MIn > 0 {
		cost += modelCost.CostPer1MIn / 1_000_000 * float64(inputTokens)
	}
	if modelCost.CostPer1MOut > 0 {
		cost += modelCost.CostPer1MOut / 1_000_000 * float64(outputTokens)
	}
	if modelCost.CostPer1MInCached > 0 && cachedInputTokens > 0 {
		cost += modelCost.CostPer1MInCached / 1_000_000 * float64(cachedInputTokens)
	}
	return cost
}
