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

// ── Windows: User PATH via setx (no admin needed) + System PATH via reg add ──

func addWindowsPath(dir string) (bool, error) {
	if CheckInPath(dir) {
		return true, nil
	}

	// Method 1: setx — writes to User PATH (HKCU\Environment), no admin required.
	// setx appends to existing value; limit is 1024 chars which is fine for our use case.
	if ok, err := addWindowsPathSetx(dir); err == nil {
		return ok, nil
	}

	// Method 2: reg add to User PATH (alternative to setx)
	if ok, err := addWindowsPathReg(dir, "HKCU", `Environment`, "PATH"); err == nil {
		return ok, nil
	}

	// Method 3: reg add to System PATH (requires elevated/admin)
	ok, err := addWindowsPathReg(dir, "HKLM", `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, "PATH")
	return ok, err
}

func addWindowsPathSetx(dir string) (bool, error) {
	// Read current user PATH first to avoid duplicates
	currentPath := os.Getenv("PATH")
	for _, p := range splitPath(currentPath) {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(dir)) {
			return true, nil
		}
	}

	// Build new PATH value
	newEntries := []string{}
	for _, p := range splitPath(currentPath) {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" && trimmed != dir {
			newEntries = append(newEntries, trimmed)
		}
	}
	newEntries = append(newEntries, dir)
	newPath := strings.Join(newEntries, ";")

	// Use setx to write User PATH (works without admin)
	cmd := exec.Command("setx", "PATH", newPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("setx: %w\n%s", err, decodeWindowsOutput(out))
	}
	return false, nil // newly added
}

func addWindowsPathReg(dir string, hive, subkey, valueName string) (bool, error) {
	// Read current value via reg query
	queryCmd := exec.Command("reg", "query", hive+`\`+subkey, "/v", valueName)
	queryOut, queryErr := queryCmd.CombinedOutput()

	var currentValue string
	if queryErr == nil {
		// Parse "    PATH    REG_EXPAND_SZ    <value>" format
		lines := strings.Split(string(queryOut), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, valueName+" ") || strings.HasPrefix(line, valueName+"\t") {
				// Extract after REG_xxx_SZ
				parts := strings.SplitN(line, "    ", 3) // name + type + value
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

	// Check duplicate
	for _, p := range splitPath(currentValue) {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(dir)) {
			return true, nil
		}
	}

	// Append and write back
	newVal := strings.TrimSpace(currentValue)
	if newVal != "" && !strings.HasSuffix(newVal, ";") {
		newVal += ";"
	}
	newVal += dir

	regKey := hive + `\` + subkey
	addCmd := exec.Command("reg", "add", regKey, "/v", valueName, "/t", "REG_EXPAND_SZ", "/d", newVal, "/f")
	out, err := addCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("reg add (%s): %w\n%s", hive, err, decodeWindowsOutput(out))
	}
	return false, nil // newly added
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
	defer func() { _ = f.Close() }()

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
