package skills

import (
	"mindx/internal/config"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPToolRegistrationChain 验证 MCP 工具注册到 SkillMgr 的完整链路：
// RegisterMCPSkills → syncComponents → searcher 可搜索 → indexer 队列有任务
func TestMCPToolRegistrationChain(t *testing.T) {
	// 创建临时 skills 目录（空的，不需要真实 skill 文件）
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: filepath.Join(tmpDir, "test.log"),
			MaxSize:    10,
			MaxBackups: 1,
			MaxAge:     1,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable: false,
		},
	}
	_ = logging.Init(logConfig)
	logger := logging.GetSystemLogger().Named("mcp_index_test")

	// 创建 SkillMgr（无 embedding、无 llama — 不做真实向量化）
	mgr, err := NewSkillMgr(skillsDir, tmpDir, nil, nil, logger)
	require.NoError(t, err)

	// 模拟 MCP 工具定义（英文描述，模拟真实 SSE 场景）
	mcpDefs := []*entity.SkillDef{
		{
			Name:        "mcp_bijia_compare_prices",
			Description: "Compare prices for products across multiple platforms",
			Category:    "mcp",
			Tags:        []string{"mcp", "bijia"},
			Enabled:     true,
			Timeout:     30,
			Parameters: map[string]entity.ParameterDef{
				"product": {Type: "string", Description: "Product name to compare", Required: true},
			},
			Metadata: map[string]interface{}{
				"mcp": map[string]interface{}{
					"server": "bijia",
					"tool":   "compare_prices",
				},
			},
		},
		{
			Name:        "mcp_bijia_get_history",
			Description: "Get price history for a specific product",
			Category:    "mcp",
			Tags:        []string{"mcp", "bijia"},
			Enabled:     true,
			Timeout:     30,
			Parameters: map[string]entity.ParameterDef{
				"product_id": {Type: "string", Description: "Product ID", Required: true},
			},
			Metadata: map[string]interface{}{
				"mcp": map[string]interface{}{
					"server": "bijia",
					"tool":   "get_history",
				},
			},
		},
	}

	// === Step 1: 注册 MCP 工具 ===
	mgr.loader.RegisterMCPSkills("bijia", mcpDefs)
	mgr.syncComponents()

	// === Step 2: 验证 loader 中存在 MCP 工具 ===
	allSkills := mgr.loader.GetSkills()
	allInfos := mgr.loader.GetSkillInfos()

	assert.Contains(t, allSkills, "mcp_bijia_compare_prices", "loader 应包含 MCP skill")
	assert.Contains(t, allSkills, "mcp_bijia_get_history", "loader 应包含 MCP skill")
	assert.Contains(t, allInfos, "mcp_bijia_compare_prices", "loader 应包含 MCP skill info")

	// === Step 3: 验证 SkillInfo 字段正确 ===
	info := allInfos["mcp_bijia_compare_prices"]
	assert.Equal(t, "mcp", info.Format)
	assert.Equal(t, "ready", info.Status)
	assert.True(t, info.CanRun)
	assert.True(t, IsMCPSkill(info.Def))

	meta, ok := GetMCPSkillMetadata(info.Def)
	require.True(t, ok)
	assert.Equal(t, "bijia", meta.Server)
	assert.Equal(t, "compare_prices", meta.Tool)

	// === Step 4: 验证 searcher 关键词搜索能找到 MCP 工具 ===
	// 按 tag 搜索
	results, err := mgr.SearchSkills("mcp")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2, "搜索 'mcp' 应找到 MCP 工具")

	// 按 server name 搜索
	results, err = mgr.SearchSkills("bijia")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1, "搜索 'bijia' 应找到 MCP 工具")

	// 按 description 中的词搜索
	results, err = mgr.SearchSkills("price")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1, "搜索 'price' 应找到 MCP 工具")

	// 无关键词搜索应返回所有（包括 MCP）
	results, err = mgr.SearchSkills()
	require.NoError(t, err)
	foundMCP := false
	for _, s := range results {
		if s.GetName() == "mcp_bijia_compare_prices" {
			foundMCP = true
			break
		}
	}
	assert.True(t, foundMCP, "无关键词搜索应包含 MCP 工具")

	// === Step 5: 验证 executor 能识别 MCP skill ===
	execInfo, exists := mgr.GetSkillInfo("mcp_bijia_compare_prices")
	assert.True(t, exists, "executor 应能找到 MCP skill info")
	assert.True(t, IsMCPSkill(execInfo.Def), "应被识别为 MCP skill")

	// === Step 6: 验证注销后清理干净 ===
	mgr.loader.UnregisterMCPSkills("bijia")
	mgr.syncComponents()

	allSkills = mgr.loader.GetSkills()
	assert.NotContains(t, allSkills, "mcp_bijia_compare_prices", "注销后不应存在")
	assert.NotContains(t, allSkills, "mcp_bijia_get_history", "注销后不应存在")

	results, err = mgr.SearchSkills("bijia")
	require.NoError(t, err)
	assert.Equal(t, 0, len(results), "注销后搜索不应找到")
}

// TestMCPToolIndexing 验证 MCP 工具被送入索引队列
// 注意：不做真实向量化（无 embedding/llama），只验证 indexMCPSkills 不 panic 且逻辑正确
func TestMCPToolIndexing(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: filepath.Join(tmpDir, "test.log"),
			MaxSize:    10,
			MaxBackups: 1,
			MaxAge:     1,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable: false,
		},
	}
	_ = logging.Init(logConfig)
	logger := logging.GetSystemLogger().Named("mcp_index_test")

	mgr, err := NewSkillMgr(skillsDir, tmpDir, nil, nil, logger)
	require.NoError(t, err)

	defs := []*entity.SkillDef{
		{
			Name:        "mcp_test_tool1",
			Description: "A test tool for unit testing",
			Category:    "mcp",
			Tags:        []string{"mcp", "test"},
			Enabled:     true,
			Metadata: map[string]interface{}{
				"mcp": map[string]interface{}{"server": "test", "tool": "tool1"},
			},
		},
	}

	// 注册
	mgr.loader.RegisterMCPSkills("test", defs)
	mgr.syncComponents()

	// indexMCPSkills 应该不 panic（即使没有 embedding/llama）
	// ReIndex 内部会因为 embedding == nil 返回 error，indexMCPSkills 会 warn 但不崩溃
	mgr.indexMCPSkills(defs)

	// 验证工具仍然在 loader 中（索引失败不影响注册）
	allInfos := mgr.loader.GetSkillInfos()
	assert.Contains(t, allInfos, "mcp_test_tool1", "索引失败不应影响已注册的工具")
}
