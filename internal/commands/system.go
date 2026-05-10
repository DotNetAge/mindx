package commands

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
)

func registerSystemCommands(r *Registry) {
	r.Register(Meta{
		Name:        "help",
		Description: "显示所有可用命令",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleHelp(ctx)
	})

	r.Register(Meta{
		Name:        "about",
		Description: "关于 MindX",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Example:     "/about",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return "MindX Agent Chat v0.1 — AI 智能体交互终端", nil
	})

	r.Register(Meta{
		Name:        "init",
		Description: "初始化会话",
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return "会话已初始化", nil
	})

	r.Register(Meta{
		Name:        "clear",
		Description: "清理当前所有上下文",
		Category:    "system",
		Scope:       gateway.ScopeBoth,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return "__clear__", nil
	})
}

func handleHelp(ctx *gateway.CommandContext) (any, error) {
	cmds := ctx.Server().CommandList()
	result := make([]string, 0, len(cmds))
	for _, meta := range cmds {
		if strings.HasPrefix(meta.Name, "_") {
			continue
		}
		result = append(result, fmt.Sprintf("  /%-12s %s", meta.Name, meta.Description))
	}
	if len(result) == 0 {
		return "(暂无可用命令)", nil
	}
	return fmt.Sprintf("可用命令:\n%s", strings.Join(result, "\n")), nil
}
