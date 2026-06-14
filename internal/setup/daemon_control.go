package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
)

// DaemonStatus represents the current state of the MindX daemon service.
type DaemonStatus int

const (
	DaemonUnknown      DaemonStatus = iota // Could not determine status
	DaemonRunning                          // Service is active and listening
	DaemonStopped                          // Service is registered but not running
	DaemonNotInstalled                     // No daemon registration found
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

// StartDaemon starts the registered daemon service.
// Returns an error if the daemon is not installed or cannot be started.
func StartDaemon() error {
	workspaceDir := core.DefaultUserPrefsDir()
	switch runtime.GOOS {
	case "darwin":
		return startDaemonMacOS(workspaceDir)
	case "linux":
		return startDaemonLinux()
	case "windows":
		return startDaemonWindows()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
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

const macosLaunchdLabel = "com.mindx.daemon"

func startDaemonMacOS(workspaceDir string) error {
	agentPlist := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com.mindx.daemon.plist")
	if _, err := os.Stat(agentPlist); os.IsNotExist(err) {
		return fmt.Errorf("daemon plist not found at %s — run 'mindx install' first", agentPlist)
	}
	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), agentPlist)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w\n%s", err, string(out))
	}
	return nil
}

func startDaemonLinux() error {
	cmd := exec.Command("systemctl", "--user", "start", linuxServiceName+".service")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl start: %w\n%s", err, string(out))
	}
	return nil
}

func startDaemonWindows() error {
	cmd := exec.Command("schtasks", "/run", "/tn", "MindXDaemon")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks run: %w\n%s", err, decodeWindowsOutput(out))
	}
	return nil
}

func checkDaemonMacOS(workspaceDir string) (DaemonStatus, error) {
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil || !cfg.Daemon.Installed {
		// 即使配置标记未安装，也通过 launchctl 实际检查（plist 可能已被手动安装）
	}

	cmd := exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return DaemonStopped, nil
	}
	if strings.Contains(string(out), "state = running") {
		return DaemonRunning, nil
	}
	return DaemonStopped, nil
}

func stopDaemonMacOS(workspaceDir string) error {
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel)
	cmd := exec.Command("launchctl", "bootout", service)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootout: %w\n%s", err, string(out))
	}
	return nil
}

// ── Linux ────────────────────────────────────────────────────────────────────

const linuxServiceName = "mindx"

func checkDaemonLinux() (DaemonStatus, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", linuxServiceName+".service")
	out, err := cmd.CombinedOutput()
	if err != nil {
		check := exec.Command("systemctl", "--user", "list-unit-files", linuxServiceName+".service")
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
	cmd := exec.Command("systemctl", "--user", "stop", linuxServiceName+".service")
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
