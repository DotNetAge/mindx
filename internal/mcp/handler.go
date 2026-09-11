package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// ── RPC Request/Response types ──────────────────────────────────────────────

// ServerAddParams is the parameter for mcp.server.add.
type ServerAddParams struct {
	Name        string            `json:"name"`
	Title       string            `json:"title,omitempty"`
	Type        ServerType        `json:"type"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Credential  map[string]string `json:"credential,omitempty"`
	IdleTTLSecs int               `json:"idle_ttl_secs"`
}

// ServerRemoveParams is the parameter for mcp.server.remove.
type ServerRemoveParams struct {
	Name string `json:"name"`
}

// ServerUpdateParams 是 mcp.server.update 的参数，字段与 ServerAddParams 一致。
// 语义差异见 handleServerUpdate：server 必须已存在；enabled 沿用现有配置；
// credential 为空时保留旧凭据。
type ServerUpdateParams = ServerAddParams

// ServerSetEnabledParams is the parameter for mcp.server.set_enabled.
type ServerSetEnabledParams struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ServerTestParams is the parameter for mcp.server.test.
type ServerTestParams struct {
	Name string `json:"name"`
}

// ServerDiscoverParams is the parameter for mcp.server.discover.
type ServerDiscoverParams struct {
	Name string `json:"name"`
}

// DiscoveredTool represents a tool returned by mcp.server.discover.
type DiscoveredTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ServerListEntry is returned by mcp.server.list.
type ServerListEntry struct {
	Name          string            `json:"name"`
	Title         string            `json:"title,omitempty"`
	Type          ServerType        `json:"type"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	URL           string            `json:"url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	IdleTTLSecs   int               `json:"idle_ttl_secs"`
	// Enabled 是否启用（mcp.json 的 enabled 属性，缺省 false）
	Enabled bool `json:"enabled"`
	// Description 面向用户的描述（mcp.json 的 description 属性）
	Description string `json:"description,omitempty"`
}

// ── RPCHandler ──────────────────────────────────────────────────────────────

// RPCHandler handles WebUI JSON-RPC calls for MCP configuration.
type RPCHandler struct {
	mgr *Manager
}

// NewRPCHandler creates a new RPC handler.
func NewRPCHandler(mgr *Manager) *RPCHandler {
	return &RPCHandler{mgr: mgr}
}

// Handle dispatches a JSON-RPC method to the appropriate handler.
func (h *RPCHandler) Handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "mcp.server.add":
		return h.handleServerAdd(ctx, params)
	case "mcp.server.update":
		return h.handleServerUpdate(ctx, params)
	case "mcp.server.remove":
		return h.handleServerRemove(ctx, params)
	case "mcp.server.set_enabled":
		return h.handleServerSetEnabled(ctx, params)
	case "mcp.server.list":
		return h.handleServerList(ctx)
	case "mcp.server.test":
		return h.handleServerTest(ctx, params)
	case "mcp.server.discover":
		return h.handleServerDiscover(ctx, params)
	case "mcp.sync":
		return h.handleSync(ctx)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (h *RPCHandler) handleServerAdd(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerAddParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	cfg := ServerConfig{
		Name:        p.Name,
		Title:       p.Title,
		Type:        p.Type,
		Command:     p.Command,
		Args:        p.Args,
		URL:         p.URL,
		Headers:     p.Headers,
		Env:         p.Env,
		IdleTTLSecs: p.IdleTTLSecs,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if len(p.Credential) > 0 {
		ref := fmt.Sprintf("mcp_cred_%s", p.Name)
		for k, v := range p.Credential {
			if err := h.mgr.credStore.Set(ref+"_"+k, v); err != nil {
				return nil, fmt.Errorf("store credential: %w", err)
			}
		}
		cfg.CredentialRef = ref
	}

	if err := h.mgr.AddServer(ctx, cfg); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// handleServerUpdate 更新一个已存在的 server 配置（mcp.server.update）。
// 与 add 的语义差异：
//   - server 必须已存在，否则报错引导用 add；
//   - enabled 沿用现有配置（更新连接参数不应改变启用状态）；
//   - credential 为空时保留旧 CredentialRef（SaveServers 的就地改写会保留
//     mcp.json 中的 credential_ref 扩展字段）；非空时按同名 ref 覆盖凭据值。
func (h *RPCHandler) handleServerUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerUpdateParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	servers, err := LoadServers(h.mgr.ConfigPath())
	if err != nil {
		return nil, err
	}
	var old *ServerConfig
	for i := range servers {
		if servers[i].Name == p.Name {
			old = &servers[i]
			break
		}
	}
	if old == nil {
		return nil, fmt.Errorf("server %q 不存在，请使用 mcp.server.add 新增", p.Name)
	}

	cfg := ServerConfig{
		Name:          p.Name,
		Title:         p.Title,
		Type:          p.Type,
		Command:       p.Command,
		Args:          p.Args,
		URL:           p.URL,
		Headers:       p.Headers,
		Env:           p.Env,
		IdleTTLSecs:   p.IdleTTLSecs,
		Enabled:       old.Enabled,
		CredentialRef: old.CredentialRef,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if len(p.Credential) > 0 {
		ref := fmt.Sprintf("mcp_cred_%s", p.Name)
		// 按同名 ref 覆盖凭据值；新凭据少于旧值时 credStore 旧 key 残留
		//（凭据存储接口无删除能力），可接受：多出的残留值不会被引用。
		for k, v := range p.Credential {
			if err := h.mgr.credStore.Set(ref+"_"+k, v); err != nil {
				return nil, fmt.Errorf("store credential: %w", err)
			}
		}
		cfg.CredentialRef = ref
	}

	if err := h.mgr.AddServer(ctx, cfg); err != nil {
		return nil, err
	}

	// 热生效：pool.Connect 会复用存活旧连接，更新后必须断开，下次使用按新配置重连；
	// 已停用的 server 直接移出连接池索引与连接。
	if cfg.Enabled {
		h.mgr.pool.Disconnect(p.Name)
	} else {
		h.mgr.pool.RemoveServer(p.Name)
	}
	return map[string]bool{"ok": true}, nil
}

func (h *RPCHandler) handleServerRemove(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerRemoveParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if err := h.mgr.RemoveServer(p.Name); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (h *RPCHandler) handleServerSetEnabled(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerSetEnabledParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := h.mgr.SetEnabled(p.Name, p.Enabled); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (h *RPCHandler) handleServerList(ctx context.Context) (any, error) {
	servers, err := LoadServers(h.mgr.ConfigPath())
	if err != nil {
		return nil, err
	}
	if servers == nil {
		return []ServerListEntry{}, nil
	}
	entries := make([]ServerListEntry, 0, len(servers))
	for _, s := range servers {
		// ServerListEntry 与 ServerConfig 字段同构，直接转换即可
		entries = append(entries, ServerListEntry(s))
	}
	return entries, nil
}

func (h *RPCHandler) handleServerTest(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerTestParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	// 走 Manager helper：disabled server 也能测（临时加进 pool 测完就删）
	if err := h.mgr.TestServerConnection(ctx, p.Name); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	return map[string]bool{"ok": true}, nil
}

func (h *RPCHandler) handleServerDiscover(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerDiscoverParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// 走 Manager helper：disabled server 也能 discover
	tools, err := h.mgr.DiscoverServerTools(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	result := make([]DiscoveredTool, 0, len(tools))
	for _, t := range tools {
		result = append(result, DiscoveredTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return result, nil
}

func (h *RPCHandler) handleSync(ctx context.Context) (any, error) {
	return h.mgr.Sync(ctx), nil
}
