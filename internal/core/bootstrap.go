package core

import (
	"fmt"
	"io/fs"
	"os"
)

func Bootstrap(embeddedFS fs.FS, workspaceDir string) (*MindxConfig, error) {
	if err := ExtractWorkspace(embeddedFS, workspaceDir); err != nil {
		return nil, fmt.Errorf("初始化工作目录失败: %w", err)
	}

	os.Setenv("MINDX_WORKSPACE", workspaceDir)

	cfg, err := LoadMindxConfig(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("加载 mindx.json 失败: %w", err)
	}

	return cfg, nil
}
