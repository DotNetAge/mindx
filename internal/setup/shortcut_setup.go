package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CreateDesktopShortcut creates a desktop shortcut (.lnk) for the given executable.
// Only meaningful on Windows; on other platforms it returns (false, nil).
func CreateDesktopShortcut(exePath string) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}

	desktop, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("get home dir: %w", err)
	}
	desktop = filepath.Join(desktop, "Desktop")

	if err := os.MkdirAll(desktop, 0755); err != nil {
		return false, fmt.Errorf("create Desktop dir: %w", err)
	}

	lnkPath := filepath.Join(desktop, "MindX.lnk")

	// Use PowerShell COM object to create a proper .lnk file.
	// This is more reliable than writing raw .lnk binary format.
	script := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell
$sc = $ws.CreateShortcut('%s')
$sc.TargetPath = '%s'
$sc.WorkingDirectory = '%s'
$sc.Description = 'MindX - AI Agent Platform'
$sc.Save()
Write-Output 'OK'`,
		strings.ReplaceAll(lnkPath, "'", "''"),
		strings.ReplaceAll(exePath, "'", "''"),
		strings.ReplaceAll(filepath.Dir(exePath), "'", "''"),
	)

	tmpScript := filepath.Join(os.TempDir(), "mindx_shortcut.ps1")
	if err := os.WriteFile(tmpScript, []byte(script), 0644); err != nil {
		return false, fmt.Errorf("write shortcut script: %w", err)
	}
	defer func() { _ = os.Remove(tmpScript) }()

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("powershell: %w\n%s", err, decodeWindowsOutput(out))
	}

	output := strings.TrimSpace(string(out))
	if output == "OK" {
		// Check if file actually exists
		if _, err := os.Stat(lnkPath); err == nil {
			return true, nil
		}
	}
	return false, fmt.Errorf("shortcut creation failed (output: %s)", decodeWindowsOutput(out))
}
