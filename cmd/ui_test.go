package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveWorkspacePath 验收：open 参数闸——相对路径落 cwd、越界路径拒绝。
func TestResolveWorkspacePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}

	// 相对路径 → 绝对路径且落在 cwd 内
	got, err := resolveWorkspacePath("report.md")
	if err != nil {
		t.Fatalf("相对路径应受理: %v", err)
	}
	if want := filepath.Join(cwd, "report.md"); got != want {
		t.Fatalf("相对路径应解析为 %s, 实际 %s", want, got)
	}

	// 绝对路径直接受理
	if got, err = resolveWorkspacePath(cwd); err != nil || got != cwd {
		t.Fatalf("绝对路径应原样受理: got=%s err=%v", got, err)
	}

	// 越界路径（..）拒绝
	if _, err = resolveWorkspacePath("../outside.md"); err == nil {
		t.Fatal("越界路径应被拒绝")
	}

	// 伪装 cwd 前缀的越界路径（../cwd-evil）拒绝
	if _, err = resolveWorkspacePath(filepath.Join("..", filepath.Base(cwd)+"-evil", "x.md")); err == nil {
		t.Fatal("同前缀越界路径应被拒绝")
	}

	// 空路径拒绝
	if _, err = resolveWorkspacePath("  "); err == nil || !strings.Contains(err.Error(), "路径为空") {
		t.Fatalf("空路径应被拒绝, 实际 err=%v", err)
	}
}
