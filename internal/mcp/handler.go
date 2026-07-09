package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ── RPC Request/Response types ──────────────────────────────────────────────

// ServerAddParams is the parameter for mcp.server.add.
type ServerAddParams struct {
	Name        string            `json:"name"`
	Type        ServerType        `json:"type"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Credential  map[string]string `json:"credential,omitempty"`
	IdleTTLSecs int               `json:"idle_ttl_secs"`
}

// ServerRemoveParams is the parameter for mcp.server.remove.
type ServerRemoveParams struct {
	Name string `json:"name"`
}

// ServerTestParams is the parameter for mcp.server.test.
type ServerTestParams struct {
	Name string `json:"name"`
}

// ServerDiscoverParams is the parameter for mcp.server.discover.
type ServerDiscoverParams struct {
	Name string `json:"name"`
}

// ManifestSaveParams is the parameter for mcp.manifest.save.
type ManifestSaveParams struct {
	Tools []ToolManifestEntry `json:"tools"`
}

// DiscoveredTool represents a tool returned by mcp.server.discover.
type DiscoveredTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ServerListEntry is returned by mcp.server.list.
type ServerListEntry struct {
	Name string     `json:"name"`
	Type ServerType `json:"type"`
	// stdio fields
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	// sse/http fields
	URL string `json:"url,omitempty"`
	// credential ref only, never the value
	CredentialRef string `json:"credential_ref,omitempty"`
	IdleTTLSecs   int    `json:"idle_ttl_secs"`
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
	case "mcp.server.remove":
		return h.handleServerRemove(ctx, params)
	case "mcp.server.list":
		return h.handleServerList(ctx)
	case "mcp.server.test":
		return h.handleServerTest(ctx, params)
	case "mcp.server.discover":
		return h.handleServerDiscover(ctx, params)
	case "mcp.manifest.save":
		return h.handleManifestSave(ctx, params)
	case "mcp.manifest.get":
		return h.handleManifestGet(ctx)
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
		Type:        p.Type,
		Command:     p.Command,
		Args:        p.Args,
		URL:         p.URL,
		Env:         p.Env,
		IdleTTLSecs: p.IdleTTLSecs,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Store credential in credStore, generate ref
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

func (h *RPCHandler) handleServerList(ctx context.Context) (any, error) {
	servers, err := LoadServers(h.mgr.storage)
	if err != nil {
		return nil, err
	}
	if servers == nil {
		return []ServerListEntry{}, nil
	}
	entries := make([]ServerListEntry, 0, len(servers))
	for _, s := range servers {
		entries = append(entries, ServerListEntry{
			Name:          s.Name,
			Type:          s.Type,
			Command:       s.Command,
			Args:          s.Args,
			URL:           s.URL,
			CredentialRef: s.CredentialRef,
			IdleTTLSecs:   s.IdleTTLSecs,
		})
	}
	return entries, nil
}

func (h *RPCHandler) handleServerTest(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerTestParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if err := h.mgr.pool.TestConnection(ctx, p.Name); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	return map[string]bool{"ok": true}, nil
}

func (h *RPCHandler) handleServerDiscover(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ServerDiscoverParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	tools, err := h.mgr.pool.DiscoverTools(ctx, p.Name)
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

func (h *RPCHandler) handleManifestSave(ctx context.Context, raw json.RawMessage) (any, error) {
	var p ManifestSaveParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Build goharness tool names from server + mcp_name
	for i := range p.Tools {
		p.Tools[i].Name = fmt.Sprintf("mcp:%s:%s", p.Tools[i].Server, p.Tools[i].MCPName)
	}

	manifest := &ToolManifest{Tools: p.Tools}
	if err := SaveManifest(h.mgr.storage, manifest); err != nil {
		return nil, err
	}

	return map[string]any{"ok": true, "tool_count": len(p.Tools)}, nil
}

func (h *RPCHandler) handleManifestGet(ctx context.Context) (any, error) {
	manifest, err := LoadManifest(h.mgr.storage)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

// toConfig converts ServerAddParams to ServerConfig.
func (p ServerAddParams) toConfig() ServerConfig {
	return ServerConfig{
		Name:        p.Name,
		Type:        ServerType(strings.ToLower(string(p.Type))),
		Command:     p.Command,
		Args:        p.Args,
		URL:         p.URL,
		Env:         p.Env,
		IdleTTLSecs: p.IdleTTLSecs,
	}
}
