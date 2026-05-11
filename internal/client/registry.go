package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Command struct {
	Name        string
	Description string
	Category    string
	Hidden      bool
	Run         func(args string) *CommandResult
	SubCommands []Command
}

type CommandResult struct {
	Message   string
	ClearChat bool
}

type SlashCommandRegistry struct {
	commands  []Command
	agents    []agentInfo
	queryFunc func(queryType, name string) (string, error)
}

func NewSlashCommandRegistry() *SlashCommandRegistry {
	return &SlashCommandRegistry{}
}

func (r *SlashCommandRegistry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
}

func (r *SlashCommandRegistry) All() []Command { return r.commands }

func (r *SlashCommandRegistry) Visible() []Command {
	var out []Command
	for _, cmd := range r.commands {
		if !cmd.Hidden {
			out = append(out, cmd)
		}
	}
	return out
}

func (r *SlashCommandRegistry) Filter(prefix string) []Command {
	var out []Command
	for _, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		if strings.HasPrefix(cmd.Name, prefix) {
			out = append(out, cmd)
		}
	}
	return out
}

func (r *SlashCommandRegistry) Find(name string) *Command {
	for i := range r.commands {
		if r.commands[i].Name == name {
			return &r.commands[i]
		}
	}
	return nil
}

func (r *SlashCommandRegistry) SetAgents(agents []agentInfo)      { r.agents = agents }
func (r *SlashCommandRegistry) SetQueryFunc(fn func(queryType, name string) (string, error)) {
	r.queryFunc = fn
}

func BuiltinCommands() *SlashCommandRegistry {
	r := NewSlashCommandRegistry()

	knownCommands := []Command{
		{Name: "help", Description: "显示所有可用命令", Category: "ui"},
		{Name: "clear", Description: "清理当前所有上下文", Category: "ui"},
		{Name: "exit", Description: "退出 MindX", Category: "ui"},
		{Name: "about", Description: "关于 MindX", Category: "system"},
		{Name: "agents", Description: "显示智能体列表", Category: "agent"},
		{Name: "models", Description: "列出所有可用模型", Category: "model"},
		{Name: "skills", Description: "列出所有可用技能", Category: "skill"},
	}

	for _, cmd := range knownCommands {
		switch cmd.Name {
		case "help":
			cmd.Run = func(args string) *CommandResult {
				return &CommandResult{Message: r.helpText()}
			}
		case "clear":
			cmd.Run = func(args string) *CommandResult {
				return &CommandResult{ClearChat: true}
			}
		case "exit":
			cmd.Run = func(args string) *CommandResult {
				return &CommandResult{ClearChat: false, Message: "EXIT"}
			}
		default:
			name := cmd.Name
			cmd.Run = func(args string) *CommandResult {
				return r.defaultQueryRun(name, args)
			}
		}

		r.Register(cmd)
	}

	return r
}

func (r *SlashCommandRegistry) defaultQueryRun(name, args string) *CommandResult {
	if r.queryFunc == nil {
		return &CommandResult{Message: fmt.Sprintf("❌ 错误: 查询功能未初始化")}
	}

	result, err := r.queryFunc(name, args)
	if err != nil {
		return &CommandResult{Message: fmt.Sprintf("❌ 查询失败: %v", err)}
	}
	if result == "" {
		result = "✅ 操作成功"
	}

	markdown := r.formatCommandResult(name, result)
	return &CommandResult{Message: markdown}
}

func (r *SlashCommandRegistry) formatCommandResult(name, jsonStr string) string {
	switch name {
	case "agents", "models", "skills":
		var items []map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			return jsonStr
		}
		if len(items) == 0 {
			return "*暂无数据*"
		}

		title := ""
		switch name {
		case "agents":
			title = "# 可用 Agent\n"
		case "models":
			title = "# 可用模型\n"
		case "skills":
			title = "# 可用 Skill\n"
		}

		var b strings.Builder
		b.WriteString(title)
		b.WriteString("\n")
		for _, item := range items {
			nameField := item["name"]
			if nameField == "" {
				nameField = item["label"]
				if nameField == "" {
					nameField = item["value"]
				}
			}
			desc := item["description"]
			if desc == "" {
				desc = item["desc"]
			}
			b.WriteString(fmt.Sprintf("- **%s** — %s\n", nameField, desc))
		}
		return b.String()
	default:
		return jsonStr
	}
}

func (r *SlashCommandRegistry) helpText() string {
	var b strings.Builder
	b.WriteString("可用命令:\n")
	for _, cmd := range r.Visible() {
		b.WriteString(fmt.Sprintf("  /%-14s %s\n", cmd.Name, cmd.Description))
		for _, sub := range cmd.SubCommands {
			b.WriteString(fmt.Sprintf("    /%-12s %s\n", sub.Name, sub.Description))
		}
	}
	return b.String()
}
