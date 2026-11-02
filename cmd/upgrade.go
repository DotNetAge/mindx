package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup"
	"github.com/DotNetAge/mindx/internal/update"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade MindX to the latest version",
	Long: `Downloads and installs the latest MindX release from GitHub.

Upgrade order (user-first): check, then download with the daemon still
serving; only with the user's confirmation does it stop the service,
swap the binary (seconds), and start the service back up.

  mindx upgrade            # Interactive: download, confirm, apply
  mindx upgrade --yes      # Non-interactive apply (scripts / MindX shell)
  mindx upgrade --stage    # Download and stage only; apply later
  mindx upgrade --check    # Only check for updates`,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	onlyCheck, _ := cmd.Flags().GetBool("check")
	stageOnly, _ := cmd.Flags().GetBool("stage")
	yes, _ := cmd.Flags().GetBool("yes")
	fromArchive, _ := cmd.Flags().GetString("from")
	fromVersion, _ := cmd.Flags().GetString("version")

	workspaceDir := core.DefaultUserPrefsDir()
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.InstalledVersion == "" && core.Version != "" {
		cfg.InstalledVersion = core.Version
	}

	updater := update.NewUpdater(
		core.Version,
		cfg.InstalledVersion,
		workspaceDir,
		func(version string) error {
			cfg.InstalledVersion = version
			return cfg.Save()
		},
		func(msg string, args ...any) {
			fmt.Fprintf(os.Stderr, "  · "+msg+"\n", args...)
		},
	)

	// 1. Check only
	if onlyCheck {
		fmt.Println("🔍 正在检查更新...")
		info := updater.Check(true)
		if info.Error != "" {
			return fmt.Errorf("检查更新失败: %s", info.Error)
		}
		if !info.UpdateAvailable {
			fmt.Printf("✅ 已是最新版本 (v%s)。\n", core.Version)
			return nil
		}
		fmt.Printf("📦 发现新版本: %s\n", info.LatestVersion)
		fmt.Printf("   当前版本: v%s\n", core.Version)
		fmt.Printf("   下载地址: %s\n", info.LatestURL)
		fmt.Println("\nℹ️  运行 'mindx upgrade'（不带 --check）即可安装更新。")
		return nil
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	// 2. 获取待应用的更新二进制，来源优先级：
	//    本地归档（--from，MindX 壳静默链用随包归档，零下载）>
	//    已持久化暂存（--yes 第二跳，直接消费 --stage 的落盘成果）>
	//    网络下载（收入持久暂存区，暂存不再随进程退出蒸发）
	var stageBinary, version string
	switch {
	case fromArchive != "":
		if fromVersion == "" {
			return fmt.Errorf("--from 需同时指定 --version（本地归档对应的版本号）")
		}
		fmt.Printf("📦 从本地归档暂存更新: %s\n", fromArchive)
		bin, stageErr := updater.StageFromArchive(fromArchive, fromVersion)
		if stageErr != nil {
			return fmt.Errorf("从本地归档暂存更新失败: %w", stageErr)
		}
		stageBinary, version = bin, fromVersion
		fmt.Printf("✅ 新版本 v%s 暂存就绪。\n", version)
	default:
		if yes {
			if bin, ver := updater.Staged(); bin != "" {
				stageBinary, version = bin, ver
				fmt.Printf("📦 应用已暂存的更新: v%s\n", version)
			}
		}
		if stageBinary == "" {
			bin, ver, acquireErr := acquireFromNetwork(updater, ctx)
			if acquireErr != nil {
				return acquireErr
			}
			if bin == "" {
				fmt.Printf("✅ 已是最新版本 (v%s)。\n", core.Version)
				return nil
			}
			stageBinary, version = bin, ver
		}
	}

	// 3. --stage：只就绪不应用（MindX 壳的静默链用：下载完提示用户一键应用）
	if stageOnly {
		fmt.Printf("ℹ️  已暂存待应用。运行 'mindx upgrade --yes' 即可应用（服务将短暂重启）。\n")
		return nil
	}

	// 4. 用户确认——停服务是用户的权力，升级进程无权擅自暂停任何服务
	if !yes {
		fmt.Print("\n应用更新需要短暂停止后台服务（通常数秒），继续? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("ℹ️  已取消应用。新版本仍处于暂存状态，随时可运行 'mindx upgrade --yes' 应用。")
			return nil
		}
	}

	// 5. Apply：停服 → 秒级替换 → 拉回。停了服务就要负责到底：
	// 替换失败必须恢复服务（实证事故——停服后下载失败，daemon 躺尸无人拉回）。
	return applyUpgrade(updater, stageBinary, version)
}

