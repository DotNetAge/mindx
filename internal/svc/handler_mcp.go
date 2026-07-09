package svc

import (
	"context"
	"encoding/json"

	"github.com/DotNetAge/mindx/internal/mcp"
)

// MCPManager returns the MCP manager instance.
func (d *Daemon) MCPManager() *mcp.Manager {
	return d.mcpMgr
}

func (d *Daemon) handleMCPServerAdd(ctx context.Context, params json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.server.add", params)
}

func (d *Daemon) handleMCPServerRemove(ctx context.Context, params json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.server.remove", params)
}

func (d *Daemon) handleMCPServerList(ctx context.Context, _ json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.server.list", nil)
}

func (d *Daemon) handleMCPServerTest(ctx context.Context, params json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.server.test", params)
}

func (d *Daemon) handleMCPServerDiscover(ctx context.Context, params json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.server.discover", params)
}

func (d *Daemon) handleMCPManifestSave(ctx context.Context, params json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.manifest.save", params)
}

func (d *Daemon) handleMCPManifestGet(ctx context.Context, _ json.RawMessage) (any, error) {
	if d.mcpMgr == nil {
		return nil, errMCPServiceUnavailable
	}
	return d.mcpMgr.RPCHandler().Handle(ctx, "mcp.manifest.get", nil)
}

var errMCPServiceUnavailable = &jsonError{code: -32000, message: "MCP service unavailable (kvstore not initialized)"}

type jsonError struct {
	code    int
	message string
}

func (e *jsonError) Error() string {
	return e.message
}
