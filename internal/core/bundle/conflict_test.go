package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

// 版本比较是 smart 策略的唯一"升级"依据，边界要钉死。
func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.4", true},
		{"1.0.4", "1.0.4", false},
		{"1.0.4", "1.0.1", false},
		{"1.0.10", "1.0.9", false},
		{"1.9", "1.10", true},
		{"v1.0.2", "1.0.3", true},
		{"1.0.0-beta.1", "1.0.0", false},
		{"", "1.0.0", false},
		{"1.0.0", "", false},
		{"abc", "1.0.0", false},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

// 四态决策：无改动不写、本地旧则覆盖、本地改过保留、默认语义报错。
func TestDecideConflict(t *testing.T) {
	cases := []struct {
		name       string
		policy     ConflictPolicy
		overwrite  bool
		identical  bool
		local, pkg string
		want       ConflictDecision
	}{
		{"显式覆盖优先", ConflictSmart, true, true, "2.0.0", "1.0.0", ConflictOverwrite},
		{"内容一致跳过", ConflictSmart, false, true, "1.0.0", "1.0.0", ConflictSkipIdentical},
		{"本地旧则覆盖", ConflictSmart, false, false, "1.0.1", "1.0.4", ConflictOverwrite},
		{"本地同版有改动保留", ConflictSmart, false, false, "1.0.4", "1.0.4", ConflictKeepLocal},
		{"本地更新保留", ConflictSmart, false, false, "1.1.0", "1.0.4", ConflictKeepLocal},
		{"版本不明保留", ConflictSmart, false, false, "", "1.0.4", ConflictKeepLocal},
		{"默认语义报错", ConflictAsk, false, false, "1.0.1", "1.0.4", ConflictAskDecision},
		{"默认语义一致仍放行", ConflictAsk, false, true, "1.0.1", "1.0.4", ConflictSkipIdentical},
	}
	for _, c := range cases {
		got := decideConflict(c.policy, c.overwrite, c.identical, c.local, c.pkg)
		if got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

// localVersion 必须读到 Agent / Team / Skill 三种定义文件的版本（技能在 metadata 块里）。
func TestLocalVersionPerKind(t *testing.T) {
	dir := t.TempDir()

	agentDir := filepath.Join(dir, "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(agentDir, identityFileName), "---\nname: a\nversion: 1.2.3\n---\nbody\n")

	teamDir := filepath.Join(dir, "team")
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(teamDir, teamDocFileName), "---\nname: t\nversion: 0.9.0\n---\nbody\n")

	skillDir := filepath.Join(dir, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(skillDir, skillDocName), "---\nname: s\nmetadata:\n  version: 0.1.1\n---\nbody\n")

	if got := localVersion(KindAgent, agentDir); got != "1.2.3" {
		t.Errorf("agent version = %q", got)
	}
	if got := localVersion(KindTeam, teamDir); got != "0.9.0" {
		t.Errorf("team version = %q", got)
	}
	if got := localVersion(KindSkill, skillDir); got != "0.1.1" {
		t.Errorf("skill version = %q", got)
	}
	if got := localVersion(KindAgent, filepath.Join(dir, "missing")); got != "" {
		t.Errorf("missing dir version = %q, want empty", got)
	}
}
