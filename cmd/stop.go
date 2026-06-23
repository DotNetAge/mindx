package cmd

import (
	"fmt"

	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the MindX daemon service",
	Long: `Stops the running MindX daemon (background service).

The daemon provides WebSocket gateway for WebUI/MacUI and runs
the scheduler for timed tasks.

Examples:
  mindx stop   # Stop daemon`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	fmt.Println("🛑 Stopping MindX daemon...")

	status, err := setup.CheckDaemon()
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}

	switch status {
	case setup.DaemonNotInstalled:
		fmt.Println("ℹ️  Daemon is not installed. Run 'mindx install' to set it up.")
		return nil
	case setup.DaemonStopped:
		fmt.Println("ℹ️  Daemon is already stopped.")
		return nil
	}

	if err := setup.StopDaemon(); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	return verifyDaemonStopped()
}
