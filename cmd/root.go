package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/client"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/setup"
	setupstyle "github.com/DotNetAge/mindx/internal/setup/style"
	"github.com/spf13/cobra"
)

func init() {
	setupstyle.GradientVersion = core.Version
}

// needsDoctor checks if the environment is healthy enough to run the chat TUI.
// Returns true if the user needs to run the setup wizard.
func needsDoctor(cfg *core.MindxConfig, workspaceDir string) bool {
	if cfg.DefaultModel == "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, ".venv")); os.IsNotExist(err) {
		return true
	}
	return false
}

var RuntimeFS fs.FS
var AppIconFS fs.FS

var rootCmd = &cobra.Command{
	Use:          "mindx",
	Short:        "MindX - AI Agent CLI",
	Long:         "", // Set dynamically in runTUI after i18n.Init()
	RunE:         runTUI,
	SilenceUsage: true,
}

func Execute() error {
	// Pre-init i18n from system locale so subcommand Short/Long keys resolve
	// before cobra displays help text. runTUI will re-init with config language.
	if err := i18n.Init(""); err != nil {
		// Non-fatal: T() falls back to returning keys as-is
	}
	return rootCmd.Execute()
}

var daemonAddr string

// requireDaemon is a PersistentPreRunE that can be used by commands that
// need the daemon to be running. It currently just returns nil (the actual
// connection check happens in each command's rpc.Dial call for
// better error messages). It exists so that these commands can have a
// consistent PersistentPreRunE set.
func requireDaemon(cmd *cobra.Command, args []string) error {
	return nil
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	rootCmd.PersistentFlags().StringVar(&daemonAddr, "daemon-addr", "", "Daemon WebSocket address (default: ws://localhost:1314/ws)")
}

func runTUI(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	cfg, err := core.Bootstrap(RuntimeFS, workspaceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s: %v\n", i18n.T("cmd.root.error.selfcheck"), err)
		return err
	}

	// Initialize i18n with language from config (defaults to system locale)
	if err := i18n.Init(cfg.Language); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  i18n init failed: %v (using default language)\n", err)
		if fallbackErr := i18n.Init("zh"); fallbackErr != nil {
			return fmt.Errorf("i18n init fallback failed: %w", fallbackErr)
		}
	}

	// Set localized help text after i18n is initialized
	cmd.Long = i18n.T("cmd.root.description")

	// Suggest install if running from non-standard location
	if installed, _, _, _ := setup.IsInstalled(); !installed {
		fmt.Printf("💡 %s\n\n", i18n.T("cmd.root.hint.install"))
	}

	if !cfg.Initialized {
		fmt.Printf("\n⚙️  %s\n\n", i18n.T("cmd.root.firstRun.detected"))

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("cmd.root.error.wizard.failed"), err)
		}

		fmt.Printf("\n✅ %s\n\n", i18n.T("cmd.root.firstRun.complete"))
	} else if needsDoctor(cfg, workspaceDir) {
		fmt.Printf("\n⚙️  %s\n\n", i18n.T("cmd.root.envcheck.needed"))

		if _, err := os.Stat(filepath.Join(workspaceDir, ".venv")); os.IsNotExist(err) {
			fmt.Printf("💡 %s\n", i18n.T("cmd.root.envcheck.python.missing"))
		}
		fmt.Print("\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		providersPath := filepath.Join(settingsDir, "providers.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		if err := setup.RunWizard(modelsPath, providersPath, agentsDir, workspaceDir, cfg, RuntimeFS); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("cmd.root.error.envfix.failed"), err)
		}

		fmt.Printf("\n✅ %s\n\n", i18n.T("cmd.root.envcheck.complete"))
	}

	cfg.AppVersion = core.Version
	if err := client.NewProgram(cfg); err != nil {
		return err
	}
	return nil
}
