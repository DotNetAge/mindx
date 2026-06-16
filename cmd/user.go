package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── user parent ────────────────────────────────────────────────

var userCmd = &cobra.Command{
	Use:               "user",
	Short:             "User configuration",
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(userCmd)
}

// ── user config ────────────────────────────────────────────────

var userConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show user configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.UserConfig()
		if err != nil {
			return err
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
	userCmd.AddCommand(userConfigCmd)
}
