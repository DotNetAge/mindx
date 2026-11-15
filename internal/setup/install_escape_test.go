package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateInstallDir 安装目录的合法性边界。
//
// 回归点（P1 §4.13）：`--dir` 在静默安装模式下由上层 Client 子进程传入，
// 以前原样返回，直接拼进 shell rc / plist / systemd unit。
func TestValidateInstallDir(t *testing.T) {
	if _, err := resolveInstallDir("relative/bin"); err == nil {
		t.Error("相对路径应被拒绝")
	}
	if _, err := resolveInstallDir("/tmp/a\nb"); err == nil {
		t.Error("含换行的路径应被拒绝")
	}
	if _, err := resolveInstallDir("   "); err == nil {
		t.Error("空白路径应被拒绝")
	}
	got, err := resolveInstallDir("/opt/mindx/bin/")
	if err != nil {
		t.Fatalf("绝对路径应被接受: %v", err)
	}
	if got != "/opt/mindx/bin" {
		t.Errorf("应规范化掉尾斜杠，得到 %q", got)
	}
	// 空格与非 ASCII 是合法路径，不得因过滤而拒绝
	if _, err := resolveInstallDir("/Users/ray/My Tools/工具"); err != nil {
		t.Errorf("含空格与非 ASCII 的绝对路径应被接受: %v", err)
	}
}

// TestAddUnixPathEscapesInjection 写入 shell rc 的路径必须是字面量：
// 目录名里的引号不能闭合引号去执行命令。
func TestAddUnixPathEscapesInjection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	canary := filepath.Join(home, "pwned")
	// 典型注入载荷：闭合双引号后追加命令
	dir := "/tmp/evil\"; touch " + canary + "; echo \""

	added, err := addUnixPath(dir)
	if err != nil {
		t.Fatalf("addUnixPath: %v", err)
	}
	_ = added

	rc := filepath.Join(home, ".bashrc")
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatalf("读取 rc 失败: %v", err)
	}
	if !strings.Contains(string(data), "PATH=") {
		t.Fatalf("rc 未写入 PATH 行: %q", string(data))
	}

	// 真执行一遍 rc：若转义有漏，canary 会被创建
	out, runErr := exec.Command("sh", "-c", "set -e; . "+rc).CombinedOutput()
	if runErr != nil {
		t.Fatalf("rc 无法被 shell 正常解析: %v\n%s", runErr, out)
	}
	if _, statErr := os.Stat(canary); statErr == nil {
		t.Fatal("rc 内容被执行了注入命令")
	}
}

// TestSystemdQuote 与 xmlEscapeText：两种宿主格式的字面量转义。
func TestSystemdQuote(t *testing.T) {
	if got := systemdQuote(`/a b/mindx`); got != `"/a b/mindx"` {
		t.Errorf("systemdQuote = %s", got)
	}
	if got := systemdQuote(`/a"b\c`); got != `"/a\"b\\c"` {
		t.Errorf("systemdQuote 转义 = %s", got)
	}
	escaped := xmlEscapeText(`/a&b<c>"d"`)
	if strings.ContainsAny(escaped, "<>") {
		t.Errorf("xmlEscapeText 仍残留未转义字符: %s", escaped)
	}
	if !strings.Contains(escaped, "&amp;") || !strings.Contains(escaped, "&lt;") {
		t.Errorf("xmlEscapeText = %s", escaped)
	}
}
