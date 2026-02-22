package cli

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/embedding"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: i18n.T("cli.skill.short"),
	Long:  i18n.T("cli.skill.long"),
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cli.skill.list.short"),
	Long:  i18n.T("cli.skill.list.long"),
	Example: fmt.Sprintf(`  # %s
  mindx skill list

  # %s
  mindx skill list --category general`,
		i18n.T("cli.skill.list.example1"),
		i18n.T("cli.skill.list.example2")),
	Run: func(cmd *cobra.Command, args []string) {
		categoryFilter, _ := cmd.Flags().GetString("category")

		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		skillInfos := mgr.GetSkillInfos()

		fmt.Println(i18n.TWithData("cli.skill.list.found", map[string]interface{}{"Count": len(skillInfos)}))
		fmt.Println()

		for name, info := range skillInfos {
			if categoryFilter != "" && info.Def.Category != categoryFilter {
				continue
			}

			statusIcon := getStatusIcon(info)

			fmt.Printf("%s %s (%s)\n", statusIcon, name, info.Def.Version)
			fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.description"), info.Def.Description)
			if len(info.Def.Tags) > 0 {
				fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.tags"), strings.Join(info.Def.Tags, ", "))
			}

			if len(info.MissingBins) > 0 {
				fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.missing_bins"), strings.Join(info.MissingBins, ", "))
			}
			if len(info.MissingEnv) > 0 {
				fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.missing_env"), strings.Join(info.MissingEnv, ", "))
			}

			fmt.Println(i18n.TWithData("cli.skill.list.stats", map[string]interface{}{
				"Success": info.SuccessCount,
				"Error":   info.ErrorCount,
				"Avg":     info.AvgExecutionMs,
			}))
			fmt.Println()
		}
	},
}

var skillRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: i18n.T("cli.skill.run.short"),
	Long:  i18n.T("cli.skill.run.long"),
	Example: `  # ` + i18n.T("cli.skill.run.example") + `
  mindx skill run github --repo owner/repo --pr 55`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		skillList, err := mgr.GetSkills()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.run.get_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		var targetSkill *core.Skill
		for _, skill := range skillList {
			if skill.GetName() == name {
				targetSkill = skill
				break
			}
		}

		if targetSkill == nil {
			fmt.Println(i18n.TWithData("cli.skill.run.not_found", map[string]interface{}{"Name": name}))
			return
		}

		params := parseParams(cmd.Flags().Args())

		fmt.Println(i18n.TWithData("cli.skill.run.running", map[string]interface{}{"Name": name}))
		fmt.Println(i18n.TWithData("cli.skill.run.params", map[string]interface{}{"Params": fmt.Sprintf("%v", params)}))
		fmt.Println()

		if err := mgr.Execute(targetSkill, params); err != nil {
			fmt.Printf("%s: %v\n", i18n.T("cli.skill.run.error"), err)
			return
		}

		fmt.Println(i18n.T("cli.skill.run.success"))
	},
}

var skillValidateCmd = &cobra.Command{
	Use:   "validate <name>",
	Short: i18n.T("cli.skill.validate.short"),
	Long:  i18n.T("cli.skill.validate.long"),
	Example: `  # ` + i18n.T("cli.skill.validate.example") + `
  mindx skill validate github`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		info, exists := mgr.GetSkillInfo(name)
		if !exists {
			fmt.Println(i18n.TWithData("cli.skill.run.not_found", map[string]interface{}{"Name": name}))
			return
		}

		fmt.Println(i18n.TWithData("cli.skill.validate.result", map[string]interface{}{"Name": name}))
		fmt.Printf("  %s: %v\n", i18n.T("cli.skill.validate.enabled"), info.Def.Enabled)
		fmt.Printf("  %s: %v\n", i18n.T("cli.skill.validate.can_run"), info.CanRun)

		if len(info.MissingBins) > 0 {
			fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.missing_bins"), strings.Join(info.MissingBins, ", "))
		}

		if len(info.MissingEnv) > 0 {
			fmt.Printf("  %s: %s\n", i18n.T("cli.skill.list.missing_env"), strings.Join(info.MissingEnv, ", "))
		}

		if info.ErrorCount > 0 {
			fmt.Printf("  %s: %s\n", i18n.T("cli.skill.validate.last_error"), info.LastError)
		}
	},
}

var skillEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: i18n.T("cli.skill.enable.short"),
	Long:  i18n.T("cli.skill.enable.long"),
	Example: `  # ` + i18n.T("cli.skill.enable.example") + `
  mindx skill enable github`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		if err := mgr.Enable(name); err != nil {
			fmt.Printf("%s: %v\n", i18n.T("cli.skill.enable.error"), err)
			return
		}

		fmt.Println(i18n.TWithData("cli.skill.enable.success", map[string]interface{}{"Name": name}))
	},
}

var skillDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: i18n.T("cli.skill.disable.short"),
	Long:  i18n.T("cli.skill.disable.long"),
	Example: `  # ` + i18n.T("cli.skill.disable.example") + `
  mindx skill disable github`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		if err := mgr.Disable(name); err != nil {
			fmt.Printf("%s: %v\n", i18n.T("cli.skill.disable.error"), err)
			return
		}

		fmt.Println(i18n.TWithData("cli.skill.disable.success", map[string]interface{}{"Name": name}))
	},
}

var skillReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: i18n.T("cli.skill.reload.short"),
	Long:  i18n.T("cli.skill.reload.long"),
	Example: `  # ` + i18n.T("cli.skill.reload.example") + `
  mindx skill reload`,
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := createSkillManager()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.skill.list.init_error", map[string]interface{}{"Error": err.Error()}))
			return
		}

		if err := mgr.LoadSkills(); err != nil {
			fmt.Printf("%s: %v\n", i18n.T("cli.skill.reload.error"), err)
			return
		}

		skillInfos := mgr.GetSkillInfos()
		fmt.Println(i18n.TWithData("cli.skill.reload.success", map[string]interface{}{"Count": len(skillInfos)}))
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)

	skillListCmd.Flags().String("category", "", i18n.T("cli.skill.list.flag_category"))
	skillCmd.AddCommand(skillListCmd)

	skillCmd.AddCommand(skillRunCmd)
	skillCmd.AddCommand(skillValidateCmd)
	skillCmd.AddCommand(skillEnableCmd)
	skillCmd.AddCommand(skillDisableCmd)
	skillCmd.AddCommand(skillReloadCmd)
}

func createSkillManager() (*skills.SkillMgr, error) {
	if err := config.EnsureWorkspace(); err != nil {
		return nil, err
	}

	workspacePath, err := config.GetWorkspacePath()
	if err != nil {
		return nil, err
	}

	_, _, _, _, err = config.InitVippers()
	if err != nil {
		return nil, fmt.Errorf("初始化配置失败: %w", err)
	}
	modelsMgr := config.GetModelsManager()

	embeddingSvc := embedding.NewEmbeddingService(nil)

	brainModels := modelsMgr.GetBrainModels()
	defaultModelName := modelsMgr.GetDefaultModel()
	indexModelName := brainModels.IndexModel
	if indexModelName == "" {
		indexModelName = defaultModelName
	}
	if indexModelName == "" {
		indexModelName = brainModels.SubconsciousModel
	}

	indexModel, err := modelsMgr.GetModel(indexModelName)
	if err != nil {
		indexModel = &config.ModelConfig{
			Name:    "qwen3:0.6b",
			BaseURL: "http://localhost:11434/v1",
		}
	}

	ollamaSvc := llama.NewOllamaService(indexModel.Name)
	if indexModel.BaseURL != "" {
		baseURL := indexModel.BaseURL
		if len(baseURL) > 3 && baseURL[len(baseURL)-3:] == "/v1" {
			baseURL = baseURL[:len(baseURL)-3]
		}
		ollamaSvc = ollamaSvc.WithBaseUrl(baseURL)
	}

	installSkillsPath, err := config.GetInstallSkillsPath()
	if err != nil {
		return nil, err
	}

	return skills.NewSkillMgr(installSkillsPath, workspacePath, embeddingSvc, ollamaSvc, logging.GetSystemLogger())
}

func getStatusIcon(info *entity.SkillInfo) string {
	if !info.Def.Enabled {
		return "🚫"
	}
	if !info.CanRun {
		return "❌"
	}
	return "✅"
}

func parseParams(args []string) map[string]interface{} {
	params := make(map[string]interface{})
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			key := strings.TrimPrefix(args[i], "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				params[key] = args[i+1]
				i++
			} else {
				params[key] = true
			}
		}
	}
	return params
}
