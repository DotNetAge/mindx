package diffutil

import (
	"strings"
	"testing"
)

func TestNormalizeHunkHeaders_IdentityOnCorrectDiff(t *testing.T) {
	// 正确的 diff（唯一 hunk，newStart 本来就对）应原样返回。
	in := "--- a/f.txt\n+++ b/f.txt\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	if got := NormalizeHunkHeaders(in); got != in {
		t.Fatalf("identity diff changed:\nwant %q\ngot  %q", in, got)
	}
}

func TestNormalizeHunkHeaders_FixesMergedHunkStart(t *testing.T) {
	// 上游缺陷场景：两个相近修改合并，第二个 hunk 的 newStart 少计了
	// hunk1 净删除的 1 行（ins=2, del=3）。
	in := "--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,3 +1,2 @@\n a\n b\n-c\n" +
		"@@ -4,2 +4,2 @@\n d\n-e\n+E\n"
	want := "--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,3 +1,2 @@\n a\n b\n-c\n" +
		"@@ -4,2 +3,2 @@\n d\n-e\n+E\n"
	if got := NormalizeHunkHeaders(in); got != want {
		t.Fatalf("merged hunk start not fixed:\nwant %q\ngot  %q", want, got)
	}
}

func TestNormalizeHunkHeaders_CumulativeAcrossHunks(t *testing.T) {
	// 多 hunk 时累计净变化：hunk1 净 -1（-c），hunk2 净 0，hunk3 的
	// newStart = oldStart3 + (ins1+ins2) - (del1+del2) = 8 + 4 - 5 = 7。
	in := "--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,3 +1,2 @@\n a\n b\n-c\n" +
		"@@ -4,2 +3,2 @@\n d\n-e\n+E\n" +
		"@@ -8,2 +8,2 @@\n g\n-h\n+H\n"
	want := strings.Replace(in, "@@ -8,2 +8,2 @@", "@@ -8,2 +7,2 @@", 1)
	if got := NormalizeHunkHeaders(in); got != want {
		t.Fatalf("cumulative offset wrong:\nwant %q\ngot  %q", want, got)
	}
}

func TestNormalizeHunkHeaders_SkipsEmptyNewSideSpecialCase(t *testing.T) {
	// "@@ -0,0 +1,N @@"（新建文件）oldStart==0，必须原样保留。
	in := "--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1,2 @@\n+a\n+b\n"
	if got := NormalizeHunkHeaders(in); got != in {
		t.Fatalf("empty old side special case changed:\nwant %q\ngot  %q", in, got)
	}
}

func TestNormalizeHunkHeaders_PreservesCountFormat(t *testing.T) {
	// 无 count 的 "@@ -N +M @@" 格式要保留（不补 ",1"）。
	// hunk1: context a（ins1 del1）、-b（del2）→ 净 -1；hunk2 修正 +4→+3。
	in := "--- a/f.txt\n+++ b/f.txt\n@@ -1,2 +1,1 @@\n a\n-b\n@@ -4 +4 @@\n-d\n+e\n"
	want := "--- a/f.txt\n+++ b/f.txt\n@@ -1,2 +1,1 @@\n a\n-b\n@@ -4 +3 @@\n-d\n+e\n"
	if got := NormalizeHunkHeaders(in); got != want {
		t.Fatalf("count format not preserved:\nwant %q\ngot  %q", want, got)
	}
}

func TestNormalizeHunkHeaders_IgnoresFileHeaderAndNoNewlineMarkers(t *testing.T) {
	// 文件头（---/+++）与 "\ No newline" 不参与行数累计：若把 "--- a/f.txt"
	// 误记为删除行，hunk2 的修正值会多 1。
	in := "--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,2 +1,2 @@\n a\n-b\n+B\n" +
		"\\ No newline at end of file\n" +
		"@@ -4,1 +4,1 @@\n d\n"
	// hunk1: ins=2（a、+B）、del=2（-b 加上……不对——a 是 context 各计 1）。
	// 实际：context a（ins+del 各 1）、-b（del）、+B（ins）→ ins=2, del=2，
	// hunk2 fixed = 4+2-2 = 4 == newStart，不改写；重点是不因头行/反斜杠行漂移。
	want := in
	if got := NormalizeHunkHeaders(in); got != want {
		t.Fatalf("file header or no-newline marker miscounted:\nwant %q\ngot  %q", want, got)
	}
}

func TestNormalizeHunkHeaders_EmptyInput(t *testing.T) {
	if got := NormalizeHunkHeaders(""); got != "" {
		t.Fatalf("empty input changed: %q", got)
	}
}
