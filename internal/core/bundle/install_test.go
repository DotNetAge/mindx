package bundle

import (
	"path/filepath"
	"testing"
)

// TestManifestNameRejectsPathTraversal 包清单里的 name 是包内 JSON 提供的自由
// 字符串，会被直接拼进安装目录。不设边界就能把包内容写到目标目录之外。
func TestManifestNameRejectsPathTraversal(t *testing.T) {
	bad := []string{
		"../../../../tmp/evil",
		"..",
		"a/b",
		`a\b`,
		"/etc/passwd",
		".hidden",
		"名字",
	}
	for _, name := range bad {
		m := &Manifest{Format: PackageFormat, Kind: KindAgent, Name: name}
		if err := m.validate(); err == nil {
			t.Errorf("包名 %q 应被拒绝", name)
		}
	}

	good := []string{"my-agent", "agent_v2", "com.example.agent", "a", "Agent-1"}
	for _, name := range good {
		m := &Manifest{Format: PackageFormat, Kind: KindAgent, Name: name}
		if err := m.validate(); err != nil {
			t.Errorf("包名 %q 应被接受: %v", name, err)
		}
	}
}

// TestJoinUnderRejectsEscape 安装目录拼接的最后一道兜底。
func TestJoinUnderRejectsEscape(t *testing.T) {
	base := t.TempDir()
	if _, err := joinUnder(base, "../../evil"); err == nil {
		t.Fatal("越界包名应被拒绝")
	}
	got, err := joinUnder(base, "My-Agent")
	if err != nil {
		t.Fatalf("合法包名应被接受: %v", err)
	}
	if want := filepath.Join(base, "my-agent"); got != want {
		t.Fatalf("joined = %q, want %q", got, want)
	}
}
