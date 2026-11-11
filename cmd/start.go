package cmd

import (
	"fmt"

	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the installed MindX daemon service",
	Long: `Starts the MindX daemon service that was previously installed via 'mindx install'.

This command tells the system service manager (launchctl / systemd)
to launch the daemon process. The daemon provides WebSocket gateway for
WebUI/MacUI and runs the scheduler for timed tasks.

Examples:
  mindx start   # Start daemon service via system service manager`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 正在启动后台服务...")

	status, err := setup.CheckDaemon()
	if err != nil {
		return fmt.Errorf("检查服务状态失败: %w", err)
	}

	switch status {
	case setup.DaemonNotInstalled:
		fmt.Println("ℹ️  后台服务尚未安装，请先运行 'mindx install'。")
		return nil
	case setup.DaemonRunning:
		fmt.Println("ℹ️  后台服务已在运行中。")
		return nil
	}

	if err := setup.StartDaemon(); err != nil {
		return fmt.Errorf("启动后台服务失败: %w\n可查看 ~/.mindx/logs/daemon.err.log 了解具体原因", err)
	}

	return verifyDaemonStarted()
}

func init() {
	rootCmd.AddCommand(startCmd)
}
