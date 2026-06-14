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
	fmt.Println("🔍 Checking for updates...")
	info := updater.Check(true)
	if info.Error != "" {
		return fmt.Errorf("check update: %s", info.Error)
	}

	if !info.UpdateAvailable {
		fmt.Printf("✅ You are up to date (v%s).\n", core.Version)
		return nil
	}

	fmt.Printf("📦 New version available: %s\n", info.LatestVersion)
	fmt.Printf("   Current version: v%s\n", core.Version)
	fmt.Printf("   Download: %s\n", info.LatestURL)

	if onlyCheck {
		fmt.Println("\nℹ️  Run 'mindx upgrade' (without --check) to install the update.")
		return nil
	}

	// Confirm
	fmt.Println("\n⬇️  Downloading and installing update...")

	// Stop daemon before replacing the binary
	daemonWasRunning := false
	status, err := setup.CheckDaemon()
	if err == nil && status == setup.DaemonRunning {
		fmt.Println("  → Stopping daemon...")
		if err := setup.StopDaemon(); err != nil {
			fmt.Printf("⚠️  Failed to stop daemon: %v\n", err)
		} else {
			daemonWasRunning = true
			fmt.Println("  ✅ Daemon stopped")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := updater.DownloadAndInstall(ctx); err != nil {
		return fmt.Errorf("install update: %w", err)
	}

	fmt.Println("✅ Update installed successfully.")

	// Restart daemon if it was running before
	if daemonWasRunning {
		fmt.Println("  → Restarting daemon...")
		if err := setup.StartDaemon(); err != nil {
			fmt.Printf("⚠️  Failed to restart daemon: %v\n", err)
			fmt.Println("   Run 'mindx restart' manually.")
		} else {
			fmt.Println("  ✅ Daemon restarted")
		}
	} else {
		fmt.Println("ℹ️  Daemon was not running. Start it with: mindx start")
	}

	return nil
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Only check for updates, don't install")
	rootCmd.AddCommand(upgradeCmd)
}
