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

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: i18n.T("cmd.daemon.short"),
	Long:  i18n.T("cmd.daemon.long"),
	RunE:  runDaemon,
}

var (
	daemonPort string
	daemonPath string
)

func init() {
	daemonCmd.Flags().StringVarP(&daemonPort, "port", "p", ":1314", i18n.T("cmd.daemon.flag.port.desc"))
	daemonCmd.Flags().StringVar(&daemonPath, "path", "", i18n.T("cmd.daemon.flag.path.desc"))
}

func runDaemon(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	if err := core.ExtractWorkspace(RuntimeFS, workspaceDir); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.daemon.error.workspace.init"), err)
	}

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.daemon.error.config.load"), err)
	}
	if !cfg.Initialized {
		return fmt.Errorf("%s", i18n.T("cmd.daemon.error.notconfigured"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wsPath := daemonPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	server, err := svc.NewServer(daemonPort, wsPath, AppIconFS, RuntimeFS)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server.RegisterBuiltinCommands()

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
