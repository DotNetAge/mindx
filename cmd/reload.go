package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload [agents|skills]",
	Short: "Reload agents or skills from disk (requires daemon)",
	Long: `Hot-reload the in-memory registry for agents or skills without restarting
the daemon. This is useful after editing agent .md files or skill SKILL.md files.

The daemon also auto-reloads via fsnotify when files change, but this command
provides an explicit on-demand trigger.

Examples:
  mindx reload agents    # Reload all agents from ~/.mindx/agents/
  mindx reload skills    # Reload all skills from ~/.mindx/skills/`,
	Args: cobra.ExactArgs(1),
	RunE: runReload,
}

var reloadAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Reload all agents from disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.AgentReload()
		if err != nil {
			return err
		}

		var resp map[string]string
		if json.Unmarshal(result, &resp) == nil && resp["status"] == "ok" {
			fmt.Println("Agents reloaded successfully.")
			return nil
		}
		var pretty interface{}
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(result))
		}
		return nil
	},
}

var reloadSkillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Reload all skills from disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.SkillReload()
		if err != nil {
			return err
		}

		var resp map[string]string
		if json.Unmarshal(result, &resp) == nil && resp["status"] == "ok" {
			fmt.Println("Skills reloaded successfully.")
			return nil
		}
		var pretty interface{}
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(result))
		}
		return nil
	},
}

func init() {
	reloadCmd.AddCommand(reloadAgentsCmd)
	reloadCmd.AddCommand(reloadSkillsCmd)
	rootCmd.AddCommand(reloadCmd)
}

func runReload(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "agents":
		return reloadAgentsCmd.RunE(cmd, args)
	case "skills":
		return reloadSkillsCmd.RunE(cmd, args)
	default:
		return fmt.Errorf("unknown target %q — use 'agents' or 'skills'", args[0])
	}
}
