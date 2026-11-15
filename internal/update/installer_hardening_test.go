package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// makeTarGz 造一个只含单条目的 tar.gz，条目类型与内容可控。
func makeTarGz(t *testing.T, name string, typ byte, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mindx_"+versionTagForTest+".tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: typ}
	if typ == tar.TypeSymlink {
		hdr.Linkname = "/bin/sh"
		hdr.Size = 0
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if typ == tar.TypeReg {
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

const versionTagForTest = "test"

// elfPayload 造一段带 ELF 魔数、体积达标的伪造二进制。
func elfPayload(size int) []byte {
	buf := make([]byte, size)
	copy(buf, []byte{0x7f, 'E', 'L', 'F'})
	return buf
}

// TestExtractTarGzRejectsSymlinkEntry 归档里名为 mindx 的 symlink 条目不得被当作二进制。
//
// 回归点（P1 §4.2）：以前只比对 filepath.Base(header.Name)，不看 Typeflag，
// 探针实测 symlink 条目会写出 0 字节文件且不报错，随后顶替正在运行的二进制。
func TestExtractTarGzRejectsSymlinkEntry(t *testing.T) {
	archive := makeTarGz(t, "mindx", tar.TypeSymlink, nil)
	dest := t.TempDir()

	if _, err := extractTarGz(archive, dest); err == nil {
		t.Fatal("symlink 条目应被跳过，最终应报「归档内找不到二进制」")
	}
	if _, statErr := os.Stat(filepath.Join(dest, "mindx")); statErr == nil {
		t.Fatal("symlink 条目不应产出文件")
	}
}

// TestExtractTarGzRejectsTinyBinary 体积异常小的「二进制」不得被接受。
func TestExtractTarGzRejectsTinyBinary(t *testing.T) {
	archive := makeTarGz(t, "mindx", tar.TypeReg, []byte("not a binary"))
	dest := t.TempDir()

	if _, err := extractTarGz(archive, dest); err == nil {
		t.Fatal("体积极小的条目应被拒绝")
	}
}

// TestExtractTarGzAcceptsRealBinary 正常条目（普通文件 + ELF 魔数 + 体积达标）可通过。
func TestExtractTarGzAcceptsRealBinary(t *testing.T) {
	archive := makeTarGz(t, "mindx", tar.TypeReg, elfPayload(minBinarySize+1024))
	dest := t.TempDir()

	got, err := extractTarGz(archive, dest)
	if err != nil {
		t.Fatalf("合法归档应被接受: %v", err)
	}
	if filepath.Base(got) != "mindx" {
		t.Fatalf("解出路径异常: %s", got)
	}
}

// TestValidateExecutable 魔数与体积双重判定。
func TestValidateExecutable(t *testing.T) {
	dir := t.TempDir()

	small := filepath.Join(dir, "small")
	if err := os.WriteFile(small, elfPayload(1024), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutable(small); err == nil {
		t.Error("体积不足应被拒绝")
	}

	text := filepath.Join(dir, "text")
	if err := os.WriteFile(text, bytes.Repeat([]byte("A"), minBinarySize+16), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutable(text); err == nil {
		t.Error("非可执行魔数应被拒绝")
	}

	ok := filepath.Join(dir, "ok")
	if err := os.WriteFile(ok, elfPayload(minBinarySize+16), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutable(ok); err != nil {
		t.Errorf("合法 ELF 应通过: %v", err)
	}
}

// TestValidateDownloadURL 升级包只从 GitHub 官方域下载。
//
// 回归点（P1 §4.3）：以前直接用 release API 给的 BrowserDownloadURL，
// 不校验 host 与协议。
func TestValidateDownloadURL(t *testing.T) {
	bad := []string{
		"https://evil.example.net/mindx.tar.gz",
		"http://github.com/a.zip",
		"https://github.com.evil.net/a.zip",
		"https://objects.githubusercontent.com@evil.net/a.zip",
		"",
	}
	for _, u := range bad {
		if err := validateDownloadURL(u); err == nil {
			t.Errorf("地址 %q 应被拒绝", u)
		}
	}
	good := []string{
		"https://github.com/DotNetAge/mindx/releases/download/v1/x.tar.gz",
		"https://objects.githubusercontent.com/abc",
		"https://api.github.com/repos/a/b/releases/latest",
	}
	for _, u := range good {
		if err := validateDownloadURL(u); err != nil {
			t.Errorf("地址 %q 应被接受: %v", u, err)
		}
	}
}

// TestInstallFromStageRejectsDowngrade 版本不得倒退。
func TestInstallFromStageRejectsDowngrade(t *testing.T) {
	u := &Updater{currentVer: "2.6.26"}
	err := u.InstallFromStage("/nonexistent/mindx", "2.0.0")
	if err == nil {
		t.Fatal("降级安装应被拒绝")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("降级")) {
		t.Fatalf("错误信息应说明降级被拒: %v", err)
	}
}
