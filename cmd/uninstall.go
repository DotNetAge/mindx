package cmd

import (
	"fmt"

	"github.com/DotNetAge/mindx/internal/setup"
	setupstyle "github.com/DotNetAge/mindx/internal/setup/style"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall MindX from system (daemon + PATH + shortcut)",
	Long: `Removes MindX system integrations, reversing the installation:

  - Stops and unregisters the daemon service
  - Removes the install directory from system PATH
  - Removes desktop shortcut (Windows only)
  - Deletes installed binary (unless managed by package manager)

Behavior adapts to how MindX was installed:
  - **Package manager** (Homebrew, etc.): Skips binary removal — use
    'brew uninstall mindx' instead. Only cleans up daemon/PATH/shortcut.
  - **Manual download**: Full cleanup including binary removal.

This is the inverse of 'mindx install'. By default all removable components are
cleaned up. Use flags to skip specific steps.

Examples:
  mindx uninstall                  # Smart uninstall (auto-detects source)
  mindx uninstall --keep-binary    # Remove integrations but keep binary
  mindx uninstall --no-daemon      # Skip daemon unregistration`,
	RunE: runUninstall,
}

var (
	uninstallNoDaemon   bool
	uninstallNoPath     bool
	uninstallNoShortcut bool
	uninstallKeepBinary bool
)

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVar(&uninstallNoDaemon, "no-daemon", false, "Skip daemon/service unregistration")
	uninstallCmd.Flags().BoolVar(&uninstallNoPath, "no-path", false, "Skip PATH cleanup")
	uninstallCmd.Flags().BoolVar(&uninstallNoShortcut, "no-shortcut", false, "Skip desktop shortcut removal")
	uninstallCmd.Flags().BoolVar(&uninstallKeepBinary, "keep-binary", false, "Keep installed binary (only clean up integrations)")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	fmt.Println(setupstyle.GradientTitle(""))
	fmt.Println()

	opts := setup.UninstallOptions{
		SkipDaemon:   uninstallNoDaemon,
		SkipPath:     uninstallNoPath,
		SkipShortcut: uninstallNoShortcut,
		KeepBinary:   uninstallKeepBinary,
	}

	result, err := setup.Uninstall(opts)
	if err != nil {
		return fmt.Errorf("uninstallation failed: %w", err)
	}

	fmt.Println()
	fmt.Println("────────────────────────────────────────")
	fmt.Println("Uninstallation complete!")
	fmt.Println()
	fmt.Printf("   Source:  %s\n", result.Source)

	if result.DaemonRemoved {
		fmt.Println("   Daemon:   unregistered")
	}
	if result.PathCleaned {
		fmt.Println("   PATH:     cleaned")
	}
	if result.ShortcutRemoved {
		fmt.Println("   Shortcut: removed")
	}
	if result.BinaryRemoved {
		fmt.Println("   Binary:   removed")
	} else if !uninstallKeepBinary && result.Source == setup.SourceManaged {
		fmt.Println("   Binary:   skipped (package manager owned)")
	}
	if uninstallKeepBinary {
		fmt.Println("   Binary:   kept (--keep-binary)")
	}

	allClean := !result.DaemonRemoved && !result.PathCleaned && !result.ShortcutRemoved && !result.BinaryRemoved
	if allClean && !uninstallKeepBinary && result.Source != setup.SourceManaged {
		fmt.Println("   (nothing to clean — already uninstalled)")
	}

	fmt.Println()
	fmt.Println("Note:")
	fmt.Println("  The ~/.mindx/ directory (data, logs, sessions) was NOT removed.")
	fmt.Println("  To remove all data: rm -rf ~/.mindx")
	fmt.Println()
	fmt.Println("  PATH changes take effect in new terminal sessions.")

	return nil
}
