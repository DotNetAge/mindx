package cmd

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/goharness/rule"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/spf13/cobra"
)

var ruleCmd = &cobra.Command{
	Use:   "rule list|get <id>",
	Short: i18n.T("cmd.rule.short"),
	Long:  i18n.T("cmd.rule.long") + "\n\nExamples:\n  mindx rule list\n  mindx rule get fs.write",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRule,
}

var ruleGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: i18n.T("cmd.rule.get.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return showRule(cmd, args[0])
	},
}

func init() {
	ruleCmd.AddCommand(ruleGetCmd)
	rootCmd.AddCommand(ruleCmd)
}

func runRule(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "list":
		return listRules(cmd)
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx rule get <id>")
		}
		return showRule(cmd, args[1])
	default:
		return fmt.Errorf("unknown subcommand %q — use list or get", args[0])
	}
}

func loadPermissionRules() (*rule.PermissionRules, error) {
	workspaceDir := core.DefaultUserPrefsDir()
	if !core.WorkspaceExists(workspaceDir) {
		return &rule.PermissionRules{}, nil
	}

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("cannot load config: %w", err)
	}
	if cfg.PermissionRules == nil {
		return &rule.PermissionRules{}, nil
	}
	return cfg.PermissionRules, nil
}

func listRules(cmd *cobra.Command) error {
	rules, err := loadPermissionRules()
	if err != nil {
		return err
	}

	total := len(rules.AlwaysAllow) + len(rules.AlwaysDeny) + len(rules.AlwaysAsk)
	if total == 0 {
		fmt.Println("No permission rules configured.")
		return nil
	}

	printRuleGroup("Always Allow", rules.AlwaysAllow)
	printRuleGroup("Always Deny", rules.AlwaysDeny)
	printRuleGroup("Always Ask", rules.AlwaysAsk)

	return nil
}

func printRuleGroup(header string, group []rule.PermissionRule) {
	if len(group) == 0 {
		return
	}
	fmt.Printf("%s:\n", header)
	fmt.Println(strings.Repeat("─", 50))
	for _, r := range group {
		desc := r.Description
		if desc == "" {
			desc = "(no description)"
		}
		source := r.Source
		if source != "" {
			source = " [" + source + "]"
		}
		pattern := r.ContentPattern
		if pattern != "" {
			pattern = " (pattern: " + pattern + ")"
		}
		fmt.Printf("  %s%s%s\n", desc, source, pattern)
	}
	fmt.Println()
}

func showRule(cmd *cobra.Command, id string) error {
	rules, err := loadPermissionRules()
	if err != nil {
		return err
	}

	// Search across all groups
	candidates := append(rules.AlwaysAllow, rules.AlwaysDeny...)
	candidates = append(candidates, rules.AlwaysAsk...)

	for _, r := range candidates {
		if r.ToolName == id {
			fmt.Printf("Tool:      %s\n", r.ToolName)
			fmt.Printf("Behavior:  %s\n", r.Behavior)
			if r.Description != "" {
				fmt.Printf("Desc:      %s\n", r.Description)
			}
			if r.ContentPattern != "" {
				fmt.Printf("Pattern:   %s\n", r.ContentPattern)
			}
			if r.Source != "" {
				fmt.Printf("Source:    %s\n", r.Source)
			}
			return nil
		}
	}
	return fmt.Errorf("no permission rule found with tool name %q", id)
}


