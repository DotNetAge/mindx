package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteLocalVersionRejectsInjection 版本号来自远端市场响应，直接拼进 frontmatter
// 会被用来注入任意 YAML 字段（例如换行 + `hired: true` 就等于替用户雇佣）。
func TestWriteLocalVersionRejectsInjection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, identityFileName)
	original := "---\nname: x\n---\n\n正文\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("准备失败: %v", err)
	}

	bad := []string{
		"1.0.0\nhired: true",
		"1.0.0\nrole: 被改写的角色",
		"../1.0.0", // 路径穿越意图
		"1.0.0;rm -rf /",
		"",
		strings.Repeat("9", 80),
	}
	for _, v := range bad {
		if err := WriteLocalVersion(KindAgent, dir, v); err == nil {
			t.Fatalf("版本号 %q 应被拒绝写入", v)
		}
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatalf("非法版本号不应改动文件：%s", string(data))
	}

	// 合法形态：semver 与历史时间戳、预发布号
	for _, v := range []string{"1.0.0", "1.2.3-beta.1", "20261116-123456", "0.0.1+build.7"} {
		if err := WriteLocalVersion(KindAgent, dir, v); err != nil {
			t.Fatalf("合法版本号 %q 写入失败: %v", v, err)
		}
		got, ok, err := LocalVersion(KindAgent, dir)
		if err != nil || !ok || got != v {
			t.Fatalf("写回后读不到 %q（得 %q ok=%v err=%v）", v, got, ok, err)
		}
	}
}

// TestWriteLocalVersionKeepsOtherFields 只动 version 一行，其余字节原样保留（含 hired）。
func TestWriteLocalVersionKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, identityFileName)
	original := "---\nname: x\nrole: 画师\nhired: true\nicon: data:image/png;base64,AAA\n---\n\n正文\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("准备失败: %v", err)
	}
	if err := WriteLocalVersion(KindAgent, dir, "1.0.0"); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "hired: true") {
		t.Fatalf("不应改动雇佣态：%s", string(data))
	}
	if !strings.Contains(string(data), "version: 1.0.0") {
		t.Fatalf("应写入版本号：%s", string(data))
	}
	if !strings.Contains(string(data), "icon: data:image/png;base64,AAA") {
		t.Fatalf("不应改动其他字段：%s", string(data))
	}
}
