package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

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
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel)

	// Phase 1: Clean up any stale registration.
	// A successful bootout means launchd had a ghost entry; wait for it to fully release.
	if out, err := exec.Command("launchctl", "bootout", service).CombinedOutput(); err == nil {
		time.Sleep(300 * time.Millisecond)
	} else {
		// bootout failed — service might not be registered at all, which is fine.
		// But if the output hints at a stale/ghost state (e.g. crash-loop), force-unload.
		outStr := string(out)
		if strings.Contains(outStr, "Could not find service") ||
			strings.Contains(outStr, "not found") {
			// Truly not registered — proceed to bootstrap directly.
		} else {
			// Ghost/stale state: try forced removal via unload (legacy API) as fallback.
			_ = exec.Command("launchctl", "unload", agentPlist).Run()
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Phase 2: Attempt bootstrap.
	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), agentPlist)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil // Success on first try.
	}

	outStr := string(out)

	// Phase 3: Recovery — macOS launchctl bootstrap commonly fails with
	// exit status 5 / "Input/output error" when launchd has an inconsistent
	// internal state for this label (stale pidfile, ghost registration, etc.).
	// Aggressive cleanup + retry usually resolves it.
	isBootstrapIOErr := strings.Contains(outStr, "Input/output error") ||
		strings.Contains(outStr, "exit status 5") ||
		strings.Contains(outStr, "already bootstrapped") ||
		strings.Contains(outStr, "service already loaded")

	if isBootstrapIOErr {
		_ = exec.Command("launchctl", "bootout", service).Run()
		time.Sleep(500 * time.Millisecond)

		retryCmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), agentPlist)
		if _, retryErr := retryCmd.CombinedOutput(); retryErr == nil {
			return nil
		}

		_ = exec.Command("launchctl", "unload", agentPlist).Run()
		time.Sleep(500 * time.Millisecond)

		finalCmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), agentPlist)
		if _, finalErr := finalCmd.CombinedOutput(); finalErr == nil {
			return nil
		}

		// All launchctl attempts exhausted. Fall through to direct-process fallback.
		fmt.Fprintf(os.Stderr, "⚠  launchctl bootstrap failed after 3 attempts, falling back to direct process start\n")
	}

	// Phase 4: Fallback — start daemon directly when launchctl is unavailable.
	return startDaemonDirect(workspaceDir)
}

// startDaemonDirect launches the mindx daemon as an OS-level background process,
// bypassing launchctl entirely. Used as fallback when launchctl bootstrap
// fails repeatedly (e.g. sandbox, restricted environment, or persistent launchd I/O errors).
//
// The daemon runs as a detached child process — it won't be managed by launchd
// (no auto-restart on crash, no login auto-start), but it will be functional.
func startDaemonDirect(workspaceDir string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	logDir := filepath.Join(workspaceDir, "logs")
_ = os.MkdirAll(logDir, 0755)

	cmd := exec.Command(exePath, "daemon")
	cmd.Env = append(os.Environ(), "MINDX_WORKSPACE="+workspaceDir)
	cmd.Dir = workspaceDir
	setDetachAttrs(cmd)

	stdoutF, _ := os.OpenFile(filepath.Join(logDir, "daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	stderrF, _ := os.OpenFile(filepath.Join(logDir, "daemon.err.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
	}

	// Detach: release the process so it outlives the parent.
	_ = cmd.Process.Release()

	// Brief pause then verify the child is still alive.
	time.Sleep(300 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return fmt.Errorf("daemon process exited immediately; check %s for details",
			filepath.Join(logDir, "daemon.err.log"))
	}

	fmt.Fprintln(os.Stderr, "  Daemon started via direct process (not launchd-managed).")

	// Attempt to repair launchd registration in background so that
	// future boots / restarts use the normal managed path.
	go func() {
		time.Sleep(2 * time.Second) // give the direct process time to settle
		if err := setupDaemonMacOS(workspaceDir); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠  Could not repair launchd registration: %v\n", err)
			fmt.Fprintln(os.Stderr, "     Run 'mindx install' to fix manually.")
			return
		}
		fmt.Fprintln(os.Stderr, "  ✅ Launchd registration repaired — daemon will auto-start on next login.")
	}()

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
		_ = cfg
	}

	// Primary check: launchd-managed service.
	cmd := exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel))
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "state = running") {
		return DaemonRunning, nil
	}

	// Fallback check: daemon may have been started directly (bypassing launchd).
	// Look for a running process matching our executable + "daemon" argument.
	exePath, _ := os.Executable()
	pgrep := exec.Command("pgrep", "-f", exePath+" daemon")
	if pgrepOut, pgrepErr := pgrep.CombinedOutput(); pgrepErr == nil {
		if strings.TrimSpace(string(pgrepOut)) != "" {
			return DaemonRunning, nil
		}
	}

	return DaemonStopped, nil
}

