package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"mindx/internal/infrastructure/bootstrap"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"

	"github.com/spf13/cobra"
)

var kernelCmd = &cobra.Command{
	Use:   "kernel",
	Short: i18n.T("cli.kernel.short"),
	Long:  i18n.T("cli.kernel.long"),
}

var kernelMainCmd = &cobra.Command{
	Use:   "run",
	Short: i18n.T("cli.kernel.run.short"),
	Long:  i18n.T("cli.kernel.run.long"),
	Run: func(cmd *cobra.Command, args []string) {
		if os.Getenv("BOT_DEV_MODE") == "true" || runtime.GOOS != "darwin" {
			fmt.Println(i18n.T("cli.kernel.run.dev_mode"))
		} else {
			if err := ensurePlistLoaded(); err != nil {
				fmt.Println(i18n.TWithData("cli.kernel.run.plist_failed", map[string]interface{}{"Error": err.Error()}))
			}
		}

		logger := logging.GetSystemLogger()
		logger.Info(i18n.T("cli.kernel.run.starting"))

		_, err := bootstrap.Startup()
		if err != nil {
			logger.Error(i18n.T("cli.kernel.run.start_failed"), logging.Err(err))
			fmt.Println(i18n.TWithData("cli.kernel.run.start_failed", map[string]interface{}{"Error": err.Error()}))
			os.Exit(1)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		sig := <-sigCh
		logger.Info(i18n.T("cli.kernel.run.shutdown_signal"), logging.String("signal", sig.String()))

		if err := bootstrap.Shutdown(); err != nil {
			logger.Error(i18n.T("cli.kernel.run.shutdown_failed"), logging.Err(err))
		}

		fmt.Println(i18n.T("cli.kernel.run.shutdown"))
	},
}

var kernelCtrlStartCmd = &cobra.Command{
	Use:   "start",
	Short: i18n.T("cli.kernel.start.short"),
	Long:  i18n.T("cli.kernel.start.long"),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(i18n.T("cli.kernel.start.not_implemented"))
		fmt.Println(i18n.T("cli.kernel.start.todo"))
	},
}

var kernelCtrlStatusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("cli.kernel.status.short"),
	Long:  i18n.T("cli.kernel.status.long"),
	Run: func(cmd *cobra.Command, args []string) {
		switch runtime.GOOS {
		case "darwin":
			checkStatusMacOS()
		case "linux":
			checkStatusLinux()
		case "windows":
			checkStatusWindows()
		default:
			fmt.Println(i18n.TWithData("cli.kernel.status.unsupported_os", map[string]interface{}{"OS": runtime.GOOS}))
		}
	},
}

var kernelCtrlStopCmd = &cobra.Command{
	Use:   "stop",
	Short: i18n.T("cli.kernel.stop.short"),
	Long:  i18n.T("cli.kernel.stop.long"),
	Run: func(cmd *cobra.Command, args []string) {
		switch runtime.GOOS {
		case "darwin":
			stopServiceMacOS()
		case "linux":
			stopServiceLinux()
		case "windows":
			stopServiceWindows()
		default:
			fmt.Println(i18n.TWithData("cli.kernel.status.unsupported_os", map[string]interface{}{"OS": runtime.GOOS}))
		}
	},
}

var kernelCtrlRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: i18n.T("cli.kernel.restart.short"),
	Long:  i18n.T("cli.kernel.restart.long"),
	Run: func(cmd *cobra.Command, args []string) {
		switch runtime.GOOS {
		case "darwin":
			restartServiceMacOS()
		case "linux":
			restartServiceLinux()
		case "windows":
			restartServiceWindows()
		default:
			fmt.Println(i18n.TWithData("cli.kernel.status.unsupported_os", map[string]interface{}{"OS": runtime.GOOS}))
		}
	},
}

func checkStatusMacOS() {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.status.query_failed", map[string]interface{}{"Error": err.Error()}))
		return
	}

	lines := strings.Split(string(output), "\n")
	found := false
 	for _, line := range lines {
		if strings.Contains(line, "com.mindx.service") {
			found = true
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				state := parts[1]
				if state == "0" {
					fmt.Println(i18n.T("cli.kernel.status.running"))
				} else {
					fmt.Println(i18n.TWithData("cli.kernel.status.stopped", map[string]interface{}{"Code": state}))
				}
			}
			break
		}
	}

	if !found {
		fmt.Println(i18n.T("cli.kernel.status.not_installed"))
	}
}

