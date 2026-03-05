package skills

import (
	"fmt"
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolAssembler_RegisterLocalTool(t *testing.T) {
	assembler := NewToolAssembler()

	tool := &LocalTool{
		Name:        "test_tool",
		Description: "测试工具",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "参数1",
				},
			},
		},
	}

	assembler.RegisterLocalTool(tool)

	assert.True(t, assembler.HasTool("test_tool"))
	assert.Contains(t, assembler.ListLocalTools(), "test_tool")
}

func TestToolAssembler_RegisterMCPTool(t *testing.T) {
	assembler := NewToolAssembler()

	tool := &MCPTool{
		Name:        "mcp_tool",
		Description: "MCP 工具",
		ServerName:  "test_server",
		Schema: entity.ToolSchema{
			Type: "function",
			Function: entity.ToolFunctionSchema{
				Name:        "mcp_tool",
				Description: "MCP 工具",
				Parameters:  map[string]interface{}{},
			},
		},
	}

	assembler.RegisterMCPTool(tool)

	assert.True(t, assembler.HasTool("mcp_tool"))
	assert.Contains(t, assembler.ListMCPTools(), "mcp_tool")
}

func TestToolAssembler_AssembleTools(t *testing.T) {
	assembler := NewToolAssembler()

	// 注册工具
	assembler.RegisterLocalTool(&LocalTool{
		Name:        "web_search",
		Description: "网页搜索",
		Parameters:  map[string]interface{}{},
	})

	assembler.RegisterLocalTool(&LocalTool{
		Name:        "http_request",
		Description: "HTTP 请求",
		Parameters:  map[string]interface{}{},
	})

	assembler.RegisterMCPTool(&MCPTool{
		Name:       "location_service",
		ServerName: "location_server",
		Schema: entity.ToolSchema{
			Type: "function",
			Function: entity.ToolFunctionSchema{
				Name:        "location_service",
				Description: "位置服务",
				Parameters:  map[string]interface{}{},
			},
		},
	})

	// 创建 Skill
	skill := &entity.Skill{
		Name:          "weather_query",
		RequiredTools: []string{"web_search", "http_request"},
		OptionalTools: []string{"location_service"},
	}

	// 组装工具
	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)

	// 验证结果
	assert.Len(t, schemas, 3)

	// 验证工具名称
	toolNames := make(map[string]bool)
	for _, schema := range schemas {
		toolNames[schema.Function.Name] = true
	}

	assert.True(t, toolNames["web_search"])
	assert.True(t, toolNames["http_request"])
	assert.True(t, toolNames["location_service"])
}

func TestToolAssembler_AssembleTools_MissingRequired(t *testing.T) {
	assembler := NewToolAssembler()

	// 只注册部分工具
	assembler.RegisterLocalTool(&LocalTool{
		Name:        "web_search",
		Description: "网页搜索",
		Parameters:  map[string]interface{}{},
	})

	// 创建 Skill（缺少 http_request）
	skill := &entity.Skill{
		Name:          "weather_query",
		RequiredTools: []string{"web_search", "http_request"},
	}

	// 组装工具（应该失败）
	_, err := assembler.AssembleTools(skill)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http_request")
}

func TestToolAssembler_AssembleTools_MissingOptional(t *testing.T) {
	assembler := NewToolAssembler()

	// 只注册必需工具
	assembler.RegisterLocalTool(&LocalTool{
		Name:        "web_search",
		Description: "网页搜索",
		Parameters:  map[string]interface{}{},
	})

	// 创建 Skill（可选工具缺失）
	skill := &entity.Skill{
		Name:          "weather_query",
		RequiredTools: []string{"web_search"},
		OptionalTools: []string{"location_service"}, // 未注册
	}

	// 组装工具（应该成功，可选工具缺失不影响）
	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)

	// 只有必需工具
	assert.Len(t, schemas, 1)
	assert.Equal(t, "web_search", schemas[0].Function.Name)
}

func TestToolAssembler_AssembleToolsByNames(t *testing.T) {
	assembler := NewToolAssembler()

	// 注册工具
	assembler.RegisterLocalTool(&LocalTool{
		Name:        "tool1",
		Description: "工具1",
		Parameters:  map[string]interface{}{},
	})

	assembler.RegisterLocalTool(&LocalTool{
		Name:        "tool2",
		Description: "工具2",
		Parameters:  map[string]interface{}{},
	})

	// 按名称组装
	schemas, err := assembler.AssembleToolsByNames([]string{"tool1", "tool2"})
	require.NoError(t, err)

	assert.Len(t, schemas, 2)
}

