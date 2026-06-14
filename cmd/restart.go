package cmd

import (
	"fmt"

	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the MindX daemon service",
	Long: `Restarts the running MindX daemon service.

This command stops and then starts the daemon via the system
service manager (launchctl / systemd / schtasks).

Examples:
  mindx restart   # Restart daemon service`,
	RunE: runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
	fmt.Println("🔄 Restarting MindX daemon...")

	status, err := setup.CheckDaemon()
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}

	switch status {
	case setup.DaemonNotInstalled:
		fmt.Println("ℹ️  Daemon is not installed. Run 'mindx install' to set it up.")
		return nil
	case setup.DaemonRunning:
		fmt.Println("  → Stopping daemon...")
		if err := setup.StopDaemon(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
	case setup.DaemonStopped:
		fmt.Println("  → Daemon is not running, starting directly...")
	}

	if err := setup.StartDaemon(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Println("✅ Daemon restarted.")
	return nil
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
