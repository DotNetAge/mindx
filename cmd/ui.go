package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// mindx ui —— Agent-Driven UI 命令通道（CLI 受理端）
//
// Agent 在技能中调用三命令，CLI 过参数闸后经 RPC 交 daemon 广播事件：
//
//	命令                      daemon 事件      客户端行为
//	mindx ui open <路径>      file_open        按文件类型分发打开对应 Detail
//	mindx ui open-link <URL>  link_open        系统浏览器打开
//	mindx ui run <命令>       terminal_run     打开终端执行命令
//
// 受理语义：RPC 返回即「已受理」，效果发生在用户屏幕上，CLI 不等回执。
// ---------------------------------------------------------------------------

var uiCmd = &cobra.Command{
	Use:   "ui open <路径>|open-link <URL>|run <命令>",
	Short: "Request the client UI to open a file, link, or terminal command",
	Long: `Ask the connected MindX client to perform a presentation action.

The daemon broadcasts the request to all connected clients; whichever
client is in focus performs it. Acceptance is fire-and-forget: the RPC
return only means "accepted and broadcast", not "rendered".

Examples:
  mindx ui open ./report.md
  mindx ui open-link https://example.com/login
  mindx ui run "pnpm test"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var uiOpenCmd = &cobra.Command{
	Use:   "open <路径>",
	Short: "Open a file in the client (routed to its matching viewer)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		abs, err := resolveWorkspacePath(args[0])
		if err != nil {
			return err
		}
		return callUI("打开文件", abs, func(cl *rpc.Client) error {
			_, err := cl.UIOpen(abs)
			return err
		})
	},
}

var uiOpenLinkCmd = &cobra.Command{
	Use:   "open-link <URL>",
	Short: "Open a web link in the client's system browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := strings.TrimSpace(args[0])
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("仅支持 http/https 链接: %s", url)
		}
		return callUI("打开链接", url, func(cl *rpc.Client) error {
			_, err := cl.UIOpenLink(url)
			return err
		})
	},
}

var uiRunCmd = &cobra.Command{
	Use:   "run <命令>",
	Short: "Run a command in the client's visible terminal",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		command := strings.Join(args, " ")
		// cwd 是客户端终端的工作目录锚点，取不到就明确报错而不是带空值广播
		//（客户端虽有 currentProjectDir 回退，但那已偏离 Agent 工作区语义）
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取工作目录失败: %w", err)
		}
		return callUI("提交到终端", command, func(cl *rpc.Client) error {
			_, err := cl.UIRun(command, cwd)
			return err
		})
	},
}

// resolveWorkspacePath 参数闸：路径解析为绝对路径后必须落在当前工作目录内
// （相对路径按 cwd 拼接；越界路径直接拒绝，与 fs.* 工具同一套纪律）。
func resolveWorkspacePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("路径为空")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(cwd, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("路径必须位于当前工作区内: %s", raw)
	}
	return p, nil
}

// callUI 统一受理调用：拨 daemon → RPC → 打印受理结果（已过闸的参数由 daemon 信任并广播）。
func callUI(action, target string, do func(cl *rpc.Client) error) error {
	cl, err := rpc.Dial(daemonAddr)
	if err != nil {
		return err
	}
	defer func() { _ = cl.Close() }()

	if err := do(cl); err != nil {
		return err
	}
	fmt.Printf("已受理: %s %s（客户端在焦点时执行）\n", action, target)
	return nil
}

func init() {
	uiCmd.AddCommand(uiOpenCmd, uiOpenLinkCmd, uiRunCmd)
	rootCmd.AddCommand(uiCmd)
}
