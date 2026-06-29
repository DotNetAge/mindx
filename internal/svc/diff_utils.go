package svc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goharnesssession "github.com/DotNetAge/goharness/session"
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

	backupPath := filepath.Join(sessionDir, "backup", filepath.Base(filePath)+".bak")
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
	info.Diff = buildUnifiedDiff(filePath, oldContent, newContent)
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

// buildUnifiedDiff generates a basic unified diff string for a modified file.
func buildUnifiedDiff(filePath, oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", filepath.Base(filePath), filepath.Base(filePath)))

	// Simple line-by-line diff: scan for changes and emit hunks.
	type diffLine struct {
		kind byte // ' ', '+', '-'
		text string
	}

	var diff []diffLine

	// Build a simple LCS-based diff
	// First pass: mark unchanged, added, removed
	oldUsed := make([]bool, len(oldLines))
	newUsed := make([]bool, len(newLines))

	// Match identical lines in order
	ni := 0
	for oi := 0; oi < len(oldLines); oi++ {
		if ni >= len(newLines) {
			break
		}
		if oldLines[oi] == newLines[ni] {
			oldUsed[oi] = true
			newUsed[ni] = true
			ni++
		} else {
			// Try to find this old line later in new lines
			found := false
			for nj := ni + 1; nj < len(newLines); nj++ {
				if oldLines[oi] == newLines[nj] {
					// Mark skipped new lines as additions
					for nk := ni; nk < nj; nk++ {
						if !newUsed[nk] {
							newUsed[nk] = true
							diff = append(diff, diffLine{kind: '+', text: newLines[nk]})
						}
					}
					oldUsed[oi] = true
					newUsed[nj] = true
					diff = append(diff, diffLine{kind: ' ', text: oldLines[oi]})
					ni = nj + 1
					found = true
					break
				}
			}
			if !found {
				diff = append(diff, diffLine{kind: '-', text: oldLines[oi]})
			}
		}
	}

	// Remaining new lines are additions
	for ; ni < len(newLines); ni++ {
		if !newUsed[ni] {
			diff = append(diff, diffLine{kind: '+', text: newLines[ni]})
		}
	}
	// Remaining old lines are deletions
	for oi := 0; oi < len(oldLines); oi++ {
		if !oldUsed[oi] {
			// Check if already added as deletion
			alreadyAdded := false
			for _, d := range diff {
				if d.kind == '-' && d.text == oldLines[oi] {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				diff = append(diff, diffLine{kind: '-', text: oldLines[oi]})
			}
		}
	}

	if len(diff) == 0 {
		return ""
	}

	// Emit hunks with context
	b.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))
	for _, d := range diff {
		b.WriteByte(d.kind)
		b.WriteString(d.text)
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

// splitLines splits content into lines, dropping the trailing empty line.
func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
