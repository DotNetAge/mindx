package client

import (
	"fmt"

	appcore "github.com/DotNetAge/mindx/internal/core"
)

type CommandResult struct {
	Message string
	Success bool
}

type CommandDef struct {
	Name          string
	Description   string
	Run           func(args []string) CommandResult
	HasSuggestion bool
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

type CommandDeps struct {
	App      *appcore.App
	OnClear  func()
	OnExit   func()
	OnDoctor func()
}

func BuiltinCommands(deps CommandDeps) *SlashCommandRegistry {
	r := NewSlashCommandRegistry()

	r.Register(CommandDef{
		Name:        "help",
		Description: "显示帮助信息",
		Run: func(args []string) CommandResult {
			var msg string
			for _, cmd := range r.List() {
				msg += fmt.Sprintf("/%-12s %s\n", cmd.Name, cmd.Description)
			}
			return CommandResult{Message: msg, Success: true}
		},
	})

	r.Register(CommandDef{
		Name:        "clear",
		Description: "清屏",
		Run: func(args []string) CommandResult {
			if deps.OnClear != nil {
				deps.OnClear()
			}
			return CommandResult{Message: "", Success: true}
		},
	})

	r.Register(CommandDef{
		Name:        "exit",
		Description: "退出程序",
		Run: func(args []string) CommandResult {
			if deps.OnExit != nil {
				deps.OnExit()
			}
			return CommandResult{Message: "退出中...", Success: true}
		},
	})

	r.Register(CommandDef{
		Name:        "doctor",
		Description: "系统诊断与安装向导",
		Run: func(args []string) CommandResult {
			if deps.OnDoctor != nil {
				deps.OnDoctor()
			}
			return CommandResult{Message: "正在启动安装向导...", Success: true}
		},
	})

	r.Register(CommandDef{
		Name:          "model",
		Description:   "切换 Agent 模型",
		HasSuggestion: true,
		Run: func(args []string) CommandResult {
			if deps.App == nil {
				return CommandResult{Message: "❌ 系统未初始化", Success: false}
			}
			if len(args) == 0 {
				var listMsg string
				models := deps.App.Models().List()
				for _, m := range models {
					listMsg += fmt.Sprintf("  %s\n", m.Name)
				}
				return CommandResult{
					Message: fmt.Sprintf("用法: /model <model_name>\n\n可用模型:\n%s", listMsg),
					Success: false,
				}
			}
			modelName := args[0]
			model := deps.App.Models().Get(modelName)
			if model == nil || !model.Enabled {
				return CommandResult{Message: fmt.Sprintf("❌ 模型 %q 不可用", modelName), Success: false}
			}
			return CommandResult{
				Message: fmt.Sprintf("✅ 已切换模型为: %s", modelName),
				Success: true,
			}
		},
	})

	r.Register(CommandDef{
		Name:          "chat",
		Description:   "管理会话 (new/clear/<session_id>)",
		HasSuggestion: true,
		Run: func(args []string) CommandResult {
			if deps.App == nil {
				return CommandResult{Message: "❌ 系统未初始化", Success: false}
			}
			if len(args) == 0 {
				return CommandResult{
					Message: "用法: /chat <session_id | new | clear>",
					Success: false,
				}
			}
			arg := args[0]
			switch arg {
			case "new":
				agentName := deps.App.CurrentAgentName()
				newSession, err := deps.App.CreateSession(agentName)
				if err != nil {
					return CommandResult{Message: fmt.Sprintf("❌ 创建会话失败: %v", err), Success: false}
				}
				cfg := deps.App.Config()
				if cfg != nil {
					cfg.LastSessionID = newSession.SessionID
					cfg.LastAgent = agentName
					_ = cfg.Save()
				}
				return CommandResult{
					Message: fmt.Sprintf("✅ 已创建新会话: %s", newSession.SessionID),
					Success: true,
				}
			case "clear":
				newSession, err := deps.App.ClearCurrentSession()
				if err != nil {
					return CommandResult{Message: fmt.Sprintf("❌ 清除会话失败: %v", err), Success: false}
				}
				cfg := deps.App.Config()
				if cfg != nil {
					cfg.LastSessionID = newSession.SessionID
					_ = cfg.Save()
				}
				return CommandResult{
					Message: fmt.Sprintf("✅ 已清除旧会话，新会话: %s", newSession.SessionID),
					Success: true,
				}
			default:
				sessionMeta, err := deps.App.SwitchSession(arg)
				if err != nil {
					return CommandResult{Message: fmt.Sprintf("❌ 切换会话失败: %v", err), Success: false}
				}
				return CommandResult{
					Message: fmt.Sprintf("✅ 已切换到会话: %s", sessionMeta.SessionID),
					Success: true,
				}
			}
		},
	})

	r.Register(CommandDef{
		Name:        "agents",
		Description: "列出所有可用 Agent",
		Run: func(args []string) CommandResult {
			return CommandResult{Message: "使用 @agent_name 切换 Agent", Success: true}
		},
	})

	return r
}
