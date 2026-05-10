package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动MindX核心服务",
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := svc.DefaultApp()
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	app.RegisterBuiltinCommands()

	return app.Start(ctx)
}
