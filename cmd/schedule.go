package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var scheduleAddFlags struct {
	Agent      string
	SessionID  string
	ProjectDir string
	Enabled    bool
	Disabled   bool
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule list|add|del",
	Short: i18n.T("cmd.schedule.short"),
	Long:  i18n.T("cmd.schedule.long") + "\n\nExamples:\n  mindx schedule list\n  mindx schedule add --agent notes \"Daily standup summary\" \"0 9 * * 1-5\"\n  mindx schedule del abc12345",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSchedule,
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add <content> <cron_expr>",
	Short: i18n.T("cmd.schedule.add.short"),
	Long: `Add a new scheduled task with the given content and cron expression.

Content is the prompt/message to send to the agent when the schedule triggers.
Cron expression follows the standard 5-field format (min hour dom mon dow).

Examples:
  mindx schedule add --agent notes "Write daily summary" "0 18 * * 1-5"
  mindx schedule add --agent reporter "Weekly report" "0 9 * * 1"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addSchedule(cmd, args[0], args[1])
	},
}

var scheduleDelCmd = &cobra.Command{
	Use:   "del <id>",
	Short: i18n.T("cmd.schedule.del.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return delSchedule(cmd, args[0])
	},
}

func init() {
	scheduleAddCmd.Flags().StringVar(&scheduleAddFlags.Agent, "agent", "", "Agent name (required)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddFlags.SessionID, "session", "", "Session ID")
	scheduleAddCmd.Flags().StringVar(&scheduleAddFlags.ProjectDir, "project-dir", "", "Project directory")
	scheduleAddCmd.Flags().BoolVar(&scheduleAddFlags.Enabled, "enabled", true, "Enable the schedule on creation")
	scheduleAddCmd.Flags().BoolVar(&scheduleAddFlags.Disabled, "disabled", false, "Disable the schedule on creation")

	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleDelCmd)
	rootCmd.AddCommand(scheduleCmd)
}

func runSchedule(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "list":
		return listSchedules(cmd)
	case "add":
		return fmt.Errorf("usage: mindx schedule add <content> <cron_expr>")
	case "del":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx schedule del <id>")
		}
		return delSchedule(cmd, args[1])
	default:
		return fmt.Errorf("unknown subcommand %q — use list, add, or del", args[0])
	}
}

func schedulesDir() string {
	prefs := core.DefaultUserPrefsDir()
	return filepath.Join(prefs, "data", "schedules")
}

func openScheduleStore() (*scheduler.FileSchedulerStore, error) {
	dir := schedulesDir()
	return scheduler.NewFileSchedulerStore(dir)
}

func listSchedules(cmd *cobra.Command) error {
	store, err := openScheduleStore()
	if err != nil {
		return fmt.Errorf("cannot open schedule store: %w", err)
	}

	entries, err := store.List(context.Background())
	if err != nil {
		return fmt.Errorf("cannot list schedules: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No scheduled tasks.")
		return nil
	}

	// Column widths
	fmt.Printf("%-10s %-12s %-20s %-12s %s\n", "ID", "Agent", "Cron", "Status", "Content")
	fmt.Println(strings.Repeat("─", 100))
	for _, e := range entries {
		status := "enabled"
		if !e.Enabled {
			status = "disabled"
		}
		content := e.Content
		if len(content) > 45 {
			content = content[:42] + "..."
		}
		agent := e.Agent
		if agent == "" {
			agent = "(default)"
		}
		fmt.Printf("%-10s %-12s %-20s %-12s %s\n", e.ID, agent, e.CronExpr, status, content)
	}

	return nil
}

func addSchedule(cmd *cobra.Command, content, cronExpr string) error {
	if scheduleAddFlags.Agent == "" {
		return fmt.Errorf("--agent is required")
	}

	enabled := scheduleAddFlags.Enabled
	if scheduleAddFlags.Disabled {
		enabled = false
	}

	entry := &scheduler.ScheduleEntry{
		ID:         uuid.NewString()[:8],
		Agent:      scheduleAddFlags.Agent,
		SessionID:  scheduleAddFlags.SessionID,
		ProjectDir: scheduleAddFlags.ProjectDir,
		Content:    content,
		CronExpr:   cronExpr,
		Enabled:    enabled,
		CreatedAt:  time.Now(),
	}

	store, err := openScheduleStore()
	if err != nil {
		return fmt.Errorf("cannot open schedule store: %w", err)
	}

	if err := store.Save(context.Background(), entry); err != nil {
		return fmt.Errorf("cannot save schedule: %w", err)
	}

	fmt.Printf("Schedule %q created (ID: %s)\n", entry.Agent, entry.ID)
	fmt.Println("Restart the daemon for the schedule to take effect: mindx restart")

	return nil
}

func delSchedule(cmd *cobra.Command, id string) error {
	store, err := openScheduleStore()
	if err != nil {
		return fmt.Errorf("cannot open schedule store: %w", err)
	}

	if err := store.Delete(context.Background(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("schedule %q not found", id)
		}
		return fmt.Errorf("cannot delete schedule: %w", err)
	}

	fmt.Printf("Schedule %q deleted\n", id)
	fmt.Println("Restart the daemon for the change to take effect: mindx restart")

	return nil
}
