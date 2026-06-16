package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallSource describes where the current binary was installed from.
type InstallSource int

const (
	// SourceUnknown means the binary location could not be classified.
	SourceUnknown InstallSource = iota

	// SourceManaged means the binary is in a package-manager or system PATH
	// directory (e.g., Homebrew, apt, dnf, pacman). No copy is needed;
	// daemon should use the binary in-place.
	SourceManaged

	// SourceCustom means the binary was downloaded manually (e.g., from GitHub
	// releases) and needs to be copied to a stable install location.
	SourceCustom

	// SourceAlreadyInstalled means the binary is already in our target install
	// directory (e.g., ~/.mindx/bin). This is an in-place upgrade scenario.
	SourceAlreadyInstalled
)

func (s InstallSource) String() string {
	switch s {
	case SourceManaged:
		return "managed (package manager / system PATH)"
	case SourceCustom:
		return "custom (manual download)"
	case SourceAlreadyInstalled:
		return "already in target directory"
	default:
		return "unknown"
	}
}

// InstallOptions controls which steps of the installation to perform.
type InstallOptions struct {
	// InstallDir overrides the default platform-specific install directory.
	// Empty means use the platform default.
	InstallDir string

	// SkipDaemon skips daemon/service registration.
	SkipDaemon bool

	// SkipPath skips PATH configuration.
	SkipPath bool

	// SkipShortcut skips desktop shortcut creation (Windows only).
	SkipShortcut bool

	// ForceCopy forces copying even when the binary is in a managed location
	// (useful for testing or non-standard setups).
	ForceCopy bool
}

// InstallResult reports what was done during installation.
type InstallResult struct {
	BinaryDest      string        // where the binary ended up (or original path if managed)
	DaemonSetup     bool          // whether daemon was registered
	PathConfigured  bool          // whether PATH was updated
	ShortcutCreated bool          // whether desktop shortcut was created
	Source          InstallSource // detected install source
	SkippedCopy     bool          // whether binary copy was skipped (managed/in-place)
}

// Install performs a platform-aware installation of MindX.
//
// Behavior depends on where the current binary came from:
//
//   - **Managed** (Homebrew, apt, system PATH): Skips copy + PATH setup.
//     Only registers/starts the daemon service. The package manager owns the binary.
//
//   - **Custom** (GitHub release, manual download): Full install — copies binary
//     to stable location, adds to PATH, registers daemon.
//
//   - **Already installed** (in target dir): In-place upgrade — no copy needed,
//     re-registers daemon with any config changes.
func Install(opts InstallOptions) (*InstallResult, error) {
	result := &InstallResult{}

	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}

	// Detect where this binary came from
	source := detectInstallSource(exePath)
	result.Source = source

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, fmt.Errorf("resolve install dir: %w", err)
	}

	shouldCopy := !opts.ForceCopy && (source == SourceCustom)

	// Step 1: Stop old daemon before replacing binary or config
	if !opts.SkipDaemon {
		if err := StopDaemon(); err != nil {
			fmt.Printf("  Stopping old daemon: %v\n", err)
		} else {
			fmt.Println("  Daemon stopped")
		}
	}

	// Step 2: Copy binary (only if needed)
	var destExe string
	if shouldCopy {
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return nil, fmt.Errorf("create install dir: %w", err)
		}
		destExe = filepath.Join(installDir, filepath.Base(exePath))
		if err := copyFile(exePath, destExe); err != nil {
			return nil, fmt.Errorf("copy binary: %w", err)
		}
		result.BinaryDest = destExe
		fmt.Printf("  Binary installed -> %s\n", destExe)
	} else {
		destExe = exePath
		result.BinaryDest = exePath
		result.SkippedCopy = true
		switch source {
		case SourceManaged:
			fmt.Printf("  Binary: using managed location (%s)\n", exePath)
			fmt.Println("  (package manager owns this binary; skipping copy)")
		case SourceAlreadyInstalled:
			fmt.Printf("  Binary: already at target location (%s)\n", exePath)
			fmt.Println("  (in-place upgrade; skipping copy)")
		default:
			fmt.Printf("  Binary: using current location (%s)\n", exePath)
		}
	}

	// Step 3: Configure system PATH (only if we actually placed a binary somewhere new)
	if !opts.SkipPath && shouldCopy {
		pathOk, pathErr := AddToSystemPath(installDir)
		if pathErr != nil {
			fmt.Printf("  PATH configuration: %v\n", pathErr)
		} else {
			result.PathConfigured = pathOk
			if pathOk {
				fmt.Printf("  System PATH updated: %s\n", installDir)
				if runtime.GOOS == "windows" {
					fmt.Println("    New terminals will pick up the change automatically.")
				} else {
					fmt.Println("    Restart your terminal or run: source ~/.zshrc (or ~/.bashrc)")
				}
			} else {
				fmt.Println("  Already in system PATH")
			}
		}
	} else if !opts.SkipPath && !shouldCopy {
		result.PathConfigured = true // Already on PATH by virtue of being managed
		fmt.Println("  PATH: already configured (managed source)")
	}

	// Step 4: Create desktop shortcut (Windows only)
	if !opts.SkipShortcut && runtime.GOOS == "windows" {
		scOk, scErr := CreateDesktopShortcut(destExe)
		if scErr != nil {
			fmt.Printf("  Desktop shortcut: %v\n", scErr)
		} else {
			result.ShortcutCreated = scOk
			if scOk {
				fmt.Println("  Desktop shortcut created")
			}
		}
	}

	// Step 5: Register daemon with updated config and start it
	if !opts.SkipDaemon {
		workspaceDir := resolveWorkspaceDir(installDir)
		if err := SetupDaemon(workspaceDir); err != nil {
			fmt.Printf("  Daemon registration: %v\n", err)
		} else {
			result.DaemonSetup = true
			fmt.Println("  Daemon auto-start registered and started")
		}
	}

	return result, nil
}

