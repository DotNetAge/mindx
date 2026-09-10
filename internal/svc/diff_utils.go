package svc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aymanbagabas/go-udiff"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/diffutil"
)

// fileDiffInfo holds per-file diff data emitted via RespFileModified.
type fileDiffInfo struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsNew     bool   `json:"isNew"`
}

// computeFileDiff reads the current file and its backup (if exists) to compute diff stats.
func computeFileDiff(sess *goharnesssession.Session, filePath string) fileDiffInfo {
	info := fileDiffInfo{Path: filePath}

	current, err := os.ReadFile(filePath)
	if err != nil {
		return info
	}
	newContent := string(current)

	sessionDir := sess.SessionDir()
	if sessionDir == "" {
		// No session dir — can't find backups, treat as new
		lines := strings.Split(newContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		info.IsNew = true
		info.Additions = len(lines)
		info.Diff = buildNewFileDiff(filePath, lines)
		return info
	}

	// 备份读取统一走 Session.BackupPathForRead（原始文件名 + 绝对路径散列，
	// 新键缺失时回退旧键 basename.bak）：与 TrackModify/Confirm/Rollback 共用
	// 同一键规则，防止不同目录同名文件互相覆盖备份造成 diff 错乱、回滚串文件；
	// 旧键回退保证升级前活跃会话的存量备份仍可读取，diff 不被误判为全量新增。
	backupPath := sess.BackupPathForRead(filePath)
	oldData, oldErr := os.ReadFile(backupPath)
	if oldErr != nil {
		// No backup — new file
		lines := strings.Split(newContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		info.IsNew = true
		info.Additions = len(lines)
		info.Diff = buildNewFileDiff(filePath, lines)
		return info
	}

	oldContent := string(oldData)
	// 用 go-udiff 生成标准 unified diff（正确 hunk 定位 + 完整上下文行），
	// 供前端 reverseUnifiedDiff 反向还原修改前快照；旧 buildUnifiedDiff 只输出
	// 变更行、hunk 头却写死覆盖整文件，前端无法校验还原。
	// go-udiff 在「相近修改合并进同一 hunk」时后续 hunk 头行号会偏小（上游未修），
	// 必须经 NormalizeHunkHeaders 归一化后再下发。
	info.Diff = diffutil.NormalizeHunkHeaders(udiff.Unified("a/"+filePath, "b/"+filePath, oldContent, newContent))
	info.Additions, info.Deletions = countDiffLines(oldContent, newContent)
	return info
}

// buildNewFileDiff generates a unified-diff-style string for a newly created file.
func buildNewFileDiff(filePath string, lines []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n", filepath.Base(filePath)))
	b.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
	for _, line := range lines {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// countDiffLines counts added and removed lines.
func countDiffLines(oldContent, newContent string) (additions, deletions int) {
	oldSet := make(map[string]int)
	for _, l := range strings.Split(oldContent, "\n") {
		oldSet[l]++
	}
	for _, l := range strings.Split(newContent, "\n") {
		if _, exists := oldSet[l]; exists {
			oldSet[l]--
		} else {
			additions++
		}
	}
	for _, count := range oldSet {
		if count > 0 {
			deletions += count
		}
	}
	return additions, deletions
}
