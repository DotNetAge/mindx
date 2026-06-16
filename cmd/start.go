package cmd

import (
	"fmt"
	"time"

	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the installed MindX daemon service",
	Long: `Starts the MindX daemon service that was previously installed via 'mindx install'.

This command tells the system service manager (launchctl / systemd / schtasks)
to launch the daemon process. The daemon provides WebSocket gateway for
WebUI/MacUI and runs the scheduler for timed tasks.

Examples:
  mindx start   # Start daemon service via system service manager`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Starting MindX daemon...")

	status, err := setup.CheckDaemon()
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}

	switch status {
	case setup.DaemonNotInstalled:
		fmt.Println("ℹ️  Daemon is not installed. Run 'mindx install' to set it up.")
		return nil
	case setup.DaemonRunning:
		fmt.Println("ℹ️  Daemon is already running.")
		return nil
	}

	if err := setup.StartDaemon(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Post-start verification: confirm the daemon is actually running.
	// Service managers (launchctl, systemd, etc.) may report success even
	// when the child process crashes immediately (e.g. port conflict, bad config).
	time.Sleep(500 * time.Millisecond)
	verifyStatus, _ := setup.CheckDaemon()
	if verifyStatus != setup.DaemonRunning {
		fmt.Println("❌ Daemon failed to start or exited immediately.")
		fmt.Println()
		fmt.Println("  Possible causes:")
		fmt.Println("    • Port 1313 or 1314 is already in use")
		fmt.Println("    • Configuration error — try 'mindx doctor --fix'")
		fmt.Println("    • Check logs for details:")
		fmt.Println("      ~/.mindx/logs/daemon.log")
		fmt.Println("      ~/.mindx/logs/daemon.err.log")
		return fmt.Errorf("daemon started but is not running (status=%s)", verifyStatus)
	}

	fmt.Println("✅ Daemon started.")
	return nil
}

func init() {
	rootCmd.AddCommand(startCmd)
}
