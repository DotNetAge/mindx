package commands

import (
	"github.com/DotNetAge/gort/pkg/gateway"
)

func registerLocalCommands(r *LocalRegistry) {
	r.Register(Meta{
		Name:        "help",
		Description: "显示所有可用命令",
		Category:    "ui",
		Scope:       gateway.ScopeLocal,
	})

	r.Register(Meta{
		Name:        "clear",
		Description: "清理当前所有上下文",
		Category:    "ui",
		Scope:       gateway.ScopeLocal,
	})

	r.Register(Meta{
		Name:        "exit",
		Description: "退出 MindX",
		Category:    "ui",
		Scope:       gateway.ScopeLocal,
	})
}
