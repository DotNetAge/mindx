// Package diffutil 提供 go-udiff 输出的 hunk 头归一化修正层。
// go-udiff（x/tools internal/diff 移植）存在上游未修缺陷：多个相近修改被
// 合并进同一 hunk 时，后续 hunk 头的新侧起始行号（ToLine/newStart）少计
// 合并进来的上下文行数（toUnified 的 "within range" 分支不累加 toLine），
// 消费方按行号定位即失败。
package diffutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(,\d+)? \+(\d+)(,\d+)? @@`)

// NormalizeHunkHeaders 修正 hunk 头的新侧起始行号。
// 依据：上游 old 侧行号（FromLine）直接取自原文件位置，可信；只有 new 侧
// 行号受缺陷影响，而两者之差恒等于「该 hunk 之前的累计插入行数 - 累计删除
// 行数」，可从 diff 体精确计数，无需原始文件内容：
//
//	newStart_i = oldStart_i + Σins(之前 hunk) - Σdel(之前 hunk)
//
// "+0,0"（空 new 侧的 GNU 特例格式）保留原样；对本来正确的 diff 是恒等变换。
func NormalizeHunkHeaders(diff string) string {
	lines := strings.Split(diff, "\n")
	ins, del := 0, 0
	inHunk := false
	for i, ln := range lines {
		if m := hunkHeaderRe.FindStringSubmatch(ln); m != nil {
			inHunk = true
			newStart, _ := strconv.Atoi(m[3])
			// oldStart==0 仅出现在 GNU 空侧特例（如新建文件 "@@ -0,0 +1,N @@"），
			// 此时 newStart 本来正确，公式会把它算成 0，必须跳过
			if oldStart, _ := strconv.Atoi(m[1]); oldStart > 0 && newStart > 0 {
				if fixed := oldStart + ins - del; fixed > 0 && fixed != newStart {
					// 只替换行号，保留原行的 count 格式（",N" 或省略）
					lines[i] = fmt.Sprintf("@@ -%d%s +%d%s @@", oldStart, m[2], fixed, m[4])
				}
			}
			continue
		}
		if !inHunk {
			continue // 文件头（---/+++）不属于任何 hunk
		}
		if strings.HasPrefix(ln, "\\") {
			continue // "\ No newline at end of file"，非内容行
		}
		switch {
		case strings.HasPrefix(ln, "-"):
			del++
		case strings.HasPrefix(ln, "+"):
			ins++
		default:
			ins++
			del++
		}
	}
	return strings.Join(lines, "\n")
}
