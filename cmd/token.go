package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── token parent ──────────────────────────────────────────────

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Token usage statistics (requires daemon)",
	Long: `Query token usage statistics from the daemon.

All operations require the daemon to be running (mindx start).

Examples:
  mindx token overview
  mindx token monthly --year 2026 --month 6
  mindx token by-model --model gpt-4o --year 2026 --month 6
  mindx token total
  mindx token session --session-id "abc123"`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}

// ── token overview ────────────────────────────────────────────

var tokenOverviewCmd = &cobra.Command{
	Use:     "overview",
	Short:   "Show token usage overview (current vs previous month)",
	Example: `  mindx token overview`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()

		result, err := cl.TokenUsageOverview()
		if err != nil {
			return err
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

// ── token monthly ─────────────────────────────────────────────

var tokenMonthlyCmd = &cobra.Command{
	Use:     "monthly",
	Short:   "Show token usage for a specific month",
	Example: `  mindx token monthly --year 2026 --month 6`,
	RunE: func(cmd *cobra.Command, args []string) error {
		year, _ := cmd.Flags().GetInt("year")
		month, _ := cmd.Flags().GetInt("month")

		now := time.Now()
		if year == 0 {
			year = now.Year()
		}
		if month == 0 {
			month = int(now.Month())
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()

		result, err := cl.TokenUsageMonthly(year, month)
		if err != nil {
			return err
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

// ── token by-model ────────────────────────────────────────────

var tokenByModelCmd = &cobra.Command{
	Use:   "by-model",
	Short: "Show token usage filtered by model",
	Example: `  mindx token by-model --model gpt-4o
  mindx token by-model --model gpt-4o --year 2026 --month 6`,
	RunE: func(cmd *cobra.Command, args []string) error {
		model, _ := cmd.Flags().GetString("model")
		if model == "" {
			return fmt.Errorf("--model is required")
		}

		year, _ := cmd.Flags().GetInt("year")
		month, _ := cmd.Flags().GetInt("month")

		now := time.Now()
		if year == 0 {
			year = now.Year()
		}
		if month == 0 {
			month = int(now.Month())
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()

		result, err := cl.TokenUsageByModel(model, year, month)
		if err != nil {
			return err
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

// ── token total ───────────────────────────────────────────────

var tokenTotalCmd = &cobra.Command{
	Use:     "total",
	Short:   "Show aggregated total token usage",
	Example: `  mindx token total`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()

		result, err := cl.TokenUsageTotal()
		if err != nil {
			return err
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

// ── token session ─────────────────────────────────────────────

var tokenSessionCmd = &cobra.Command{
	Use:     "session",
	Short:   "Show token usage for a specific session",
	Example: `  mindx token session --session-id "abc123"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		if sessionID == "" {
			return fmt.Errorf("--session-id is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()

		result, err := cl.TokenUsageSession(sessionID)
		if err != nil {
			return err
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
	tokenMonthlyCmd.Flags().Int("year", 0, "Year (default: current)")
	tokenMonthlyCmd.Flags().Int("month", 0, "Month 1-12 (default: current)")
	tokenByModelCmd.Flags().String("model", "", "Model name (required)")
	tokenByModelCmd.Flags().Int("year", 0, "Year (default: current)")
	tokenByModelCmd.Flags().Int("month", 0, "Month 1-12 (default: current)")
	tokenSessionCmd.Flags().String("session-id", "", "Session ID (required)")

	tokenCmd.AddCommand(tokenOverviewCmd)
	tokenCmd.AddCommand(tokenMonthlyCmd)
	tokenCmd.AddCommand(tokenByModelCmd)
	tokenCmd.AddCommand(tokenTotalCmd)
	tokenCmd.AddCommand(tokenSessionCmd)
}
