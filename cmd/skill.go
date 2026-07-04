package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/goharness/skill"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill list|get <name>|add <path>|validate <name>",
	Short: "Manage installed skills",
	Long: `List, inspect, install, or validate installed MindX skills.

The daemon manages the skill registry. Normal commands operate on skill names.
Use --json to query the daemon and output rich JSON (for LLM consumption).

Examples:
  mindx skill list
  mindx skill list --json
  mindx skill get batch
  mindx skill validate batch

Use add only when installing a skill from a local development directory:
  mindx skill add ./my-skill`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkill,
}

var skillListJSONCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")
		filter, _ := cmd.Flags().GetStringSlice("filter")

		if useJSON {
			cl, err := rpc.Dial(daemonAddr)
			if err != nil {
				return fmt.Errorf("cannot connect to daemon: %w", err)
			}
			defer func() { _ = cl.Close() }()

			result, err := cl.SkillList(strings.Join(filter, ","))
			if err != nil {
				return err
			}
			var pretty interface{}
			if err := json.Unmarshal(result, &pretty); err == nil {
				formatted, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(formatted))
				return nil
			}
			fmt.Println(string(result))
			return nil
		}

		return listSkills(cmd, filter)
	},
}

var skillGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show details of a specific skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return showSkillDetail(cmd, args[0])
	},
}

var skillAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Install or update a skill from a local directory",
	Long: `Install a skill from a local development directory into the managed registry.

The source directory must contain a SKILL.md file. The skill name is taken from
the SKILL.md frontmatter. The daemon manages where the skill is stored after
installation.

Examples:
  mindx skill add ./my-skill
  mindx skill add /absolute/path/to/my-skill`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addSkill(args[0])
	},
}

var skillValidateCmd = &cobra.Command{
	Use:   "validate <name>",
	Short: "Validate an installed skill",
	Long: `Check that an installed skill's frontmatter and structure are valid.

The skill must already be installed in the managed registry. Use "mindx skill add"
to install a skill from a local directory first if needed.

Examples:
  mindx skill validate batch`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return validateSkill(args[0])
	},
}

func init() {
	skillListJSONCmd.Flags().Bool("json", false, "Output JSON via daemon (requires mindx start)")
	skillListJSONCmd.Flags().StringSliceP("filter", "f", nil, "Filter skills by name or description (case-insensitive, comma-separated)")
	skillCmd.AddCommand(skillListJSONCmd)
	skillCmd.AddCommand(skillGetCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillValidateCmd)
	rootCmd.AddCommand(skillCmd)
}

func runSkill(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "list":
		return skillListJSONCmd.RunE(cmd, args)
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx skill get <name>")
		}
		return showSkillDetail(cmd, args[1])
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx skill add <path>")
		}
		return addSkill(args[1])
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx skill validate <name>")
		}
		return validateSkill(args[1])
	default:
		return fmt.Errorf("unknown subcommand %q — use list, get, add, or validate", args[0])
	}
}

func skillsDir() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "skills")
}