func TestToolAssembler_GetTool(t *testing.T) {
	assembler := NewToolAssembler()

	localTool := &LocalTool{
		Name:        "local_tool",
		Description: "本地工具",
		Parameters:  map[string]interface{}{},
	}

	mcpTool := &MCPTool{
		Name:       "mcp_tool",
		ServerName: "test_server",
		Schema:     entity.ToolSchema{},
	}

	assembler.RegisterLocalTool(localTool)
	assembler.RegisterMCPTool(mcpTool)

	// 获取本地工具
	tool, err := assembler.GetTool("local_tool")
	require.NoError(t, err)
	assert.IsType(t, &LocalTool{}, tool)

	// 获取 MCP 工具
	tool, err = assembler.GetTool("mcp_tool")
	require.NoError(t, err)
	assert.IsType(t, &MCPTool{}, tool)

	// 获取不存在的工具
	_, err = assembler.GetTool("non_existent")
	assert.Error(t, err)
}

func TestToolAssembler_HasTool(t *testing.T) {
	assembler := NewToolAssembler()

	assembler.RegisterLocalTool(&LocalTool{
		Name: "tool1",
	})

	assert.True(t, assembler.HasTool("tool1"))
	assert.False(t, assembler.HasTool("tool2"))
}

func TestToolAssembler_ListTools(t *testing.T) {
	assembler := NewToolAssembler()

	assembler.RegisterLocalTool(&LocalTool{Name: "local1"})
	assembler.RegisterLocalTool(&LocalTool{Name: "local2"})
	assembler.RegisterMCPTool(&MCPTool{Name: "mcp1"})

	// 列出所有工具
	allTools := assembler.ListTools()
	assert.Len(t, allTools, 3)
	assert.Contains(t, allTools, "local1")
	assert.Contains(t, allTools, "local2")
	assert.Contains(t, allTools, "mcp1")

	// 列出本地工具
	localTools := assembler.ListLocalTools()
	assert.Len(t, localTools, 2)
	assert.Contains(t, localTools, "local1")
	assert.Contains(t, localTools, "local2")

	// 列出 MCP 工具
	mcpTools := assembler.ListMCPTools()
	assert.Len(t, mcpTools, 1)
	assert.Contains(t, mcpTools, "mcp1")
}

func TestToolAssembler_UnregisterTool(t *testing.T) {
	assembler := NewToolAssembler()

	assembler.RegisterLocalTool(&LocalTool{Name: "local_tool"})
	assembler.RegisterMCPTool(&MCPTool{Name: "mcp_tool"})

	assert.True(t, assembler.HasTool("local_tool"))
	assert.True(t, assembler.HasTool("mcp_tool"))

	// 注销本地工具
	assembler.UnregisterLocalTool("local_tool")
	assert.False(t, assembler.HasTool("local_tool"))

	// 注销 MCP 工具
	assembler.UnregisterMCPTool("mcp_tool")
	assert.False(t, assembler.HasTool("mcp_tool"))
}

func TestToolAssembler_Clear(t *testing.T) {
	assembler := NewToolAssembler()

	assembler.RegisterLocalTool(&LocalTool{Name: "tool1"})
	assembler.RegisterLocalTool(&LocalTool{Name: "tool2"})
	assembler.RegisterMCPTool(&MCPTool{Name: "tool3"})

	local, mcp, total := assembler.GetToolCount()
	assert.Equal(t, 2, local)
	assert.Equal(t, 1, mcp)
	assert.Equal(t, 3, total)

	// 清空
	assembler.Clear()

	local, mcp, total = assembler.GetToolCount()
	assert.Equal(t, 0, local)
	assert.Equal(t, 0, mcp)
	assert.Equal(t, 0, total)
}

func TestToolAssembler_GetToolCount(t *testing.T) {
	assembler := NewToolAssembler()

	// 初始为空
	local, mcp, total := assembler.GetToolCount()
	assert.Equal(t, 0, local)
	assert.Equal(t, 0, mcp)
	assert.Equal(t, 0, total)

	// 添加工具
	assembler.RegisterLocalTool(&LocalTool{Name: "local1"})
	assembler.RegisterLocalTool(&LocalTool{Name: "local2"})
	assembler.RegisterMCPTool(&MCPTool{Name: "mcp1"})

	local, mcp, total = assembler.GetToolCount()
	assert.Equal(t, 2, local)
	assert.Equal(t, 1, mcp)
	assert.Equal(t, 3, total)
}

