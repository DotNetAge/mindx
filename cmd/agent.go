package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── agent parent ───────────────────────────────────────────────

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
	Long: `List, inspect, add, remove, hire or score AI agents.

By default reads agents from local config files (hired only, use --all for all).
Use --json to query the daemon and output JSON (for LLM consumption).

Examples:
  mindx agent list
  mindx agent list --all
  mindx agent list --json
  mindx agent get writer
  mindx agent add writer --role "Writer"
  mindx agent hire writer
  mindx agent fire writer
  mindx agent rm writer`,
}

// ── agent list ─────────────────────────────────────────────────

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hired agents (use --all to include all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")
		showAll, _ := cmd.Flags().GetBool("all")

		if useJSON {
			cl, err := rpc.Dial(daemonAddr)
			if err != nil {
				return fmt.Errorf("cannot connect to daemon: %w", err)
			}
			defer func() { _ = cl.Close() }()

			result, err := cl.AgentList()
			if err != nil {
				return err
			}

			// daemon agent.list 恒为全量返回；hired 过滤在客户端展示层完成
			var list []map[string]any
			if err := json.Unmarshal(result, &list); err != nil {
				// 兜底：无法解析为列表时原样输出
				fmt.Println(string(result))
				return nil
			}
			filtered := make([]map[string]any, 0, len(list))
			for _, m := range list {
				meta, _ := m["meta"].(map[string]any)
				if !showAll && !core.AgentMetaIsHired(meta) {
					continue
				}
				filtered = append(filtered, m)
			}
			formatted, _ := json.MarshalIndent(filtered, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}

		dir := agentDir()
		registry, err := goharnessconfig.LoadAgentsFrom(dir)
		if err != nil {
			return fmt.Errorf("cannot load agents: %w", err)
		}

		agents := registry.List()
		if !showAll {
			// 默认只显示已雇佣的 Agent（雇佣视图）
			hired := agents[:0]
			for _, a := range agents {
				if core.AgentIsHired(a) {
					hired = append(hired, a)
				}
			}
			agents = hired
		}
		if len(agents) == 0 {
			if showAll {
				fmt.Printf("No agents found in %s.\n", dir)
			} else {
				fmt.Printf("暂无已雇佣的 Agent（目录：%s）。\n", dir)
				fmt.Println("提示：使用 `mindx agent hire <name>` 雇佣，或 `mindx agent list --all` 查看全部。")
			}
			return nil
		}

		table := render.NewTable([]string{"Name", "Role", "Description", "Skills"}, 100)
		for _, a := range agents {
			role := a.Role
			if role == "" {
				role = "—"
			}
			desc := a.Description
			if desc == "" {
				desc = "—"
			}
			skills := ""
			if len(a.Skills) > 0 {
				skills = strings.Join(a.Skills, ", ")
			}
			table.AddRow([]string{a.Name, role, desc, skills})
		}
		fmt.Println(table.Render())
		if showAll {
			fmt.Printf("\n%d agent(s) configured.\n", len(agents))
		} else {
			fmt.Printf("\n%d hired agent(s). Use --all to see all.\n", len(agents))
		}
		return nil
	},
}

// ── agent get ──────────────────────────────────────────────────

var agentGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show agent details via daemon (JSON output)",
	Long: `Query the daemon for a single agent's full configuration.
Outputs rich JSON suitable for LLM consumption.

Example:
  mindx agent get writer
  mindx agent get project-manager`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.AgentGet(name)
		if err != nil {
			return err
		}

		var pretty interface{}
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── agent score ────────────────────────────────────────────────

var agentScoreFlags struct {
	agentName string
	task      string
	score     int
	notes     string
}

var agentScoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Score an agent's task performance (via daemon)",
	Long: `Record a performance score for an agent on a specific task.

Score 1-10: 9-10 exceptional, 7-8 good, 5-6 adequate, 3-4 gaps, 1-2 unusable.

Example:
  mindx agent score --agent-name writer --task "Write blog post" --score 8
  mindx agent score --agent-name researcher --task "Research topic" --score 6 --notes "Missed key sources"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentScoreFlags.agentName == "" {
			return fmt.Errorf("--agent-name is required")
		}
		if agentScoreFlags.task == "" {
			return fmt.Errorf("--task is required")
		}
		if agentScoreFlags.score < 1 || agentScoreFlags.score > 10 {
			return fmt.Errorf("--score must be between 1 and 10")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.AgentScore(rpc.AgentScoreParams{
			AgentName: agentScoreFlags.agentName,
			Task:      agentScoreFlags.task,
			Score:     agentScoreFlags.score,
			Notes:     agentScoreFlags.notes,
		})
		if err != nil {
			return err
		}

		var pretty interface{}
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── agent update ────────────────────────────────────────────────

var agentUpdateFlags struct {
	agentName    string
	role         string
	description  string
	introduction string
	model        string
	skills       string
	excludeTools string
}

var agentUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing agent's configuration (via daemon)",
	Long: `Partially update an agent's configuration. Only specified fields are changed;
unspecified fields preserve their current values.

Examples:
  mindx agent update --agent-name writer --role "Senior Writer"
  mindx agent update --agent-name coder --model "claude-sonnet-4" --skills "find-experts,code-review"
  mindx agent update --agent-name helper --exclude-tools "bash,sub-agent"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentUpdateFlags.agentName == "" {
			return fmt.Errorf("--agent-name is required")
		}

		params := rpc.AgentUpdateParams{
			Name: agentUpdateFlags.agentName,
		}
		if agentUpdateFlags.role != "" {
			params.Role = agentUpdateFlags.role
		}
		if agentUpdateFlags.description != "" {
			params.Description = agentUpdateFlags.description
		}
		if agentUpdateFlags.introduction != "" {
			params.Introduction = agentUpdateFlags.introduction
		}
		if agentUpdateFlags.model != "" {
			params.Model = agentUpdateFlags.model
		}
		if agentUpdateFlags.skills != "" {
			params.Skills = splitComma(agentUpdateFlags.skills)
		}
		if agentUpdateFlags.excludeTools != "" {
			params.ExcludeTools = splitComma(agentUpdateFlags.excludeTools)
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.AgentUpdate(params)
		if err != nil {
			return err
		}

		var pretty interface{}
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── agent hire / agent fire ────────────────────────────────────

// runAgentHire 是 hire/fire 的公共实现：走 daemon 专用 RPC，
// 文本级修改 Agent 文件的 meta.hired 并同步 daemon 内存注册表。
func runAgentHire(name string, hired bool) error {
	cl, err := rpc.Dial(daemonAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to daemon: %w", err)
	}
	defer func() { _ = cl.Close() }()

	var result json.RawMessage
	if hired {
		result, err = cl.AgentHire(name)
	} else {
		result, err = cl.AgentFire(name)
	}
	if err != nil {
		return err
	}

	// RPC 成功即生效；返回体中的 message 由 handler 提供中文说明
	var pretty map[string]any
	if err := json.Unmarshal(result, &pretty); err == nil {
		if msg, ok := pretty["message"].(string); ok && msg != "" {
			fmt.Println(msg)
			return nil
		}
	}
	if hired {
		fmt.Printf("Agent %q 已雇佣。\n", name)
	} else {
		fmt.Printf("Agent %q 已解雇。\n", name)
	}
	return nil
}

var agentHireCmd = &cobra.Command{
	Use:   "hire <name>",
	Short: "Hire an agent (enable session usage)",
	Long: `Mark an agent as hired (meta.hired=true) so it becomes available
for sessions, /agent switching and scheduled tasks.

Example:
  mindx agent hire writer`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgentHire(args[0], true)
	},
}

var agentFireCmd = &cobra.Command{
	Use:   "fire <name>",
	Short: "Fire an agent (disable session usage)",
	Long: `Mark an agent as fired (meta.hired=false) so it is no longer
available for sessions, /agent switching or scheduled tasks.
The agent remains visible for browsing via ` + "`mindx agent list --all`" + `.

Example:
  mindx agent fire writer`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgentHire(args[0], false)
	},
}