func listSkills(cmd *cobra.Command, filter []string) error {
	dir := skillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No skills found (skills directory does not exist).")
			return nil
		}
		return fmt.Errorf("cannot read skills directory: %w", err)
	}

	type skillInfo struct {
		Name        string
		Description string
		Tools       string
	}

	var skills []skillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sk, err := skill.LoadSkillFromDir(filepath.Join(dir, e.Name()), "filesystem")
		if err != nil {
			continue
		}
		if sk == nil {
			continue
		}
		skills = append(skills, skillInfo{
			Name:        sk.Name,
			Description: strings.TrimSpace(sk.Description),
			Tools:       sk.AllowedTools,
		})
	}

	if len(filter) > 0 {
		var filtered []skillInfo
		for _, s := range skills {
			nameLower := strings.ToLower(s.Name)
			descLower := strings.ToLower(s.Description)
			match := false
			for _, f := range filter {
				f = strings.TrimSpace(f)
				if f == "" {
					continue
				}
				fLower := strings.ToLower(f)
				if strings.Contains(nameLower, fLower) || strings.Contains(descLower, fLower) {
					match = true
					break
				}
			}
			if match {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	if len(skills) == 0 {
		if len(filter) > 0 {
			fmt.Printf("No skills found matching %q.\n", strings.Join(filter, ","))
			return nil
		}
		fmt.Println("No skills found.")
		return nil
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	// Name column width
	maxName := 6
	for _, s := range skills {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
	}

	if len(filter) > 0 {
		fmt.Printf("Skills matching %q:\n", strings.Join(filter, ","))
	}
	fmt.Printf("%-*s  %s\n", maxName, "Name", "Description")
	fmt.Println(strings.Repeat("─", maxName+2) + "──────────────────────────────")
	for _, s := range skills {
		fmt.Printf("%-*s  %s\n", maxName, s.Name, s.Description)
	}

	return nil
}

func showSkillDetail(cmd *cobra.Command, name string) error {
	dir := filepath.Join(skillsDir(), name)
	sk, err := skill.LoadSkillFromDir(dir, "filesystem")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found", name)
		}
		return fmt.Errorf("cannot read skill %q: %w", name, err)
	}
	if sk == nil {
		return fmt.Errorf("skill %q not found", name)
	}

	fmt.Printf("Name:        %s\n", sk.Name)
	fmt.Printf("Description: %s\n", strings.TrimSpace(sk.Description))
	if sk.AllowedTools != "" {
		fmt.Printf("Allowed Tools: %s\n", sk.AllowedTools)
	}
	if sk.RootDir != "" {
		fmt.Printf("Root Dir:    %s\n", sk.RootDir)
	}
	if sk.Requires != nil {
		fmt.Println("Requires:")
		if len(sk.Requires.Bins) > 0 {
			fmt.Printf("  Bins: %s\n", strings.Join(sk.Requires.Bins, ", "))
		}
		if len(sk.Requires.Env) > 0 {
			fmt.Printf("  Env:  %s\n", strings.Join(sk.Requires.Env, ", "))
		}
	}
	if len(sk.Metadata) > 0 {
		var keys []string
		for k := range sk.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println("Metadata:")
		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, sk.Metadata[k])
		}
	}
	return nil
}

func addSkill(srcPath string) error {
	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("cannot resolve source path: %w", err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("cannot access source path %q: %w", srcPath, err)
	}

	var skillDir string
	if info.IsDir() {
		skillDir = srcPath
	} else if info.Name() == "SKILL.md" {
		skillDir = filepath.Dir(srcPath)
	} else {
		return fmt.Errorf("source path must be a skill directory or a SKILL.md file: %s", srcPath)
	}

	sk, err := skill.LoadSkillFromDir(skillDir, "filesystem")
	if err != nil {
		return fmt.Errorf("skill validation failed: %w", err)
	}
	if sk == nil {
		return fmt.Errorf("no SKILL.md found in %s", skillDir)
	}

	destDir := filepath.Join(skillsDir(), sk.Name)

	// Remove existing directory if present
	if _, err := os.Stat(destDir); err == nil {
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("cannot remove existing skill directory: %w", err)
		}
		fmt.Printf("Existing skill %q removed before reinstall.\n", sk.Name)
	}

	if err := copyDir(skillDir, destDir); err != nil {
		return fmt.Errorf("cannot copy skill: %w", err)
	}

	fmt.Printf("Skill %q installed.\n", sk.Name)

	// Best-effort reload via daemon
	cl, err := rpc.Dial(daemonAddr)
	if err == nil {
		defer func() { _ = cl.Close() }()
		if _, reloadErr := cl.SkillReload(); reloadErr == nil {
			fmt.Println("Skills reloaded.")
		}
	}

	return nil
}

func validateSkill(name string) error {
	dir := filepath.Join(skillsDir(), name)
	sk, err := skill.LoadSkillFromDir(dir, "filesystem")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found", name)
		}
		return fmt.Errorf("validation failed for skill %q: %w", name, err)
	}
	if sk == nil {
		return fmt.Errorf("skill %q not found", name)
	}

	fmt.Printf("Skill %q is valid.\n", sk.Name)
	fmt.Printf("  Description: %s\n", strings.TrimSpace(sk.Description))
	if sk.AllowedTools != "" {
		fmt.Printf("  Allowed Tools: %s\n", sk.AllowedTools)
	}
	return nil
}

// copyDir recursively copies src to dst.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())

		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
