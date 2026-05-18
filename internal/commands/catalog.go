package commands

import (
	"fmt"

	"github.com/DotNetAge/gort/pkg/gateway"
)

// CatalogDeps holds external dependencies for catalog commands.
type CatalogDeps struct {
	ListAgents  func() ([]map[string]string, error)
	ListModels  func() ([]map[string]string, error)
	ListSkills  func() ([]map[string]string, error)
}

var catalogDeps CatalogDeps

// SetCatalogDeps sets the dependencies for catalog commands.
func SetCatalogDeps(deps CatalogDeps) {
	catalogDeps = deps
}

func registerCatalogCommands(r *Registry) {
	r.Register(Meta{
		Name:        "agents",
		Description: "显示智能体列表",
		Category:    "agent",
		Scope:       gateway.ScopeRemote,
		Example:     "/agents",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleAgents(ctx)
	})

	r.Register(Meta{
		Name:        "models",
		Description: "列出所有可用模型",
		Category:    "agent",
		Scope:       gateway.ScopeRemote,
		Example:     "/models",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleModels(ctx)
	})

	r.Register(Meta{
		Name:        "skills",
		Description: "列出所有可用技能",
		Category:    "agent",
		Scope:       gateway.ScopeRemote,
		Example:     "/skills",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleSkills(ctx)
	})
}

func handleAgents(ctx *gateway.CommandContext) (any, error) {
	if catalogDeps.ListAgents == nil {
		return nil, fmt.Errorf("agents command not configured")
	}
	list, err := catalogDeps.ListAgents()
	if err != nil {
		return nil, err
	}

	// Send structured notification for TUI rendering
	ctx.RespondWithType(gateway.RespTable, "Available Agents", map[string]interface{}{
		"headers": []string{"Name", "Role", "Description"},
		"rows":    toAgentTableRows(list),
	})

	// Return raw data in the JSON-RPC response so clients using
	// Call/SendCommand can parse it directly.
	result := make([]map[string]string, 0, len(list))
	for _, item := range list {
		if item["model"] == "" {
			return nil, fmt.Errorf("agent %q has no model configured", item["name"])
		}
		r := map[string]string{
			"label": item["name"],
			"value": item["name"],
			"role":  item["role"],
			"desc":  item["description"],
			"model": item["model"],
		}
		// Pass through the active flag from the backend (if present).
		if item["active"] == "true" {
			r["active"] = "true"
		}
		result = append(result, r)
	}
	return result, nil
}

func handleModels(ctx *gateway.CommandContext) (any, error) {
	if catalogDeps.ListModels == nil {
		return nil, fmt.Errorf("models command not configured")
	}
	list, err := catalogDeps.ListModels()
	if err != nil {
		return nil, err
	}

	// Send structured notification for TUI rendering
	ctx.RespondWithType(gateway.RespTable, "可用模型", map[string]interface{}{
		"headers": []string{"名称", "描述"},
		"rows":    toTableRows(list, "name", "description"),
	})

	// Return raw data in JSON-RPC response for Call-based clients
	result := make([]map[string]string, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]string{
			"label": item["name"],
			"value": item["name"],
			"desc":  item["description"],
		})
	}
	return result, nil
}

func handleSkills(ctx *gateway.CommandContext) (any, error) {
	if catalogDeps.ListSkills == nil {
		return nil, fmt.Errorf("skills command not configured")
	}
	list, err := catalogDeps.ListSkills()
	if err != nil {
		return nil, err
	}

	// Send structured notification for TUI rendering
	ctx.RespondWithType(gateway.RespTable, "可用技能", map[string]interface{}{
		"headers": []string{"名称", "描述"},
		"rows":    toTableRows(list, "name", "description"),
	})

	// Return raw data in JSON-RPC response for Call-based clients
	result := make([]map[string]string, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]string{
			"label": item["name"],
			"value": item["name"],
			"desc":  item["description"],
		})
	}
	return result, nil
}

func toAgentTableRows(items []map[string]string) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item["name"],
			item["role"],
			item["description"],
		})
	}
	return rows
}

func toTableRows(items []map[string]string, columns ...string) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = item[col]
		}
		rows = append(rows, row)
	}
	return rows
}
