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
	Long: `List, inspect, add, remove or score AI agents.

By default reads agents from local config files.
Use --json to query the daemon and output JSON (for LLM consumption).

Examples:
  mindx agent list
  mindx agent list --json
  mindx agent get writer
  mindx agent add writer --role "Writer"
  mindx agent rm writer`,
}

// ── agent list ─────────────────────────────────────────────────

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")

		if useJSON {
			cl, err := rpc.Dial(daemonAddr)
			if err != nil {
				return fmt.Errorf("cannot connect to daemon: %w", err)
			}
			defer cl.Close()

			result, err := cl.AgentList()
			if err != nil {
				return err
			}
			// Pretty-print the JSON for LLM readability
			var pretty interface{}
			if err := json.Unmarshal(result, &pretty); err == nil {
				formatted, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(formatted))
				return nil
			}
			fmt.Println(string(result))
			return nil
		}

		dir := agentDir()
		registry, err := goharnessconfig.LoadAgentsFrom(dir)
		if err != nil {
			return fmt.Errorf("cannot load agents: %w", err)
		}

		agents := registry.List()
		if len(agents) == 0 {
			fmt.Printf("No agents found in %s.\n", dir)
			return nil
		}

		table := render.NewTable([]string{"Name", "Role", "Model", "Skills"}, 100)
		for _, a := range agents {
			role := a.Role
			if role == "" {
				role = "—"
			}
			model := a.Model
			if model == "" {
				model = "—"
			}
			skills := ""
			if len(a.Skills) > 0 {
				skills = fmt.Sprintf("%d", len(a.Skills))
			}
			table.AddRow([]string{a.Name, role, model, skills})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d agent(s) configured.\n", len(agents))
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
		defer cl.Close()

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
		defer cl.Close()

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
		}
		return nil
	},
}

// ── init ───────────────────────────────────────────────────────

func init() {
	agentListCmd.Flags().Bool("json", false, "Output JSON via daemon (requires mindx start)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.agentName, "agent-name", "", "Agent name (required)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.task, "task", "", "Task description (required)")
	agentScoreCmd.Flags().IntVar(&agentScoreFlags.score, "score", 0, "Score 1-10 (required)")
	agentScoreCmd.Flags().StringVar(&agentScoreFlags.notes, "notes", "", "Optional evaluation notes")
	agentAddCmd.Flags().StringVar(&agentAddFlags.role, "role", "", "Agent role/title")
	agentAddCmd.Flags().StringVar(&agentAddFlags.description, "description", "", "Agent description")
	agentAddCmd.Flags().StringVar(&agentAddFlags.skills, "skills", "", "Comma-separated skill names")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentGetCmd)
	agentCmd.AddCommand(agentScoreCmd)
	agentCmd.AddCommand(agentRmCmd)
	agentCmd.AddCommand(agentAddCmd)
	rootCmd.AddCommand(agentCmd)
}

func agentDir() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "agents")
}
