package core

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/logging"
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

// fileHash returns the lowercase hex SHA-256 of data.
func fileHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// SyncRuntimeAssets synchronises embedded runtime assets to disk when the app
// version has changed. Behaviour differs by directory:
//
//   - schemas, web — unconditionally overwritten (program assets)
//   - agents, skills — overwritten only if the file has not been modified
//     by the user, determined by comparing the on-disk SHA-256 against the
//     checksum stored in cfg.AgentSkillChecksums at deploy time.
//
// Settings and data directories are intentionally skipped.
//
// Returns true when any file was synced.
func SyncRuntimeAssets(embeddedFS fs.FS, workspaceDir, appVersion string, cfg *MindxConfig) (bool, error) {
	if appVersion == "" || appVersion == cfg.RuntimeSyncedVersion {
		return false, nil
	}

	// Directories that should be unconditionally overwritten
	overwriteDirs := map[string]bool{
		"schemas": true,
		"web":     true,
	}

	// Directories that should be protected from overwriting user edits
	checkDirs := map[string]bool{
		"agents": true,
		"skills": true,
	}

	// Initialise checksum map on first call
	if cfg.AgentSkillChecksums == nil {
		cfg.AgentSkillChecksums = make(map[string]string)
	}

	synced := false
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(workspaceDir, "logs", "mindx.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})

	err := fs.WalkDir(embeddedFS, "runtime", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, rErr := filepath.Rel("runtime", path)
		if rErr != nil {
			return rErr
		}
		if relPath == "." {
			return nil
		}

		firstComponent := relPath
		if idx := strings.IndexByte(relPath, '/'); idx > 0 {
			firstComponent = relPath[:idx]
		}

		targetPath := filepath.Join(workspaceDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		embeddedData, readErr := fs.ReadFile(embeddedFS, path)
		if readErr != nil {
			return fmt.Errorf(i18n.T("error.embedded.file.read"), path, readErr)
		}

		// ── Unconditionally overwrite (schemas, web) ───────────────
		if overwriteDirs[firstComponent] {
			synced = true
			return os.WriteFile(targetPath, embeddedData, 0644)
		}

		// ── Check before overwrite (agents, skills) ────────────────
		if checkDirs[firstComponent] {
			deployedData, statErr := os.ReadFile(targetPath)
			if os.IsNotExist(statErr) {
				// New file — create
				synced = true
				cfg.AgentSkillChecksums[relPath] = fileHash(embeddedData)
				return os.WriteFile(targetPath, embeddedData, 0644)
			}
			if statErr != nil {
				return statErr
			}

			embeddedHash := fileHash(embeddedData)
			deployedHash := fileHash(deployedData)

			if deployedHash == embeddedHash {
				// Content already matches — nothing to do
				return nil
			}

			// Check if the deployed file was last written by us
			storedHash, hasStored := cfg.AgentSkillChecksums[relPath]
			if !hasStored || deployedHash == storedHash {
				// File is in the state we deployed — safe to overwrite
				synced = true
				cfg.AgentSkillChecksums[relPath] = embeddedHash
				return os.WriteFile(targetPath, embeddedData, 0644)
			}

			// User has modified the file — skip, warn once
			logger.Warn("sync: skipping user-modified file", "file", relPath)
			return nil
		}

		// Skip all other directories (settings, data, etc.)
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return synced, err
	}

	cfg.RuntimeSyncedVersion = appVersion
	if saveErr := cfg.Save(); saveErr != nil {
		return synced, fmt.Errorf("save config after runtime sync: %w", saveErr)
	}
	return synced, nil
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
