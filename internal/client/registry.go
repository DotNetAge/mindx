package client

import (
	"fmt"
)

type CommandResult struct {
	Message string
}

type CommandDef struct {
	Name        string
	Description string
	Run         func(args []string) CommandResult
}

type SlashCommandRegistry struct {
	commands map[string]*CommandDef
}

func NewSlashCommandRegistry() *SlashCommandRegistry {
	return &SlashCommandRegistry{
		commands: make(map[string]*CommandDef),
	}
}

func (r *SlashCommandRegistry) Register(cmd CommandDef) {
	r.commands[cmd.Name] = &cmd
}

func (r *SlashCommandRegistry) Get(name string) *CommandDef {
	return r.commands[name]
}

func (r *SlashCommandRegistry) List() []CommandDef {
	list := make([]CommandDef, 0, len(r.commands))
	for _, cmd := range r.commands {
		list = append(list, *cmd)
	}
	return list
}

func helpHandler(args []string) CommandResult {
	return CommandResult{
		Message: "可用命令: /help, /clear, /exit, /transcript, /agents",
	}
}

func clearHandler(args []string) CommandResult {
	return CommandResult{Message: ""}
}

func exitHandler(args []string) CommandResult {
	return CommandResult{Message: "退出中..."}
}

func transcriptHandler(args []string) CommandResult {
	return CommandResult{Message: "切换转录模式"}
}

func agentsHandler(args []string) CommandResult {
	return CommandResult{Message: "使用 @agent_name 切换 Agent"}
}

func BuiltinCommands() *SlashCommandRegistry {
	r := NewSlashCommandRegistry()
	r.Register(CommandDef{
		Name:        "help",
		Description: "显示帮助信息",
		Run: func(args []string) CommandResult {
			var msg string
			for _, cmd := range r.List() {
				msg += fmt.Sprintf("/%s - %s\n", cmd.Name, cmd.Description)
			}
			return CommandResult{Message: msg}
		},
	})
	r.Register(CommandDef{
		Name:        "clear",
		Description: "清屏",
		Run:         clearHandler,
	})
	r.Register(CommandDef{
		Name:        "exit",
		Description: "退出程序",
		Run:         exitHandler,
	})
	r.Register(CommandDef{
		Name:        "transcript",
		Description: "切换转录视图",
		Run:         transcriptHandler,
	})
	r.Register(CommandDef{
		Name:        "agents",
		Description: "列出所有可用 Agent",
		Run:         agentsHandler,
	})
	return r
}
