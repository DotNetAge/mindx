// Node 运行环境装配（2026-10-24 用户定案）：Node 与 Python 同为技能执行标配，
// 环境责任归 mindx 本体——doctor 检查项 + --fix 自动安装（对齐 python_setup 模式）；
// GUI Client（mindx-work）的一键安装仅为兜底，不承担主装配职责。
// Windows 原生不做：Windows 上跑的是 WSL（Linux），Node 由 WSL 内的 Linux 侧装配。
package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/i18n"
)

// DetectNode 检测 PATH 上的 node，返回版本号（如 v22.4.0）；未安装返回空串。
func DetectNode() string {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// InstallNode 经系统包管理器安装 Node.js（macOS: Homebrew；Linux: apt/dnf/yum）。
// 已安装时直接返回（幂等，绝不覆盖用户已有环境）。Windows 原生不支持。
func InstallNode() error {
	if DetectNode() != "" {
		return nil // 已装：零改动
	}
	switch runtime.GOOS {
	case "darwin":
		return installNodeMacOS()
	case "linux":
		return installNodeLinux()
	default:
		return fmt.Errorf(i18n.T("setup.node.unsupported.platform"), runtime.GOOS)
	}
}

func installNodeMacOS() error {
	if _, err := exec.LookPath("brew"); err != nil {
		return errors.New(i18n.T("setup.node.homebrew.missing"))
	}
	cmd := exec.Command("brew", "install", "node")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(i18n.T("setup.node.install.failed"), err)
	}
	return nil
}

func installNodeLinux() error {
	switch {
	case execCmd("apt-get", "--version") == nil:
		// apt 先刷新索引（失败不阻断——本地缓存可能仍可用）
		_ = exec.Command("apt-get", "update").Run()
		return runPkgInstall("apt-get", []string{"install", "-y", "nodejs", "npm"})
	case execCmd("dnf", "--version") == nil:
		return runPkgInstall("dnf", []string{"install", "-y", "nodejs"})
	case execCmd("yum", "--version") == nil:
		return runPkgInstall("yum", []string{"install", "-y", "nodejs"})
	default:
		return errors.New(i18n.T("setup.linux.pkgmanager.missing"))
	}
}

// runPkgInstall 经包管理器安装（无 sudo——daemon 场景多由系统服务/终端用户运行，
// 权限不足时报错提示手动；GUI 兜底路径自带 sudo 提权逻辑）
func runPkgInstall(manager string, args []string) error {
	cmd := exec.Command(manager, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(i18n.T("setup.linux.root.required"), manager, strings.Join(args, " "))
	}
	return nil
}
