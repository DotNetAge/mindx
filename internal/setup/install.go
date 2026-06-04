package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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
}

// InstallResult reports what was done during installation.
type InstallResult struct {
	BinaryDest    string // where the binary was installed
	DaemonSetup   bool   // whether daemon was registered
	PathConfigured bool  // whether PATH was updated
	ShortcutCreated bool // whether desktop shortcut was created
}

// Install performs a full platform-aware installation of MindX.
//
// On all platforms:
//   - Copies the current binary to the install directory
//   - Extracts runtime assets alongside the binary
//
// Platform-specific extras:
//   - macOS/Linux: adds install dir to shell rc PATH, registers LaunchAgent/systemd service
//   - Windows: adds install dir to System PATH, creates desktop .lnk, registers schtasks task
func Install(opts InstallOptions) (*InstallResult, error) {
	result := &InstallResult{}

	// Step 1: Determine install directory and copy binary
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, fmt.Errorf("resolve install dir: %w", err)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("create install dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}

	destExe := filepath.Join(installDir, filepath.Base(exePath))
	if err := copyFile(exePath, destExe); err != nil {
		return nil, fmt.Errorf("copy binary: %w", err)
	}
	result.BinaryDest = destExe
	fmt.Printf("✅ Binary installed → %s\n", destExe)

	// Note: Runtime assets (agents, prompts, models) are extracted at startup
	// by core.ExtractWorkspace() from the embedded FS. No need to duplicate here.

	// Step 2: Configure system PATH
	if !opts.SkipPath {
		pathOk, pathErr := AddToSystemPath(installDir)
		if pathErr != nil {
			fmt.Printf("⚠️  PATH configuration: %v\n", pathErr)
		} else {
			result.PathConfigured = pathOk
			if pathOk {
				fmt.Printf("✅ System PATH updated: %s\n", installDir)
				if runtime.GOOS == "windows" {
					fmt.Println("   New terminals will pick up the change automatically.")
				} else {
					fmt.Println("   Restart your terminal or run: source ~/.zshrc (or ~/.bashrc)")
				}
			} else {
				fmt.Println("ℹ️  Already in system PATH")
			}
		}
	}

	// Step 4: Create desktop shortcut (Windows only)
	if !opts.SkipShortcut && runtime.GOOS == "windows" {
		scOk, scErr := CreateDesktopShortcut(destExe)
		if scErr != nil {
			fmt.Printf("⚠️  Desktop shortcut: %v\n", scErr)
		} else {
			result.ShortcutCreated = scOk
			if scOk {
				fmt.Println("✅ Desktop shortcut created")
			}
		}
	}

	// Step 5: Register daemon / auto-start service
	if !opts.SkipDaemon {
		workspaceDir := resolveWorkspaceDir(installDir)
		if err := SetupDaemon(workspaceDir); err != nil {
			fmt.Printf("⚠️  Daemon registration: %v\n", err)
		} else {
			result.DaemonSetup = true
			fmt.Println("✅ Daemon auto-start registered")
		}
	}

	return result, nil
}

// IsInstalled checks whether MindX appears to be properly installed.
func IsInstalled() (bool, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, "", fmt.Errorf("get executable: %w", err)
	}
	expectedDir, _ := resolveInstallDir("")
	actualDir := filepath.Dir(exePath)
	return actualDir == expectedDir, actualDir, nil
}

// ── Internal helpers ────────────────────────────────────────────────────────

// resolveInstallDir returns the platform-appropriate default install directory,
// or the user-specified override.
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
	// Workspace stays in user home regardless of install location
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
