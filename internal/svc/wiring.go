package svc

import (
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/commands"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

func RegisterBuiltinCommands(gw *gateway.Server, app *core.App, d *Daemon) {
	commands.SetCatalogDeps(commands.CatalogDeps{
		ListAgents: func() ([]map[string]string, error) { return listAgents(app), nil },
		ListModels: func() ([]map[string]string, error) { return listModels(app), nil },
		ListSkills: func() ([]map[string]string, error) { return listSkills(app), nil },
	})

	commands.SetSchedulerDeps(commands.SchedulerDeps{
		SchedulerDB: func() *scheduler.FileSchedulerStore { return d.SchedulerDB() },
		Scheduler:   func() *scheduler.Scheduler { return d.Scheduler() },
	})

	commands.New().RegisterAll(gw)
}

func GetCommandMetas() []gateway.CommandMeta {
	return commands.New().Metas()
}

func listAgents(app *core.App) []map[string]string {
	registry := app.Agents()
	agents := registry.List()
	activeName := app.CurrentAgentName()

	var result []map[string]string
	for _, agent := range agents {
		entry := map[string]string{
			"name":        agent.Name,
			"role":        agent.Role,
			"description": agent.Description,
			"model":       agent.Model,
		}
		if agent.Name == activeName {
			entry["active"] = "true"
		}
		result = append(result, entry)
	}
	return result
}

func listSkills(app *core.App) []map[string]string {
	m, err := app.CurrentAgent()
	if err != nil {
		return []map[string]string{}
	}
	if m.Reactor() == nil {
		return []map[string]string{}
	}
	skills := m.Reactor().SkillRegistry().ListSkills()

	var result []map[string]string
	for _, skill := range skills {
		result = append(result, map[string]string{
			"name":        skill.Name,
			"description": skill.Description,
		})
	}
	return result
}

func listModels(app *core.App) []map[string]string {
	models := app.Models().List()
	var result []map[string]string
	for _, model := range models {
		result = append(result, map[string]string{
			"name":        model.Name,
			"description": model.Description,
		})
	}
	return result
}
