package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
		// Verify daemon actually stopped and wait for port release.
		// Without this, StartDaemon() may fail with "address already in use"
		// because the OS hasn't fully released the port yet (TIME_WAIT, etc.).
		if err := verifyDaemonStopped(); err != nil {
			return fmt.Errorf("stop verification: %w", err)
		}
	case setup.DaemonStopped:
		fmt.Println("  → Daemon is not running, starting directly...")
	}

	if err := setup.StartDaemon(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	return verifyDaemonStarted()
}

// verifyDaemonStarted checks whether the daemon is actually alive after
// StartDaemon() returned success. Service managers (launchctl, systemd,
// schtasks) may report success even when the child process crashes
// immediately due to port conflicts or configuration errors.
func verifyDaemonStarted() error {
	time.Sleep(500 * time.Millisecond)

	status, _ := setup.CheckDaemon()
	if status == setup.DaemonRunning {
		fmt.Println("✅ Daemon started.")
		return nil
	}

	fmt.Println("❌ Daemon failed to start or exited immediately.")

	if portConflict := detectPortConflict(); portConflict != "" {
		fmt.Println()
		fmt.Println("  🔴 Port conflict detected:")
		fmt.Printf("    %s\n", portConflict)
		fmt.Println()
		fmt.Println("  To fix:")
		fmt.Println("    1. Stop the process using the port:")
		fmt.Println("       lsof -i :1313 -i :1314")
		fmt.Println("    2. Kill it: kill <PID>")
		fmt.Println("    3. Then retry: mindx start")
	} else {
		fmt.Println()
		fmt.Println("  Possible causes:")
		fmt.Println("    • Configuration error — try 'mindx doctor --fix'")
		fmt.Println("    • Check logs for details:")
		fmt.Println("      ~/.mindx/logs/daemon.log")
		fmt.Println("      ~/.mindx/logs/daemon.err.log")
	}

	return fmt.Errorf("daemon started but is not running (status=%s)", status)
}

// detectPortConflict checks if ports 1313/1314 are already in use
// and returns a human-readable description of the conflict.
// Uses platform-appropriate tools: lsof (macOS/Linux), netstat (Windows).
func detectPortConflict() string {
	var conflicts []string

	for _, port := range []string{":1313", ":1314"} {
		out, err := runPortCheck(port)
		if err != nil || len(out) == 0 {
			continue
		}
		info := parsePortOutput(string(out), port)
		if info != "" {
			conflicts = append(conflicts, info)
		}
	}

	if len(conflicts) > 0 {
		return strings.Join(conflicts, "\n    ")
	}
	return ""
}

// runPortCheck runs the platform-appropriate command to check a port's usage.
func runPortCheck(port string) ([]byte, error) {
	portNum := strings.TrimPrefix(port, ":")
	switch runtime.GOOS {
	case "windows":
		return exec.Command("netstat", "-ano").CombinedOutput()
	default:
		// macOS / Linux: try lsof first, fall back to ss (Linux) then netstat
		if out, err := exec.Command("lsof", "-i", port).CombinedOutput(); err == nil {
			return out, nil
		}
		if out, err := exec.Command("ss", "-tlnp", "sport", "=:", portNum).CombinedOutput(); err == nil {
			return out, nil
		}
		return exec.Command("netstat", "-tlnp").CombinedOutput()
	}
}

// parsePortOutput parses port-check tool output into a human-readable string.
// Each platform's output format is handled separately.
func parsePortOutput(output, port string) string {
	portNum := strings.TrimPrefix(port, ":")
	lines := strings.Split(strings.TrimSpace(output), "\n")

	switch runtime.GOOS {
	case "windows":
		// netstat -ano format:   TCP    0.0.0.0:1314    0.0.0.0:0    LISTENING    57714
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 5 || fields[1] != "LISTENING" {
				continue
			}
			addr := fields[1] // e.g. "0.0.0.0:1314" or "[::]:1314"
			if !strings.HasSuffix(addr, ":"+portNum) && !strings.Contains(addr, ":"+portNum+" ") {
				continue
			}
			return fmt.Sprintf("Port %s is held by PID %s", port, fields[len(fields)-1])
		}

	default:
		// lsof format (macOS/Linux):
		//   COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
		//   mindx   57714 ray   23u  IPv6  ... TCP *:bmc_patroldb (LISTEN)
		if len(lines) >= 2 {
			for i := 1; i < len(lines); i++ {
				fields := strings.Fields(lines[i])
				if len(fields) >= 2 {
					return fmt.Sprintf("Port %s is held by PID %s (%s)", port, fields[1], fields[0])
				}
			}
		}

		// ss/netstat format (Linux fallback):
		//   tcp  LISTEN 0 128  *:1314  users:(("mindx",pid=57714,fd=23))
		for _, line := range lines {
			if !strings.Contains(line, ":"+portNum) {
				continue
			}
			if idx := strings.Index(line, "pid="); idx >= 0 {
				pidStr := line[idx+4:]
				if endIdx := strings.IndexAny(pidStr, ",)"); endIdx > 0 {
					pidStr = pidStr[:endIdx]
				}
				return fmt.Sprintf("Port %s is held by PID %s", port, pidStr)
			}
		}
	}

	return ""
}

// verifyDaemonStopped checks whether the daemon has actually stopped after
// StopDaemon() returned success. Polls with backoff up to 5 seconds,
// also checking for lingering port holders (e.g., direct-process fallback
// that launchd cannot manage).
func verifyDaemonStopped() error {
	const maxWait = 5 * time.Second
	const interval = 500 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		status, _ := setup.CheckDaemon()
		if status == setup.DaemonNotInstalled || status == setup.DaemonStopped {
			fmt.Println("✅ Daemon stopped.")
			return nil
		}
		time.Sleep(interval)
		elapsed += interval
	}

	fmt.Println("⚠️  Daemon may not have fully stopped.")
	if portConflict := detectPortConflict(); portConflict != "" {
		fmt.Println("  Port still in use:")
		fmt.Printf("    %s\n", portConflict)
		fmt.Println("  The process holding the port may not be managed by launchd.")
		fmt.Println("  Try: lsof -i :1313 -i :1314  then  kill <PID>")
	} else {
		fmt.Println("  Check: mindx status")
	}
	return fmt.Errorf("daemon stop could not be verified within %v", maxWait)
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
