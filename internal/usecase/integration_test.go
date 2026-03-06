package usecase

import (
	"mindx/internal/entity"
	"mindx/internal/usecase/mcp"
	"mindx/internal/usecase/skills"
	"mindx/internal/usecase/tools"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolManagerIntegration 测试 ToolManager 集成
func TestToolManagerIntegration(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	// 创建测试工具
	testToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "测试工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo",
		"parameters": {},
		"timeout": 10
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	// 1. 加载工具
	toolManager := tools.NewToolManager(toolsDir)
	err := toolManager.LoadTools()
	require.NoError(t, err)

	// 2. 验证工具加载
	assert.True(t, toolManager.HasTool("test_tool"))
	assert.Equal(t, 1, toolManager.GetToolCount())

	// 3. 创建 ToolAssembler
	assembler := skills.NewToolAssembler(toolManager, nil)

	// 4. 组装工具
	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"test_tool"},
	}

	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)
	assert.Len(t, schemas, 1)
	assert.Equal(t, "test_tool", schemas[0].Function.Name)
}

// TestToolAssemblerIntegration 测试 ToolAssembler 集成
func TestToolAssemblerIntegration(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")

	// 创建多个测试工具
	toolNames := []string{"tool1", "tool2", "tool3"}
	for _, toolName := range toolNames {
		toolDir := filepath.Join(toolsDir, toolName)
		require.NoError(t, os.MkdirAll(toolDir, 0755))

		toolJSON := `{
			"name": "` + toolName + `",
			"description": "测试工具",
			"version": "1.0.0",
			"type": "shell",
			"command": "echo"
		}`

		require.NoError(t, os.WriteFile(
			filepath.Join(toolDir, "tool.json"),
			[]byte(toolJSON),
			0644,
		))
	}

	// 1. 加载工具
	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	// 2. 创建 ToolAssembler
	assembler := skills.NewToolAssembler(toolManager, nil)

	// 3. 测试必需工具和可选工具
	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"tool1", "tool2"},
		OptionalTools: []string{"tool3", "missing_tool"},
	}

	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)

	// 应该有 3 个工具（2 个必需 + 1 个可选，missing_tool 被跳过）
	assert.Len(t, schemas, 3)

	// 验证工具名称
	assembledTools := make([]string, len(schemas))
	for i, schema := range schemas {
		assembledTools[i] = schema.Function.Name
	}
	assert.Contains(t, assembledTools, "tool1")
	assert.Contains(t, assembledTools, "tool2")
	assert.Contains(t, assembledTools, "tool3")
}

// TestMCPManagerIntegration 测试 MCPManager 集成
func TestMCPManagerIntegration(t *testing.T) {
	t.Skip("需要实际的 MCP 服务器")

	// 创建临时配置
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp_servers.json")

	config := `{
		"servers": {
			"test_server": {
				"name": "test_server",
				"command": "node",
				"args": ["server.js"]
			}
		}
	}`

	require.NoError(t, os.WriteFile(configPath, []byte(config), 0644))

	// 1. 加载配置
	mcpManager := mcp.NewMCPManager(configPath)
	err := mcpManager.LoadConfig()
	require.NoError(t, err)

	// 2. 连接所有服务器（会自动发现工具）
	err = mcpManager.Connect()
	require.NoError(t, err)

	// 3. 验证工具
	assert.True(t, mcpManager.GetToolCount() > 0)

	// 4. 清理
	defer mcpManager.Close()
}

// TestFullPipeline 测试完整流程
func TestFullPipeline(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")

	// 创建测试工具
	calculatorDir := filepath.Join(toolsDir, "calculator")
	require.NoError(t, os.MkdirAll(calculatorDir, 0755))

	toolJSON := `{
		"name": "calculator",
		"description": "计算器",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo",
		"parameters": {
			"expression": {
				"type": "string",
				"required": true
			}
		}
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(calculatorDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	// 1. 初始化组件
	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := skills.NewToolAssembler(toolManager, nil)

	// 2. 创建 Skill
	skill := &entity.Skill{
		Name:          "math_calculation",
		Description:   "数学计算",
		RequiredTools: []string{"calculator"},
	}

	// 3. 组装工具
	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)
	assert.Len(t, schemas, 1)

	// 4. 验证工具 Schema
	assert.Equal(t, "calculator", schemas[0].Function.Name)
	assert.Equal(t, "计算器", schemas[0].Function.Description)
	assert.NotNil(t, schemas[0].Function.Parameters)
}

// TestToolPriority 测试工具优先级
func TestToolPriority(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")

	// 创建本地工具
	localToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(localToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "本地工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(localToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	// 1. 加载本地工具
	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	// 2. 创建 ToolAssembler（没有 MCP）
	assembler := skills.NewToolAssembler(toolManager, nil)

	// 3. 验证本地工具优先
	assert.True(t, assembler.HasTool("test_tool"))

	// 4. 通过组装工具来验证本地工具被使用
	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"test_tool"},
	}

	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)
	assert.Len(t, schemas, 1)
	assert.Equal(t, "test_tool", schemas[0].Function.Name)
	assert.Equal(t, "本地工具", schemas[0].Function.Description)
}
