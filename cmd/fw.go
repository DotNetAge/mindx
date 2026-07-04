package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── fw parent ─────────────────────────────────────────────────

var fwCmd = &cobra.Command{
	Use:               "fw",
	Short:             "File watcher operations",
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(fwCmd)
}

// ── fw start ───────────────────────────────────────────────────

var fwStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the file watcher",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FilewatchStart()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fw stop ────────────────────────────────────────────────────

var fwStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the file watcher",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FilewatchStop()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── fw status ──────────────────────────────────────────────────

var fwStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show file watcher status",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.FilewatchStatus()
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

		table := render.NewTable([]string{"Key", "Value"}, 100)
		for k, v := range data {
			table.AddRow([]string{k, fmt.Sprintf("%v", v)})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── init subcommands ───────────────────────────────────────────

func init() {
	fwStatusCmd.Flags().Bool("json", false, "Output raw JSON")

	fwCmd.AddCommand(fwStartCmd)
	fwCmd.AddCommand(fwStopCmd)
	fwCmd.AddCommand(fwStatusCmd)
}
