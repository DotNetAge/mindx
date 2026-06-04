package setup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// AddToSystemPath adds the given directory to the platform-appropriate system PATH.
// Returns (alreadyExists, error).
//
// Platform behavior:
//   - Windows: writes to System PATH (requires admin). Falls back to User PATH.
//   - macOS/Linux: appends to the user's shell rc file (~/.zshrc or ~/.bashrc).
func AddToSystemPath(dir string) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		return addWindowsPath(dir)
	default:
		return addUnixPath(dir)
	}
}

// CheckInPath checks whether dir is already in the effective system PATH.
func CheckInPath(dir string) bool {
	pathVar := os.Getenv("PATH")
	for _, p := range splitPath(pathVar) {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(dir)) {
			return true
		}
	}
	return false
}

// ── Windows: System PATH via registry + fallback to User PATH ──────────────

func addWindowsPath(dir string) (bool, error) {
	if CheckInPath(dir) {
		return true, nil
	}

	// Try System PATH first (requires admin / elevated process)
	if ok, err := setRegistryPath(dir, "HKLM", `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, "PATH"); err == nil {
		return ok, nil
	}

	// Fallback: User PATH
	ok, err := setRegistryPath(dir, "HKCU", `Environment`, "PATH")
	return ok, err
}

func setRegistryPath(dir string, hive, subkey, valueName string) (bool, error) {
	// Use PowerShell to read current value, check if already present, then write back.
	// This avoids Go's golang.org/x/sys/windows dependency and handles quoting reliably.
	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$dir = '%s'
$hive = '%s'
$subKey = '%s'
$valName = '%s'

# Read current PATH
try {
    $regPath = "%s::%s"
    $current = [Microsoft.Win32.Registry]::GetValue($regPath, $valName, $null)
} catch {
    Write-Output "REG_READ_FAIL"; exit 1
}

if ($null -eq $current) { $current = "" }

# Check if already present
foreach ($p in ($current -split ';')) { if ($p.Trim() -eq $dir.Trim()) { Write-Output "ALREADY_EXISTS"; exit 0 } }

# Append and write back
$newVal = $current.TrimEnd(';') + ';' + $dir
[Microsoft.Win32.Registry]::SetValue($regPath, $valName, $newVal, 'ExpandString')
Write-Output "OK"`,
		strings.ReplaceAll(dir, "'", "''"),
		hive,
		subkey,
		valueName,
		hive,
		subkey,
	)

	tmpScript := filepathJoin(os.TempDir(), "mindx_path_setup.ps1")
	if err := os.WriteFile(tmpScript, []byte(script), 0644); err != nil {
		return false, fmt.Errorf("write script: %w", err)
	}
	defer os.Remove(tmpScript)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpScript)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return false, fmt.Errorf("powershell (%s): %w\n%s", hive, err, decodeWindowsOutput(out))
	}

	switch output {
	case "ALREADY_EXISTS":
		return true, nil
	case "OK":
		return false, nil // newly added
	default:
		return false, fmt.Errorf("unexpected output: %s", decodeWindowsOutput(out))
	}
}

// filepathJoin is a local helper to avoid importing path/filepath in this file.
func filepathJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathSeparator))
}

// ── Unix: shell rc file injection ───────────────────────────────────────────

func addUnixPath(dir string) (bool, error) {
	if CheckInPath(dir) {
		return true, nil
	}

	rcFile := detectShellRC()
	if rcFile == "" {
		return false, fmt.Errorf("cannot determine shell rc file")
	}

	line := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)

	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read shell rc: %w", err)
	}

	content := string(data)
	// Avoid duplicates
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line || strings.Contains(l, "mindx") && strings.Contains(l, "PATH") {
			return true, nil
		}
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("open shell rc: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "# MindX")
	fmt.Fprintln(f, line)
	return false, nil
}

// detectShellRC returns the user's active shell rc file path.
func detectShellRC() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "zsh"):
		return filepathJoin(home, ".zshrc")
	case strings.HasSuffix(shell, "bash"):
		return filepathJoin(home, ".bashrc")
	default:
		// Fallback: prefer .zshrc on macOS, .bashrc elsewhere
		if runtime.GOOS == "darwin" {
			rc := filepathJoin(home, ".zshrc")
			if _, err := os.Stat(rc); err == nil {
				return rc
			}
		}
		return filepathJoin(home, ".bash_profile")
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func splitPath(pathVar string) []string {
	sep := ";"
	if runtime.GOOS != "windows" {
		sep = ":"
	}
	return strings.Split(pathVar, sep)
}
