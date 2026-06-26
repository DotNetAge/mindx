package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// UninstallOptions controls which steps of the uninstallation to perform.
type UninstallOptions struct {
	// SkipDaemon skips daemon/service unregistration and stop.
	SkipDaemon bool
	// SkipPath skips PATH cleanup.
	SkipPath bool
	// SkipShortcut skips desktop shortcut removal.
	SkipShortcut bool
	// KeepBinary keeps the installed binary (only cleans up integrations).
	KeepBinary bool
}

// UninstallResult reports what was done during uninstallation.
type UninstallResult struct {
	DaemonRemoved   bool          // whether daemon service was unregistered
	PathCleaned     bool          // whether PATH entry was removed
	ShortcutRemoved bool          // whether desktop shortcut was deleted
	BinaryRemoved   bool          // whether installed binary was deleted
	Source          InstallSource // detected install source (for informational output)
}

// Uninstall performs a full platform-aware uninstallation of MindX,
// reversing the steps performed by Install().
//
// Steps (in order):
//  1. Stop the running daemon
//  2. Unregister the daemon service (launchd/systemd/schtasks)
//  3. Remove install directory from system PATH
//  4. Remove desktop shortcut (Windows only)
//  5. Delete installed binary (unless KeepBinary is set)
func Uninstall(opts UninstallOptions) (*UninstallResult, error) {
	result := &UninstallResult{}

	// Detect install source to guide cleanup decisions
	exePath, exeErr := os.Executable()
	if exeErr == nil {
		result.Source = DetectInstallSource(exePath)
	}

	installDir, err := resolveInstallDir("")
	if err != nil {
		return nil, fmt.Errorf("resolve install dir: %w", err)
	}

	// Step 1 (mandatory): Stop daemon before any cleanup.
	// The daemon must not be running when we remove its binary or service registration.
	if !opts.SkipDaemon {
		if err := StopDaemon(); err != nil {
			// Check if daemon was simply never installed — that's fine, continue
			status, _ := CheckDaemon()
			if status == DaemonNotInstalled {
				fmt.Println("  Daemon: not installed (nothing to stop)")
			} else {
				return nil, fmt.Errorf("failed to stop daemon (must be stopped before uninstall): %w", err)
			}
		} else {
			fmt.Println("  Daemon stopped")
		}
	}

	// Step 2: Unregister daemon service (remove plist/service file/schtask)
	if !opts.SkipDaemon {
		removed, unregErr := UnregisterDaemon()
		if unregErr != nil {
			return nil, fmt.Errorf("daemon unregister failed: %w", unregErr)
		}
		result.DaemonRemoved = removed
		if removed {
			fmt.Println("  Daemon service unregistered")
		} else {
			fmt.Println("  No daemon service found (already clean)")
		}
	}

	// Step 3: Clean up PATH
	if !opts.SkipPath {
		cleaned, pathErr := RemoveFromSystemPath(installDir)
		if pathErr != nil {
			fmt.Printf("  PATH cleanup: %v\n", pathErr)
		} else {
			result.PathCleaned = cleaned
			if cleaned {
				fmt.Println("  System PATH cleaned")
			} else {
				fmt.Println("  Not in system PATH")
			}
		}
	}

	// Step 4: Remove desktop shortcut (Windows only)
	if !opts.SkipShortcut && runtime.GOOS == "windows" {
		removed, scErr := RemoveDesktopShortcut()
		if scErr != nil {
			fmt.Printf("  Desktop shortcut removal: %v\n", scErr)
		} else {
			result.ShortcutRemoved = removed
			if removed {
				fmt.Println("  Desktop shortcut removed")
			}
		}
	}

	// Step 5: Delete installed binary
	// Skip if the binary is from a package manager (managed) — use `brew uninstall` etc.
	shouldRemoveBinary := !opts.KeepBinary && result.Source == SourceCustom
	if result.Source == SourceManaged {
		shouldRemoveBinary = false
		fmt.Println("  Binary: skipping removal (package manager owned)")
		fmt.Println("    Use 'brew uninstall mindx' (or your package manager) to remove the binary.")
	}
	if shouldRemoveBinary {
		if exeErr == nil {
			destExe := filepath.Join(installDir, filepath.Base(exePath))
			if _, statErr := os.Stat(destExe); statErr == nil {
				if rmErr := os.Remove(destExe); rmErr != nil {
					fmt.Printf("  Binary removal: %v\n", rmErr)
				} else {
					result.BinaryRemoved = true
					fmt.Printf("  Binary removed from %s\n", installDir)

					// Also remove VBS launcher on Windows
					if runtime.GOOS == "windows" {
						vbsPath := filepath.Join(installDir, "MindXDaemon.vbs")
						_ = os.Remove(vbsPath)
					}

					// Try to remove empty install directory
					if entries, dirErr := os.ReadDir(installDir); dirErr == nil && len(entries) == 0 {
						_ = os.Remove(installDir)
					}
				}
			} else {
				fmt.Printf("  No binary found at %s (already removed?)\n", destExe)
			}
		}
	}

	return result, nil
}

