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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show MindX system status",
	Long: `Displays the current status of all MindX components:
  - Binary location and version
  - Daemon service state
  - Configuration status
  - Python environment
  - Embedder model availability
  - System PATH registration

Examples:
  mindx status`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("📊 MindX System Status")
	fmt.Println(strings.Repeat("─", 45))

	workspaceDir := core.DefaultUserPrefsDir()

	// Binary location & version
	exePath, _ := os.Executable()
	fmt.Printf("\n📦 Binary:   %s\n", exePath)
	fmt.Printf("   Version:  %s", core.Version)
	if core.Commit != "unknown" {
		fmt.Printf(" (%s)", core.Commit)
	}
	fmt.Println()
	if core.Dirty == "dirty" {
		fmt.Println("   ⚠️  dirty build (uncommitted changes)")
	}

	installed, installDir, _, _ := setup.IsInstalled()
	if installed {
		fmt.Printf("   Status:   installed (system)\n")
	} else {
		fmt.Printf("   Status:   running from %s\n", installDir)
	}

	// Config
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		fmt.Printf("\n⚙️  Config:   ❌ not initialized\n")
	} else {
		fmt.Printf("\n⚙️  Config:   ✅ initialized\n")
		if cfg.DefaultModel != "" {
			fmt.Printf("   Model:    %s\n", cfg.DefaultModel)
		}
		if cfg.HasEmbedder() {
			fmt.Printf("   Embedder: %s\n", cfg.EmbedderModel)
		}
	}

	// Daemon
	daemonStatus, _ := setup.CheckDaemon()
	daemonIcon := "❓"
	switch daemonStatus {
	case setup.DaemonRunning:
		daemonIcon = "✅"
	case setup.DaemonStopped:
		daemonIcon = "⏹️ "
	case setup.DaemonNotInstalled:
		daemonIcon = "⬜ "
	}
	fmt.Printf("\n🔄 Daemon:   %s %s", daemonIcon, daemonStatus)

	// PATH
	exeDir := filepath.Dir(exePath)
	if setup.CheckInPath(exeDir) {
		fmt.Printf("\n🔗 PATH:     ✅ in system PATH\n")
	} else {
		fmt.Printf("\n🔗 PATH:     ⚠️  not in PATH (run 'mindx install')\n")
	}

	// Python venv
	venvPath := filepath.Join(workspaceDir, ".venv")
	if _, err := os.Stat(venvPath); err == nil {
		fmt.Printf("🐍 Python:   ✅ venv exists\n")
		if pyCfg := cfg.Python; pyCfg.Version != "" {
			fmt.Printf("   Version:  %s\n", pyCfg.Version)
		}
	} else {
		fmt.Printf("🐍 Python:   ⚠️  no venv (run 'mindx doctor')\n")
	}

	// Embedder model file check
	modelPath := filepath.Join(workspaceDir, "data", "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf("🧠 Embedder: ✅ model_q4.onnx present\n")
	} else if cfg != nil && cfg.HasEmbedder() {
		fmt.Printf("🧠 Embedder: ⚠️  configured but file missing (%s)\n", modelPath)
	} else {
		fmt.Printf("🧠 Embedder: ⬜  not configured\n")
	}

	// Platform info
	fmt.Printf("\n💻 Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return nil
}
