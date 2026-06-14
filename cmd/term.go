package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── term parent ────────────────────────────────────────────────

var termCmd = &cobra.Command{
	Use:               "term",
	Short:             "Terminal management",
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(termCmd)
}

// ── term start ─────────────────────────────────────────────────

var termStartCmd = &cobra.Command{
	Use:     "start",
	Short:   "Start a new terminal session",
	Example: `  mindx term start --cwd /workspace/project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.TerminalStart(cwd)
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if sid, ok := resp["session_id"].(string); ok {
				fmt.Printf("Terminal session started: %s\n", sid)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── term input ─────────────────────────────────────────────────

var termInputCmd = &cobra.Command{
	Use:     "input",
	Short:   "Send input to a terminal session",
	Example: `  mindx term input --session-id "01ABCDEF..." --data "ls -la"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		data, _ := cmd.Flags().GetString("data")
		if sessionID == "" {
			return fmt.Errorf("--session-id is required")
		}
		if data == "" {
			return fmt.Errorf("--data is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.TerminalInput(sessionID, data)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── term resize ────────────────────────────────────────────────

var termResizeCmd = &cobra.Command{
	Use:     "resize",
	Short:   "Resize a terminal session",
	Example: `  mindx term resize --session-id "01ABCDEF..." --rows 24 --cols 80`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		if sessionID == "" {
			return fmt.Errorf("--session-id is required")
		}
		rows, _ := strconv.ParseUint(cmd.Flags().Lookup("rows").Value.String(), 10, 16)
		cols, _ := strconv.ParseUint(cmd.Flags().Lookup("cols").Value.String(), 10, 16)

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.TerminalResize(sessionID, uint16(rows), uint16(cols), 0, 0)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── term kill ──────────────────────────────────────────────────

var termKillCmd = &cobra.Command{
	Use:     "kill",
	Short:   "Kill a terminal session",
	Example: `  mindx term kill --session-id "01ABCDEF..."`,
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
		result, err := cl.TerminalKill(sessionID)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── term list ──────────────────────────────────────────────────

var termListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all terminal sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.TerminalList()
		if err != nil {
			return err
		}

		var sessions []map[string]interface{}
		if err := json.Unmarshal(result, &sessions); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"ID", "Cwd"}, 100)
		for _, s := range sessions {
			id, _ := s["session_id"].(string)
			cwd, _ := s["cwd"].(string)
			table.AddRow([]string{truncateStr(id, 12), cwd})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── init subcommands ───────────────────────────────────────────

func init() {
	termStartCmd.Flags().String("cwd", "", "Working directory for the terminal")
	termInputCmd.Flags().String("session-id", "", "Terminal session ID (required)")
	termInputCmd.Flags().String("data", "", "Input data to send (required)")
	termResizeCmd.Flags().String("session-id", "", "Terminal session ID (required)")
	termResizeCmd.Flags().Uint16("rows", 24, "Number of rows")
	termResizeCmd.Flags().Uint16("cols", 80, "Number of columns")
	termKillCmd.Flags().String("session-id", "", "Terminal session ID (required)")
	termListCmd.Flags().String("session-id", "", "Terminal session ID")

	termCmd.AddCommand(termStartCmd)
	termCmd.AddCommand(termInputCmd)
	termCmd.AddCommand(termResizeCmd)
	termCmd.AddCommand(termKillCmd)
	termCmd.AddCommand(termListCmd)
}
