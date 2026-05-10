package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
)

// Command is a TUI slash command with execution logic.
type Command struct {
	gateway.CommandMeta
	Hidden      bool
	Run         func(args string) *CommandResult
	SubCommands []Command
}

type CommandResult struct {
	Message   string
	ClearChat bool
}

// SlashCommandRegistry holds registered slash commands and remote data.
type SlashCommandRegistry struct {
	commands      []Command
	agents        []agentInfo
	models        []gateway.CommandMeta
	skills        []gateway.CommandMeta
	commandSender func(name, args string) (string, error) // 发送命令到服务器的回调
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

func (r *SlashCommandRegistry) SetAgents(agents []agentInfo)           { r.agents = agents }
func (r *SlashCommandRegistry) SetModels(models []gateway.CommandMeta) { r.models = models }
func (r *SlashCommandRegistry) SetSkills(skills []gateway.CommandMeta) { r.skills = skills }
func (r *SlashCommandRegistry) SetCommandSender(sender func(name, args string) (string, error)) {
	r.commandSender = sender
}

func (r *SlashCommandRegistry) SyncRemoteCommands(metas []gateway.CommandMeta) {
	existing := make(map[string]bool, len(r.commands))
	for _, c := range r.commands {
		existing[c.Name] = true
	}

	for _, meta := range metas {
		if strings.HasPrefix(meta.Name, "_") {
			continue
		}
		if !existing[meta.Name] {
			r.Register(Command{
				CommandMeta: meta,
				Run:         r.defaultRun(&meta),
			})
		}
	}
}

func BuiltinCommands() *SlashCommandRegistry {
	r := NewSlashCommandRegistry()

	// 预注册所有已知命令（包括远程命令）
	// 这样即使还没连接服务器，命令也在 registry 中
	knownCommands := []gateway.CommandMeta{
		{Name: "help", Description: "显示所有可用命令", Category: "ui"},
		{Name: "clear", Description: "清理当前所有上下文", Category: "ui"},
		{Name: "exit", Description: "退出 MindX", Category: "ui"},
		{Name: "about", Description: "关于 MindX", Category: "system"},
		{Name: "init", Description: "初始化会话", Category: "system"},
		{Name: "agents", Description: "显示智能体列表", Category: "agent"},
		{Name: "models", Description: "列出所有可用模型", Category: "model"},
		{Name: "skills", Description: "列出所有可用技能", Category: "skill"},
		{Name: "job-add", Description: "添加计划任务", Category: "system"},
		{Name: "job-list", Description: "列出所有计划任务", Category: "system"},
		{Name: "job-del", Description: "删除计划任务", Category: "system"},
	}

	for _, meta := range knownCommands {
		cmd := Command{CommandMeta: meta}

		switch meta.Name {
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
			cmd.Run = r.defaultRun(&meta)
		}

		r.Register(cmd)
	}

	return r
}

func (r *SlashCommandRegistry) defaultRun(meta *gateway.CommandMeta) func(string) *CommandResult {
	name := meta.Name
	return func(args string) *CommandResult {
		if r.commandSender == nil {
			return &CommandResult{Message: fmt.Sprintf("❌ 错误: 未连接到服务器")}
		}
		result, err := r.commandSender(name, args)
		if err != nil {
			return &CommandResult{Message: fmt.Sprintf("❌ 执行失败: %v", err)}
		}
		if result == "" {
			result = "✅ 操作成功"
		}

		markdown := r.formatCommandResult(name, result)
		return &CommandResult{Message: markdown}
	}
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
			nameField := item["label"]
			if nameField == "" {
				nameField = item["name"]
				if nameField == "" {
					nameField = item["value"]
				}
			}
			desc := item["desc"]
			if desc == "" {
				desc = item["description"]
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

func (r *SlashCommandRegistry) formatAgents() string {
	if len(r.agents) == 0 {
		return "*暂无可用 Agent*"
	}
	var b strings.Builder
	b.WriteString("# 可用 Agent\n\n")
	for _, a := range r.agents {
		master := ""
		if a.master {
			master = " ⭐ (Master)"
		}
		b.WriteString(fmt.Sprintf("**%s**%s — %s", a.name, master, a.description))
		if a.model != "" {
			b.WriteString(fmt.Sprintf("\n  模型: `%s`", a.model))
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func (r *SlashCommandRegistry) formatModels() string {
	if len(r.models) == 0 {
		return "*暂无可用模型*"
	}
	var b strings.Builder
	b.WriteString("# 可用模型\n\n")
	for _, m := range r.models {
		b.WriteString(fmt.Sprintf("**%s** — %s\n\n", m.Name, m.Description))
	}
	return b.String()
}

func (r *SlashCommandRegistry) formatSkills() string {
	if len(r.skills) == 0 {
		return "*暂可用 Skill*"
	}
	var b strings.Builder
	b.WriteString("# 可用 Skill\n\n")
	for _, s := range r.skills {
		b.WriteString(fmt.Sprintf("**%s** — %s\n\n", s.Name, s.Description))
	}
	return b.String()
}
