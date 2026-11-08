package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

func DetectPython() core.PythonConfig {
	info := core.PythonConfig{}

	pythonCommands := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		pythonCommands = []string{"python", "python3", "py"}
	}

	for _, cmd := range pythonCommands {
		out, err := exec.Command(cmd, "--version").Output()
		if err != nil {
			continue
		}
		version := strings.TrimSpace(string(out))
		version = strings.TrimPrefix(version, "Python ")
		info.Detected = true
		info.Version = version
		return info
	}

	return info
}

func SetupPython(workspaceDir string) (core.PythonConfig, error) {
	info := DetectPython()
	if !info.Detected {
		if err := InstallPython(); err != nil {
			return info, err
		}
		info = DetectPython()
		if !info.Detected {
			return info, errors.New(i18n.T("setup.python.install.detect.failed"))
		}
	}

	venvPath := filepath.Join(workspaceDir, ".venv")

	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		pythonCmd := "python3"
		if runtime.GOOS == "windows" {
			pythonCmd = "python"
		}

		cmd := exec.Command(pythonCmd, "-m", "venv", venvPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return info, fmt.Errorf(i18n.T("setup.python.venv.create.failed"), err, string(out))
		}
	}

	info.VenvPath = venvPath

	skillsDir := filepath.Join(workspaceDir, "skills")
	reqFiles := findRequirementsFiles(skillsDir)
	if len(reqFiles) > 0 {
		if err := InstallPipRequirements(venvPath, reqFiles...); err != nil {
			return info, fmt.Errorf(i18n.T("setup.python.skill.install.failed"), err)
		}
	}

	return info, nil
}

func findRequirementsFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "requirements.txt") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func GetVenvPipPath(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "pip")
	}
	return filepath.Join(venvPath, "bin", "pip")
}

// pythonInstallTimeout / pipInstallTimeout 给外部下载命令设硬上限：网络不可用时
// brew/apt/winget/pip 会长时间不返回，上层装配流程（mindx doctor --fix）跟着一起挂住，
// 用户只能看到一直转圈。超时后统一收敛成「暂时无法下载 + 手动安装办法」的可操作提示。
const (
	pythonInstallTimeout = 5 * time.Minute
	pipInstallTimeout    = 5 * time.Minute
)

// errCommandTimeout 标识「命令在超时预算内没有返回」，与其它失败分开收口：
// 前者翻译成网络不可用的可操作提示，后者保留原始错误便于排查。
var errCommandTimeout = errors.New("命令执行超时")

// runWithTimeout 执行外部命令并强制限时。
func runWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errCommandTimeout
		}
		return err
	}
	return nil
}

func InstallPipRequirements(venvPath string, reqFiles ...string) error {
	pipPath := GetVenvPipPath(venvPath)

	if _, err := os.Stat(pipPath); os.IsNotExist(err) {
		return fmt.Errorf("pip not found in virtual environment: %s", pipPath)
	}

	for _, reqFile := range reqFiles {
		if _, err := os.Stat(reqFile); os.IsNotExist(err) {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), pipInstallTimeout)
		// 显式写死 pip 的下载预算：单次读超时 15 秒、最多重试 3 次，
		// 避免默认重试策略在网络抖动时把等待拖长
		cmd := exec.CommandContext(ctx, pipPath, "install", "--timeout", "15", "--retries", "3", "-r", reqFile)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			if timedOut {
				return fmt.Errorf(i18n.T("setup.python.pip.network"), reqFile)
			}
			return fmt.Errorf("pip install requirements (%s): %w\n%s", reqFile, err, stderr.String())
		}
	}

	return nil
}

func InstallPython() error {
	switch runtime.GOOS {
	case "darwin":
		return installPythonMacOS()
	case "linux":
		return installPythonLinux()
	case "windows":
		return installPythonWindows()
	default:
		return fmt.Errorf(i18n.T("setup.python.unsupported.platform"), runtime.GOOS)
	}
}

func installPythonMacOS() error {
	if _, err := exec.LookPath("brew"); err != nil {
		return errors.New(i18n.T("setup.python.homebrew.missing"))
	}
	if err := runWithTimeout(pythonInstallTimeout, "brew", "install", "python3"); err != nil {
		if errors.Is(err, errCommandTimeout) {
			return errors.New(i18n.T("setup.python.install.network.macos"))
		}
		return fmt.Errorf(i18n.T("setup.python.install.failed.macos"), err)
	}
	return nil
}

func installPythonLinux() error {
	var pkgManager, pkgArgs string
	if execCmd("apt-get", "--version") == nil {
		pkgManager = "apt-get"
		pkgArgs = "install -y python3 python3-venv python3-pip"
	} else if execCmd("yum", "--version") == nil {
		pkgManager = "yum"
		pkgArgs = "install -y python3 python3-pip"
	} else if execCmd("dnf", "--version") == nil {
		pkgManager = "dnf"
		pkgArgs = "install -y python3 python3-pip"
	} else {
		return errors.New(i18n.T("setup.linux.pkgmanager.missing"))
	}

	parts := strings.Split(pkgArgs, " ")
	if err := runWithTimeout(pythonInstallTimeout, pkgManager, parts...); err != nil {
		if errors.Is(err, errCommandTimeout) {
			return errors.New(i18n.T("setup.python.install.network.linux"))
		}
		return fmt.Errorf(i18n.T("setup.python.install.failed.linux"), err)
	}
	return nil
}

func installPythonWindows() error {
	if execCmd("winget", "--version") == nil {
		err := runWithTimeout(pythonInstallTimeout, "winget", "install", "Python.Python.3.12", "--accept-source-agreements", "--silent")
		if err == nil {
			return nil
		}
		if errors.Is(err, errCommandTimeout) {
			return errors.New(i18n.T("setup.python.install.network.windows"))
		}
	}
	return errors.New(i18n.T("setup.python.manual.install"))
}

func execCmd(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
