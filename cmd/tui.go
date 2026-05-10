package cmd

import (
	"github.com/DotNetAge/mindx/internal/client"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "启动 TUI 聊天界面",
	RunE:  runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
	p := client.NewProgram()
	_, err := p.Run()
	return err
}
