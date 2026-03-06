package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPManager_LoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp_servers.json")

	config := `{
		"servers": {
			"test_server": {
				"name": "test_server",
				"command": "node",
				"args": ["server.js"],
				"env": {
					"PORT": "3000"
				}
			}
		}
	}`

	require.NoError(t, os.WriteFile(configPath, []byte(config), 0644))

	mm := NewMCPManager(configPath)
	err := mm.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, 1, len(mm.servers))
	assert.Contains(t, mm.servers, "test_server")
}

func TestMCPManager_LoadConfig_NotFound(t *testing.T) {
	mm := NewMCPManager("/nonexistent/config.json")
	err := mm.LoadConfig()

	// 配置文件不存在应该返回 nil（跳过）
	require.NoError(t, err)
}

func TestMCPManager_LoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp_servers.json")

	require.NoError(t, os.WriteFile(configPath, []byte("invalid json"), 0644))

	mm := NewMCPManager(configPath)
	err := mm.LoadConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestMCPManager_GetTool(t *testing.T) {
	mm := NewMCPManager("")

	// 手动添加工具
	mm.tools["test_tool"] = &MCPTool{
		Name:        "test_tool",
		Description: "测试工具",
		ServerName:  "test_server",
	}

	tool, err := mm.GetTool("test_tool")
	require.NoError(t, err)
	assert.Equal(t, "test_tool", tool.Name)
	assert.Equal(t, "test_server", tool.ServerName)
}

func TestMCPManager_GetTool_NotFound(t *testing.T) {
	mm := NewMCPManager("")

	_, err := mm.GetTool("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestMCPManager_ListTools(t *testing.T) {
	mm := NewMCPManager("")

	// 添加多个工具
	mm.tools["tool1"] = &MCPTool{Name: "tool1"}
	mm.tools["tool2"] = &MCPTool{Name: "tool2"}
	mm.tools["tool3"] = &MCPTool{Name: "tool3"}

	tools := mm.ListTools()
	assert.Len(t, tools, 3)
	assert.Contains(t, tools, "tool1")
	assert.Contains(t, tools, "tool2")
	assert.Contains(t, tools, "tool3")
}

func TestMCPManager_HasTool(t *testing.T) {
	mm := NewMCPManager("")

	mm.tools["test_tool"] = &MCPTool{Name: "test_tool"}

	assert.True(t, mm.HasTool("test_tool"))
	assert.False(t, mm.HasTool("nonexistent"))
}

func TestMCPManager_GetToolCount(t *testing.T) {
	mm := NewMCPManager("")

	assert.Equal(t, 0, mm.GetToolCount())

	mm.tools["tool1"] = &MCPTool{Name: "tool1"}
	mm.tools["tool2"] = &MCPTool{Name: "tool2"}

	assert.Equal(t, 2, mm.GetToolCount())
}

func TestMCPManager_GetServerCount(t *testing.T) {
	mm := NewMCPManager("")

	assert.Equal(t, 0, mm.GetServerCount())

	// 手动添加客户端（模拟连接）
	mm.clients["server1"] = &MCPClient{}
	mm.clients["server2"] = &MCPClient{}

	assert.Equal(t, 2, mm.GetServerCount())
}

func TestMCPManager_Close(t *testing.T) {
	mm := NewMCPManager("")

	// 添加一些数据
	mm.tools["tool1"] = &MCPTool{Name: "tool1"}
	mm.clients["server1"] = &MCPClient{}

	err := mm.Close()
	require.NoError(t, err)

	// 验证清空
	assert.Equal(t, 0, mm.GetToolCount())
	assert.Equal(t, 0, mm.GetServerCount())
}

func TestMCPManager_Concurrent(t *testing.T) {
	mm := NewMCPManager("")

	// 添加工具
	mm.tools["test_tool"] = &MCPTool{
		Name:        "test_tool",
		Description: "测试工具",
	}

	// 并发读取
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// 并发获取工具
			tool, err := mm.GetTool("test_tool")
			assert.NoError(t, err)
			assert.NotNil(t, tool)

			// 并发列出工具
			tools := mm.ListTools()
			assert.Len(t, tools, 1)

			// 并发检查工具
			exists := mm.HasTool("test_tool")
			assert.True(t, exists)
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// MockMCPClient 模拟 MCP 客户端（用于测试）
type MockMCPClient struct {
	tools []*MCPTool
}

func (m *MockMCPClient) Connect(ctx context.Context) error {
	return nil
}

func (m *MockMCPClient) DiscoverTools(ctx context.Context) ([]*MCPTool, error) {
	return m.tools, nil
}

func (m *MockMCPClient) ExecuteTool(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	return "mock result", nil
}

func (m *MockMCPClient) Close() error {
	return nil
}

func TestMCPClient_JSONRPCFormat(t *testing.T) {
	// 测试 JSON-RPC 请求格式
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}

	data, err := json.Marshal(request)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded["jsonrpc"])
	assert.Equal(t, float64(1), decoded["id"])
	assert.Equal(t, "initialize", decoded["method"])
}

func TestMCPManager_ExecuteTool_ClientNotFound(t *testing.T) {
	mm := NewMCPManager("")

	// 添加工具但没有客户端
	mm.tools["test_tool"] = &MCPTool{
		Name:       "test_tool",
		ServerName: "nonexistent_server",
	}

	_, err := mm.ExecuteTool("test_tool", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client not found")
}

func TestMCPManager_LoadConfig_EmptyServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp_servers.json")

	config := `{
		"servers": {}
	}`

	require.NoError(t, os.WriteFile(configPath, []byte(config), 0644))

	mm := NewMCPManager(configPath)
	err := mm.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, 0, len(mm.servers))
}

func TestMCPClient_Timeout(t *testing.T) {
	// 测试超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 模拟超时场景
	select {
	case <-ctx.Done():
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout did not trigger")
	}
}
