package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

//go:embed catalog/mcp_catalog.json
var builtinCatalogData []byte

type MCPCatalog struct {
	Version int            `json:"version"`
	Servers []CatalogEntry `json:"servers"`
}

type CatalogEntry struct {
	ID          string            `json:"id"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`
	Tags        []string          `json:"tags"`
	Author      string            `json:"author"`
	Homepage    string            `json:"homepage"`
	Connection  CatalogConnection `json:"connection"`
	Variables   []CatalogVariable `json:"variables"`
	Tools       []CatalogTool     `json:"tools"`
}

type CatalogConnection struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type CatalogVariable struct {
	Key         string            `json:"key"`
	Label       map[string]string `json:"label"`
	Description map[string]string `json:"description,omitempty"`
	Type        string            `json:"type"` // string | secret | path | url
	Required    bool              `json:"required"`
	Default     string            `json:"default,omitempty"`
}

type CatalogTool struct {
	Name        string            `json:"name"`
	Description map[string]string `json:"description"`
}

// LoadBuiltinCatalog 加载内嵌的目录数据
func LoadBuiltinCatalog() (*MCPCatalog, error) {
	var catalog MCPCatalog
	if err := json.Unmarshal(builtinCatalogData, &catalog); err != nil {
		return nil, fmt.Errorf("parse builtin catalog: %w", err)
	}
	return &catalog, nil
}

// FetchRemoteCatalog 从远程 URL 拉取目录数据
func FetchRemoteCatalog(url string) (*MCPCatalog, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch remote catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote catalog returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read remote catalog: %w", err)
	}

	var catalog MCPCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse remote catalog: %w", err)
	}
	return &catalog, nil
}

// MergeCatalogs 合并内置和远程目录，远程条目覆盖同 ID 的内置条目，新条目追加
func MergeCatalogs(builtin, remote *MCPCatalog) *MCPCatalog {
	if remote == nil {
		return builtin
	}

	idMap := make(map[string]int, len(builtin.Servers))
	merged := &MCPCatalog{
		Version: builtin.Version,
		Servers: make([]CatalogEntry, len(builtin.Servers)),
	}
	copy(merged.Servers, builtin.Servers)

	for i, s := range merged.Servers {
		idMap[s.ID] = i
	}

	for _, rs := range remote.Servers {
		if idx, ok := idMap[rs.ID]; ok {
			merged.Servers[idx] = rs // 覆盖
		} else {
			merged.Servers = append(merged.Servers, rs) // 追加
		}
	}
	return merged
}

// ResolveCatalogEntry 将目录条目 + 用户变量解析为可用的 MCPServerEntry
func ResolveCatalogEntry(entry *CatalogEntry, vars map[string]string) MCPServerEntry {
	replacer := func(s string) string {
		return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
			varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
			if val, ok := vars[varName]; ok {
				return val
			}
			return match // 保留未解析的占位符
		})
	}

	result := MCPServerEntry{
		Type:    entry.Connection.Type,
		Command: entry.Connection.Command,
		URL:     replacer(entry.Connection.URL),
		Enabled: true,
	}

	// 解析 args 中的变量
	if len(entry.Connection.Args) > 0 {
		result.Args = make([]string, len(entry.Connection.Args))
		for i, arg := range entry.Connection.Args {
			result.Args[i] = replacer(arg)
		}
	}

	// 解析 env — secret 类型变量存入 env
	result.Env = make(map[string]string)
	for k, v := range entry.Connection.Env {
		result.Env[k] = replacer(v)
	}

	// 解析 headers
	if len(entry.Connection.Headers) > 0 {
		result.Headers = make(map[string]string)
		for k, v := range entry.Connection.Headers {
			result.Headers[k] = replacer(v)
		}
	}

	return result
}

// GetCatalogTags 返回指定 server 在 catalog 中定义的 tags
func GetCatalogTags(serverID string) []string {
	catalog, err := LoadBuiltinCatalog()
	if err != nil {
		return nil
	}

	for _, s := range catalog.Servers {
		if s.ID == serverID {
			return s.Tags
		}
	}
	return nil
}

// normalizeMCPToolName 标准化工具名用于匹配
// MCP server 返回的工具名和 catalog 中人工填写的名字可能用不同的分隔符
// 例如 server 返回 "get-quote"，catalog 写的 "get_quote" 或 "get_stock_quote"
func normalizeMCPToolName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}

// GetCatalogToolDescriptions 返回指定 server 的工具描述映射 (toolName → description)
// lang 为首选语言（如 "zh"），如果没有则回退到 "en"
// 匹配时会标准化工具名（统一分隔符），并支持 catalog 名是实际名的子串匹配
func GetCatalogToolDescriptions(serverID string, lang string) map[string]string {
	catalog, err := LoadBuiltinCatalog()
	if err != nil {
		return nil
	}

	for _, s := range catalog.Servers {
		if s.ID == serverID {
			result := make(map[string]string, len(s.Tools))
			for _, t := range s.Tools {
				desc := t.Description[lang]
				if desc == "" {
					desc = t.Description["en"]
				}
				if desc != "" {
					result[t.Name] = desc
				}
			}
			return result
		}
	}
	return nil
}

// MatchCatalogToolDescription 根据 MCP server 返回的实际工具名，从 catalog 描述中查找匹配
// 优先精确匹配，其次标准化匹配（忽略 -/_ 差异），最后子串匹配
func MatchCatalogToolDescription(descriptions map[string]string, actualToolName string) (string, bool) {
	if descriptions == nil || actualToolName == "" {
		return "", false
	}

	// 1. 精确匹配
	if desc, ok := descriptions[actualToolName]; ok {
		return desc, true
	}

	normalizedActual := normalizeMCPToolName(actualToolName)

	// 2. 标准化匹配（忽略 -/_ 差异）
	for catalogName, desc := range descriptions {
		if normalizeMCPToolName(catalogName) == normalizedActual {
			return desc, true
		}
	}

	// 3. 分词包含匹配：actual 的所有词段都出现在 catalog 名中
	// 例如 catalog="get_stock_quote", actual="get-quote" → actual 词段 [get, quote] 都在 catalog 中
	actualParts := strings.Split(normalizedActual, "_")
	for catalogName, desc := range descriptions {
		nc := normalizeMCPToolName(catalogName)
		allFound := true
		for _, part := range actualParts {
			if part != "" && !strings.Contains(nc, part) {
				allFound = false
				break
			}
		}
		if allFound {
			return desc, true
		}
	}

	return "", false
}
