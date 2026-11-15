package svc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNewFileDiff(t *testing.T) {
	got := buildNewFileDiff("/tmp/proj/src/main.go", []string{"package main", "func main() {}"})
	want := "--- /dev/null\n+++ b/main.go\n@@ -0,0 +1,2 @@\n+package main\n+func main() {}\n"
	if got != want {
		t.Errorf("buildNewFileDiff =\n%q\nwant\n%q", got, want)
	}
}

func TestCountDiffLines(t *testing.T) {
	old := "a\nb\nc\n"
	// 新增 x,y；b 保留；c 删除
	add, del := countDiffLines(old, "a\nb\nx\ny\n")
	if add != 2 || del != 1 {
		t.Errorf("count = %d/%d, want 2/1", add, del)
	}

	// 相同内容 → 0/0
	if add, del = countDiffLines(old, old); add != 0 || del != 0 {
		t.Errorf("identical = %d/%d, want 0/0", add, del)
	}

	// 多重集合语义：旧行出现 2 次删 1 次只算 1 删除
	add, del = countDiffLines("dup\ndup\n", "dup\n")
	if add != 0 || del != 1 {
		t.Errorf("multiset = %d/%d, want 0/1", add, del)
	}

	// 空内容
	add, del = countDiffLines("", "only\n")
	if add != 1 || del != 0 {
		t.Errorf("empty old = %d/%d, want 1/0", add, del)
	}
}

func TestComputeFileDiff_NewFile(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")

	// 无备份的新文件
	file := filepath.Join(t.TempDir(), "fresh.txt")
	if err := os.WriteFile(file, []byte("line1\nline2"), 0644); err != nil {
		t.Fatal(err)
	}
	info := computeFileDiff(sess, file)
	if !info.IsNew || info.Additions != 2 || info.Deletions != 0 {
		t.Errorf("new file info = %+v", info)
	}
	if !strings.HasPrefix(info.Diff, "--- /dev/null\n+++ b/fresh.txt") {
		t.Errorf("new file diff = %q", info.Diff)
	}

	// 文件不存在 → 零值信息
	info = computeFileDiff(sess, filepath.Join(t.TempDir(), "missing.txt"))
	if info.Diff != "" || info.Additions != 0 || info.IsNew {
		t.Errorf("missing file info = %+v", info)
	}
}

func TestComputeFileDiff_Modified(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	sess := loadTestSession(t, d, sid, "tester")

	base := t.TempDir()
	file := filepath.Join(base, "code.txt")
	newContent := "kept\nchanged-new\n"
	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}
	// 手工落备份（BackupPathForRead 只做路径推导，写入即模拟 TrackModify 留下的备份）
	backupPath := sess.BackupPathForRead(file)
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("kept\nchanged-old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info := computeFileDiff(sess, file)
	if info.IsNew {
		t.Errorf("modified file should not be marked new: %+v", info)
	}
	if info.Additions != 1 || info.Deletions != 1 {
		t.Errorf("stats = +%d/-%d, want +1/-1", info.Additions, info.Deletions)
	}
	// 标准 unified diff：hunk 头 + 加减行
	if !strings.Contains(info.Diff, "@@") || !strings.Contains(info.Diff, "-changed-old") || !strings.Contains(info.Diff, "+changed-new") {
		t.Errorf("diff = %q", info.Diff)
	}
	if !strings.Contains(info.Diff, "a/"+file) {
		t.Errorf("diff header should reference original path: %q", info.Diff)
	}
}
