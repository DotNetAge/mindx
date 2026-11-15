package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/commands"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

func RegisterBuiltinCommands(gw *gateway.Server, app *core.App, d *Daemon) {
	registerBuiltinCommandsWith(gw.RegisterCommand, app, d)
}

// registerBuiltinCommandsWith 是命令注册的唯一实现：注册动作可注入，
// 生产环境传 gw.RegisterCommand，测试传记录器以校验每条命令都被闸包住。
func registerBuiltinCommandsWith(register commands.Registerer, app *core.App, d *Daemon) {
	commands.SetCatalogDeps(commands.CatalogDeps{
		ListAgents: func() ([]map[string]string, error) { return listAgents(app), nil },
		ListModels: func() ([]map[string]string, error) { return listModels(app), nil },
		ListSkills: func() ([]map[string]string, error) { return listSkills(app), nil },
	})

	commands.SetSchedulerDeps(commands.SchedulerDeps{
		SchedulerDB: func() *scheduler.FileSchedulerStore { return d.SchedulerDB() },
		Scheduler:   func() *scheduler.Scheduler { return d.Scheduler() },
	})

	commands.SetCompactDeps(commands.CompactDeps{
		CompactSession: func(ctx context.Context, params rpc.SessionCompactParams) (map[string]any, error) {
			raw, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			result, err := d.handleSessionCompact(ctx, raw)
			if err != nil {
				return nil, err
			}
			out, ok := result.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected compact result type: %T", result)
			}
			return out, nil
		},
	})

	// 命令与 RPC 共用同一道接入闸：RegisterCommand 不走 RegisterMethod，
	// 少了这层包装就等于「注册路径不同 = 鉴权不同」。
	commands.New().RegisterAllWith(register, d.guardCommand)
}

func GetCommandMetas() []gateway.CommandMeta {
	return commands.New().Metas()
}

func listAgents(app *core.App) []map[string]string {
	registry := app.Agents()
	if registry == nil {
		return nil
	}
	agents := registry.List()
	activeName := app.CurrentAgentName()

	var result []map[string]string
	for _, agent := range agents {
		entry := map[string]string{
			"name":        agent.Meta.Name,
			"nick_name":   agent.Meta.NickName,
			"role":        agent.Meta.Role,
			"description": agent.Meta.Description,
		}
		if agent.Meta.Name == activeName {
			entry["active"] = "true"
		}
		result = append(result, entry)
	}
	return result
}

func listSkills(app *core.App) []map[string]string {
	skills := app.Skills().Global().List()

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
	modelReg := app.Models()
	if modelReg == nil {
		return nil
	}
	models := modelReg.List()
	var result []map[string]string
	for _, model := range models {
		result = append(result, map[string]string{
			"name":        model.Name,
			"description": model.Description,
		})
	}
	return result
}