func stopDaemonMacOS(workspaceDir string) error {
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel)

	// Primary: try launchd-managed shutdown.
	cmd := exec.Command("launchctl", "bootout", service)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := string(out)

	// If launchctl says "no such process", the daemon may have been
	// started via direct-process fallback. Fall back to SIGTERM.
	isNoSuchProcess := strings.Contains(outStr, "No such process") ||
		strings.Contains(outStr, "exit status 3") ||
		strings.Contains(outStr, "not found")

	if isNoSuchProcess {
		return stopDaemonDirect()
	}

	return fmt.Errorf("launchctl bootout: %w\n%s", err, outStr)
}

// stopDaemonDirect terminates a daemon that was started outside of launchd
// (e.g. via startDaemonDirect fallback). Sends SIGTERM first, then
// SIGKILL after a grace period if the process hasn't exited.
func stopDaemonDirect() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	pgrep := exec.Command("pgrep", "-f", exePath+" daemon")
	pids, pgrepErr := pgrep.CombinedOutput()
	if pgrepErr != nil || strings.TrimSpace(string(pids)) == "" {
		fmt.Fprintln(os.Stderr, "  No running daemon process found.")
		return nil // Already stopped — not an error.
	}

	// Parse PIDs and send SIGTERM to each.
	pidStrs := strings.Fields(strings.TrimSpace(string(pids)))
	for _, pidStr := range pidStrs {
		pid := parseIntSafe(pidStr)
		if pid <= 0 {
			continue
		}
		if proc, perr := os.FindProcess(pid); perr == nil {
			_ = proc.Signal(syscall.SIGTERM)
			fmt.Fprintf(os.Stderr, "  Sent SIGTERM to daemon PID %d\n", pid)
		}
	}

	// Wait up to 3 seconds for graceful exit.
	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		recheck := exec.Command("pgrep", "-f", exePath+" daemon")
		if out, _ := recheck.CombinedOutput(); strings.TrimSpace(string(out)) == "" {
			fmt.Fprintln(os.Stderr, "  Daemon stopped gracefully.")
			return nil
		}
	}

	// Force kill if still alive.
	for _, pidStr := range pidStrs {
		pid := parseIntSafe(pidStr)
		if pid <= 0 {
			continue
		}
		if proc, perr := os.FindProcess(pid); perr == nil {
			_ = proc.Signal(syscall.SIGKILL)
			fmt.Fprintf(os.Stderr, "  Sent SIGKILL to daemon PID %d\n", pid)
		}
	}

	fmt.Fprintln(os.Stderr, "  Daemon force-stopped.")
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

// parseIntSafe parses a string as an integer. Returns 0 on failure (never panics).
func parseIntSafe(s string) int {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// isRunningTask checks if the MindXDaemon scheduled task is currently running.
	//nolint:unused
func _isRunningTask() (bool, error) {
	cmd := exec.Command("schtasks", "/query", "/tn", "MindXDaemon", "/fo", "CSV", "/nh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "Running"), nil
}
