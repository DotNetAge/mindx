package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose MindX system health and fix issues",
	Long: `Checks all MindX components for common problems and offers fixes.

Unlike 'status' (which shows current state), 'doctor' actively checks
for misconfigurations, missing files, and integration issues.

Examples:
  mindx doctor        # Run all checks
  mindx doctor --fix   # Auto-fix where possible`,
	RunE: runDoctor,
}

var (
	doctorFix bool
)

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVarP(&doctorFix, "fix", "f", false, "Auto-fix issues where possible")
}

// Check represents a single diagnostic check result.
type Check struct {
	Name    string // Component name
	Status  string // ✅ ⚠️ ❌ ℹ️
	Message string // Human-readable description
	Fix     func() error // Auto-fix function (may be nil)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 MindX Health Check")
	fmt.Println(strings.Repeat("─", 50))

	checks := runAllChecks()

	hasIssues := false
	for i, c := range checks {
		icon := c.Status
		if icon == "⚠️" || icon == "❌" {
			hasIssues = true
		}
		fmt.Printf("\n%d. %s %s\n", i+1, icon, c.Name)
		fmt.Printf("   %s\n", c.Message)

		if (c.Status == "⚠️" || c.Status == "❌") && c.Fix != nil && !doctorFix {
			fmt.Println("   💡 Run with --fix to auto-resolve")
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))

	if !hasIssues {
		fmt.Println("✅ All systems healthy!")
		return nil
	}

	if doctorFix {
		fmt.Println("\n🔧 Applying fixes...")
		fixed := 0
		for _, c := range checks {
			if c.Fix != nil && (c.Status == "⚠️" || c.Status == "❌") {
				if err := c.Fix(); err != nil {
					fmt.Printf("   ❌ Fix failed for %s: %v\n", c.Name, err)
				} else {
					fmt.Printf("   ✅ Fixed: %s\n", c.Name)
					fixed++
				}
			}
		}
		fmt.Printf("\n✅ %d issue(s) fixed.\n", fixed)
	} else {
		fmt.Println(fmt.Sprintf("\n⚠️  Found issues. Run 'mindx doctor --fix' to resolve automatically."))
		fmt.Println("   Or use individual commands:")
		fmt.Println("     mindx install          # Full system installation")
		fmt.Println("     mindx install --no-daemon # Install without daemon")
		fmt.Println("     mindx stop             # Stop daemon if stuck")
	}

	return nil
}

// ── Diagnostic checks ───────────────────────────────────────────────────────

func runAllChecks() []Check {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	workspaceDir := core.DefaultUserPrefsDir()

	var checks []Check

	// 1. Config initialized?
	cfg, cfgErr := core.LoadMindxConfig(workspaceDir)
	if cfgErr != nil {
		checks = append(checks, Check{
			Name:    "Configuration",
			Status:  "❌",
			Message: fmt.Sprintf("Not initialized: %v", cfgErr),
			Fix:     nil, // Needs interactive setup → suggest wizard
		})
	} else {
		checks = append(checks, Check{
			Name:    "Configuration",
			Status:  "✅",
			Message: fmt.Sprintf("Initialized (model: %s)", cfg.DefaultModel),
		})
	}

	// 2. System PATH
	if setup.CheckInPath(exeDir) {
		checks = append(checks, Check{
			Name:    "System PATH",
			Status:  "✅",
			Message: fmt.Sprintf("%s is in PATH", exeDir),
		})
	} else {
		dir := exeDir
		checks = append(checks, Check{
			Name:    "System PATH",
			Status:  "❌",
			Message: fmt.Sprintf("%s not in system PATH — 'mindx' command won't work globally", dir),
			Fix: func() error { _, err := setup.AddToSystemPath(dir); return err },
		})
	}

	// 3. Daemon service
	daemonStatus, _ := setup.CheckDaemon()
	switch daemonStatus {
	case setup.DaemonRunning:
		checks = append(checks, Check{Name: "Daemon Service", Status: "✅", Message: "Running"})
	case setup.DaemonStopped:
		checks = append(checks, Check{
			Name:    "Daemon Service",
			Status:  "⚠️",
			Message: "Registered but not running — WebUI/MacUI may not connect",
			Fix:     nil, // User should explicitly start it
		})
	case setup.DaemonNotInstalled:
		checks = append(checks, Check{
			Name:    "Daemon Service",
			Status:  "⚠️",
			Message: "Not installed — auto-start disabled",
			Fix:     func() error { return setup.SetupDaemon(workspaceDir) },
		})
	default:
		checks = append(checks, Check{Name: "Daemon Service", Status: "❓", Message: "Cannot determine status"})
	}

	// 3a. Daemon service — verify VBS launcher on Windows
	if runtime.GOOS == "windows" && (daemonStatus == setup.DaemonRunning || daemonStatus == setup.DaemonStopped) {
		vbsPath := filepath.Join(workspaceDir, "bin", "MindXDaemon.vbs")
		if _, err := os.Stat(vbsPath); os.IsNotExist(err) {
			checks = append(checks, Check{
				Name:    "Daemon Launcher",
				Status:  "⚠️",
				Message: "Daemon registered but VBS launcher missing — may have been installed by an older version with a buggy launcher",
				Fix:     func() error { return setup.SetupDaemon(workspaceDir) },
			})
		}
	}

	// 4. Python venv
	venvPath := filepath.Join(workspaceDir, ".venv")
	if _, err := os.Stat(venvPath); err == nil {
		pyVer := ""
		if cfgErr == nil && cfg.Python.Version != "" {
			pyVer = " (" + cfg.Python.Version + ")"
		}
		checks = append(checks, Check{
			Name:    "Python Environment",
			Status:  "✅",
			Message: fmt.Sprintf("Virtual environment found%s", pyVer),
		})
	} else {
		checks = append(checks, Check{
			Name:    "Python Environment",
			Status:  "⚠️",
			Message: "No venv found — some skills may not work (e.g., xlsx, pdf)",
			Fix:     nil, // Requires user interaction (version selection)
		})
	}

	// 5. Embedder model
	modelPath := filepath.Join(workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		checks = append(checks, Check{
			Name:    "Embedder Model",
			Status:  "✅",
			Message: "model_q4.onnx present — memory search enabled",
		})
	} else if cfgErr == nil && cfg.HasEmbedder() {
		checks = append(checks, Check{
			Name:    "Embedder Model",
			Status:  "❌",
			Message: fmt.Sprintf("Configured but file missing at %s — memory search will fail", modelPath),
			Fix:     nil, // Needs download step
		})
	} else {
		checks = append(checks, Check{
			Name:    "Embedder Model",
			Status:  "ℹ️",
			Message: "Not configured — memory search disabled (optional)",
		})
	}

	// 6. Workspace directory permissions
	if info, err := os.Stat(workspaceDir); err == nil {
		if info.Mode().Perm()&0200 == 0 {
			checks = append(checks, Check{
				Name:    "Workspace Permissions",
				Status:  "❌",
				Message: fmt.Sprintf("%s is not writable", workspaceDir),
			})
		}
	}

	// 7. Port availability (quick check)
	if daemonStatus == setup.DaemonRunning {
		// If daemon is running, assume port is fine
		checks = append(checks, Check{Name: "WebUI Port", Status: "✅", Message: "Daemon managing port allocation"})
	}

	return checks
}
