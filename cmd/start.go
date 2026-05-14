package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 MindX Daemon 服务（含 Gateway + Scheduler）",
	Long: `启动后台守护进程，提供 WebSocket 网关服务供 WebUI/MacUI 接入。
同时运行 Scheduler 执行定时任务。

示例:
  mindx start              # 使用默认配置 (:1314)
  mindx start --port 8080  # 指定端口`,
	RunE: runStart,
}

var (
	startPort string
	startPath string
)

func init() {
	startCmd.Flags().StringVarP(&startPort, "port", "p", ":1314", "WebSocket 监听地址")
	startCmd.Flags().StringVar(&startPath, "path", "", "WebSocket 路径 (默认: /ws)")
}

func runStart(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()

	if err := core.ExtractWorkspace(RuntimeFS, workspaceDir); err != nil {
		return fmt.Errorf("初始化工作目录失败: %w", err)
	}
	os.Setenv("MINDX_WORKSPACE", workspaceDir)

	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if !cfg.Initialized {
		return fmt.Errorf("MindX 尚未配置，请先运行 'mindx' 完成首次配置后再启动 Daemon 服务")
	}

	if cfg.LastAgent != "" {
		os.Setenv("MINDX_MASTER", cfg.LastAgent)
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
