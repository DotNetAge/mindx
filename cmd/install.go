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
	Short: "Install MindX to system (daemon + PATH + shortcut)",
	Long: `Installs MindX and configures all necessary system integrations.

Behavior adapts to how MindX was installed:

  - **Package manager** (Homebrew, apt, etc.): Skips binary copy and PATH setup.
    Only registers the daemon service. The package manager owns the binary.
  - **Manual download**: Copies binary to a stable location, adds to PATH,
    registers daemon, and creates desktop shortcut (Windows).

This command requires administrator / elevated privileges on Windows (for System PATH
and schtasks registration). On macOS/Linux, user-level installation is sufficient.

Examples:
  mindx install                  # Smart install (auto-detects source)
  mindx install --force-copy     # Force copy even from managed location
  mindx install --no-daemon      # Skip daemon registration
  mindx install --dir /opt/mindx # Custom install directory`,
	RunE: runInstall,
}

var (
	installDir        string
	installNoDaemon   bool
	installNoPath     bool
	installNoShortcut bool
	installForceCopy  bool
)

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVar(&installDir, "dir", "", "Custom install directory (default: platform-specific)")
	installCmd.Flags().BoolVar(&installNoDaemon, "no-daemon", false, "Skip daemon/service registration")
	installCmd.Flags().BoolVar(&installNoPath, "no-path", false, "Skip PATH configuration")
	installCmd.Flags().BoolVar(&installNoShortcut, "no-shortcut", false, "Skip desktop shortcut creation")
	installCmd.Flags().BoolVar(&installForceCopy, "force-copy", false, "Force copy binary even from managed location")
}

func runInstall(cmd *cobra.Command, args []string) error {
	fmt.Println(setupstyle.GradientTitle(""))
	fmt.Println()

	// ── 检查是否已初始化，未初始化则弹出安装向导 ──
	workspaceDir := core.DefaultUserPrefsDir()
	cfg, cfgErr := core.LoadMindxConfig(workspaceDir)
	if cfgErr == nil && !cfg.Initialized {
		fmt.Println("⚙️  Configuration not complete — launching setup wizard...")
		fmt.Println()

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Setup wizard failed: %v\n", err)
			fmt.Println("   You can run 'mindx' later to complete configuration.")
		} else {
			fmt.Println()
			fmt.Println("✅ Configuration complete!")
		}
		fmt.Println()
	}

	opts := setup.InstallOptions{
		InstallDir:   installDir,
		SkipDaemon:   installNoDaemon,
		SkipPath:     installNoPath,
		SkipShortcut: installNoShortcut,
		ForceCopy:    installForceCopy,
	}

	result, err := setup.Install(opts)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Println()
	fmt.Println("────────────────────────────────────────")
	fmt.Println("Installation complete!")
	fmt.Println()
	fmt.Printf("   Source:  %s\n", result.Source)
	fmt.Printf("   Binary:  %s\n", result.BinaryDest)

	if result.PathConfigured {
		fmt.Println("   PATH:   configured")
	}
	if result.DaemonSetup {
		fmt.Println("   Daemon: registered")
		// Verify daemon is actually running — launchctl bootstrap may succeed
		// even when the daemon process crashes immediately (port conflict, etc.)
		daemonStatus, _ := setup.CheckDaemon()
		if daemonStatus != setup.DaemonRunning {
			fmt.Println("   ⚠️  Daemon registered but not yet running.")
			fmt.Println("      Run 'mindx start' to launch it, or check:")
			fmt.Println("        ~/.mindx/logs/daemon.err.log")
		}
	}
	if result.ShortcutCreated {
		fmt.Println("   Shortcut: Desktop")
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Open a new terminal (PATH changes take effect)")
	if !result.DaemonSetup {
		fmt.Println("  2. Run 'mindx start' to launch the daemon")
	}
	fmt.Println("  3. Run 'mindx' to start chatting")

	return nil
}
