package setup

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// AddToSystemPath adds the given directory to the user's shell PATH configuration
// (shell rc file). Returns (alreadyExists, error).
func AddToSystemPath(dir string) (bool, error) {
	return addUnixPath(dir)
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

	_, _ = fmt.Fprintln(f, "")
	_, _ = fmt.Fprintln(f, "# MindX")
	_, _ = fmt.Fprintln(f, line)
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
	return strings.Split(pathVar, ":")
}
