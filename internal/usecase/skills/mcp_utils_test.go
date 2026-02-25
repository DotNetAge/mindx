package skills

import (
	"encoding/json"
	"mindx/internal/entity"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPToolToSkillDef(t *testing.T) {
	tests := []struct {
		name        string
		serverName  string
		tool        *mcp.Tool
		catalogTags []string
		check       func(t *testing.T, def *entity.SkillDef)
	}{
		{
			name:       "基本转换",
			serverName: "test-server",
			tool:       &mcp.Tool{Name: "my_tool", Description: "A test tool"},
			check: func(t *testing.T, def *entity.SkillDef) {
				assert.Equal(t, "mcp_test-server_my_tool", def.Name)
				assert.Equal(t, "A test tool", def.Description)
				assert.Equal(t, "mcp", def.Category)
				assert.True(t, def.Enabled)
				assert.Equal(t, 30, def.Timeout)
				assert.Contains(t, def.Tags, "mcp")
				assert.Contains(t, def.Tags, "test-server")
			},
		},
		{
			name:        "带catalogTags",
			serverName:  "sina-finance",
			tool:        &mcp.Tool{Name: "get-quote", Description: "Get stock quote"},
			catalogTags: []string{"stock", "finance", "A股"},
			check: func(t *testing.T, def *entity.SkillDef) {
				assert.Equal(t, "mcp_sina-finance_get-quote", def.Name)
				assert.Contains(t, def.Tags, "mcp")
				assert.Contains(t, def.Tags, "sina-finance")
				assert.Contains(t, def.Tags, "stock")
				assert.Contains(t, def.Tags, "finance")
				assert.Contains(t, def.Tags, "A股")
			},
		},
		{
			name:       "无catalogTags",
			serverName: "server",
			tool:       &mcp.Tool{Name: "tool1", Description: "desc"},
			check: func(t *testing.T, def *entity.SkillDef) {
				assert.Equal(t, []string{"mcp", "server"}, def.Tags)
			},
		},
		{
			name:       "带InputSchema_map",
			serverName: "server",
			tool: &mcp.Tool{
				Name:        "tool_with_params",
				Description: "Has params",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol": map[string]any{
							"type":        "string",
							"description": "Stock symbol",
						},
						"count": map[string]any{
							"type":        "integer",
							"description": "Number of results",
						},
					},
					"required": []any{"symbol"},
				},
			},
			check: func(t *testing.T, def *entity.SkillDef) {
				require.Len(t, def.Parameters, 2)
				sym := def.Parameters["symbol"]
				assert.Equal(t, "string", sym.Type)
				assert.Equal(t, "Stock symbol", sym.Description)
				assert.True(t, sym.Required)
				cnt := def.Parameters["count"]
				assert.Equal(t, "integer", cnt.Type)
				assert.False(t, cnt.Required)
			},
		},
		{
			name:       "无InputSchema",
			serverName: "server",
			tool:       &mcp.Tool{Name: "no_params", Description: "No params"},
			check: func(t *testing.T, def *entity.SkillDef) {
				assert.Empty(t, def.Parameters)
			},
		},
		{
			name:       "Metadata正确",
			serverName: "my-server",
			tool:       &mcp.Tool{Name: "my-tool", Description: "desc"},
			check: func(t *testing.T, def *entity.SkillDef) {
				meta, ok := GetMCPSkillMetadata(def)
				require.True(t, ok)
				assert.Equal(t, "my-server", meta.Server)
				assert.Equal(t, "my-tool", meta.Tool)
			},
		},
		{
			name:       "InputSchema为json.RawMessage",
			serverName: "server",
			tool: &mcp.Tool{
				Name:        "raw_schema",
				Description: "Raw schema",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string","description":"query"}},"required":["q"]}`),
			},
			check: func(t *testing.T, def *entity.SkillDef) {
				require.Len(t, def.Parameters, 1)
				q := def.Parameters["q"]
				assert.Equal(t, "string", q.Type)
				assert.True(t, q.Required)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var def *entity.SkillDef
			if tc.catalogTags != nil {
				def = MCPToolToSkillDef(tc.serverName, tc.tool, tc.catalogTags)
			} else {
				def = MCPToolToSkillDef(tc.serverName, tc.tool)
			}
			require.NotNil(t, def)
			tc.check(t, def)
		})
	}
}

