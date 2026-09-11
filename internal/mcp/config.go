package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ── Logger ──────────────────────────────────────────────────────────────────

// Logger defines the logging interface for the MCP package.
// Matches goharness/logging.Logger so the Daemon's logger can be passed directly.
type Logger interface {
	Info(msg string, keyvals ...any)
	Error(msg string, err error, keyvals ...any)
	Debug(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
}

// ── Server Config ───────────────────────────────────────────────────────────

// ServerType represents the transport type of an MCP server.
type ServerType string

const (
	ServerTypeStdio ServerType = "stdio"
	ServerTypeSSE   ServerType = "sse"
	ServerTypeHTTP  ServerType = "http"
)

// ServerConfig 对应 mcp.json 里的一个 server 条目。
// Go 侧只管协议相关的字段；title / url(GitHub) / stars / description 等
// 扩展字段由 mcp.json 直接承载，SaveServers 时原样保留。
type ServerConfig struct {
	Name          string            `json:"-"` // 由 map key 决定，不序列化
	Title         string            `json:"title,omitempty"`
	Type          ServerType        `json:"-"` // 由 command/url 推断，不序列化
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	URL           string            `json:"url,omitempty"` // MCP 远程端点（SSE/HTTP）
	Headers       map[string]string `json:"headers,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	IdleTTLSecs   int               `json:"idle_ttl_secs,omitempty"`
	// Enabled 是否启用该 server（mcp.json 的 enabled 属性）。
	// 缺省视为 false——连接器是货架：装了不等于启用，用户按需开启。
	// 显式写 true 才会加载进连接池并提供工具。
	Enabled bool `json:"-"`
	// Description 面向用户的描述（mcp.json 的 description 属性）。
	// 只读透出给前端展示；写回由 SaveServers 的「扩展字段原样保留」机制承担。
	Description string `json:"-"`
}

// Validate checks that required fields are present based on server type.
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if c.IdleTTLSecs <= 0 {
		c.IdleTTLSecs = 300
	}
	switch c.Type {
	case ServerTypeStdio:
		if c.Command == "" {
			return fmt.Errorf("command is required for stdio server")
		}
	case ServerTypeSSE:
		fallthrough
	case ServerTypeHTTP:
		if c.URL == "" {
			return fmt.Errorf("url is required for %s server", c.Type)
		}
	default:
		return fmt.Errorf("unknown server type: %s", c.Type)
	}
	return nil
}

// ── mcp.json 文件读写（servers 唯一数据源）───────────────────────────────────

// loadRawFile 读取 mcp.json 顶层 raw map，保留所有扩展字段。
// 文件不存在返回空 schema，不是错误——首次启动无配置属正常情况。
func loadRawFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"mcpServers": map[string]any{}}, nil
		}
		return nil, fmt.Errorf("读取 mcp.json 失败: %w", err)
	}
	if len(data) == 0 {
		return map[string]any{"mcpServers": map[string]any{}}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 mcp.json 失败: %w", err)
	}
	if _, ok := raw["mcpServers"]; !ok {
		raw["mcpServers"] = map[string]any{}
	}
	return raw, nil
}

// LoadServers 从 mcp.json 读取 server 配置。
// 自动推断 transport 类型，不合法的条目跳过（不阻断其他有效条目）。
// 文件不存在返回 (nil, nil)——首次启动无配置属正常情况。
func LoadServers(path string) ([]ServerConfig, error) {
	raw, err := loadRawFile(path)
	if err != nil {
		return nil, err
	}

	rawServers, ok := raw["mcpServers"].(map[string]any)
	if !ok || len(rawServers) == 0 {
		return nil, nil
	}

	var servers []ServerConfig
	for name, v := range rawServers {
		serverMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		cfg := ServerConfig{Name: name}
		cfg.Title = stringField(serverMap, "title")
		cfg.Command = stringField(serverMap, "command")
		cfg.URL = stringField(serverMap, "url")
		cfg.Args = stringSliceField(serverMap, "args")
		cfg.Env = stringMapField(serverMap, "env")
		cfg.Headers = stringMapField(serverMap, "headers")
		cfg.IdleTTLSecs = intField(serverMap, "idle_ttl_secs")
		cfg.Enabled = boolFieldWithDefault(serverMap, "enabled", false)
		cfg.Description = stringField(serverMap, "description")
		// 凭据只存 ref（值在 credStore），加载时必须读回，否则建连时无法解析凭据
		cfg.CredentialRef = stringField(serverMap, "credential_ref")

		// 推断 transport 类型
		switch {
		case cfg.Command != "":
			cfg.Type = ServerTypeStdio
		case cfg.URL != "":
			cfg.Type = classifyRemoteURL(cfg.URL)
		default:
			continue // 既无 command 也无 url，跳过
		}

		if cfg.IdleTTLSecs <= 0 {
			cfg.IdleTTLSecs = 300
		}

		if err := cfg.Validate(); err != nil {
			continue
		}
		servers = append(servers, cfg)
	}
	return servers, nil
}

// SaveServers 把 server 配置写回 mcp.json。
// 关键设计：读出现有文件 → 用新 servers 的协议字段覆盖 → 原样保留扩展字段
// （title / stars / description / GitHub repo url 等 Go 侧不管理的字段）。
// 这样用户直接编辑 JSON 和通过 RPC 增删改，两边操作的都是同一个文件，互不丢失。
func SaveServers(path string, servers []ServerConfig) error {
	raw, err := loadRawFile(path)
	if err != nil {
		return err
	}

	rawServers := raw["mcpServers"].(map[string]any)

	// 收集传入的 name 集合，用于清理被删除的 server
	incomingNames := make(map[string]struct{}, len(servers))
	for _, s := range servers {
		incomingNames[s.Name] = struct{}{}
	}

	for _, s := range servers {
		entry, exists := rawServers[s.Name].(map[string]any)
		if !exists {
			entry = map[string]any{}
		}

		// 只写协议相关字段
		writeString(entry, "title", s.Title)
		writeString(entry, "command", s.Command)
		writeStringSlice(entry, "args", s.Args)
		writeString(entry, "url", s.URL)
		writeStringMap(entry, "headers", s.Headers)
		writeStringMap(entry, "env", s.Env)
		if s.IdleTTLSecs > 0 {
			entry["idle_ttl_secs"] = s.IdleTTLSecs
		}
		// enabled 由 Go 侧管理，显式写布尔值（缺省 false，见 LoadServers）
		entry["enabled"] = s.Enabled
		// credential_ref 由 Go 侧管理：值为空时删除该键（此时凭据未配置）
		writeString(entry, "credential_ref", s.CredentialRef)
		rawServers[s.Name] = entry
	}

	// 清理已删除的 server
	for name := range rawServers {
		if _, keep := incomingNames[name]; !keep {
			delete(rawServers, name)
		}
	}

	raw["mcpServers"] = rawServers

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 mcp.json 目录失败: %w", err)
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 mcp.json 失败: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 mcp.json 失败: %w", err)
	}
	return os.Rename(tmpPath, path)
}

// ── 版本字段读写（mcp.json 顶层）────────────────────────────────────────────

// GetMeta 返回 mcp.json 的顶层版本元信息。文件不存在时返回零值。
// 用于对比本地与远程哪个新。
func GetMeta(path string) (version int, updatedAt string, err error) {
	raw, err := loadRawFile(path)
	if err != nil {
		return 0, "", err
	}
	version = intField(raw, "version")
	updatedAt = stringField(raw, "updated_at")
	return version, updatedAt, nil
}

// MergeRemote 把远程 server 条目合并进本地 mcp.json。
// 合并策略：
//   - 远程有、本地没有 → 新增进本地
//   - 两边都有 → 保留本地的（用户自己加的或之前同步时就有的）
//   - 远程没有、本地有 → 保留本地的（用户自己加的，不能被删除）
//
// 返回值：added=本次新增的 server 名列表；keep=本地原有（含冲突保留）的 server 数。
func MergeRemote(path string, remoteData []byte) (added []string, err error) {
	// 解析远程 mcp.json
	var remoteRaw map[string]any
	if err := json.Unmarshal(remoteData, &remoteRaw); err != nil {
		return nil, fmt.Errorf("解析远程 mcp.json 失败: %w", err)
	}
	remoteServers, _ := remoteRaw["mcpServers"].(map[string]any)
	if len(remoteServers) == 0 {
		return nil, nil // 远程空，无需合并
	}

	// 读取本地 raw map（保留用户添加的扩展字段）
	localRaw, err := loadRawFile(path)
	if err != nil {
		return nil, err
	}
	localServers, _ := localRaw["mcpServers"].(map[string]any)
	if localServers == nil {
		localServers = map[string]any{}
	}

	// 收集本地已有集合
	localNames := make(map[string]struct{}, len(localServers))
	for name := range localServers {
		localNames[name] = struct{}{}
	}

	// 把远程有本地没有的加进去
	for name, entry := range remoteServers {
		if _, exists := localNames[name]; exists {
			continue // 冲突：保留本地
		}
		localServers[name] = entry
		added = append(added, name)
	}

	if len(added) == 0 {
		return nil, nil // 无新增，不触发写入
	}

	// 写回本地
	localRaw["mcpServers"] = localServers
	// 刷新本地版本号（每次合并递增，方便下次对比）
	localRaw["version"] = intField(localRaw, "version") + 1

	// 原子写入
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 mcp.json 目录失败: %w", err)
	}
	data, err := json.MarshalIndent(localRaw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 mcp.json 失败: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("写入 mcp.json 失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("替换 mcp.json 失败: %w", err)
	}
	return added, nil
}

// ── ServerConfig 增删改的文件级辅助 ───────────────────────────────────────────

// AddOrUpdateServer 在 mcp.json 中新增或更新一个 server。
func AddOrUpdateServer(path string, cfg ServerConfig) error {
	servers, err := LoadServers(path)
	if err != nil {
		return err
	}
	for i, s := range servers {
		if s.Name == cfg.Name {
			servers[i] = cfg
			return SaveServers(path, servers)
		}
	}
	servers = append(servers, cfg)
	return SaveServers(path, servers)
}

// RemoveServerByName 从 mcp.json 中删除指定 server。
func RemoveServerByName(path, name string) error {
	servers, err := LoadServers(path)
	if err != nil {
		return err
	}
	filtered := make([]ServerConfig, 0, len(servers))
	for _, s := range servers {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	return SaveServers(path, filtered)
}

// ── 字段提取辅助 ────────────────────────────────────────────────────────────

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func stringSliceField(m map[string]any, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]any); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func stringMapField(m map[string]any, key string) map[string]string {
	if v, ok := m[key]; ok {
		if mm, ok := v.(map[string]any); ok {
			result := make(map[string]string, len(mm))
			for k, vv := range mm {
				if s, ok := vv.(string); ok {
					result[k] = s
				}
			}
			return result
		}
	}
	return nil
}

func intField(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

// boolFieldWithDefault 读取布尔字段，缺失或类型不合法时返回默认值。
func boolFieldWithDefault(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func writeString(m map[string]any, key, val string) {
	if val == "" {
		delete(m, key)
	} else {
		m[key] = val
	}
}

func writeStringSlice(m map[string]any, key string, vals []string) {
	if len(vals) == 0 {
		delete(m, key)
	} else {
		m[key] = vals
	}
}

func writeStringMap(m map[string]any, key string, vals map[string]string) {
	if len(vals) == 0 {
		delete(m, key)
	} else {
		m[key] = vals
	}
}

// ── URL 类型推断 ─────────────────────────────────────────────────────────────

// classifyRemoteURL 从 URL 推断远程 server 类型。
// 按 MCP 社区惯例，SSE 端点路径为 /sse（或以 /sse 开头的子路径，如 /sse?sessionId=..）；
// 仅对解析后的 path 做匹配，避免 host/查询参数里的 "sse" 子串误判（如 api.sservice.com）。
// 其余一律视为 HTTP（Streamable HTTP，当前主流）。
func classifyRemoteURL(rawURL string) ServerType {
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		path = u.Path
	}
	path = strings.ToLower(path)
	if path == "/sse" || strings.HasPrefix(path, "/sse/") || strings.HasSuffix(path, "/sse") {
		return ServerTypeSSE
	}
	return ServerTypeHTTP
}
