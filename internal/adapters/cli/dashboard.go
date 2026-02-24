package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"mindx/pkg/i18n"

	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: i18n.T("cli.dashboard.short"),
	Run: func(cmd *cobra.Command, args []string) {
		serverPort := 911
		dashboardUrl := fmt.Sprintf("http://localhost:%d", serverPort)

		var openCmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			openCmd = exec.Command("cmd", "/c", "start", dashboardUrl)
		case "linux":
			openCmd = exec.Command("xdg-open", dashboardUrl)
		default:
			openCmd = exec.Command("open", dashboardUrl)
		}

		err := openCmd.Run()
		if err != nil {
			// On headless Linux or missing xdg-open, just print the URL
			fmt.Println(i18n.TWithData("cli.dashboard.visit", map[string]interface{}{"URL": dashboardUrl}))
		} else {
			fmt.Println(i18n.T("cli.dashboard.success"))
		}
	},
}
