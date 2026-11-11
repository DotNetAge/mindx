package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	setupstyle "github.com/DotNetAge/mindx/internal/setup/style"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install MindX to system (daemon + PATH)",
	Long: `Installs MindX and configures all necessary system integrations.

Behavior adapts to how MindX was installed:

  - **Package manager** (Homebrew, apt, etc.): Skips binary copy and PATH setup.
    Only registers the daemon service. The package manager owns the binary.
  - **Manual download**: Copies binary to a stable location, adds to PATH,
    and registers daemon.

On macOS/Linux, user-level installation is sufficient.

Examples:
  mindx install                  # Smart install (auto-detects source)
  mindx install --force-copy     # Force copy even from managed location
  mindx install --no-daemon      # Skip daemon registration
  mindx install --dir /opt/mindx # Custom install directory
  mindx install -s               # Silent: skip the interactive setup wizard (no model config), for use by other Clients`,
	RunE: runInstall,
}

var (
	installDir       string
	installNoDaemon  bool
	installNoPath    bool
	installForceCopy bool
	installSilent    bool
)

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVar(&installDir, "dir", "", "Custom install directory (default: platform-specific)")
	installCmd.Flags().BoolVar(&installNoDaemon, "no-daemon", false, "Skip daemon/service registration")
	installCmd.Flags().BoolVar(&installNoPath, "no-path", false, "Skip PATH configuration")
	installCmd.Flags().BoolVar(&installForceCopy, "force-copy", false, "Force copy binary even from managed location")
	installCmd.Flags().BoolVarP(&installSilent, "silent", "s", false, "Skip the interactive setup wizard (provider/model); install without configuring a model")
}

func runInstall(cmd *cobra.Command, args []string) error {
	fmt.Println(setupstyle.GradientTitle(""))
	fmt.Println()

	// ── 检查是否需要弹出安装向导：未初始化，或已初始化但未配置模型 ──
	// 静默模式（-s/--silent）跳过交互向导，不配置模型，供其它 Client 子进程调用
	workspaceDir := core.DefaultUserPrefsDir()
	cfg, cfgErr := core.LoadMindxConfig(workspaceDir)
	if cfgErr != nil {
		// 配置文件损坏或不可读：按未初始化处理，引导走向导重建。
		cfg = core.DefaultMindxConfig(workspaceDir)
	}
	needsWizard := !installSilent && (cfgErr != nil || !cfg.Initialized || cfg.DefaultModel == "")
	if needsWizard {
		fmt.Println("⚙️  配置尚未完成，正在打开设置向导...")
		fmt.Println()

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		clientProvidersPath := filepath.Join(settingsDir, "client-provider.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, clientProvidersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  设置向导未完成: %v\n", err)
			fmt.Println("   稍后可运行 'mindx' 继续完成配置。")
		} else {
			fmt.Println()
			fmt.Println("✅ 配置完成！")
		}
		fmt.Println()
	}

	opts := setup.InstallOptions{
		InstallDir: installDir,
		SkipDaemon: installNoDaemon,
		SkipPath:   installNoPath,
		ForceCopy:  installForceCopy,
	}

	result, err := setup.Install(opts)
	if err != nil {
		return fmt.Errorf("安装失败: %w", err)
	}

	// 静默模式安装成功后标记为已初始化，否则 daemon 会因未初始化而拒绝启动
	if installSilent && cfgErr == nil && !cfg.Initialized {
		if err := cfg.MarkInitialized(); err != nil {
			return fmt.Errorf("mark initialized after silent install: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("────────────────────────────────────────")
	fmt.Println("安装完成！")
	fmt.Println()
	fmt.Printf("   来源:     %s\n", result.Source)
	fmt.Printf("   程序位置: %s\n", result.BinaryDest)

	if result.PathConfigured {
		fmt.Println("   PATH:    已配置")
	}
	if result.DaemonSetup {
		fmt.Println("   后台服务: 已注册")
		// Verify daemon is actually running — launchctl bootstrap may succeed
		// even when the daemon process crashes immediately (port conflict, etc.)
		daemonStatus, _ := setup.CheckDaemon()
		if daemonStatus != setup.DaemonRunning {
			fmt.Println("   ⚠️  后台服务已注册但尚未运行。")
			fmt.Println("      可运行 'mindx start' 启动，或查看日志排查原因:")
			fmt.Println("        ~/.mindx/logs/daemon.err.log")
		}
	} else if !installNoDaemon {
		// 注册已尝试但失败：不打印「安装完成」，明确失败并指引。
		return fmt.Errorf("后台服务注册失败（原因见上方输出）；解决问题后重新运行 'mindx install' 即可")
	}

	fmt.Println()
	fmt.Println("后续步骤:")
	fmt.Println("  1. 打开一个新终端（PATH 变更生效）")
	if !result.DaemonSetup {
		fmt.Println("  2. 运行 'mindx start' 启动后台服务")
	}
	fmt.Println("  3. 运行 'mindx' 开始使用")

	return nil
}