func TestToolAssembler_ValidateSkillTools(t *testing.T) {
	assembler := NewToolAssembler()

	// 注册部分工具
	assembler.RegisterLocalTool(&LocalTool{Name: "tool1"})
	assembler.RegisterLocalTool(&LocalTool{Name: "tool2"})

	skill := &entity.Skill{
		RequiredTools: []string{"tool1", "tool2", "tool3"}, // tool3 缺失
		OptionalTools: []string{"tool4", "tool5"},          // 都缺失
	}

	missing, optional := assembler.ValidateSkillTools(skill)

	// 验证缺失的必需工具
	assert.Len(t, missing, 1)
	assert.Contains(t, missing, "tool3")

	// 验证缺失的可选工具
	assert.Len(t, optional, 2)
	assert.Contains(t, optional, "tool4")
	assert.Contains(t, optional, "tool5")
}

func TestToolAssembler_LocalToolToSchema(t *testing.T) {
	assembler := NewToolAssembler()

	tool := &LocalTool{
		Name:        "test_tool",
		Description: "测试工具",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "参数1",
				},
			},
			"required": []string{"param1"},
		},
	}

	schema := assembler.localToolToSchema(tool)

	assert.Equal(t, "function", schema.Type)
	assert.Equal(t, "test_tool", schema.Function.Name)
	assert.Equal(t, "测试工具", schema.Function.Description)
	assert.NotNil(t, schema.Function.Parameters)
}

func TestToolAssembler_PriorityLocalOverMCP(t *testing.T) {
	assembler := NewToolAssembler()

	// 注册同名的本地工具和 MCP 工具
	assembler.RegisterLocalTool(&LocalTool{
		Name:        "duplicate_tool",
		Description: "本地工具",
		Parameters:  map[string]interface{}{},
	})

	assembler.RegisterMCPTool(&MCPTool{
		Name:       "duplicate_tool",
		ServerName: "test_server",
		Schema: entity.ToolSchema{
			Type: "function",
			Function: entity.ToolFunctionSchema{
				Name:        "duplicate_tool",
				Description: "MCP 工具",
				Parameters:  map[string]interface{}{},
			},
		},
	})

	// 获取工具（应该返回本地工具）
	tool, err := assembler.GetTool("duplicate_tool")
	require.NoError(t, err)
	assert.IsType(t, &LocalTool{}, tool)

	// 组装工具（应该使用本地工具）
	schemas, err := assembler.AssembleToolsByNames([]string{"duplicate_tool"})
	require.NoError(t, err)
	assert.Len(t, schemas, 1)
	assert.Equal(t, "本地工具", schemas[0].Function.Description)
}

func TestToolAssembler_ConcurrentAccess(t *testing.T) {
	assembler := NewToolAssembler()

	// 并发注册工具
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			tool := &LocalTool{
				Name:        fmt.Sprintf("tool_%d", id),
				Description: fmt.Sprintf("工具 %d", id),
				Parameters:  map[string]interface{}{},
			}
			assembler.RegisterLocalTool(tool)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有工具都已注册
	local, _, _ := assembler.GetToolCount()
	assert.Equal(t, 10, local)
}

func TestToolAssembler_EmptySkill(t *testing.T) {
	assembler := NewToolAssembler()

	// 注册一些工具
	assembler.RegisterLocalTool(&LocalTool{Name: "tool1"})

	// 创建没有工具依赖的 Skill
	skill := &entity.Skill{
		Name:          "simple_skill",
		RequiredTools: []string{},
		OptionalTools: []string{},
	}

	// 组装工具（应该返回空列表）
	schemas, err := assembler.AssembleTools(skill)
	require.NoError(t, err)
	assert.Len(t, schemas, 0)
}

func BenchmarkToolAssembler_AssembleTools(b *testing.B) {
	assembler := NewToolAssembler()

	// 注册 100 个工具
	for i := 0; i < 100; i++ {
		assembler.RegisterLocalTool(&LocalTool{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: fmt.Sprintf("工具 %d", i),
			Parameters:  map[string]interface{}{},
		})
	}

	skill := &entity.Skill{
		RequiredTools: []string{"tool_0", "tool_1", "tool_2"},
		OptionalTools: []string{"tool_3", "tool_4"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.AssembleTools(skill)
	}
}
