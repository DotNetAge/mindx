package svc

import (
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/commands"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

// RegisterBuiltinCommands wires up all commands to the gateway server.
// This is a thin routing layer — all command definitions and logic live in
// the internal/commands package.
func RegisterBuiltinCommands(gw *gateway.Server, app *App) {
	// 1. Inject dependencies into the commands package
	commands.SetCatalogDeps(commands.CatalogDeps{
		ListAgents: func() ([]map[string]string, error) { return listAgents(app) },
		ListModels: func() ([]map[string]string, error) { return listModels(app) },
		ListSkills: func() ([]map[string]string, error) { return listSkills(app) },
	})

	commands.SetSchedulerDeps(commands.SchedulerDeps{
		SchedulerDB: func() *scheduler.FileSchedulerStore { return app.SchedulerDB() },
		Scheduler:   func() *scheduler.Scheduler { return app.Scheduler() },
	})

	// 2. Register all commands to the gateway
	commands.New().RegisterAll(gw)
}

// GetCommandMetas returns all command metadata for client sync.
func GetCommandMetas() []gateway.CommandMeta {
	return commands.New().Metas()
}

func listAgents(app *App) ([]map[string]string, error) {
	registry := app.Agents()
	agents := registry.List()
	masterName := app.settings.MasterAgent

	var result []map[string]string
	for _, agent := range agents {
		entry := map[string]string{
			"name":        agent.Name,
			"role":        agent.Role,
			"description": agent.Description,
			"model":       agent.Model,
		}
		if agent.Name == masterName {
			entry["master"] = "true"
		}
		result = append(result, entry)
	}
	return result, nil
}

func listSkills(app *App) ([]map[string]string, error) {
	// Master agent must be initialized before skills are available.
	// If master is not configured (no MINDX_MASTER), return empty list gracefully.
	m, err := app.getMaster()
	if err != nil {
		return []map[string]string{}, nil
	}
	if m.Reactor() == nil {
		return []map[string]string{}, nil
	}
	skills := m.Reactor().SkillRegistry().ListSkills()

	var result []map[string]string
	for _, skill := range skills {
		result = append(result, map[string]string{
			"name":        skill.Name,
			"description": skill.Description,
		})
	}
	return result, nil
}

func listModels(app *App) ([]map[string]string, error) {
	models := app.Models().List()
	var result []map[string]string
	for _, model := range models {
		result = append(result, map[string]string{
			"name":        model.Name,
			"description": model.Description,
		})
	}
	return result, nil
}