// UnregisterDaemon removes the platform-specific daemon service registration.
// Returns (removed, error) where removed indicates something was actually cleaned up.
func UnregisterDaemon() (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return unregisterDaemonMacOS()
	case "linux":
		return unregisterDaemonLinux()
	case "windows":
		return unregisterDaemonWindows()
	default:
		return false, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func unregisterDaemonMacOS() (bool, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return false, fmt.Errorf("cannot determine home directory")
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mindx.daemon.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return false, nil // Nothing to do
	}

	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), macosLaunchdLabel)

	// bootout stops + unloads
	cmd := exec.Command("launchctl", "bootout", service)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		errStr := err.Error()
		// exit status 3/5 means not loaded — still safe to delete plist
		if strings.Contains(outStr, "Could not find service") ||
			strings.Contains(outStr, "not found") ||
			strings.Contains(outStr, "No such process") ||
			strings.Contains(errStr, "exit status 3") ||
			strings.Contains(errStr, "exit status 5") {
			// Fall through to delete plist
		} else {
			return false, fmt.Errorf("launchctl bootout: %w\n%s", err, outStr)
		}
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove plist: %w", err)
	}

	return true, nil
}

func unregisterDaemonLinux() (bool, error) {
	serviceName := linuxServiceName + ".service"
	var cleaned bool

	// Disable the service (prevents auto-start on next login)
	if err := exec.Command("systemctl", "--user", "disable", serviceName).Run(); err == nil {
		cleaned = true
	}

	// Remove unit file
	home, _ := os.UserHomeDir()
	servicePath := filepath.Join(home, ".config", "systemd", "user", serviceName)
	if _, err := os.Stat(servicePath); err == nil {
		if rmErr := os.Remove(servicePath); rmErr == nil {
			cleaned = true
		}
	}

	// Reload systemd to pick up removed unit
	exec.Command("systemctl", "--user", "daemon-reload")

	return cleaned, nil
}

func unregisterDaemonWindows() (bool, error) {
	taskName := "MindXDaemon"

	// schtasks /delete removes the scheduled task entirely
	cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		// "The specified task name does not exist" means already gone
		if strings.Contains(strings.ToLower(outStr), "does not exist") {
			return false, nil
		}
		return false, fmt.Errorf("schtasks delete: %w\n%s", err, decodeWindowsOutput(out))
	}

	// Remove VBS launcher
	home, _ := os.UserHomeDir()
	vbsPath := filepath.Join(home, ".mindx", "bin", "MindXDaemon.vbs")
	_ = os.Remove(vbsPath)

	return true, nil
}

// RemoveFromSystemPath removes the given directory from the platform-appropriate system PATH.
// Returns (cleaned, error) where cleaned indicates the entry existed and was removed.
func RemoveFromSystemPath(dir string) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		return removeWindowsPath(dir)
	default:
		return removeUnixPath(dir)
	}
}

func removeUnixPath(dir string) (bool, error) {
	rcFile := detectShellRC()
	if rcFile == "" {
		return false, fmt.Errorf("cannot determine shell rc file")
	}

	data, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read shell rc: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	var newLines []string
	inMindXBlock := false
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect start of MindX block
		if trimmed == "# MindX" {
			inMindXBlock = true
			continue
		}

		if inMindXBlock {
			// End of block: next non-empty line that's not a PATH export or comment
			if trimmed == "" || strings.HasPrefix(trimmed, "export PATH=") || strings.HasPrefix(trimmed, "#") {
				// Still part of block
				if strings.Contains(line, "mindx") || strings.Contains(line, dir) {
					found = true
				}
				continue
			}
			inMindXBlock = false
		}

		// Also catch standalone mindx PATH lines outside blocks
		if strings.Contains(line, "mindx") && strings.Contains(line, "PATH") {
			found = true
			continue
		}

		newLines = append(newLines, line)
	}

	if !found {
		return false, nil
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(rcFile, []byte(newContent), 0644); err != nil {
		return false, fmt.Errorf("write shell rc: %w", err)
	}

	return true, nil
}

func removeWindowsPath(dir string) (bool, error) {
	// Read current User PATH via registry query
	queryCmd := exec.Command("reg", "query", `HKCU\Environment`, "/v", "PATH")
	queryOut, queryErr := queryCmd.CombinedOutput()

	var currentValue string
	if queryErr == nil {
		lines := strings.Split(string(queryOut), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PATH ") || strings.HasPrefix(line, "PATH\t") {
				parts := strings.SplitN(line, "    ", 3)
				if len(parts) >= 3 {
					currentValue = parts[2]
				} else if len(parts) == 2 {
					parts2 := strings.SplitN(parts[1], "\t", 3)
					if len(parts2) >= 2 {
						currentValue = parts2[1]
					}
				}
				break
			}
		}
	}

	if currentValue == "" {
		return false, nil
	}

	// Check if our dir is actually in there
	found := false
	var newEntries []string
	for _, p := range splitPath(currentValue) {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" && !strings.EqualFold(trimmed, dir) {
			newEntries = append(newEntries, trimmed)
		} else if trimmed != "" {
			found = true
		}
	}

	if !found {
		return false, nil
	}

	newPath := strings.Join(newEntries, ";")

	// Write back via setx
	setxCmd := exec.Command("setx", "PATH", newPath)
	out, err := setxCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("setx: %w\n%s", err, decodeWindowsOutput(out))
	}

	return true, nil
}

// RemoveDesktopShortcut deletes the MindX desktop shortcut (.lnk).
// Only meaningful on Windows; on other platforms returns (false, nil).
func RemoveDesktopShortcut() (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("get home dir: %w", err)
	}

	lnkPath := filepath.Join(home, "Desktop", "MindX.lnk")
	if _, err := os.Stat(lnkPath); os.IsNotExist(err) {
		return false, nil
	}

	if err := os.Remove(lnkPath); err != nil {
		return false, fmt.Errorf("remove shortcut: %w", err)
	}

	return true, nil
}
