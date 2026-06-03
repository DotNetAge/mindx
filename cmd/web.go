package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "打开 MindX WebUI",
	Long: `在浏览器中打开 MindX Web 界面。

如果 Daemon 尚未启动，会先提示启动。
WebUI 通过 localhost:1313 提供服务。

示例:
  mindx web           # 打开默认 WebUI
  mindx web --port 8080 # 指定端口（需配合 mindx start 使用）`,
	RunE: runWeb,
}

var webPort string

func init() {
	webCmd.Flags().StringVarP(&webPort, "port", "p", ":1313", "WebUI 服务端口")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	workspaceDir := core.DefaultUserPrefsDir()
	webDir := filepath.Join(workspaceDir, "web")

	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		return fmt.Errorf("WebUI 文件未找到 (%s)，请运行 'mindx start' 启动 Daemon 或重新运行安装向导", webDir)
	}

	port := webPort
	if port == "" || port == ":1313" {
		port = ":1313"
	}
	url := fmt.Sprintf("http://localhost%s", port)

	fmt.Printf("🌐 正在打开 MindX WebUI: %s\n\n", url)
	fmt.Printf("   如果页面无法访问，请确保 Daemon 已运行:\n")
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
