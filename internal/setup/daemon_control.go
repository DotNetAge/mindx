package setup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
)

// DaemonStatus represents the current state of the MindX daemon service.
type DaemonStatus int

const (
	DaemonUnknown   DaemonStatus = iota // Could not determine status
	DaemonRunning                      // Service is active and listening
	DaemonStopped                       // Service is registered but not running
	DaemonNotInstalled                 // No daemon registration found
)

func (s DaemonStatus) String() string {
	switch s {
	case DaemonRunning:
		return "running"
	case DaemonStopped:
		return "stopped"
	case DaemonNotInstalled:
		return "not installed"
	default:
		return "unknown"
	}
}

// CheckDaemon returns the current daemon status.
func CheckDaemon() (DaemonStatus, error) {
	workspaceDir := core.DefaultUserPrefsDir()
	switch runtime.GOOS {
	case "darwin":
		return checkDaemonMacOS(workspaceDir)
	case "linux":
		return checkDaemonLinux()
	case "windows":
		return checkDaemonWindows()
	default:
		return DaemonUnknown, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// StopDaemon stops the currently running daemon service.
// Returns an error if the daemon cannot be stopped or was not installed.
func StopDaemon() error {
	workspaceDir := core.DefaultUserPrefsDir()
	switch runtime.GOOS {
	case "darwin":
		return stopDaemonMacOS(workspaceDir)
	case "linux":
		return stopDaemonLinux()
	case "windows":
		return stopDaemonWindows()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// ── macOS ───────────────────────────────────────────────────────────────────

func checkDaemonMacOS(workspaceDir string) (DaemonStatus, error) {
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil || !cfg.Daemon.Installed {
		return DaemonNotInstalled, nil
	}

	cmd := exec.Command("launchctl", "list", "com.mindx.daemon")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// launchctl list returns non-zero if service not loaded but plist exists → stopped
		return DaemonStopped, nil
	}
	// If PID field is present (not "-" or empty), it's running
	if len(out) > 0 && out[0] >= '0' && out[0] <= '9' {
		return DaemonRunning, nil
	}
	return DaemonStopped, nil
}

func stopDaemonMacOS(workspaceDir string) error {
	plistPath := fmt.Sprintf(
		"%s/Library/LaunchAgents/com.mindx.daemon.plist",
		os.Getenv("HOME"),
	)
	cmd := exec.Command("launchctl", "unload", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload: %w\n%s", err, string(out))
	}
	return nil
}

// ── Linux ────────────────────────────────────────────────────────────────────

func checkDaemonLinux() (DaemonStatus, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", "mindx-daemon.service")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Check if unit exists at all
		check := exec.Command("systemctl", "--user", "list-unit-files", "mindx-daemon.service")
		if _, checkErr := check.CombinedOutput(); checkErr != nil {
			return DaemonNotInstalled, nil
		}
		return DaemonStopped, nil
	}
	status := string(out)
	if status == "active\n" || status == "activating\n" {
		return DaemonRunning, nil
	}
	return DaemonStopped, nil
}

func stopDaemonLinux() error {
	cmd := exec.Command("systemctl", "--user", "stop", "mindx-daemon.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop: %w\n%s", err, string(out))
	}
	return nil
}

// ── Windows ──────────────────────────────────────────────────────────────────

func checkDaemonWindows() (DaemonStatus, error) {
	// Check via PowerShell Get-ScheduledTask which returns English state values.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`try { (Get-ScheduledTask -TaskName "MindXDaemon" -ErrorAction Stop).State } catch { Write-Output "NotFound" }`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return DaemonNotInstalled, nil
	}
	state := strings.TrimSpace(string(out))
	switch state {
	case "Running":
		return DaemonRunning, nil
	case "Ready", "Disabled":
		return DaemonStopped, nil
	default:
		return DaemonNotInstalled, nil
	}
}

func stopDaemonWindows() error {
	// schtasks /end only stops a currently running instance; /delete would remove it.
	// We want graceful stop, so try /end first.
	cmd := exec.Command("schtasks", "/end", "/tn", "MindXDaemon")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks end: %w\n%s", err, decodeWindowsOutput(out))
	}
	return nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// isRunningTask checks if the MindXDaemon scheduled task is currently running.
