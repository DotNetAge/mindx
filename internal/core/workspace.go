package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/DotNetAge/mindx/internal/i18n"
)

// DefaultUserPrefsDir returns the platform-appropriate user preferences directory.
// This is the root directory for all MindX user data (config, sessions, memory, skills, etc.).
//   - macOS/Linux: ~/.mindx
//   - Windows:     %APPDATA%\mindx  (typically C:\Users\<user>\AppData\Roaming\mindx)
func DefaultUserPrefsDir() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "mindx")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mindx")
	}
	return filepath.Join(home, ".mindx")
}

func ExtractWorkspace(embeddedFS fs.FS, workspaceDir string) error {
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
			return fmt.Errorf(i18n.T("error.workspace.init"), workspaceDir, err)
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
				return fmt.Errorf(i18n.T("error.embedded.file.read"), path, err)
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

// SyncEmbeddedFile 从 embedded FS 中读取指定文件并强制写入目标路径（覆盖已有文件）。
// 用于确保用户目录中的配置文件始终与内置版本一致（如 providers.yml 的环境变量名更新）。
func SyncEmbeddedFile(embeddedFS fs.FS, embeddedPath, targetPath string) error {
	data, err := fs.ReadFile(embeddedFS, embeddedPath)
	if err != nil {
		return fmt.Errorf(i18n.T("error.embedded.file.read"), embeddedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf(i18n.T("error.target.dir.create"), err)
	}
	return os.WriteFile(targetPath, data, 0644)
}
