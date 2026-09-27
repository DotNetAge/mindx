package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/skillstore"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Discover dynamic skills in a project directory",
	Long: `Discover dynamic skills under <projectDir>/.agents/skills.

Dynamic skills live inside the working directory and are NEVER injected into
the system prompt. Use this command to list their names and descriptions, then
load a matched skill on demand via the Skill tool.

Examples:
  mindx skills discovery
  mindx skills discovery --dir /path/to/project
  mindx skills discovery --json`,
}

func init() {
	discoveryCmd.Flags().String("dir", "", "Project directory to scan (defaults to the current working directory)")
	discoveryCmd.Flags().Bool("json", false, "Output structured JSON (for LLM consumption)")
	skillsCmd.AddCommand(discoveryCmd)
	rootCmd.AddCommand(skillsCmd)
}

var discoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: "List dynamic skills found in the project directory",
	Long: `List dynamic skills found under <projectDir>/.agents/skills with their
names and descriptions. Reads the disk directly and does not require the daemon.
Use --json for structured output (for LLM consumption).`,
	Args: cobra.NoArgs,
	RunE: runSkillsDiscovery,
}

func runSkillsDiscovery(cmd *cobra.Command, _ []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	useJSON, _ := cmd.Flags().GetBool("json")
	return skillsDiscovery(dir, useJSON)
}

// skillsDiscovery 扫描 <dir>/.agents/skills 内的动态技能并输出名称与描述列表。
// 直接读盘，不依赖 daemon。
func skillsDiscovery(projectDir string, useJSON bool) error {
	if strings.TrimSpace(projectDir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("无法确定工作目录: %w", err)
		}
		projectDir = wd
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return fmt.Errorf("无法解析项目目录: %w", err)
	}

	skillsRoot := filepath.Join(projectDir, ".agents", "skills")
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			if useJSON {
				fmt.Println("[]")
				return nil
			}
			fmt.Printf("No dynamic skills found (%s does not exist).\n", skillsRoot)
			return nil
		}
		return fmt.Errorf("无法读取动态技能目录: %w", err)
	}

	type discovered struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RootDir     string `json:"root_dir"`
	}

	var skills []discovered
	var warnings []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sk, _, loadErr := skillstore.LoadSkillFromDir(filepath.Join(skillsRoot, e.Name()), "project")
		if loadErr != nil || sk == nil {
			warnings = append(warnings, fmt.Sprintf("跳过 %s: %v", e.Name(), loadErr))
			continue
		}
		skills = append(skills, discovered{
			Name:        sk.Name,
			Description: strings.TrimSpace(sk.Description),
			RootDir:     filepath.Join(skillsRoot, e.Name()),
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	if useJSON {
		out, err := json.MarshalIndent(skills, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		return nil
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if len(skills) == 0 {
		fmt.Println("No dynamic skills found.")
		return nil
	}

	fmt.Printf("Dynamic skills in %s:\n", skillsRoot)
	maxName := 4
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