func TestExtractParameters(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
		check  func(t *testing.T, params map[string]entity.ParameterDef)
	}{
		{
			name: "标准schema",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "User name"},
					"age":  map[string]any{"type": "integer", "description": "User age"},
				},
				"required": []any{"name"},
			},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				require.Len(t, params, 2)
				assert.Equal(t, "string", params["name"].Type)
				assert.True(t, params["name"].Required)
				assert.Equal(t, "integer", params["age"].Type)
				assert.False(t, params["age"].Required)
			},
		},
		{
			name: "无required字段",
			schema: map[string]any{
				"properties": map[string]any{
					"q": map[string]any{"type": "string"},
				},
			},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				require.Len(t, params, 1)
				assert.False(t, params["q"].Required)
			},
		},
		{
			name: "无type默认string",
			schema: map[string]any{
				"properties": map[string]any{
					"q": map[string]any{"description": "query"},
				},
			},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				assert.Equal(t, "string", params["q"].Type)
			},
		},
		{
			name:   "空properties",
			schema: map[string]any{"properties": map[string]any{}},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				assert.Empty(t, params)
			},
		},
		{
			name:   "properties不是map",
			schema: map[string]any{"properties": "invalid"},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				assert.Empty(t, params)
			},
		},
		{
			name:   "无properties",
			schema: map[string]any{"type": "object"},
			check: func(t *testing.T, params map[string]entity.ParameterDef) {
				assert.Empty(t, params)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := make(map[string]entity.ParameterDef)
			extractParameters(tc.schema, params)
			tc.check(t, params)
		})
	}
}

func TestIsMCPSkill(t *testing.T) {
	tests := []struct {
		name   string
		def    *entity.SkillDef
		expect bool
	}{
		{
			name: "正常MCP技能",
			def: &entity.SkillDef{
				Metadata: map[string]interface{}{
					"mcp": map[string]interface{}{"server": "s", "tool": "t"},
				},
			},
			expect: true,
		},
		{name: "nil def", def: nil, expect: false},
		{name: "nil metadata", def: &entity.SkillDef{}, expect: false},
		{
			name:   "metadata无mcp key",
			def:    &entity.SkillDef{Metadata: map[string]interface{}{"other": 1}},
			expect: false,
		},
		{
			name:   "mcp值不是map",
			def:    &entity.SkillDef{Metadata: map[string]interface{}{"mcp": "string"}},
			expect: false,
		},
		{
			name: "缺少server",
			def: &entity.SkillDef{
				Metadata: map[string]interface{}{
					"mcp": map[string]interface{}{"tool": "t"},
				},
			},
			expect: false,
		},
		{
			name: "缺少tool",
			def: &entity.SkillDef{
				Metadata: map[string]interface{}{
					"mcp": map[string]interface{}{"server": "s"},
				},
			},
			expect: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, IsMCPSkill(tc.def))
		})
	}
}

func TestGetMCPSkillMetadata(t *testing.T) {
	tests := []struct {
		name         string
		def          *entity.SkillDef
		expectOK     bool
		expectServer string
		expectTool   string
	}{
		{
			name: "正常提取",
			def: &entity.SkillDef{
				Metadata: map[string]interface{}{
					"mcp": map[string]interface{}{"server": "sina-finance", "tool": "get-quote"},
				},
			},
			expectOK:     true,
			expectServer: "sina-finance",
			expectTool:   "get-quote",
		},
		{name: "nil def", def: nil, expectOK: false},
		{name: "nil metadata", def: &entity.SkillDef{}, expectOK: false},
		{
			name: "server为空字符串",
			def: &entity.SkillDef{
				Metadata: map[string]interface{}{
					"mcp": map[string]interface{}{"server": "", "tool": "t"},
				},
			},
			expectOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta, ok := GetMCPSkillMetadata(tc.def)
			assert.Equal(t, tc.expectOK, ok)
			if tc.expectOK {
				require.NotNil(t, meta)
				assert.Equal(t, tc.expectServer, meta.Server)
				assert.Equal(t, tc.expectTool, meta.Tool)
			}
		})
	}
}
