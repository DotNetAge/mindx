package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/DotNetAge/mindx/internal/appicon"
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage macOS .app bundle for MindX",
	Long: `Generate a macOS application bundle (.app) so that Finder and Dock
display the custom MindX icon.

Examples:
  mindx app create          Create .app in /Applications
  mindx app create -o ~/Desktop  Create .app on Desktop
  mindx app icon /tmp/icon.png  Export embedded icon to file`,
}

var appOutputDir string

func init() {
	appCmd.PersistentFlags().StringVarP(&appOutputDir, "output", "o", "/Applications", "Output directory for .app bundle")
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(createAppCmd)
	appCmd.AddCommand(exportIconCmd)
}

// mindx app create
var createAppCmd = &cobra.Command{
	Use:   "create",
	Short: "Create macOS .app bundle with embedded icon",
	RunE:  runCreateApp,
}

func runCreateApp(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("app bundle creation is only supported on macOS (current: %s)", runtime.GOOS)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	fmt.Printf("Creating %s.app → %s/\n", appicon.Name, appOutputDir)

	appPath, err := appicon.CreateAppBundle(AppIconFS, appOutputDir, self)
	if err != nil {
		return fmt.Errorf("create app bundle: %w", err)
	}

	fmt.Printf("  Bundle: %s\n", appPath)
	fmt.Printf("  Icon:   Contents/Resources/%s\n", appicon.IconName)
	fmt.Println("\nYou can now drag the .app to Dock or launch from Finder.")
	return nil
}

// mindx app icon <dest>
var exportIconCmd = &cobra.Command{
	Use:   "icon [destination]",
	Short: "Export the embedded app icon to a file",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExportIcon,
}

func runExportIcon(cmd *cobra.Command, args []string) error {
	dest := appicon.IconName
	if len(args) > 0 {
		dest = args[0]
	}
	if !filepath.IsAbs(dest) {
		wd, _ := os.Getwd()
		dest = filepath.Join(wd, dest)
	}

	fmt.Printf("Exporting app icon → %s\n", dest)

	if err := appicon.Write(AppIconFS, dest); err != nil {
		return fmt.Errorf("export icon: %w", err)
	}

	info, err := os.Stat(dest)
	if err == nil {
		fmt.Printf("  Size: %d bytes\n", info.Size())
	}
	fmt.Println("Done.")
	return nil
}