// acquireFromNetwork 网络链路：检查 → 下载 → 收入持久暂存区。
// 返回空 binary 表示已是最新（无更新可装）。
func acquireFromNetwork(updater *update.Updater, ctx context.Context) (string, string, error) {
	fmt.Println("🔍 正在检查更新...")
	info := updater.Check(true)
	if info.Error != "" {
		return "", "", fmt.Errorf("检查更新失败: %s", info.Error)
	}
	if !info.UpdateAvailable {
		return "", "", nil
	}
	fmt.Printf("📦 发现新版本: %s\n", info.LatestVersion)
	fmt.Printf("   下载地址: %s\n", info.LatestURL)

	fmt.Println("\n⬇️  正在下载更新（后台服务运行不受影响）...")
	tmpBinary, version, cleanup, err := updater.Download(ctx)
	defer cleanup()
	if err != nil {
		return "", "", fmt.Errorf("下载更新失败（服务未受影响，可稍后重试）: %w", err)
	}
	if tmpBinary == "" {
		return "", "", nil
	}
	bin, err := updater.PersistStage(tmpBinary, version)
	if err != nil {
		return "", "", fmt.Errorf("暂存更新失败: %w", err)
	}
	fmt.Printf("✅ 新版本 v%s 下载就绪。\n", version)
	return bin, version, nil
}

func applyUpgrade(updater *update.Updater, stageBinary, version string) error {
	daemonWasRunning := false
	status, err := setup.CheckDaemon()
	if err == nil && status == setup.DaemonRunning {
		fmt.Println("  → 正在停止后台服务...")
		if err := setup.StopDaemon(); err != nil {
			// 停不下来就中止：旧服务可能仍占用端口，此时替换二进制后新服务起不来。
			return fmt.Errorf("无法停止正在运行的后台服务，本次升级已中止: %w\n旧服务可能仍占用端口，请手动运行 'mindx stop' 后重试", err)
		}
		daemonWasRunning = true
		fmt.Println("  ✅ 后台服务已停止")
	}

	if err := updater.InstallFromStage(stageBinary, version); err != nil {
		if daemonWasRunning {
			fmt.Println("⚠️  安装失败，正在恢复后台服务...")
			if startErr := setup.StartDaemon(); startErr != nil {
				return fmt.Errorf("安装更新失败: %v\n且后台服务自动恢复失败: %v\n请手动运行 'mindx start' 启动服务", err, startErr)
			}
			fmt.Println("  ✅ 后台服务已恢复（版本保持不变）")
		}
		return fmt.Errorf("安装更新失败: %w", err)
	}

	fmt.Println("✅ 更新安装成功。")
	updater.ClearStage()

	if daemonWasRunning {
		fmt.Println("  → 正在重启后台服务...")
		if err := setup.StartDaemon(); err != nil {
			fmt.Printf("⚠️  后台服务重启失败: %v\n", err)
			fmt.Println("   请手动运行 'mindx restart'。")
		} else {
			fmt.Println("  ✅ 后台服务已重启")
		}
	} else {
		fmt.Println("ℹ️  升级前后台服务未在运行，需要时运行 'mindx start' 启动。")
	}

	return nil
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Only check for updates, don't install")
	upgradeCmd.Flags().Bool("stage", false, "Download and stage the update only; apply later with --yes")
	upgradeCmd.Flags().Bool("yes", false, "Apply without confirmation (service restarts briefly)")
	upgradeCmd.Flags().String("from", "", "Stage from a local release archive instead of downloading (MindX shell bundled kernel)")
	upgradeCmd.Flags().String("version", "", "Version of the --from archive (required with --from)")
	rootCmd.AddCommand(upgradeCmd)
}
