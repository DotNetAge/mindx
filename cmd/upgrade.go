package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/DotNetAge/mindx/internal/update"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade MindX to the latest version",
	Long: `Downloads and installs the latest MindX release from GitHub.

Automatically checks for a new version, downloads the appropriate
binary for your platform, and replaces the current installation.

After upgrading, you should restart the daemon service:
  mindx restart

Examples:
  mindx upgrade          # Check and apply the latest update
  mindx upgrade --check  # Only check for updates, don't install`,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	onlyCheck, _ := cmd.Flags().GetBool("check")

	workspaceDir := core.DefaultUserPrefsDir()
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.InstalledVersion == "" && core.Version != "" {
		cfg.InstalledVersion = core.Version
	}

	updater := update.NewUpdater(
		core.Version,
		cfg.InstalledVersion,
		workspaceDir,
		func(version string) error {
			cfg.InstalledVersion = version
			return cfg.Save()
		},
		func(msg string, args ...any) {
			fmt.Fprintf(os.Stderr, "  · "+msg+"\n", args...)
		},
	)

	// Run check
	fmt.Println("🔍 正在检查更新...")
	info := updater.Check(true)
	if info.Error != "" {
		return fmt.Errorf("检查更新失败: %s", info.Error)
	}

	if !info.UpdateAvailable {
		fmt.Printf("✅ 已是最新版本 (v%s)。\n", core.Version)
		return nil
	}

	fmt.Printf("📦 发现新版本: %s\n", info.LatestVersion)
	fmt.Printf("   当前版本: v%s\n", core.Version)
	fmt.Printf("   下载地址: %s\n", info.LatestURL)

	if onlyCheck {
		fmt.Println("\nℹ️  运行 'mindx upgrade'（不带 --check）即可安装更新。")
		return nil
	}

	// Confirm
	fmt.Println("\n⬇️  正在下载并安装更新...")

	// Stop daemon before replacing the binary
	daemonWasRunning := false
	status, err := setup.CheckDaemon()
	if err == nil && status == setup.DaemonRunning {
		fmt.Println("  → 正在停止后台服务...")
		if err := setup.StopDaemon(); err != nil {
			// 停不下来就中止：旧服务可能仍占用端口，此时替换二进制后新服务起不来。
			return fmt.Errorf("无法停止正在运行的后台服务，本次升级已中止: %w\n旧服务可能仍占用端口，请手动运行 'mindx stop' 后重试", err)
		}
		daemonWasRunning = true
		fmt.Println("  ✅ 后台服务已停止")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := updater.DownloadAndInstall(ctx); err != nil {
		return fmt.Errorf("安装更新失败: %w", err)
	}

	fmt.Println("✅ 更新安装成功。")

	// Restart daemon if it was running before
	if daemonWasRunning {
		fmt.Println("  → 正在重启后台服务...")
		if err := setup.StartDaemon(); err != nil {
			fmt.Printf("⚠️  后台服务重启失败: %v\n", err)
			fmt.Println("   请手动运行 'mindx restart'。")
		} else {
			fmt.Println("  ✅ 后台服务已重启")
		}
	} else {
		fmt.Println("ℹ️  升级前后台服务未在运行，需要时运行 'mindx start' 启动。")
	}

	return nil
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Only check for updates, don't install")
	rootCmd.AddCommand(upgradeCmd)
}
