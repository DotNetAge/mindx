// secret 子命令：从系统凭据存储只读指定键的值。
// 只提供 get——写入仍由各业务命令完成（provider setkey、设备流登录等），
// 避免此命令成为绕过业务纪律的凭据写入口。
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/spf13/cobra"
)

// mindx secret get <key>
//
// 用途：让本机 Agent（技能）以管道方式安全取回已保存的登录态（如
// market:auth_token，设备流登录时由 daemon 写入 Keychain），再携带它调用
// 资源社区的接口。安全纪律：取到的值只允许在管道/变量中转发，禁止落盘、
// 禁止写日志、禁止回显到任何持久输出。
var secretGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Read a secret from the system credential store",
	Long: `Print the secret value stored under <key> in the system credential store.

On macOS the value lives in the system Keychain (service "mindx");
on Linux/Windows in an AES-GCM encrypted file under the user prefs dir.

Output discipline: the value is meant to be forwarded via pipes/shell
variables (e.g. $(mindx secret get market:auth_token)). Never write it
to disk, logs, or any persistent output.

Example:
  mindx secret get market:auth_token`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceDir := core.DefaultUserPrefsDir()
		credStore := core.NewCredentialStore(filepath.Clean(workspaceDir))
		value, err := credStore.Get(args[0])
		if err != nil || value == "" {
			return fmt.Errorf("secret %q not found: %w", args[0], err)
		}
		// 只输出值本身（无前缀无换行附加说明），便于 $(...) 管道内联
		fmt.Println(value)
		return nil
	},
}

func init() {
	secretCmd := &cobra.Command{
		Use:   "secret",
		Short: "Read-only access to the system credential store",
	}
	secretCmd.AddCommand(secretGetCmd)
	rootCmd.AddCommand(secretCmd)
}
