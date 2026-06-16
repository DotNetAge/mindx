package cmd

import (
	"fmt"

	"github.com/DotNetAge/mindx/internal/setup"
	setupstyle "github.com/DotNetAge/mindx/internal/setup/style"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall MindX from system (binary + PATH + daemon + shortcut)",
	Long: `Removes MindX from the system, reversing the installation:

  - Stops and unregisters the daemon service
  - Removes the install directory from system PATH
  - Removes desktop shortcut (Windows only)
  - Deletes the installed binary

This is the inverse of 'mindx install'. By default all components are removed.
Use flags to skip specific cleanup steps.

Examples:
  mindx uninstall                  # Full uninstallation with all defaults
  mindx uninstall --no-daemon      # Skip daemon unregistration
  mindx uninstall --keep-binary    # Remove integrations but keep binary
  mindx uninstall --dir /opt/mindx # Custom install directory`,
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
	}
	if uninstallKeepBinary {
		fmt.Println("   Binary:   kept (--keep-binary)")
	}

	allClean := !result.DaemonRemoved && !result.PathCleaned && !result.ShortcutRemoved && !result.BinaryRemoved
	if allClean && !uninstallKeepBinary {
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
