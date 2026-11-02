package update

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func newTestUpdater(t *testing.T) *Updater {
	t.Helper()
	// 走 NewUpdater 与生产同路径（logf 缺省注入）；saveConfig 测试不需要
	return NewUpdater("1.0.0", "1.0.0", t.TempDir(), nil, nil)
}

// makeFakeTarGz 打一个内含 mindx 二进制占位文件的 tar.gz，模拟 MindX 壳随包归档。
func makeFakeTarGz(t *testing.T, dest, payload string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer func() { _ = f.Close() }()
	gzr := gzip.NewWriter(f)
	tw := tar.NewWriter(gzr)
	if err := tw.WriteHeader(&tar.Header{Name: "mindx", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzr.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
}

func TestStageFromArchiveAndStaged(t *testing.T) {
	u := newTestUpdater(t)
	archive := filepath.Join(t.TempDir(), "mindx-2.6.14-darwin-arm64.tar.gz")
	makeFakeTarGz(t, archive, "fake-binary-payload")

	staged, err := u.StageFromArchive(archive, "2.6.14")
	if err != nil {
		t.Fatalf("StageFromArchive: %v", err)
	}

	// 暂存二进制内容一致、可执行位在
	got, err := os.ReadFile(staged)
	if err != nil || string(got) != "fake-binary-payload" {
		t.Fatalf("staged binary content mismatch: err=%v got=%q", err, got)
	}
	if info, err := os.Stat(staged); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("staged binary perm = %v, err = %v", info, err)
	}

	// Staged() 能读回版本
	bin, ver := u.Staged()
	if bin != staged || ver != "2.6.14" {
		t.Fatalf("Staged() = (%q, %q), want (%q, 2.6.14)", bin, ver, staged)
	}
}

func TestStaged_MissingPieces(t *testing.T) {
	u := newTestUpdater(t)
	if bin, ver := u.Staged(); bin != "" || ver != "" {
		t.Fatalf("empty workspace Staged() = (%q, %q), want empty", bin, ver)
	}

	// 只有标记没有二进制 → 视为无暂存
	if err := os.MkdirAll(u.stageDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(u.stageVersionPath(), []byte(`{"version":"9.9.9"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if bin, _ := u.Staged(); bin != "" {
		t.Fatalf("marker without binary Staged() = %q, want empty", bin)
	}
}

func TestClearStage(t *testing.T) {
	u := newTestUpdater(t)
	archive := filepath.Join(t.TempDir(), "mindx-1.0.0.tar.gz")
	makeFakeTarGz(t, archive, "x")
	if _, err := u.StageFromArchive(archive, "1.0.0"); err != nil {
		t.Fatalf("StageFromArchive: %v", err)
	}
	u.ClearStage()
	if bin, _ := u.Staged(); bin != "" {
		t.Fatalf("after ClearStage Staged() = %q, want empty", bin)
	}
}

func TestPersistStage_CrossDeviceCopy(t *testing.T) {
	// 源在另一目录（模拟 os.MkdirTemp 与 ~/.mindx 不同文件系统），
	// PersistStage 必须以 copy 收编而非 rename
	u := newTestUpdater(t)
	tmpBin := filepath.Join(t.TempDir(), "mindx")
	if err := os.WriteFile(tmpBin, []byte("netbin"), 0o755); err != nil {
		t.Fatal(err)
	}
	staged, err := u.PersistStage(tmpBin, "2.0.0")
	if err != nil {
		t.Fatalf("PersistStage: %v", err)
	}
	if got, err := os.ReadFile(staged); err != nil || string(got) != "netbin" {
		t.Fatalf("staged content mismatch: err=%v got=%q", err, got)
	}
	// 源文件必须仍在（copy 语义，调用方的 cleanup 负责删临时目录）
	if _, err := os.Stat(tmpBin); err != nil {
		t.Fatalf("source binary removed by PersistStage: %v", err)
	}
}
