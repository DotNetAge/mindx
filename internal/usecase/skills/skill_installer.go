package skills

import (
	"fmt"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"os/exec"
)

// Installer 安装管理器
type Installer struct {
	logger logging.Logger
}

// NewInstaller 创建安装管理器
func NewInstaller(logger logging.Logger) *Installer {
	return &Installer{
		logger: logger.Named("Installer"),
	}
}

// InstallDependency 安装依赖
func (i *Installer) InstallDependency(method entity.InstallMethod) error {
	var cmd *exec.Cmd

	switch method.Kind {
	case "brew":
		if method.Formula != "" {
			cmd = exec.Command("brew", "install", method.Formula)
		} else {
			cmd = exec.Command("brew", "install", method.Package)
		}
	case "apt":
		cmd = exec.Command("sudo", "apt", "install", "-y", method.Package)
	case "yum", "dnf":
		cmd = exec.Command("sudo", method.Kind, "install", "-y", method.Package)
	case "npm":
		cmd = exec.Command("npm", "install", "-g", method.Package)
	case "pip", "pip3":
		pip := method.Kind
		if _, err := exec.LookPath(pip); err != nil {
			pip = "pip3"
		}
		cmd = exec.Command(pip, "install", method.Package)
	case "snap":
		cmd = exec.Command("sudo", "snap", "install", method.Package)
	case "choco":
		cmd = exec.Command("choco", "install", "-y", method.Package)
	default:
		return fmt.Errorf("不支持的安装方式: %s", method.Kind)
	}

	// 设置命令输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	i.logger.Info(i18n.T("skill.installing_dep"), logging.String("package", method.Package), logging.String("method", method.Kind))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装失败: %w", err)
	}

	i.logger.Info(i18n.T("skill.dep_installed"), logging.String("package", method.Package))
	return nil
}
