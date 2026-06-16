package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── entity-tags parent ─────────────────────────────────────────

var entityTagsCmd = &cobra.Command{
	Use:               "entity-tags",
	Short:             "Entity tags management",
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(entityTagsCmd)
}

// ── entity-tags get ────────────────────────────────────────────

var entityTagsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get entity tag definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.EntityTagsGet()
		if err != nil {
			return err
		}

		var defs []rpc.EntityTagDef
		if err := json.Unmarshal(result, &defs); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"Name", "Title", "Desc", "Category"}, 100)
		for _, d := range defs {
			table.AddRow([]string{
				d.Name,
				d.Title,
				truncateStr(d.Desc, 50),
				d.Category,
			})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── entity-tags save ───────────────────────────────────────────

var entityTagsSaveCmd = &cobra.Command{
	Use:     "save",
	Short:   "Save entity tag definitions",
	Example: `  mindx entity-tags save --types '[{"name":"person","title":"Person","desc":"A person","category":"core"}]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		typesJSON, _ := cmd.Flags().GetString("types")
		if typesJSON == "" {
			return fmt.Errorf("--types is required")
		}

		var defs []rpc.EntityTagDef
		if err := json.Unmarshal([]byte(typesJSON), &defs); err != nil {
			return fmt.Errorf("invalid --types JSON: %w", err)
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.EntityTagsSave(defs)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── init subcommands ───────────────────────────────────────────

func init() {
	entityTagsSaveCmd.Flags().String("types", "", "JSON array of entity tag definitions (required)")
	entityTagsCmd.AddCommand(entityTagsGetCmd)
	entityTagsCmd.AddCommand(entityTagsSaveCmd)
}
