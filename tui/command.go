package tui

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const maxSuggestionRows = 5

// Command represents a local slash command in the TUI.
type Command struct {
	Name        string
	Description string
	Hidden      bool // hidden from suggestion list but still accessible
	Run         func(args string) *CommandResult
	SubCommands []Command // nested subcommands for hierarchical browsing
}

// CommandResult is the result of executing a local command.
type CommandResult struct {
	Message   string
	ClearChat bool
}

// CommandRegistry holds registered slash commands.
type CommandRegistry struct {
	commands []Command
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{}
}

func (r *CommandRegistry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
}

func (r *CommandRegistry) All() []Command {
	return r.commands
}

// Visible returns all non-hidden commands.
func (r *CommandRegistry) Visible() []Command {
	var out []Command
	for _, cmd := range r.commands {
		if !cmd.Hidden {
			out = append(out, cmd)
		}
	}
	return out
}

// Filter returns commands whose name starts with prefix (case-insensitive), excluding hidden ones.
func (r *CommandRegistry) Filter(prefix string) []Command {
	lower := strings.ToLower(prefix)
	var out []Command
	for _, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		if strings.HasPrefix(strings.ToLower(cmd.Name), lower) {
			out = append(out, cmd)
		}
	}
	return out
}

// Find returns the command matching name exactly (case-insensitive), including hidden.
func (r *CommandRegistry) Find(name string) *Command {
	lower := strings.ToLower(name)
	for i := range r.commands {
		if strings.ToLower(r.commands[i].Name) == lower {
			return &r.commands[i]
		}
	}
	return nil
}

// lsFiles returns a list of FileEntry for the given path (supports ~/ expansion).
func lsFiles(path string) ([]FileEntry, error) {
	path = expandHome(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		files = append(files, FileEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
			Path:  filepath.Join(path, e.Name()),
		})
	}
	return files, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return path
		}
		return filepath.Join(usr.HomeDir, path[2:])
	}
	return path
}

// FileEntry represents a file or directory in the ls output.
type FileEntry struct {
	Name  string
	IsDir bool
	Size  int64
	Path  string
}

// formatFileEntry formats a single file entry as a suggestion line.
func formatFileEntry(e FileEntry) string {
	if e.IsDir {
		return fmt.Sprintf("  %s/", e.Name)
	}
	return fmt.Sprintf("  %s", e.Name)
}

// BuiltinCommands creates the default set of built-in commands.
func BuiltinCommands() *CommandRegistry {
	r := NewCommandRegistry()

	r.Register(Command{
		Name:        "help",
		Description: "显示所有可用命令",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: r.helpText()}
		},
	})

	r.Register(Command{
		Name:        "about",
		Description: "关于 MindX",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "MindX Agent Chat v0.1 — AI 智能体交互终端"}
		},
	})

	r.Register(Command{
		Name:        "init",
		Description: "初始化会话",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "/init"}
		},
	})

	r.Register(Command{
		Name:        "clear",
		Description: "清理当前所有上下文",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "已清理当前上下文", ClearChat: true}
		},
	})

	r.Register(Command{
		Name:        "compress",
		Description: "压缩上下文",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "/compress"}
		},
	})

	r.Register(Command{
		Name:        "agents",
		Description: "显示智能体列表",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "/agents"}
		},
		SubCommands: []Command{
			{
				Name:        "list",
				Description: "列出所有智能体",
				Run:         func(args string) *CommandResult { return &CommandResult{Message: "/agents list"} },
			},
			{
				Name:        "switch",
				Description: "切换当前智能体",
				Run:         func(args string) *CommandResult { return &CommandResult{Message: "/agents switch"} },
			},
			{
				Name:        "add",
				Description: "添加自定义智能体",
				Run:         func(args string) *CommandResult { return &CommandResult{Message: "/agents add"} },
			},
		},
	})

	// Hidden: ls command, triggered by @ prefix
	r.Register(Command{
		Name:        "ls",
		Description: "浏览文件",
		Hidden:      true,
		Run: func(args string) *CommandResult {
			dir := "~/"
			if args != "" {
				dir = args
			}
			entries, err := lsFiles(dir)
			if err != nil {
				return &CommandResult{Message: fmt.Sprintf("无法读取目录: %v", err)}
			}
			if len(entries) == 0 {
				return &CommandResult{Message: "(空目录)"}
			}
			var lines []string
			for _, e := range entries {
				lines = append(lines, formatFileEntry(e))
			}
			return &CommandResult{Message: strings.Join(lines, "\n")}
		},
	})

	return r
}

func (r *CommandRegistry) helpText() string {
	var b strings.Builder
	b.WriteString("可用命令:\n")
	for _, cmd := range r.Visible() {
		b.WriteString(fmt.Sprintf("  /%-14s %s\n", cmd.Name, cmd.Description))
		// Show subcommands indented
		for _, sub := range cmd.SubCommands {
			b.WriteString(fmt.Sprintf("    /%-12s %s\n", sub.Name, sub.Description))
		}
	}
	return b.String()
}
