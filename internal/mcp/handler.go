package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// ── RPC Request/Response types ──────────────────────────────────────────────

// ServerAddParams is the parameter for mcp.server.add.
type ServerAddParams struct {
	Name       string            `json:"name"`
	Title      string            `json:"title,omitempty"`
	Type       ServerType        `json:"type"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Credential map[string]string `json:"credential,omitempty"`
	// CredentialGroup 账号级凭据组（与 Credential 二选一：给了组就只存组级密钥）
	CredentialGroup string `json:"credential_group,omitempty"`
	IdleTTLSecs     int    `json:"idle_ttl_secs"`
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

// PendingConfirmParams is the parameter for mcp.pending.confirm.
type PendingConfirmParams struct {
	Names []string `json:"names"`
}

// ToolCallParams is the parameter for mcp.tool.call.
type ToolCallParams struct {
	Server string         `json:"server"`
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
}

// ToolCallResult is the result of mcp.tool.call.
type ToolCallResult struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
	Result string `json:"result"`
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
	// CredentialGroup 账号级凭据组名（如 aliyun），连接器只记组不存值
	CredentialGroup string `json:"credential_group,omitempty"`
	IdleTTLSecs     int    `json:"idle_ttl_secs"`
	// Enabled 是否启用（mcp.json 的 enabled 属性，缺省 false）
	Enabled bool `json:"enabled"`
	// Description 面向用户的描述（mcp.json 的 description 属性）
	Description string `json:"description,omitempty"`
	// CredentialSet 该连接器的凭据是否已落库（前端据此决定还显不显示「填密钥」）
	CredentialSet bool `json:"credential_set"`
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
	case "mcp.credential.list":
		return h.handleCredentialList(ctx)
	case "mcp.credential.set_group":
		return h.handleCredentialSetGroup(ctx, params)
	case "mcp.server.test":
		return h.handleServerTest(ctx, params)
	case "mcp.server.discover":
		return h.handleServerDiscover(ctx, params)
	case "mcp.tool.call":
		return h.handleToolCall(ctx, params)
	case "mcp.sync":
		return h.handleSync(ctx)
	case "mcp.pending.list":
		return h.handlePendingList()
	case "mcp.pending.confirm":
		return h.handlePendingConfirm(params)
	case "mcp.pending.dismiss":
		return h.handlePendingDismiss(params)
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
	// CLI 场景允许不传 type，按 command/url 推断（与 LoadServers 同规则）
	if cfg.Type == "" {
		switch {
		case cfg.Command != "":
			cfg.Type = ServerTypeStdio
		case cfg.URL != "":
			cfg.Type = classifyRemoteURL(cfg.URL)
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if len(p.Credential) > 0 {
		// 对称键写入：<mcp_name>:<env_name>（与技能凭据同构，resolveCreds
		// 按 env 声明逐键解析）。历史 mcp_cred_<name>_<k> 键因存储接口无
		// 删除能力而残留，但不再被解析引用。
		for k, v := range p.Credential {
			if err := h.mgr.credStore.Set(p.Name+":"+k, v); err != nil {
				return nil, fmt.Errorf("store credential: %w", err)
			}
		}
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

	title := p.Title
	if title == "" {
		title = old.Title // 前端只传凭据时不应把展示名冲掉
	}
	group := p.CredentialGroup
	if group == "" {
		group = old.CredentialGroup // 同理，别把已有的组引用冲掉
	}
	cfg := ServerConfig{
		Name:            p.Name,
		Title:           title,
		Type:            p.Type,
		Command:         p.Command,
		Args:            p.Args,
		URL:             p.URL,
		Headers:         p.Headers,
		Env:             p.Env,
		IdleTTLSecs:     p.IdleTTLSecs,
		Enabled:         old.Enabled,
		CredentialRef:   old.CredentialRef,
		CredentialGroup: group,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if len(p.Credential) > 0 {
		// 凭据对应的 env / header 声明置空：解析侧只认「值未填」的键（见
		// ConnectionPool.resolveCreds），留着 `<YOUR_XX>` 占位会被当成真值。
		if cfg.Env != nil {
			for k := range p.Credential {
				if _, ok := cfg.Env[k]; ok {
					cfg.Env[k] = ""
				}
			}
		}
		if cfg.Headers != nil {
			for k := range p.Credential {
				if _, ok := cfg.Headers[k]; ok {
					cfg.Headers[k] = ""
				}
			}
		}
		// 对称键写入：<mcp_name>:<env_name>（同 add）。新凭据少于旧值时
		// credStore 旧 key 残留（凭据存储接口无删除能力），可接受：多出的
		// 残留值不会被 resolveCreds 引用（解析只认 env 声明中值为空的键）。
		for k, v := range p.Credential {
			if err := h.mgr.credStore.Set(p.Name+":"+k, v); err != nil {
				return nil, fmt.Errorf("store credential: %w", err)
			}
		}
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

// handleCredentialList 列出账号级凭据组（只有展示名与是否已填，不回值）。
func (h *RPCHandler) handleCredentialList(ctx context.Context) (any, error) {
	groups, err := h.mgr.ListCredentialGroups()
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// CredentialGroupParams is the parameter for mcp.credential.set_group.
type CredentialGroupParams struct {
	Name    string            `json:"group"`
	Display string            `json:"display"`
	Fields  map[string]string `json:"fields"`
	Values  map[string]string `json:"values"`
	Servers []string          `json:"servers"`
}

// handleCredentialSetGroup 写入账号级凭据组，并把引用该组的连接器打上组标记。
//
// 为什么单列一个 RPC：阿里云一套 AccessKey 顶着 15 个产品，逐个连接器写会
// 变成 15 份拷贝——轮换要改 15 处、漏一处就是「有的能用有的不能用」。
// 组级只存一份，连接器只记组名，读写都省。
func (h *RPCHandler) handleCredentialSetGroup(ctx context.Context, raw json.RawMessage) (any, error) {
	var p CredentialGroupParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("group 不能为空")
	}
	if len(p.Values) == 0 {
		return nil, fmt.Errorf("没有要保存的凭据值")
	}
	for k, v := range p.Values {
		if err := h.mgr.credStore.Set("group:"+p.Name+":"+k, v); err != nil {
			return nil, fmt.Errorf("store credential: %w", err)
		}
	}
	// 展示名与字段中文名落元信息文件（值不进，只进 Keychain）
	if err := h.mgr.SaveGroupMeta(p.Name, p.Display, p.Fields); err != nil {
		return nil, fmt.Errorf("save credential meta: %w", err)
	}
	servers, err := LoadServers(h.mgr.ConfigPath())
	if err != nil {
		return nil, err
	}
	stamped := make([]string, 0, len(p.Servers))
	for i := range servers {
		s := &servers[i]
		if !contains(p.Servers, s.Name) {
			continue
		}
		s.CredentialGroup = p.Name
		if err := h.mgr.AddServer(ctx, *s); err != nil {
			return nil, err
		}
		if s.Enabled {
			h.mgr.pool.Disconnect(s.Name)
		}
		stamped = append(stamped, s.Name)
	}
	return map[string]any{"ok": true, "servers": stamped}, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// hasCredential 判断连接器是否已经有可用凭据：任一声明的 env/header 键在凭据库
// 里存有非空值，或配置了 credential_ref，即视为已配置。
func (h *RPCHandler) hasCredential(s ServerConfig) bool {
	if s.CredentialRef != "" {
		return true
	}
	if h.mgr.credStore == nil {
		return false
	}
	keys := func(k string) []string {
		out := []string{s.Name + ":" + k}
		if s.CredentialGroup != "" {
			out = append(out, "group:"+s.CredentialGroup+":"+k)
		}
		return out
	}
	for k := range s.Env {
		for _, key := range keys(k) {
			if v, err := h.mgr.credStore.Get(key); err == nil && v != "" {
				return true
			}
		}
	}
	for k := range s.Headers {
		for _, key := range keys(k) {
			if v, err := h.mgr.credStore.Get(key); err == nil && v != "" {
				return true
			}
		}
	}
	return false
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
		entry := ServerListEntry{
			Name:            s.Name,
			Title:           s.Title,
			Type:            s.Type,
			Command:         s.Command,
			Args:            s.Args,
			URL:             s.URL,
			Headers:         s.Headers,
			Env:             s.Env,
			CredentialRef:   s.CredentialRef,
			CredentialGroup: s.CredentialGroup,
			IdleTTLSecs:     s.IdleTTLSecs,
			Enabled:         s.Enabled,
			Description:     s.Description,
		}
		entry.CredentialSet = h.hasCredential(s)
		entries = append(entries, entry)
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

// handleToolCall 调用一个已启用 server 的工具（mcp.tool.call）。
// pool.Call 只认 enabled server（disabled 不进 pool 索引），故先做前置校验，
// 用报错文案引导 CLI 用户走 add / enable 流程。
func (h *RPCHandler) handleToolCall(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ToolCallParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Server == "" || p.Tool == "" {
		return nil, fmt.Errorf("server 与 tool 均不能为空")
	}

	cfg, err := h.mgr.lookupConfig(p.Server)
	if err != nil {
		return nil, fmt.Errorf("%q 不在 mcp.json 中，请先 mindx mcp add", p.Server)
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("%q 未启用，请先 mindx mcp enable %s", p.Server, p.Server)
	}

	result, err := h.mgr.pool.Call(ctx, p.Server, p.Tool, p.Args)
	if err != nil {
		return nil, err
	}
	return ToolCallResult{Server: p.Server, Tool: p.Tool, Result: result}, nil
}

// handlePendingList 返回待确认的远程条目（远程声明的本地进程）。
//
// 为什么需要这个只读口：远程同步在智能主机启动时自动跑，用户打开面板时
// 同步早已结束，拿不到当次响应；面板必须能主动查询「有哪些条目在等我确认」。
func (h *RPCHandler) handlePendingList() (any, error) {
	pending := h.mgr.ListPending()
	if pending == nil {
		return []PendingServer{}, nil
	}
	return pending, nil
}

// handlePendingDismiss 丢弃用户明确不启用的待确认条目。
func (h *RPCHandler) handlePendingDismiss(raw json.RawMessage) (any, error) {
	var p PendingConfirmParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if len(p.Names) == 0 {
		return nil, fmt.Errorf("请选择要忽略的条目")
	}
	moved, err := h.mgr.DismissPending(p.Names)
	if err != nil {
		return nil, err
	}
	return map[string]any{"dismissed": moved}, nil
}

// handlePendingConfirm 启用用户已确认的远程条目（远程 stdio 条目的唯一启用入口）。
func (h *RPCHandler) handlePendingConfirm(raw json.RawMessage) (any, error) {
	var p PendingConfirmParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if len(p.Names) == 0 {
		return nil, fmt.Errorf("请选择要启用的条目")
	}
	moved, err := h.mgr.ConfirmPending(p.Names)
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": moved}, nil
}
