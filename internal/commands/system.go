package commands

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
)

func registerSystemCommands(r *Registry) {
	r.Register(Meta{
		Name:        "help",
		Description: i18n.T("cmd.system.help.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return handleHelp(ctx)
	})

	r.Register(Meta{
		Name:        "about",
		Description: i18n.T("cmd.system.about.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
		Example:     "/about",
	}, func(ctx *gateway.CommandContext) (any, error) {
		return i18n.T("cmd.system.about.output"), nil
	})

	r.Register(Meta{
		Name:        "init",
		Description: i18n.T("cmd.system.init.desc"),
		Category:    "system",
		Scope:       gateway.ScopeRemote,
	}, func(ctx *gateway.CommandContext) (any, error) {
		return i18n.T("cmd.system.init.output"), nil
	})

	r.Register(Meta{
		Name:        "clear",
		Description: i18n.T("cmd.system.clear.desc"),
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
		return i18n.T("cmd.system.help.empty"), nil
	}
	return fmt.Sprintf(i18n.T("cmd.system.help.available")+"%s", strings.Join(result, "\n")), nil
}
