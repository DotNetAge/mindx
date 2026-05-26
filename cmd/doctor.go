package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "重新运行配置向导以修改环境设置",
	Long: `显示首次配置向导界面，允许修改模型、API Key、Daemon 自启动、
Python 虚拟环境和记忆体 Embedder 模型等设置。

已有的配置值会自动保留，你可根据需要调整各选项。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceDir := core.DefaultUserPrefsDir()

		cfg, err := core.LoadMindxConfig(workspaceDir)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		fmt.Print("\n⚙️  MindX 环境检查与配置...\n\n")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg); err != nil {
			return fmt.Errorf("配置失败: %w", err)
		}

		fmt.Print("\n✅ 配置已更新！\n")
		return nil
	},
}

func init() {
	// Check if workspace exists before running doctor
	doctorCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		workspaceDir := core.DefaultUserPrefsDir()
		if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
			return fmt.Errorf("工作目录 %s 不存在，请先直接运行 'mindx' 完成初始化", workspaceDir)
		}
		return nil
	}
}