// IsInstalled checks whether MindX appears to be properly installed.
// Returns true if either:
//   - The binary is in a managed/system location AND daemon is registered, OR
//   - The binary is in our target install directory.
func IsInstalled() (bool, InstallSource, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, SourceUnknown, "", fmt.Errorf("get executable: %w", err)
	}

	source := detectInstallSource(exePath)
	actualDir := filepath.Dir(exePath)

	switch source {
	case SourceManaged:
		// Managed: check if daemon is registered (the real indicator of "installed")
		status, _ := CheckDaemon()
		return status == DaemonRunning || status == DaemonStopped, source, actualDir, nil
	case SourceAlreadyInstalled:
		return true, source, actualDir, nil
	default:
		expectedDir, _ := resolveInstallDir("")
		return actualDir == expectedDir, source, actualDir, nil
	}
}

// ── Install source detection ────────────────────────────────────────────────

// detectInstallSource classifies where the given executable path came from.
func detectInstallSource(exePath string) InstallSource {
	dir := filepath.Dir(filepath.Clean(exePath))
	home, _ := os.UserHomeDir()

	// Check: already in our target install directory?
	targetDir, _ := resolveInstallDir("")
	if dir == filepath.Clean(targetDir) {
		return SourceAlreadyInstalled
	}

	// Check: managed locations (package managers / system PATH)?
	for _, prefix := range managedPrefixes(home) {
		if strings.HasPrefix(dir, prefix) {
			return SourceManaged
		}
	}

	// Default: custom/manual download
	return SourceCustom
}

// managedPrefixes returns directory prefixes that indicate a package-managed
// or system-PATH binary (no copy needed).
func managedPrefixes(home string) []string {
	prefixes := []string{
		// Homebrew (Apple Silicon + Intel)
		"/opt/homebrew",
		"/usr/local/Cellar",
		"/usr/local/opt",
		// Homebrew (Linux)
		"/home/linuxbrew/.linuxbrew",
		// Standard system bin directories
		"/usr/local/bin",
		"/usr/bin",
		"/usr/sbin",
		"/bin",
		"/sbin",
		// Snap
		"/snap/",
		// Flatpak (via symlink)
		".local/share/flatpak",
		// Nix
		"/nix/store",
		// Conda/Mamba
		"miniconda3",
		"anaconda3",
		"miniforge3",
		// MacPorts
		"/opt/local",
	}

	// Also include home-relative paths that are common for user-level managers
	if home != "" {
		prefixes = append(prefixes,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".cargo", "bin"),
		)
	}

	return prefixes
}

// ── Internal helpers ────────────────────────────────────────────────────────

// resolveInstallDir returns the platform-appropriate default install directory,
// or the user-specified override. This is only used for custom-source installs.
func resolveInstallDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("ProgramFiles"), "mindx"), nil
	case "darwin", "linux":
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, ".mindx", "bin"), nil
		}
		return "/usr/local/bin", nil
	default:
		return "/usr/local/bin", nil
	}
}

// resolveWorkspaceDir returns the workspace/data directory for the given install location.
func resolveWorkspaceDir(installDir string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".mindx")
	}
	return installDir
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, srcInfo.Mode())
}
