package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: i18n.T("cmd.web.short"),
	Long:  i18n.T("cmd.web.long"),
	RunE:  runWeb,
}

var webPort string

func init() {
	webCmd.Flags().StringVarP(&webPort, "port", "p", ":1313", "WebUI service port")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()
	webDir := filepath.Join(workspaceDir, "web")

	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		return fmt.Errorf("%s: %s", i18n.T("cmd.web.error.webui.notfound"), webDir)
	}

	port := webPort
	if port == "" || port == ":1313" {
		port = ":1313"
	}
	url := fmt.Sprintf("http://localhost%s", port)

	fmt.Printf("%s %s\n", i18n.T("cmd.web.output.opening"), url)
	fmt.Printf("   If the page is not accessible, make sure Daemon is running:\n")
	fmt.Printf("     mindx start\n\n")

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", url)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", url)
	case "linux":
		openCmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := openCmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
