package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── log parent ────────────────────────────────────────────────

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Daemon log operations (requires daemon)",
	Long: `Read, clear, and count daemon log entries.

All operations require the daemon to be running (mindx start).

Examples:
  mindx log read
  mindx log read --limit 20 --stream error
  mindx log read --offset 50 --limit 10
  mindx log clear --confirm
  mindx log count`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(logCmd)
}

// ── log read ──────────────────────────────────────────────────

var logReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read daemon log entries (reverse paginated)",
	Example: `  mindx log read
  mindx log read --limit 20 --stream error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")
		stream, _ := cmd.Flags().GetString("stream")
		jsonOut, _ := cmd.Flags().GetBool("json")

		if limit <= 0 {
			limit = 10
		}
		if stream == "" {
			stream = "main"
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.LogRead(offset, limit, stream)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		lines, _ := resp["lines"].([]interface{})
		if len(lines) == 0 {
			fmt.Println("No log entries.")
			return nil
		}

		table := render.NewTable([]string{"Line", "Content"}, 120)
		for i, l := range lines {
			lineStr, _ := l.(string)
			table.AddRow([]string{
				fmt.Sprintf("%d", i+1),
				lineStr,
			})
		}
		fmt.Println(table.Render())

		total, _ := resp["total"].(float64)
		returned, _ := resp["returned"].(float64)
		hasMore, _ := resp["has_more"].(bool)
		fmt.Printf("\n%d of %d lines shown", int(returned), int(total))
		if hasMore {
			fmt.Print(" (more available)")
		}
		fmt.Println()
		return nil
	},
}

// ── log clear ─────────────────────────────────────────────────

var logClearCmd = &cobra.Command{
	Use:     "clear",
	Short:   "Clear all daemon log files",
	Example: `  mindx log clear --confirm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		confirmed, _ := cmd.Flags().GetBool("confirm")
		if !confirmed {
			return fmt.Errorf("--confirm is required to clear logs")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.LogClear(true)
		if err != nil {
			return err
		}

		fmt.Println(string(result))
		return nil
	},
}

// ── log count ─────────────────────────────────────────────────

var logCountCmd = &cobra.Command{
	Use:     "count",
	Short:   "Show log entry counts per stream",
	Example: `  mindx log count`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.LogCount()
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var data map[string]interface{}
		if err := json.Unmarshal(result, &data); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"Metric", "Value"}, 60)
		for k, v := range data {
			valStr := fmt.Sprintf("%v", v)
			table.AddRow([]string{k, valStr})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	logReadCmd.Flags().Int("offset", 0, "Line offset from end (0 = most recent)")
	logReadCmd.Flags().Int("limit", 10, "Number of lines to read")
	logReadCmd.Flags().String("stream", "main", "Log stream: main or error")
	logReadCmd.Flags().Bool("json", false, "Output raw JSON")
	logCountCmd.Flags().Bool("json", false, "Output raw JSON")
	logClearCmd.Flags().Bool("confirm", false, "Confirm log clear (required)")

	logCmd.AddCommand(logReadCmd)
	logCmd.AddCommand(logClearCmd)
	logCmd.AddCommand(logCountCmd)
}
