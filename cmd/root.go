package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/client"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
)

var RuntimeFS fs.FS

var rootCmd = &cobra.Command{
	Use:   "mindx",
	Short: "MindX - AI Agent CLI",
	Long: `MindX 是一个 AI-native 的多 Agent 对话平台。

默认行为:
  直接运行 mindx 将启动 TUI（终端界面）进行对话。
  
子命令:
  start   启动后台 Daemon 服务（供 WebUI/MacUI 接入）

示例:
  mindx                    # 启动 TUI 聊天界面
  mindx start             # 启动后台服务
  mindx start --port 8080 # 指定端口`,
	RunE:         runTUI,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func defaultWorkspaceDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".mindx")
}

func runTUI(cmd *cobra.Command, args []string) error {
	workspaceDir := defaultWorkspaceDir()

	cfg, err := core.Bootstrap(RuntimeFS, workspaceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 自检失败: %v\n", err)
		return err
	}

	p := client.NewProgram(cfg)
	_, err = p.Run()
	return err
}
