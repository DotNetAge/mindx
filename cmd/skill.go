package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	AllowedTools string            `yaml:"allowed-tools"`
	Metadata     map[string]string `yaml:"metadata"`
}

var skillCmd = &cobra.Command{
	Use:   "skill list|get <name>",
	Short: "Manage installed skills",
	Long: `List or inspect installed MindX skills.

By default reads skills from local SKILL.md files.
Use --json to query the daemon and output rich JSON (for LLM consumption).

Examples:
  mindx skill list
  mindx skill list --json
  mindx skill get batch`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkill,
}

var skillListJSONCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")

		if useJSON {
			cl, err := rpc.Dial(daemonAddr)
			if err != nil {
				return fmt.Errorf("cannot connect to daemon: %w", err)
			}
			defer func() { _ = cl.Close() }()

			result, err := cl.SkillList("")
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

		return listSkills(cmd)
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

func init() {
	skillListJSONCmd.Flags().Bool("json", false, "Output JSON via daemon (requires mindx start)")
	skillCmd.AddCommand(skillListJSONCmd)
	skillCmd.AddCommand(skillGetCmd)
	rootCmd.AddCommand(skillCmd)
}

func runSkill(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "list":
		// Re-run the list subcommand's RunE
		return skillListJSONCmd.RunE(cmd, args)
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: mindx skill get <name>")
		}
		return showSkillDetail(cmd, args[1])
	default:
		return fmt.Errorf("unknown subcommand %q — use list or get", args[0])
	}
}

func skillsDir() string {
	return filepath.Join(core.DefaultUserPrefsDir(), "skills")
}

func listSkills(cmd *cobra.Command) error {
	dir := skillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No skills found (skills directory does not exist).")
			return nil
		}
		return fmt.Errorf("cannot read skills directory %s: %w", dir, err)
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
		fm, err := parseSkillFrontmatter(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		skills = append(skills, skillInfo{
			Name:        fm.Name,
			Description: strings.TrimSpace(fm.Description),
			Tools:       fm.AllowedTools,
		})
	}

	if len(skills) == 0 {
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

	fmt.Printf("%-*s  %s\n", maxName, "Name", "Description")
	fmt.Println(strings.Repeat("─", maxName+2) + "──────────────────────────────")
	for _, s := range skills {
		fmt.Printf("%-*s  %s\n", maxName, s.Name, s.Description)
	}

	return nil
}

func showSkillDetail(cmd *cobra.Command, name string) error {
	dir := filepath.Join(skillsDir(), name)
	fm, err := parseSkillFrontmatter(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found at %s", name, dir)
		}
		return fmt.Errorf("cannot read skill %q: %w", name, err)
	}

	fmt.Printf("Name:        %s\n", fm.Name)
	fmt.Printf("Description: %s\n", strings.TrimSpace(fm.Description))
	if fm.AllowedTools != "" {
		fmt.Printf("Allowed Tools: %s\n", fm.AllowedTools)
	}
	if fm.Metadata != nil {
		var keys []string
		for k := range fm.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println("Metadata:")
		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, fm.Metadata[k])
		}
	}
	return nil
}

// parseSkillFrontmatter reads a SKILL.md file and extracts its YAML frontmatter
// (between the opening and closing --- delimiters).
func parseSkillFrontmatter(path string) (*skillFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Find opening ---
	const delim = "---"
	start := strings.Index(content, delim)
	if start != 0 {
		return nil, fmt.Errorf("no YAML frontmatter found (must start with ---)")
	}

	// Find closing ---
	rest := content[len(delim):]
	end := strings.Index(rest, delim)
	if end < 0 {
		return nil, fmt.Errorf("unclosed YAML frontmatter")
	}

	yamlBlock := rest[:end]
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return nil, fmt.Errorf("cannot parse YAML frontmatter: %w", err)
	}
	return &fm, nil
}
