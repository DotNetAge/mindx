package cli

import (
	"fmt"
	"os"

	"mindx/internal/config"
	"mindx/pkg/i18n"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mindx",
	Short: i18n.T("cli.root.short"),
	Long:  i18n.T("cli.root.long"),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show MindX version",
	Long:  "Display the current version of MindX",
	Run: func(cmd *cobra.Command, args []string) {
		version, buildTime, gitCommit := config.GetBuildInfo()
		
		fmt.Printf("MindX version: %s\n", version)
		if buildTime != "" {
			fmt.Printf("Build time:   %s\n", buildTime)
		}
		if gitCommit != "" {
			fmt.Printf("Git commit:   %s\n", gitCommit)
		}
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(kernelCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(trainCmd)
	rootCmd.AddCommand(versionCmd)

	modelCmd.AddCommand(testCmd)
	kernelCmd.AddCommand(kernelMainCmd)
	kernelCmd.AddCommand(kernelCtrlStartCmd)
	kernelCmd.AddCommand(kernelCtrlStopCmd)
	kernelCmd.AddCommand(kernelCtrlRestartCmd)
	kernelCmd.AddCommand(kernelCtrlStatusCmd)
}

func Execute() {
	if err := i18n.Init(); err != nil {
		fmt.Printf("i18n init failed: %v\n", err)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
