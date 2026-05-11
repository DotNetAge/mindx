package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func ExtractWorkspace(embeddedFS fs.FS, workspaceDir string) error {
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("创建工作目录失败 %s: %w", workspaceDir, err)
	}

	return fs.WalkDir(embeddedFS, "runtime", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel("runtime", path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(workspaceDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if _, statErr := os.Stat(targetPath); statErr == nil {
			return nil
		}

		data, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return fmt.Errorf("读取嵌入文件失败 %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		return os.WriteFile(targetPath, data, 0644)
	})
}

func WorkspaceExists(workspaceDir string) bool {
	info, err := os.Stat(workspaceDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}
