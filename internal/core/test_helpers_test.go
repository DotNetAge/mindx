package core

import (
	"testing"

	"github.com/DotNetAge/goreact/config"
)

// NewTestModelRegistry 创建一个包含指定模型名称的 ModelRegistry（测试辅助函数）。
func NewTestModelRegistry(modelNames ...string) *config.ModelRegistry {
	reg := &config.ModelRegistry{}
	for _, name := range modelNames {
		reg.Register(name, &config.ModelConfig{
			Name:        name,
			Description: "test model " + name,
			Provider:    "test-provider",
			Enabled:     true,
		})
	}
	return reg
}

// NewTestAgentRegistry 创建一个包含指定 Agent 名称的 AgentRegistry（测试辅助函数）。
func NewTestAgentRegistry(t *testing.T, names ...string) *config.AgentRegistry {
	t.Helper()
	tmpDir := t.TempDir()
	reg, err := config.LoadAgentsFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadAgentsFrom failed: %v", err)
	}
	for _, name := range names {
		if saveErr := reg.SaveTo(&config.AgentConfig{
			Name:        name,
			Role:        "assistant",
			Description: "test agent " + name,
			Model:       "gpt-4",
		}); saveErr != nil {
			t.Fatalf("SaveTo(%q) failed: %v", name, saveErr)
		}
	}
	return reg
}

// contains 检查字符串 s 是否包含 substr。
func contains(s, substr string) bool {
	n := len(s)
	m := len(substr)
	if m > n {
		return false
	}
	for i := 0; i <= n-m; i++ {
		if s[i:i+m] == substr {
			return true
		}
	}
	return false
}
