package setup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
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
			return info, fmt.Errorf("Python 安装后仍无法检测到，请手动安装")
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
			return info, fmt.Errorf("create virtual environment: %w\n%s", err, string(out))
		}
	}

	info.VenvPath = venvPath

	skillsDir := filepath.Join(workspaceDir, "skills")
	reqFiles := findRequirementsFiles(skillsDir)
	if len(reqFiles) > 0 {
		if err := InstallPipRequirements(venvPath, reqFiles...); err != nil {
			return info, fmt.Errorf("安装 skill 依赖失败: %w", err)
		}
	}

	return info, nil
}

func findRequirementsFiles(root string) []string {
	var files []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

func InstallPipRequirements(venvPath string, reqFiles ...string) error {
	pipPath := GetVenvPipPath(venvPath)

	if _, err := os.Stat(pipPath); os.IsNotExist(err) {
		return fmt.Errorf("pip not found in virtual environment: %s", pipPath)
	}

	for _, reqFile := range reqFiles {
		if _, err := os.Stat(reqFile); os.IsNotExist(err) {
			continue
		}

		cmd := exec.Command(pipPath, "install", "-r", reqFile)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
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
		return fmt.Errorf("不支持自动安装 Python 的平台: %s", runtime.GOOS)
	}
}

func installPythonMacOS() error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("Homebrew 未安装，请访问 python.org 手动安装 Python")
	}
	cmd := exec.Command("brew", "install", "python3")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		return fmt.Errorf("未找到支持的包管理器 (apt-get, yum, dnf)")
	}

	parts := strings.Split(pkgArgs, " ")
	args := append([]string{pkgManager}, parts...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("需要 root 权限手动安装: sudo %s %s", pkgManager, pkgArgs)
	}
	return nil
}

func installPythonWindows() error {
	if execCmd("winget", "--version") == nil {
		cmd := exec.Command("winget", "install", "Python.Python.3.12", "--accept-source-agreements", "--silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("请访问 python.org 手动安装 Python")
}

func execCmd(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