// ── agent rm ───────────────────────────────────────────────────

var agentRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dir := agentDir()

		registry, err := goharnessconfig.LoadAgentsFrom(dir, goharnessconfig.WithRegistryLogger(logging.DefaultLogger()))
		if err != nil {
			return fmt.Errorf("cannot load agents: %w", err)
		}

		if err := registry.Remove(name); err != nil {
			return err
		}

		fmt.Printf("Agent %q removed.\n", name)
		return nil
	},
}

// ── agent add ──────────────────────────────────────────────────

var agentAddFlags struct {
	role        string
	description string
	model       string
	skills      string
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new agent",
	Long: `Create a new agent with the given name and configuration.

The agent is stored as a Markdown file with YAML frontmatter
in the agents directory (~/.mindx/agents/{name}.md).

Examples:
  mindx agent add my-agent --role "Assistant" --description "My custom agent"
  mindx agent add helper --role "Helper" --skills "file-organizer,pdf"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dir := agentDir()

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create agents directory: %w", err)
		}

		registry, err := goharnessconfig.LoadAgentsFrom(dir, goharnessconfig.WithRegistryLogger(logging.DefaultLogger()))
		if err != nil {
			return fmt.Errorf("cannot load agents: %w", err)
		}

		existing := registry.Get(name)

		agent := &goharnessconfig.AgentConfig{
			Name:        name,
			Role:        agentAddFlags.role,
			Description: agentAddFlags.description,
		}
		if agentAddFlags.skills != "" {
			agent.Skills = strings.Split(agentAddFlags.skills, ",")
			for i := range agent.Skills {
				agent.Skills[i] = strings.TrimSpace(agent.Skills[i])
			}
		}

		if err := registry.SaveTo(agent); err != nil {
			return fmt.Errorf("cannot save agent: %w", err)
		}

		if existing != nil {
			fmt.Printf("Agent %q updated.\n", name)
		} else {
			fmt.Printf("Agent %q created (%s).\n", name, filepath.Join(dir, strings.ToLower(name)+".md"))
			// 新建 Agent 默认未雇佣（meta.hired 缺省 false），提示雇佣入口
			fmt.Println("提示：新智能体默认未雇佣，执行 `mindx agent hire " + name + "` 后即可用于会话。")
		}
		return nil
	},
}

// ── init ───────────────────────────────────────────────────────

func init() {
	agentListCmd.Flags().Bool("json", false, "Output JSON via daemon (requires mindx start)")
	agentListCmd.Flags().Bool("all", false, "Include all agents (default shows hired only)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.agentName, "agent-name", "", "Agent name (required)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.task, "task", "", "Task description (required)")
	agentScoreCmd.Flags().IntVar(&agentScoreFlags.score, "score", 0, "Score 1-10 (required)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.notes, "notes", "", "Optional evaluation notes")
	agentAddCmd.Flags().StringVar(&agentAddFlags.role, "role", "", "Agent role/title")
	agentAddCmd.Flags().StringVar(&agentAddFlags.description, "description", "", "Agent description")
	agentAddCmd.Flags().StringVar(&agentAddFlags.skills, "skills", "", "Comma-separated skill names")

	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.agentName, "agent-name", "", "Agent name (required)")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.role, "role", "", "New role/title")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.description, "description", "", "New description")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.introduction, "introduction", "", "New introduction/prompt")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.model, "model", "", "New model identifier")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.skills, "skills", "", "New comma-separated skill names (replaces current)")
	agentUpdateCmd.Flags().StringVar(&agentUpdateFlags.excludeTools, "exclude-tools", "", "New comma-separated tool names to exclude")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentGetCmd)
	agentCmd.AddCommand(agentScoreCmd)
	agentCmd.AddCommand(agentUpdateCmd)
	agentCmd.AddCommand(agentHireCmd)
	agentCmd.AddCommand(agentFireCmd)
	agentCmd.AddCommand(agentRmCmd)
	agentCmd.AddCommand(agentAddCmd)
	rootCmd.AddCommand(agentCmd)
}

func agentDir() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "agents")
}