func checkStatusLinux() {
	cmd := exec.Command("systemctl", "is-active", "mindx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s", strings.TrimSpace(string(output)))
		return
	}

	fmt.Printf("%s", strings.TrimSpace(string(output)))
}

func checkStatusWindows() {
	cmd := exec.Command("sc", "query", "MindX")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.status.query_failed", map[string]interface{}{"Error": err.Error()}))
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "STATE") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				state := parts[3]
				switch state {
				case "RUNNING":
					fmt.Println(i18n.T("cli.kernel.status.running"))
				case "STOPPED":
					fmt.Println(i18n.T("cli.kernel.status.stopped"))
				default:
					fmt.Printf("%s: %s\n", i18n.T("cli.kernel.status.label"), state)
				}
			}
			break
		}
	}
}

func stopServiceMacOS() {
	cmd := exec.Command("launchctl", "stop", "com.mindx.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.stop.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.stop.command_sent"))
}

func stopServiceLinux() {
	cmd := exec.Command("systemctl", "stop", "mindx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.stop.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.stop.stopped"))
}

func stopServiceWindows() {
	cmd := exec.Command("sc", "stop", "MindX")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.stop.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.stop.command_sent"))
}

func restartServiceMacOS() {
	fmt.Println(i18n.T("cli.kernel.restart.stopping"))
	stopServiceMacOS()

	fmt.Println(i18n.T("cli.kernel.restart.waiting"))

	fmt.Println(i18n.T("cli.kernel.restart.starting"))
	cmd := exec.Command("launchctl", "start", "com.mindx.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.restart.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.restart.restarted"))
}

func restartServiceLinux() {
	cmd := exec.Command("systemctl", "restart", "mindx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.restart.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.restart.restarted"))
}

func restartServiceWindows() {
	fmt.Println(i18n.T("cli.kernel.restart.stopping"))
	stopServiceWindows()

	fmt.Println(i18n.T("cli.kernel.restart.waiting"))

	fmt.Println(i18n.T("cli.kernel.restart.starting"))
	cmd := exec.Command("sc", "start", "MindX")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(i18n.TWithData("cli.kernel.restart.failed", map[string]interface{}{"Error": fmt.Sprintf("%v\n%s", err, string(output))}))
		return
	}

	fmt.Println(i18n.T("cli.kernel.restart.restarted"))
}

func getPlistPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return homeDir + "/Library/LaunchAgents/com.mindx.service.plist"
}

func checkPlistExists() bool {
	path := getPlistPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func generatePlistContent() string {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "/usr/local/bin/mindx"
	}

	homeDir, _ := os.UserHomeDir()
	workspace := os.Getenv("MINDX_WORKSPACE")
	if workspace == "" {
		workspace = homeDir + "/.mindx"
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.mindx.service</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>kernel</string>
    <string>run</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>MINDX_WORKSPACE</key>
    <string>%s</string>
  </dict>
  <key>StandardOutPath</key>
  <string>/tmp/mindx.out.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/mindx.err.log</string>
</dict>
</plist>
`, execPath, workspace, workspace)
}

func createPlistFile() error {
	path := getPlistPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	content := generatePlistContent()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	return nil
}

func loadPlist() error {
	path := getPlistPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	cmd := exec.Command("launchctl", "load", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load plist: %w\n%s", err, string(output))
	}

	return nil
}

func ensurePlistLoaded() error {
	if !checkPlistExists() {
		fmt.Println(i18n.T("cli.kernel.plist.not_exists"))
		if err := createPlistFile(); err != nil {
			return err
		}
		fmt.Println(i18n.TWithData("cli.kernel.plist.created", map[string]interface{}{"Path": getPlistPath()}))

		fmt.Println(i18n.T("cli.kernel.plist.loading"))
		if err := loadPlist(); err != nil {
			return err
		}
		fmt.Println(i18n.T("cli.kernel.plist.loaded"))
	}
	return nil
}
