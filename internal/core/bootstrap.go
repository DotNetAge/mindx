package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Bootstrap(embeddedFS fs.FS, workspaceDir string) (*MindxConfig, error) {
	if !WorkspaceExists(workspaceDir) {
		fmt.Println("🔧 首次运行，正在初始化工作目录...")
		if err := ExtractWorkspace(embeddedFS, workspaceDir); err != nil {
			return nil, fmt.Errorf("初始化工作目录失败: %w", err)
		}
		fmt.Println("✅ 工作目录初始化完成:", workspaceDir)
	}

	os.Setenv("MINDX_WORKSPACE", workspaceDir)

	cfg, err := LoadMindxConfig(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("加载 mindx.json 失败: %w", err)
	}

	if !cfg.Initialized {
		fmt.Print("\n⚙️  检测到首次运行，进入配置向导...\n\n")

		settingsDir := filepath.Join(workspaceDir, "settings")
		modelsPath := filepath.Join(settingsDir, "models.yml")
		agentsDir := filepath.Join(workspaceDir, "agents")

		result := runFirstRunWizard(modelsPath, agentsDir, cfg)
		if result.Err != nil {
			return nil, fmt.Errorf("配置向导异常: %w", result.Err)
		}

		if err := ApplyFirstRunResult(result, modelsPath, agentsDir, cfg); err != nil {
			return nil, fmt.Errorf("应用首次配置失败: %w", err)
		}

		fmt.Print("\n✅ 配置完成！正在启动 MindX...\n\n")
	}

	if cfg.LastAgent != "" {
		os.Setenv("MINDX_MASTER", cfg.LastAgent)
	}

	return cfg, nil
}
