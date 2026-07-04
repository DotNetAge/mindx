package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── rule parent ───────────────────────────────────────────────

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Behavior rule management (requires daemon)",
	Long: `Create, list, get, update, and delete behavior rules.

Behavior rules define what an AI agent should or must not do.
They are injected into the system prompt as MUST-follow norms.

All operations require the daemon to be running (mindx start).

Examples:
  mindx rule list
  mindx rule get --id "no-delete-prod"
  mindx rule create --id "my-rule" --intro "Always ask before destructive actions"
  mindx rule update --id "my-rule" --priority 50 --enabled false
  mindx rule delete --id "my-rule"`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(ruleCmd)
}

// ── response types (aligned with RPC) ─────────────────────────

type ruleEntry struct {
	ID       string `json:"id"`
	Intro    string `json:"intro"`
	Scope    string `json:"scope"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

// ── rule list ─────────────────────────────────────────────────

var ruleListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all behavior rules",
	Example: `  mindx rule list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.RuleList()
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var resp struct {
			Count int         `json:"count"`
			Rules []ruleEntry `json:"rules"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if resp.Count == 0 {
			fmt.Println("No rules configured.")
			return nil
		}

		table := render.NewTable([]string{"ID", "Intro", "Scope", "Priority", "Enabled"}, 100)
		for _, r := range resp.Rules {
			enabled := "yes"
			if !r.Enabled {
				enabled = "no"
			}
			table.AddRow([]string{
				r.ID,
				r.Intro,
				r.Scope,
				fmt.Sprintf("%d", r.Priority),
				enabled,
			})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n%d rule(s)\n", resp.Count)
		return nil
	},
}

// ── rule get ──────────────────────────────────────────────────

var ruleGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Show details for a behavior rule",
	Example: `  mindx rule get --id "no-delete-prod"`,
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

		result, err := cl.RuleGet(id)
		if err != nil {
			return err
		}

		var resp struct {
			Found bool       `json:"found"`
			Rule  *ruleEntry `json:"rule,omitempty"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if !resp.Found || resp.Rule == nil {
			return fmt.Errorf("rule %q not found", id)
		}

		r := resp.Rule
		enabled := "yes"
		if !r.Enabled {
			enabled = "no"
		}
		fmt.Printf("ID:       %s\n", r.ID)
		fmt.Printf("Intro:    %s\n", r.Intro)
		fmt.Printf("Scope:    %s\n", r.Scope)
		fmt.Printf("Priority: %d\n", r.Priority)
		fmt.Printf("Enabled:  %s\n", enabled)
		return nil
	},
}

// ── rule create ───────────────────────────────────────────────

var ruleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new behavior rule",
	Example: `  mindx rule create --id "my-rule" --intro "Always ask before destructive actions"
  mindx rule create --id "my-rule" --intro "Be concise" --scope global --priority 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		intro, _ := cmd.Flags().GetString("intro")
		scope, _ := cmd.Flags().GetString("scope")
		priority, _ := cmd.Flags().GetInt("priority")
		enabled, _ := cmd.Flags().GetBool("enabled")

		if id == "" {
			return fmt.Errorf("--id is required")
		}
		if intro == "" {
			return fmt.Errorf("--intro is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.RuleCreate(rpc.RuleCreateParams{
			ID:       id,
			Intro:    intro,
			Scope:    scope,
			Priority: priority,
			Enabled:  enabled,
		})
		if err != nil {
			return err
		}

		// Parse response to show rule ID
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if rid, ok := resp["id"].(string); ok {
				fmt.Printf("Rule created: %s\n", rid)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── rule update ───────────────────────────────────────────────

var ruleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing behavior rule",
	Example: `  mindx rule update --id "my-rule" --intro "Updated description"
  mindx rule update --id "my-rule" --priority 50 --enabled false`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		params := rpc.RuleUpdateParams{ID: id}

		if cmd.Flags().Changed("intro") {
			v, _ := cmd.Flags().GetString("intro")
			params.Intro = &v
		}
		if cmd.Flags().Changed("scope") {
			v, _ := cmd.Flags().GetString("scope")
			params.Scope = &v
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetInt("priority")
			params.Priority = &v
		}
		if cmd.Flags().Changed("enabled") {
			v, _ := cmd.Flags().GetBool("enabled")
			params.Enabled = &v
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.RuleUpdate(params)
		if err != nil {
			return err
		}

		// Parse response to show rule ID
		var resp map[string]interface{}
		if json.Unmarshal(result, &resp) == nil {
			if rid, ok := resp["id"].(string); ok {
				fmt.Printf("Rule updated: %s\n", rid)
				return nil
			}
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── rule delete ───────────────────────────────────────────────

var ruleDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a behavior rule by ID",
	Example: `  mindx rule delete --id "my-rule"`,
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

		result, err := cl.RuleDelete(id)
		if err != nil {
			return err
		}

		fmt.Println(string(result))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	ruleListCmd.Flags().Bool("json", false, "Output raw JSON")
	ruleGetCmd.Flags().String("id", "", "Rule ID (required)")
	ruleCreateCmd.Flags().String("id", "", "Unique rule identifier (required)")
	ruleCreateCmd.Flags().String("intro", "", "Behavioral description (required)")
	ruleCreateCmd.Flags().String("scope", "global", "Scope: global, local, or conversation")
	ruleCreateCmd.Flags().Int("priority", 0, "Priority (higher = more important)")
	ruleCreateCmd.Flags().Bool("enabled", true, "Enable the rule immediately")
	ruleUpdateCmd.Flags().String("id", "", "Rule ID to update (required)")
	ruleUpdateCmd.Flags().String("intro", "", "New behavioral description")
	ruleUpdateCmd.Flags().String("scope", "", "New scope: global, local, or conversation")
	ruleUpdateCmd.Flags().Int("priority", 0, "New priority")
	ruleUpdateCmd.Flags().Bool("enabled", true, "New enabled state")
	ruleDeleteCmd.Flags().String("id", "", "Rule ID to delete (required)")

	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(ruleGetCmd)
	ruleCmd.AddCommand(ruleCreateCmd)
	ruleCmd.AddCommand(ruleUpdateCmd)
	ruleCmd.AddCommand(ruleDeleteCmd)
}
