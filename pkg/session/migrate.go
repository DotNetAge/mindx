package session

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// MigrateLegacySessions 将旧全局布局的会话一次性搬迁到各工作目录分片。
//
// 旧布局：<legacyDir>/<agent>/<sessionID>/{session.yml, meta.json, ...}
// 新布局：<project_dir>/.sessions/<agent>/<sessionID>/（project_dir 取自 meta.json）
//
// 语义（与 agentstore 一次性迁移风格一致：一次搬迁不留双格式读路径）：
//   - meta.json 缺失/损坏/无 project_dir 的会话留守原地（日志 WARN，不阻塞启动）；
//   - 目标同名目录已存在（重名冲突）时保守留守，不覆盖；
//   - 每个会话按「登记清单 → 执行搬迁」顺序处理，任一步失败立即返回错误，
//     已完成的搬迁不回滚——流程幂等，下次启动对剩余会话重试；
//   - 全部搬迁完成且无留守时，旧目录整体更名为 <legacyDir>.migrated.bak 保留备份；
//     留守会话存在时旧目录保留原位（下次启动继续重试）。
//
// register 是搬迁成功后的清单登记回调（RoutedSessionStore.RegisterDir），
// 在 rename 之前调用——保证崩溃后重入时清单与磁盘状态一致。
func MigrateLegacySessions(legacyDir string, register func(absProjectDir string) error) (moved, kept int, err error) {
	if strings.TrimSpace(legacyDir) == "" {
		return 0, 0, fmt.Errorf("迁移源目录不能为空")
	}
	agentEntries, err := os.ReadDir(legacyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil // 无旧数据，无需迁移
		}
		return 0, 0, fmt.Errorf("读取迁移源目录 %s 失败: %w", legacyDir, err)
	}

	for _, agentEntry := range agentEntries {
		if !agentEntry.IsDir() {
			continue // 非会话载体（散文件等）忽略
		}
		agentDir := filepath.Join(legacyDir, agentEntry.Name())
		sessionEntries, readErr := os.ReadDir(agentDir)
		if readErr != nil {
			kept++
			log.Printf("[WARN] session-migrate: 读取智能体目录失败，留守 %s: %v", agentDir, readErr)
			continue
		}
		for _, sessionEntry := range sessionEntries {
			if !sessionEntry.IsDir() {
				continue
			}
			sessionDir := filepath.Join(agentDir, sessionEntry.Name())
			if _, statErr := os.Stat(filepath.Join(sessionDir, "meta.json")); statErr != nil {
				kept++
				log.Printf("[WARN] session-migrate: 会话缺 meta.json，留守 %s", sessionDir)
				continue
			}

			meta, loadErr := LoadSessionMeta(sessionDir)
			if loadErr != nil || strings.TrimSpace(meta.ProjectDir) == "" {
				kept++
				log.Printf("[WARN] session-migrate: 会话无 project_dir 无法归属，留守 %s", sessionDir)
				continue
			}

			projectDir := filepath.Clean(meta.ProjectDir)
			target := filepath.Join(projectDir, ".sessions", agentEntry.Name(), sessionEntry.Name())
			if _, statErr := os.Stat(target); statErr == nil {
				kept++
				log.Printf("[WARN] session-migrate: 目标目录已存在（重名冲突），留守 %s -> %s", sessionDir, target)
				continue
			}

			// 先登记清单（工作目录与 .sessions 分片随登记创建），再执行搬迁：
			// 崩溃窗口内最多出现「已登记未搬迁」（下次重试搬迁）或「搬迁失败」，
			// 不会出现「已搬迁未登记」（会话丢失可见性）。
			if regErr := register(projectDir); regErr != nil {
				return moved, kept, fmt.Errorf("登记工作目录 %s 失败: %w", projectDir, regErr)
			}
			if mvErr := moveDir(sessionDir, target); mvErr != nil {
				return moved, kept, fmt.Errorf("搬迁会话 %s -> %s 失败: %w", sessionDir, target, mvErr)
			}
			moved++
		}
	}

	// 全部搬迁完成且无留守：旧目录整体更名为备份目录，不留双格式读路径
	if moved > 0 && kept == 0 {
		backup := legacyDir + ".migrated.bak"
		if _, statErr := os.Stat(backup); statErr == nil {
			log.Printf("[WARN] session-migrate: 备份目录已存在，旧目录保留原位 %s", legacyDir)
		} else if renameErr := os.Rename(legacyDir, backup); renameErr != nil {
			log.Printf("[WARN] session-migrate: 旧目录更名备份失败（下次启动重试） %s -> %s: %v", legacyDir, backup, renameErr)
		} else {
			log.Printf("[INFO] session-migrate: 旧会话目录已备份为 %s", backup)
		}
	} else if kept > 0 {
		log.Printf("[WARN] session-migrate: %d 个会话留守旧目录（将在下次启动重试或需人工归属），%d 个已搬迁", kept, moved)
	}
	return moved, kept, nil
}

// moveDir 移动目录：优先同卷 rename（原子）；跨卷（EXDEV 等）失败时回退复制+删除。
func moveDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标父目录: %w", err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("跨卷复制: %w", err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("删除源目录: %w", err)
	}
	return nil
}

// copyDir 递归复制目录（保留文件权限）。
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
