package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCostRegistry_SetGet(t *testing.T) {
	reg := NewCostRegistry()

	// Get missing
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get nonexistent should return false")
	}

	// Set and Get
	reg.Set("gpt-4", ModelCost{CostPer1MIn: 30, CostPer1MOut: 60})
	cost, ok := reg.Get("gpt-4")
	if !ok {
		t.Fatal("Get gpt-4 should return true")
	}
	if cost.CostPer1MIn != 30 {
		t.Errorf("CostPer1MIn = %f, want 30", cost.CostPer1MIn)
	}
	if cost.CostPer1MOut != 60 {
		t.Errorf("CostPer1MOut = %f, want 60", cost.CostPer1MOut)
	}
}

func TestCostRegistry_List(t *testing.T) {
	reg := NewCostRegistry()
	reg.Set("a", ModelCost{CostPer1MIn: 1})
	reg.Set("b", ModelCost{CostPer1MOut: 2})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(list))
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name                  string
		cost                  ModelCost
		input, output, cached int64
		want                  float64
	}{
		{
			name:   "only input tokens",
			cost:   ModelCost{CostPer1MIn: 10},
			input:  1_000_000,
			output: 0,
			want:   10.0,
		},
		{
			name:   "input and output",
			cost:   ModelCost{CostPer1MIn: 5, CostPer1MOut: 15},
			input:  1_000_000,
			output: 500_000,
			want:   5.0 + 7.5,
		},
		{
			name:   "with cached input",
			cost:   ModelCost{CostPer1MIn: 10, CostPer1MInCached: 1},
			input:  1_000_000,
			output: 0,
			cached: 500_000,
			want:   10.0 + 0.5,
		},
		{
			name:   "zero cost model",
			cost:   ModelCost{},
			input:  1_000_000,
			output: 1_000_000,
			want:   0.0,
		},
		{
			name:   "partial tokens",
			cost:   ModelCost{CostPer1MIn: 10},
			input:  100,
			output: 0,
			want:   10.0 / 1_000_000 * 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.cost, tt.input, tt.output, tt.cached)
			if got != tt.want {
				t.Errorf("CalculateCost = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestLoadCostsFromModelsFile(t *testing.T) {
	// Non-existent file
	reg, err := LoadCostsFromModelsFile("/tmp/nonexistent-models.yml")
	if err != nil {
		t.Fatalf("LoadCostsFromModelsFile on non-existent file: %v", err)
	}
	if reg == nil {
		t.Fatal("LoadCostsFromModelsFile returned nil reg")
	}

	// Valid file
	tmpDir := t.TempDir()
	ymlPath := filepath.Join(tmpDir, "models.yml")
	ymlContent := `
models:
  - name: gpt-4
    cost_per_1m_in: 30
    cost_per_1m_out: 60
  - name: gpt-3.5
    cost_per_1m_in: 1
    cost_per_1m_out: 2
  - name: free-model
`
	if err := os.WriteFile(ymlPath, []byte(ymlContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	reg, err = LoadCostsFromModelsFile(ymlPath)
	if err != nil {
		t.Fatalf("LoadCostsFromModelsFile failed: %v", err)
	}

	cost, ok := reg.Get("gpt-4")
	if !ok {
		t.Fatal("gpt-4 not found")
	}
	if cost.CostPer1MIn != 30 {
		t.Errorf("gpt-4 CostPer1MIn = %f, want 30", cost.CostPer1MIn)
	}

	// free-model has zero cost and should not be registered
	if _, ok := reg.Get("free-model"); ok {
		t.Error("free-model should not be registered (all costs are zero)")
	}
}
