package skills

import (
	"mindx/internal/entity"
	"mindx/internal/usecase/tools"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolAssembler_AssembleTools(t *testing.T) {
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
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	// 创建 ToolManager
	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	// 创建 ToolAssembler
	assembler := NewToolAssembler(toolManager, nil)

	// 创建测试 Skill
	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"test_tool"},
	}

	// 组装工具
	schemas, err := assembler.AssembleTools(skill)

	require.NoError(t, err)
	assert.Len(t, schemas, 1)
	assert.Equal(t, "test_tool", schemas[0].Function.Name)
}

func TestToolAssembler_AssembleTools_MissingRequired(t *testing.T) {
	assembler := NewToolAssembler(nil, nil)

	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"nonexistent_tool"},
	}

	_, err := assembler.AssembleTools(skill)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required tools not found")
}

func TestToolAssembler_AssembleTools_OptionalMissing(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "测试工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := NewToolAssembler(toolManager, nil)

	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"test_tool"},
		OptionalTools: []string{"optional_tool"}, // 不存在
	}

	schemas, err := assembler.AssembleTools(skill)

	// 可选工具缺失不应该报错
	require.NoError(t, err)
	assert.Len(t, schemas, 1) // 只有必需工具
}

func TestToolAssembler_HasTool(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "测试工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := NewToolAssembler(toolManager, nil)

	assert.True(t, assembler.HasTool("test_tool"))
	assert.False(t, assembler.HasTool("nonexistent_tool"))
}

func TestToolAssembler_ListTools(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	// 创建两个工具
	for _, name := range []string{"tool1", "tool2"} {
		toolDir := filepath.Join(toolsDir, name)
		require.NoError(t, os.MkdirAll(toolDir, 0755))

		toolJSON := `{
			"name": "` + name + `",
			"description": "测试工具",
			"version": "1.0.0",
			"type": "shell",
			"command": "echo.sh"
		}`

		require.NoError(t, os.WriteFile(
			filepath.Join(toolDir, "tool.json"),
			[]byte(toolJSON),
			0644,
		))
	}

	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := NewToolAssembler(toolManager, nil)

	tools := assembler.ListTools()
	assert.Len(t, tools, 2)
	assert.Contains(t, tools, "tool1")
	assert.Contains(t, tools, "tool2")
}

func TestToolAssembler_GetToolCount(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "测试工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := NewToolAssembler(toolManager, nil)

	local, mcp, total := assembler.GetToolCount()
	assert.Equal(t, 1, local)
	assert.Equal(t, 0, mcp)
	assert.Equal(t, 1, total)
}

func TestToolAssembler_ValidateSkillTools(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testToolDir := filepath.Join(toolsDir, "test_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	toolJSON := `{
		"name": "test_tool",
		"description": "测试工具",
		"version": "1.0.0",
		"type": "shell",
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	toolManager := tools.NewToolManager(toolsDir)
	require.NoError(t, toolManager.LoadTools())

	assembler := NewToolAssembler(toolManager, nil)

	skill := &entity.Skill{
		Name:          "test_skill",
		RequiredTools: []string{"test_tool", "missing_tool"},
		OptionalTools: []string{"optional_tool"},
	}

	missing, optional := assembler.ValidateSkillTools(skill)

	assert.Len(t, missing, 1)
	assert.Contains(t, missing, "missing_tool")
	assert.Len(t, optional, 1)
	assert.Contains(t, optional, "optional_tool")
}

func TestToolAssembler_NilManagers(t *testing.T) {
	// 测试没有任何管理器的情况
	assembler := NewToolAssembler(nil, nil)

	assert.False(t, assembler.HasTool("any_tool"))
	assert.Empty(t, assembler.ListTools())

	local, mcp, total := assembler.GetToolCount()
	assert.Equal(t, 0, local)
	assert.Equal(t, 0, mcp)
	assert.Equal(t, 0, total)
}
