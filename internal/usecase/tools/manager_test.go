package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolManager_LoadTools(t *testing.T) {
	// 创建临时工具目录
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
		"command": "echo.sh",
		"parameters": {
			"type": "object",
			"properties": {
				"message": {
					"type": "string"
				}
			}
		},
		"timeout": 10
	}`

	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(toolJSON),
		0644,
	))

	// 创建 ToolManager
	tm := NewToolManager(toolsDir)

	// 加载工具
	err := tm.LoadTools()
	require.NoError(t, err)

	// 验证
	assert.Equal(t, 1, tm.GetToolCount())
	assert.True(t, tm.HasTool("test_tool"))

	tool, err := tm.GetTool("test_tool")
	require.NoError(t, err)
	assert.Equal(t, "test_tool", tool.Name)
	assert.Equal(t, "shell", tool.Type)
	assert.Equal(t, 10, tool.Timeout)
}

func TestToolManager_LoadTools_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	tm := NewToolManager(toolsDir)
	err := tm.LoadTools()

	require.NoError(t, err)
	assert.Equal(t, 0, tm.GetToolCount())
}

func TestToolManager_LoadTools_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "nonexistent")

	tm := NewToolManager(toolsDir)
	err := tm.LoadTools()

	// 不存在的目录应该返回 nil（不是错误）
	require.NoError(t, err)
	assert.Equal(t, 0, tm.GetToolCount())
}

func TestToolManager_GetTool(t *testing.T) {
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

	tm := NewToolManager(toolsDir)
	require.NoError(t, tm.LoadTools())

	// 测试获取存在的工具
	tool, err := tm.GetTool("test_tool")
	require.NoError(t, err)
	assert.Equal(t, "test_tool", tool.Name)

	// 测试获取不存在的工具
	_, err = tm.GetTool("nonexistent")
	assert.Error(t, err)
}

func TestToolManager_ListTools(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	// 创建多个工具
	for _, name := range []string{"tool1", "tool2", "tool3"} {
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

	tm := NewToolManager(toolsDir)
	require.NoError(t, tm.LoadTools())

	tools := tm.ListTools()
	assert.Len(t, tools, 3)
	assert.Contains(t, tools, "tool1")
	assert.Contains(t, tools, "tool2")
	assert.Contains(t, tools, "tool3")
}

func TestToolManager_ReloadTool(t *testing.T) {
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

	toolJSONPath := filepath.Join(testToolDir, "tool.json")
	require.NoError(t, os.WriteFile(toolJSONPath, []byte(toolJSON), 0644))

	tm := NewToolManager(toolsDir)
	require.NoError(t, tm.LoadTools())

	// 修改工具配置
	updatedJSON := `{
		"name": "test_tool",
		"description": "更新后的工具",
		"version": "2.0.0",
		"type": "shell",
		"command": "echo.sh"
	}`

	require.NoError(t, os.WriteFile(toolJSONPath, []byte(updatedJSON), 0644))

	// 重新加载
	err := tm.ReloadTool("test_tool")
	require.NoError(t, err)

	// 验证更新
	tool, err := tm.GetTool("test_tool")
	require.NoError(t, err)
	assert.Equal(t, "更新后的工具", tool.Description)
	assert.Equal(t, "2.0.0", tool.Version)
}

func TestToolManager_Clear(t *testing.T) {
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

	tm := NewToolManager(toolsDir)
	require.NoError(t, tm.LoadTools())
	assert.Equal(t, 1, tm.GetToolCount())

	// 清空
	tm.Clear()
	assert.Equal(t, 0, tm.GetToolCount())
}

func TestToolManager_InvalidToolJSON(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testToolDir := filepath.Join(toolsDir, "invalid_tool")
	require.NoError(t, os.MkdirAll(testToolDir, 0755))

	// 无效的 JSON
	invalidJSON := `{invalid json`
	require.NoError(t, os.WriteFile(
		filepath.Join(testToolDir, "tool.json"),
		[]byte(invalidJSON),
		0644,
	))

	tm := NewToolManager(toolsDir)
	err := tm.LoadTools()

	// 应该成功加载（跳过无效工具）
	require.NoError(t, err)
	assert.Equal(t, 0, tm.GetToolCount())
}

func TestToolManager_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))

	testCases := []struct {
		name     string
		toolJSON string
	}{
		{
			name: "missing_name",
			toolJSON: `{
				"description": "测试工具",
				"type": "shell",
				"command": "echo.sh"
			}`,
		},
		{
			name: "missing_type",
			toolJSON: `{
				"name": "test_tool",
				"description": "测试工具",
				"command": "echo.sh"
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toolDir := filepath.Join(toolsDir, tc.name)
			require.NoError(t, os.MkdirAll(toolDir, 0755))

			require.NoError(t, os.WriteFile(
				filepath.Join(toolDir, "tool.json"),
				[]byte(tc.toolJSON),
				0644,
			))
		})
	}

	tm := NewToolManager(toolsDir)
	err := tm.LoadTools()

	// 应该成功加载（跳过无效工具）
	require.NoError(t, err)
	assert.Equal(t, 0, tm.GetToolCount())
}

func TestToolManager_Concurrent(t *testing.T) {
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

	tm := NewToolManager(toolsDir)
	require.NoError(t, tm.LoadTools())

	// 并发读取
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// 并发获取工具
			tool, err := tm.GetTool("test_tool")
			assert.NoError(t, err)
			assert.NotNil(t, tool)

			// 并发列出工具
			tools := tm.ListTools()
			assert.Len(t, tools, 1)

			// 并发检查工具
			exists := tm.HasTool("test_tool")
			assert.True(t, exists)
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
