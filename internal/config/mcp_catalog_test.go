package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBuiltinCatalog(t *testing.T) {
	catalog, err := LoadBuiltinCatalog()
	require.NoError(t, err)
	assert.Greater(t, len(catalog.Servers), 0, "内置目录应包含至少一个 server")

	// 验证 everything server 存在
	var found *CatalogEntry
	for i := range catalog.Servers {
		if catalog.Servers[i].ID == "everything" {
			found = &catalog.Servers[i]
			break
		}
	}
	require.NotNil(t, found, "应包含 everything server")
	assert.Equal(t, "🧪", found.Icon)
	assert.Equal(t, "testing", found.Category)
	assert.Equal(t, "stdio", found.Connection.Type)
	assert.Equal(t, "npx", found.Connection.Command)
	assert.Greater(t, len(found.Tools), 0)
}

func TestResolveCatalogEntry(t *testing.T) {
	entry := &CatalogEntry{
		ID: "test",
		Connection: CatalogConnection{
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "some-pkg", "${WORKSPACE}"},
			Env:     map[string]string{"API_KEY": "${MY_KEY}"},
		},
	}

	vars := map[string]string{
		"WORKSPACE": "/home/user",
		"MY_KEY":    "sk-123",
	}

	result := ResolveCatalogEntry(entry, vars)
	assert.Equal(t, "stdio", result.Type)
	assert.Equal(t, "npx", result.Command)
	assert.Equal(t, []string{"-y", "some-pkg", "/home/user"}, result.Args)
	assert.Equal(t, "sk-123", result.Env["API_KEY"])
	assert.True(t, result.Enabled)
}

func TestResolveCatalogEntry_SSE(t *testing.T) {
	entry := &CatalogEntry{
		ID: "test-sse",
		Connection: CatalogConnection{
			Type:    "sse",
			URL:     "https://api.example.com/${ENDPOINT}/sse",
			Headers: map[string]string{"Authorization": "Bearer ${TOKEN}"},
		},
	}

	vars := map[string]string{
		"ENDPOINT": "v1",
		"TOKEN":    "my-token",
	}

	result := ResolveCatalogEntry(entry, vars)
	assert.Equal(t, "sse", result.Type)
	assert.Equal(t, "https://api.example.com/v1/sse", result.URL)
	assert.Equal(t, "Bearer my-token", result.Headers["Authorization"])
}

func TestMatchCatalogToolDescription(t *testing.T) {
	descriptions := map[string]string{
		"get_stock_quote": "获取股票实时行情",
		"list_orders":     "查看订单列表",
		"create-event":    "创建日程",
	}

	tests := []struct {
		name       string
		actualName string
		wantDesc   string
		wantOK     bool
	}{
		{"精确匹配", "get_stock_quote", "获取股票实时行情", true},
		{"标准化匹配: - vs _", "get-stock-quote", "获取股票实时行情", true},
		{"标准化匹配: _ vs -", "create_event", "创建日程", true},
		{"大小写不敏感", "Get_Stock_Quote", "获取股票实时行情", true},
		{"子串匹配: actual 包含 catalog", "get-quote", "获取股票实时行情", true},
		{"完全不匹配", "send_message", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			desc, ok := MatchCatalogToolDescription(descriptions, tc.actualName)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantDesc, desc)
			}
		})
	}

	t.Run("nil map", func(t *testing.T) {
		_, ok := MatchCatalogToolDescription(nil, "anything")
		assert.False(t, ok)
	})
}

func TestMatchCatalogToolDescription_EdgeCases(t *testing.T) {
	descriptions := map[string]string{
		"get_stock_quote": "获取股票实时行情",
		"list_orders":     "查看订单列表",
		"create-event":    "创建日程",
		"send_message":    "发送消息",
	}

	tests := []struct {
		name       string
		actualName string
		wantDesc   string
		wantOK     bool
	}{
		{"空actualToolName", "", "", false},
		{"全大写", "GET_STOCK_QUOTE", "获取股票实时行情", true},
		{"混合分隔符get_stock-quote", "get_stock-quote", "获取股票实时行情", true},
		{"多个候选只匹配一个", "orders", "查看订单列表", true},
		{"完全无关", "delete_user", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			desc, ok := MatchCatalogToolDescription(descriptions, tc.actualName)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantDesc, desc)
			}
		})
	}

	t.Run("空descriptions", func(t *testing.T) {
		_, ok := MatchCatalogToolDescription(map[string]string{}, "anything")
		assert.False(t, ok)
	})
}

func TestMergeCatalogs(t *testing.T) {
	builtin := &MCPCatalog{
		Version: 1,
		Servers: []CatalogEntry{
			{ID: "a", Category: "old"},
			{ID: "b", Category: "keep"},
		},
	}
	remote := &MCPCatalog{
		Version: 2,
		Servers: []CatalogEntry{
			{ID: "a", Category: "updated"},
			{ID: "c", Category: "new"},
		},
	}

	merged := MergeCatalogs(builtin, remote)
	assert.Equal(t, 3, len(merged.Servers))

	idMap := make(map[string]string)
	for _, s := range merged.Servers {
		idMap[s.ID] = s.Category
	}
	assert.Equal(t, "updated", idMap["a"], "远程应覆盖内置")
	assert.Equal(t, "keep", idMap["b"], "未覆盖的保留")
	assert.Equal(t, "new", idMap["c"], "新条目应追加")
}
