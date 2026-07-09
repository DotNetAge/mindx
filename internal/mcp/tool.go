package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/goharness/tools"
)

// mcpTool implements tools.FuncTool for a single MCP tool.
type mcpTool struct {
	name        string
	description string
	schema      map[string]any
	server      string
	mcpName     string
	pool        *ConnectionPool
}

// Ensure mcpTool implements tools.FuncTool.
var _ tools.FuncTool = (*mcpTool)(nil)

// Info returns the tool metadata, exposed to the LLM via Tool Catalog and
// Tool Definitions (when activated via ToolSelector).
func (t *mcpTool) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        t.name,
		Description: t.description,
		Parameters:  convertSchema(t.schema),
	}
}

// Execute invokes the MCP tool via the connection pool.
func (t *mcpTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := t.pool.Call(ctx, t.server, t.mcpName, params)
	if err != nil {
		return nil, fmt.Errorf("mcp tool %q: %w", t.name, err)
	}
	return result, nil
}

// BuildTools creates mcpTool instances from the manifest.
// Each entry in the manifest becomes a separate FuncTool registered in goharness.
func BuildTools(manifest *ToolManifest, pool *ConnectionPool) []tools.FuncTool {
	out := make([]tools.FuncTool, 0, len(manifest.Tools))
	for _, entry := range manifest.Tools {
		out = append(out, &mcpTool{
			name:        entry.Name,
			description: entry.Description,
			schema:      entry.InputSchema,
			server:      entry.Server,
			mcpName:     entry.MCPName,
			pool:        pool,
		})
	}
	return out
}

// convertSchema converts an MCP JSON Schema inputSchema to goharness []Parameter.
func convertSchema(schema map[string]any) []tools.Parameter {
	if schema == nil {
		return nil
	}

	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return nil
	}

	// Build required set for quick lookup.
	reqSet := make(map[string]bool)
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				reqSet[name] = true
			}
		}
	}

	var params []tools.Parameter
	for name, propRaw := range props {
		prop, _ := propRaw.(map[string]any)
		if prop == nil {
			continue
		}
		typ, _ := prop["type"].(string)
		desc, _ := prop["description"].(string)

		p := tools.Parameter{
			Name:        name,
			Type:        typ,
			Description: desc,
			Required:    reqSet[name],
		}
		if def, ok := prop["default"]; ok {
			p.Default = def
		}
		if enum, ok := prop["enum"].([]any); ok {
			p.Enum = enum
		}
		params = append(params, p)
	}
	return params
}
