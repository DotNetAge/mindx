package core

import (
	"testing"

	"github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
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

// NewTestAgentStore 创建一个包含指定 Agent 名称的 AgentStore（测试辅助函数）。
func NewTestAgentStore(t *testing.T, names ...string) *agentstore.AgentStore {
	t.Helper()
	tmpDir := t.TempDir()
	store, _, err := agentstore.Load(tmpDir)
	if err != nil {
		t.Fatalf("agentstore.Load failed: %v", err)
	}
	for _, name := range names {
		agent := &agentstore.Agent{
			Meta: agentstore.AgentMeta{
				Name:        name,
				Role:        "assistant",
				Description: "test agent " + name,
			},
		}
		if saveErr := store.Save(agent); saveErr != nil {
			t.Fatalf("Save(%q) failed: %v", name, saveErr)
		}
	}
	return store
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
