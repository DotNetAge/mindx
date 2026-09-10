package svc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aymanbagabas/go-udiff"

	"github.com/DotNetAge/mindx/internal/diffutil"
)

// 构造三处修改：前两处相距 4 行（<= 6，go-udiff 会把它们合并进同一个 hunk），
// 第三处距离较远（应另开 hunk）——翻译类批量编辑的典型分布
func buildThreeEditContent() (oldContent, newContent string) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	oldContent = strings.Join(lines, "\n") + "\n"
	lines[4] = "CHANGED A"
	lines[8] = "CHANGED B"
	lines[24] = "CHANGED C"
	newContent = strings.Join(lines, "\n") + "\n"
	return oldContent, newContent
}

// TestGoUdiffHunkHeaderBug 复现 go-udiff 上游缺陷（铁证）：
// 多个相近修改合并进同一 hunk 后，后续 hunk 头 newStart 少计合并进来的
// 上下文行数（toUnified 的 "within range" 分支不累加 toLine）。
// 期望第二块头为 @@ -22,7 +22,7 @@，缺陷输出却是 +19（少了 3 行上下文）。
func TestGoUdiffHunkHeaderBug(t *testing.T) {
	oldContent, newContent := buildThreeEditContent()
	raw := udiff.Unified("a", "b", oldContent, newContent)
	if !strings.Contains(raw, "@@ -22,7 +19,7 @@") {
		t.Errorf("预期复现上游缺陷输出 +19,7，实际输出:\n%s", raw)
	}
}

// TestNormalizeHunkHeadersFixes 验证修正层把缺陷行号纠正为正确值
func TestNormalizeHunkHeadersFixes(t *testing.T) {
	oldContent, newContent := buildThreeEditContent()
	fixed := diffutil.NormalizeHunkHeaders(udiff.Unified("a", "b", oldContent, newContent))
	if !strings.Contains(fixed, "@@ -22,7 +22,7 @@") {
		t.Errorf("预期修正为 +22,7，实际输出:\n%s", fixed)
	}
	// 首块头不受缺陷影响，应保持原样
	if !strings.Contains(fixed, "@@ -2,11 +2,11 @@") {
		t.Errorf("首块头应保持 -2,11 +2,11，实际输出:\n%s", fixed)
	}
}

// TestNormalizeHunkHeadersIdentity 对本来正确的 diff 应为恒等变换
func TestNormalizeHunkHeadersIdentity(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	oldContent := strings.Join(lines, "\n") + "\n"
	// 只有两处且相距很远（>= 7 行）时不会触发合并，diff 本身正确
	lines[2] = "CHANGED A"
	lines[20] = "CHANGED B"
	newContent := strings.Join(lines, "\n") + "\n"

	raw := udiff.Unified("a", "b", oldContent, newContent)
	if got := diffutil.NormalizeHunkHeaders(raw); got != raw {
		t.Errorf("正确 diff 应为恒等变换。\n原始:\n%s\n归一化后:\n%s", raw, got)
	}
}

// TestNormalizePreservesNewFileDiff 新建文件 diff（GNU 空侧特例 @@ -0,0 +1,N @@）
// 的 newStart 本来正确，归一化必须原样保留。
// 回归防护：曾因缺少 oldStart>0 条件把 newStart 由 1 错改为 0，
// 触发前端 reverseUnifiedDiff 的 newStart<=0 守卫，新建文件条目退回普通编辑器。
func TestNormalizePreservesNewFileDiff(t *testing.T) {
	newFileDiff := "--- a/new.txt\n+++ b/new.txt\n@@ -0,0 +1,3 @@\n+alpha\n+beta\n+gamma\n"
	if got := diffutil.NormalizeHunkHeaders(newFileDiff); got != newFileDiff {
		t.Errorf("新建文件 diff 应为恒等变换。\n原始:\n%s\n归一化后:\n%s", newFileDiff, got)
	}
	// 前端同款守卫：newStart==0 的 diff 会被判 null，故必须保证修正后仍 > 0
	if _, ok := reverseApply(newFileDiff, "alpha\nbeta\ngamma\n"); !ok {
		t.Errorf("新建文件 diff 应能反向还原为空原文")
	}
}

// reverseApply 与前端 reverseUnifiedDiff 同款算法：
// 以当前内容为基准反向应用 diff，还原修改前原文；任一块校验失败返回 false
func reverseApply(diff, current string) (string, bool) {
	hunkHeaderRe := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	type hunk struct {
		newStart int
		lines    []string
	}
	var hunks []*hunk
	var cur *hunk
	for _, raw := range strings.Split(diff, "\n") {
		if m := hunkHeaderRe.FindStringSubmatch(raw); m != nil {
			start, _ := strconv.Atoi(m[1])
			cur = &hunk{newStart: start}
			hunks = append(hunks, cur)
			continue
		}
		if cur != nil && raw != "" {
			cur.lines = append(cur.lines, raw)
		}
	}
	if len(hunks) == 0 {
		return "", false
	}
	row := strings.Split(current, "\n")
	for i := len(hunks) - 1; i >= 0; i-- {
		h := hunks[i]
		if h.newStart <= 0 {
			return "", false
		}
		var newSide, oldSide []string
		for _, l := range h.lines {
			switch {
			case strings.HasPrefix(l, "\\"):
				continue
			case strings.HasPrefix(l, "-"):
				oldSide = append(oldSide, l[1:])
			case strings.HasPrefix(l, "+"):
				newSide = append(newSide, l[1:])
			default:
				text := strings.TrimPrefix(l, " ")
				newSide = append(newSide, text)
				oldSide = append(oldSide, text)
			}
		}
		start := h.newStart - 1
		if start+len(newSide) > len(row) {
			return "", false
		}
		if strings.Join(row[start:start+len(newSide)], "\n") != strings.Join(newSide, "\n") {
			return "", false
		}
		row = append(row[:start], append(append([]string{}, oldSide...), row[start+len(newSide):]...)...)
	}
	return strings.Join(row, "\n"), true
}

// TestReverseApplyEndToEnd 端到端验证：以新内容为基准反向应用修正后的 diff，
// 必须精确还原出修改前原文；未修正的缺陷 diff 则必须还原失败。
func TestReverseApplyEndToEnd(t *testing.T) {
	oldContent, newContent := buildThreeEditContent()
	raw := udiff.Unified("a", "b", oldContent, newContent)

	// 缺陷 diff：第二块 +19 定位错误，反向还原必须失败（这正是线上现象）
	if _, ok := reverseApply(raw, newContent); ok {
		t.Errorf("未修正的缺陷 diff 不应能反向还原成功")
	}

	// 修正后：必须成功且结果与原文逐字节一致
	fixed := diffutil.NormalizeHunkHeaders(raw)
	got, ok := reverseApply(fixed, newContent)
	if !ok {
		t.Fatalf("修正后的 diff 反向还原失败")
	}
	if got != oldContent {
		t.Errorf("反向还原结果与原文不一致。\n期望:\n%s\n实际:\n%s", oldContent, got)
	}
}
