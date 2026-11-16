package teamstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionRoundTrip 团队定义版本随 TEAM.md 往返（与 Agent 同口径的血统载体）。
func TestVersionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := mustLoad(t, dir)
	team := &Team{
		Meta: TeamMeta{
			Name:        "beta",
			NickName:    "贝塔",
			Description: "带版本的团队",
			Leader:      "architect",
			Members:     []string{"architect", "backend-dev"},
			Version:     "2.0.1",
		},
		Profile: "团队定位正文",
	}
	if err := store.Save(team); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "beta", "TEAM.md"))
	if err != nil {
		t.Fatalf("读取 TEAM.md 失败: %v", err)
	}
	if !strings.Contains(string(raw), "version: 2.0.1") {
		t.Fatalf("frontmatter 未写入版本：%s", string(raw))
	}

	got := mustLoad(t, dir).Get("beta")
	if got == nil {
		t.Fatal("重载后未找到团队 beta")
	}
	if got.Meta.Version != "2.0.1" {
		t.Fatalf("版本丢失：%q", got.Meta.Version)
	}
}
