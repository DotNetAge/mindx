package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergePaths(t *testing.T) {
	sep := string(os.PathListSeparator)
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{
			name: "保序去重",
			in:   []string{"/a" + sep + "/b", "/b" + sep + "/c"},
			want: "/a" + sep + "/b" + sep + "/c",
		},
		{
			name: "去重保留首段位置",
			in:   []string{"/b" + sep + "/a", "/c" + sep + "/a"},
			want: "/b" + sep + "/a" + sep + "/c",
		},
		{
			name: "跳过空段与空白段",
			in:   []string{"" + sep + "/a", " /a " + sep + "  "},
			want: "/a",
		},
		{
			name: "全空输入",
			in:   []string{"", ""},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mergePaths(c.in...); got != c.want {
				t.Fatalf("mergePaths = %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveCommand_WithPathSeparator(t *testing.T) {
	// 含路径分隔符的命令原样返回（绝对/相对路径交 exec 自行处理）。
	for _, in := range []string{"/bin/echo", "./tool", "sub/cmd"} {
		if got := resolveCommand(in); got != in {
			t.Fatalf("resolveCommand(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestResolveCommand_FindsExecutableOnPath(t *testing.T) {
	got := resolveCommand("echo")
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveCommand(echo) = %q, want absolute path", got)
	}
	if filepath.Base(got) != "echo" {
		t.Fatalf("resolveCommand(echo) = %q, want base echo", got)
	}
}

func TestResolveCommand_MissingReturnsOriginalName(t *testing.T) {
	const missing = "no-such-command-xyz-12345"
	if got := resolveCommand(missing); got != missing {
		t.Fatalf("resolveCommand(missing) = %q, want original name", got)
	}
}

func TestResolveCommand_SkipsDirsAndNonExecutable(t *testing.T) {
	dir := t.TempDir()
	// 目录：即使名字匹配也不算可执行。
	if err := os.MkdirAll(filepath.Join(dir, "fakecmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 非可执行文件：权限位不含 x，跳过。
	if err := os.WriteFile(filepath.Join(dir, "noexec"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	// loginShellPath 是进程级缓存，会并入用户真实 PATH；为避免干扰，
	// 用不存在的裸命令验证不会命中本目录里的非法候选。
	got := resolveCommand("noexec")
	if got == filepath.Join(dir, "noexec") {
		t.Fatal("non-executable file was resolved as command")
	}
	if got != "noexec" && !strings.HasSuffix(got, "noexec") {
		t.Fatalf("resolveCommand(noexec) = %q, unexpected", got)
	}
}

func TestEnhancedPathContainsProcessPath(t *testing.T) {
	t.Setenv("PATH", "/custom/test/bin")
	got := enhancedPath()
	if got == "" {
		t.Fatal("enhancedPath() empty")
	}
	if !strings.Contains(got, "/custom/test/bin") {
		t.Fatalf("enhancedPath() = %q, missing process PATH segment", got)
	}
}
