package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── schedule parent ───────────────────────────────────────────

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Scheduled task operations (requires daemon)",
	Long: `Manage recurring scheduled tasks.

All operations require the daemon to be running (mindx start).

Examples:
  mindx schedule list
  mindx schedule add --agent writer --content "Daily standup" --cron "0 0 9 * * *"
  mindx schedule delete --id a1b2c3d4`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
}

// ── response type (aligned with RPC) ──────────────────────────

type scheduleEntry struct {
	ID        string `json:"id"`
	Agent     string `json:"agent"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
	CronExpr  string `json:"cron_expr"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

// ── schedule list ─────────────────────────────────────────────

var scheduleListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all scheduled tasks",
	Example: `  mindx schedule list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ScheduleList()
		if err != nil {
			return err
		}

		var entries []scheduleEntry
		if err := json.Unmarshal(result, &entries); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if len(entries) == 0 {
			fmt.Println("No scheduled tasks.")
			return nil
		}

		table := render.NewTable([]string{"ID", "Agent", "Cron", "Enabled", "Created"}, 100)
		for _, e := range entries {
			enabled := "yes"
			if !e.Enabled {
				enabled = "no"
			}
			table.AddRow([]string{truncateStr(e.ID, 12), e.Agent, e.CronExpr, enabled, e.CreatedAt})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d scheduled task(s)\n", len(entries))
		return nil
	},
}

// ── schedule add ──────────────────────────────────────────────

var scheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new scheduled task",
	Example: `  mindx schedule add --agent writer --content "Daily standup" --cron "0 0 9 * * *"
  mindx schedule add --agent writer --content "Blog post" --cron "0 0 9 * * 1" --session-id "task-abc123" --project-dir /path/to/project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		content, _ := cmd.Flags().GetString("content")
		cron, _ := cmd.Flags().GetString("cron")
		sessionID, _ := cmd.Flags().GetString("session-id")
		projectDir, _ := cmd.Flags().GetString("project-dir")
		enabled, _ := cmd.Flags().GetBool("enabled")

		if agent == "" {
			return fmt.Errorf("--agent is required")
		}
		if content == "" {
			return fmt.Errorf("--content is required")
		}
		if cron == "" {
			return fmt.Errorf("--cron is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ScheduleAdd(rpc.ScheduleAddParams{
			Agent:      agent,
			Content:    content,
			CronExpr:   cron,
			SessionID:  sessionID,
			ProjectDir: projectDir,
			Enabled:    enabled,
		})
		if err != nil {
			return err
		}
		// Parse response to show schedule ID
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if id, ok := resp["id"].(string); ok {
				fmt.Printf("Scheduled task created: %s\n", id)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── schedule delete ───────────────────────────────────────────

var scheduleDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a scheduled task",
	Example: `  mindx schedule delete --id a1b2c3d4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.ScheduleDelete(id)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	scheduleAddCmd.Flags().String("agent", "", "Target agent name (e.g. writer)")
	scheduleAddCmd.Flags().String("content", "", "Prompt content to send to the agent")
	scheduleAddCmd.Flags().String("cron", "", "6-field cron expression")
	scheduleAddCmd.Flags().String("session-id", "", "Session UUID or graph task ID to link")
	scheduleAddCmd.Flags().String("project-dir", "", "Project working directory")
	scheduleAddCmd.Flags().Bool("enabled", true, "Enable the schedule immediately")
	scheduleDeleteCmd.Flags().String("id", "", "Schedule entry ID to delete")

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleDeleteCmd)
}
