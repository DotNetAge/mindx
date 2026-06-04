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
	  start    启动后台 Daemon 服务（供 WebUI/MacUI 接入）
	  stop     停止 Daemon 服务
	  install  安装到系统（PATH + Daemon + 快捷方式）
	  status   查看系统状态
	  doctor   诊断系统健康
	  logs     查看或追踪 Daemon 日志
	  version  打印版本和构建信息
	  query    搜索长期记忆
	  provider 管理 LLM 供应商（list / rm / add）
	  model    管理 LLM 模型（list / rm / add）
	  agent    管理 AI 代理（list / rm / add）
	  web      打开浏览器访问 WebUI 界面

	示例:
	  mindx                    # 启动 TUI 聊天界面
	  mindx start             # 启动后台服务
	  mindx start --port 8080 # 指定端口
	  mindx install           # 安装到系统
	  mindx status            # 查看状态
	  mindx logs              # 查看日志
	  mindx logs -n 100 -f    # 实时追踪日志
	  mindx version           # 查看版本
	  mindx query "术语"      # 搜索长期记忆
	  mindx provider list    # 列出供应商
	  mindx model list       # 列出模型
	  mindx model set gpt-4  # 设置默认模型
	  mindx agent list       # 列出代理
	  mindx doctor            # 诊断健康
	  mindx doctor --fix      # 自动修复
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

	// Suggest install if running from non-standard location
	if installed, _, _ := setup.IsInstalled(); !installed {
		fmt.Print("💡 提示: 运行 'mindx install' 可将 MindX 安装到系统（配置 PATH / Daemon / 快捷方式）\n\n")
	}

	if !cfg.Initialized {
		fmt.Print("\n⚙️  检测到首次运行，进入配置向导...\n\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			return fmt.Errorf("配置向导异常: %w", err)
		}

		fmt.Print("\n✅ 配置完成！正在启动 MindX...\n\n")
	} else if needsDoctor(cfg, workspaceDir) {
		fmt.Print("\n⚙️  环境检查：部分组件未就绪，进入配置向导...\n\n")

		if _, err := os.Stat(filepath.Join(workspaceDir, ".venv")); os.IsNotExist(err) {
			fmt.Print("💡 Python 虚拟环境未创建，技能依赖未安装。\n")
		}
		fmt.Print("\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			return fmt.Errorf("环境修复失败: %w", err)
		}

		fmt.Print("\n✅ 环境已更新！正在启动 MindX...\n\n")
	}

	if err := client.NewProgram(cfg); err != nil {
		return err
	}
	return nil
}
