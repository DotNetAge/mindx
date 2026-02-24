package skills

import (
	"context"
	"mindx/internal/config"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPStdioIntegration 使用官方 everything MCP server 测试完整链路：
// 连接 → 工具发现 → 注册到 loader → searcher 可搜索 → executor 可识别
//
// 需要 npx 可用，运行：
//
//	MINDX_WORKSPACE=$(pwd)/.test go test ./internal/usecase/skills/ -run TestMCPStdioIntegration -v -count=1
func TestMCPStdioIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（需要 npx 和网络）")
	}

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
	logger := logging.GetSystemLogger().Named("mcp_stdio_test")

	mgr, err := NewSkillMgr(skillsDir, tmpDir, nil, nil, logger)
	require.NoError(t, err)
	defer mgr.mcpMgr.Close()

	// 使用官方 everything MCP server（专门用于测试）
	entry := config.MCPServerEntry{
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-everything"},
		Enabled: true,
	}

	serverName := "everything"

	// === Step 1: 连接 MCP server ===
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("正在连接高德地图 MCP server（首次运行需要下载 npm 包）...")
	err = mgr.mcpMgr.ConnectServer(ctx, serverName, entry)
	require.NoError(t, err, "连接 MCP server 应成功")

	// === Step 2: 验证工具发现 ===
	state, ok := mgr.mcpMgr.GetServerState(serverName)
	require.True(t, ok, "应能获取 server 状态")
	assert.Equal(t, MCPServerStatusConnected, state.Status, "状态应为 connected")
	assert.Greater(t, len(state.Tools), 0, "应发现至少一个工具")

	t.Logf("发现 %d 个工具:", len(state.Tools))
	for _, tool := range state.Tools {
		t.Logf("  - %s: %s", tool.Name, tool.Description)
	}

	// === Step 3: 注册到 SkillMgr ===
	tools, err := mgr.mcpMgr.GetDiscoveredTools(serverName)
	require.NoError(t, err)

	defs := make([]*entity.SkillDef, 0, len(tools))
	for _, tool := range tools {
		defs = append(defs, MCPToolToSkillDef(serverName, tool))
	}

	mgr.loader.RegisterMCPSkills(serverName, defs)
	mgr.syncComponents()

	// === Step 4: 验证 loader 注册 ===
	allInfos := mgr.loader.GetSkillInfos()
	mcpCount := 0
	for name, info := range allInfos {
		if IsMCPSkill(info.Def) {
			mcpCount++
			t.Logf("已注册 MCP skill: %s (desc: %s)", name, info.Def.Description)
		}
	}
	assert.Equal(t, len(tools), mcpCount, "注册的 MCP skill 数量应与发现的工具数一致")

	// === Step 5: 验证 searcher 搜索 ===
	// 按 tag 搜索
	results, err := mgr.SearchSkills("everything")
	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "搜索 'everything' 应找到工具")
	t.Logf("搜索 'everything' 找到 %d 个结果", len(results))

	// 按 "mcp" 搜索
	results, err = mgr.SearchSkills("mcp")
	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "搜索 'mcp' 应找到工具")

	// === Step 6: 验证 executor 识别 ===
	for _, def := range defs {
		info, exists := mgr.GetSkillInfo(def.Name)
		assert.True(t, exists, "executor 应能找到: %s", def.Name)
		if exists {
			assert.True(t, IsMCPSkill(info.Def), "%s 应被识别为 MCP skill", def.Name)
		}
	}

	t.Log("完整链路验证通过")
}
