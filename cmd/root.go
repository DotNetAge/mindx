package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/client"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

// needsDoctor checks if the environment is healthy enough to run the chat TUI.
// Returns true if the user needs to run the setup wizard.
func needsDoctor(cfg *core.MindxConfig, workspaceDir string) bool {
	if cfg.DefaultModel == "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, ".venv")); os.IsNotExist(err) {
		return true
	}
	return false
}

var RuntimeFS fs.FS

var rootCmd = &cobra.Command{
	Use:   "mindx",
	Short: "MindX - AI Agent CLI",
	Long: `MindX 是一个 AI-native 的多 Agent 对话平台。

	默认行为:
	  直接运行 mindx 将启动 TUI（终端界面）进行对话。

	子命令:
	  start   启动后台 Daemon 服务（供 WebUI/MacUI 接入）
	  doctor  重新运行配置向导
	  web     打开浏览器访问 WebUI 界面

	示例:
	  mindx                    # 启动 TUI 聊天界面
	  mindx start             # 启动后台服务
	  mindx start --port 8080 # 指定端口
	  mindx doctor            # 重新配置环境
	  mindx web               # 打开 WebUI`,
	RunE:         runTUI,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(webCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	cfg, err := core.Bootstrap(RuntimeFS, workspaceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 自检失败: %v\n", err)
		return err
	}

	if !cfg.Initialized {
		fmt.Print("\n⚙️  检测到首次运行，进入配置向导...\n\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg); err != nil {
			return fmt.Errorf("配置向导异常: %w", err)
		}

		fmt.Print("\n✅ 配置完成！正在启动 MindX...\n\n")
	} else if needsDoctor(cfg, workspaceDir) {
		fmt.Print("\n⚙️  环境检查：部分组件未就绪，进入配置向导...\n\n")

		if !cfg.HasEmbedder() {
			fmt.Print("💡 Embedder 模型未配置，语义记忆不可用。\n")
		}
		if _, err := os.Stat(filepath.Join(workspaceDir, ".venv")); os.IsNotExist(err) {
			fmt.Print("💡 Python 虚拟环境未创建，技能依赖未安装。\n")
		}
		fmt.Print("\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg); err != nil {
			return fmt.Errorf("环境修复失败: %w", err)
		}

		fmt.Print("\n✅ 环境已更新！正在启动 MindX...\n\n")
	}

	if err := client.NewProgram(cfg); err != nil {
		return err
	}
	return nil
}
