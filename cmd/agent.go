package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
)

// ── agent parent ───────────────────────────────────────────────

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
	Long: `List, add, or remove AI agents.

Each agent is stored as a Markdown file with YAML frontmatter
in the agents directory (~/.mindx/agents/).`,
}

// ── agent list ─────────────────────────────────────────────────

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured agents",
	RunE: func(cmd *cobra.Command, args []string) error {
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
  mindx agent add my-agent --role "Assistant" --model gpt-4 --description "My custom agent"
  mindx agent add helper --role "Helper" --model qwen3.6-plus --skills "file-organizer,pdf"`,
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
			Model:       agentAddFlags.model,
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

func init() {
	agentAddCmd.Flags().StringVar(&agentAddFlags.role, "role", "", "Agent role/title")
	agentAddCmd.Flags().StringVar(&agentAddFlags.description, "description", "", "Agent description")
	agentAddCmd.Flags().StringVar(&agentAddFlags.model, "model", "", "Default model name")
	agentAddCmd.Flags().StringVar(&agentAddFlags.skills, "skills", "", "Comma-separated skill names")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentRmCmd)
	agentCmd.AddCommand(agentAddCmd)
	rootCmd.AddCommand(agentCmd)
}

func agentDir() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "agents")
}
