package cli

import (
	"fmt"
	"os/exec"

	"mindx/pkg/i18n"

	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: i18n.T("cli.dashboard.short"),
	Run: func(cmd *cobra.Command, args []string) {
		serverPort := 911
		dashboardUrl := fmt.Sprintf("http://localhost:%d", serverPort)
		openCmd := exec.Command("open", dashboardUrl)
		err := openCmd.Run()
		if err != nil {
			fmt.Println(i18n.TWithData("cli.dashboard.failed", map[string]interface{}{"Error": err.Error()}))
		} else {
			fmt.Println(i18n.T("cli.dashboard.success"))
		}
	},
}
