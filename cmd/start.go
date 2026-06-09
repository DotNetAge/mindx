package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: i18n.T("cmd.start.short"),
	Long:  i18n.T("cmd.start.long"),
	RunE:  runStart,
}

var (
	startPort string
	startPath string
)

func init() {
	startCmd.Flags().StringVarP(&startPort, "port", "p", ":1314", i18n.T("cmd.start.flag.port.desc"))
	startCmd.Flags().StringVar(&startPath, "path", "", i18n.T("cmd.start.flag.path.desc"))
}

func runStart(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	if err := core.ExtractWorkspace(RuntimeFS, workspaceDir); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.start.error.workspace.init"), err)
	}

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.start.error.config.load"), err)
	}
	if !cfg.Initialized {
		return fmt.Errorf("%s", i18n.T("cmd.start.error.notconfigured"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wsPath := startPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	server, err := svc.NewServer(startPort, wsPath)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server.RegisterBuiltinCommands()

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
