package skillstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeSkillDir 落一个带 frontmatter 的临时技能目录，返回目录路径。
func writeSkillDir(t *testing.T, frontmatter string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestLoadSkillDetail_RequiresOAuthAndInstall 定稿 frontmatter 全量解析：
// requires.oauth（provider+env）、envs 名单、metadata.install 平台命令块。
func TestLoadSkillDetail_RequiresOAuthAndInstall(t *testing.T) {
	dir := writeSkillDir(t, `---
name: demo-skill
description: 测试技能
metadata:
  requires:
    bins: ["dotnet"]
    envs: ["MINDX_API_KEY"]
    oauth:
      provider: gemini
      env: GEMINI_TOKEN
  install:
    darwin: ["brew install dotnet"]
    linux: ["apt-get install -y dotnet-sdk-8.0"]
---
正文
`)
	detail, _, err := LoadSkillDetailFromDir(dir, "test")
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.NotNil(t, detail.Requires)
	require.Equal(t, []string{"dotnet"}, detail.Requires.Bins)
	require.Equal(t, []string{"MINDX_API_KEY"}, detail.Requires.Env)
	require.NotNil(t, detail.Requires.OAuth)
	require.Equal(t, "gemini", detail.Requires.OAuth.Provider)
	require.Equal(t, "GEMINI_TOKEN", detail.Requires.OAuth.Env)
	require.Equal(t, map[string][]string{
		"darwin": {"brew install dotnet"},
		"linux":  {"apt-get install -y dotnet-sdk-8.0"},
	}, detail.InstallCommands)
}

// TestExtractRequires_EnvKeyCompat 存量 env 键兼容（定稿 envs 优先，env 兜底）。
func TestExtractRequires_EnvKeyCompat(t *testing.T) {
	legacy := extractRequires(map[string]any{
		"requires": map[string]any{
			"env": []any{"A_LEGACY_ENV"},
		},
	})
	require.NotNil(t, legacy)
	require.Equal(t, []string{"A_LEGACY_ENV"}, legacy.Env)

	// envs 与 env 同时出现时 envs 优先
	both := extractRequires(map[string]any{
		"requires": map[string]any{
			"envs": []any{"A_NEW_ENV"},
			"env":  []any{"A_LEGACY_ENV"},
		},
	})
	require.NotNil(t, both)
	require.Equal(t, []string{"A_NEW_ENV"}, both.Env)
}

// TestExtractRequires_OAuthIncomplete oauth 两字段任一为空则整条忽略（不产生半残声明）。
func TestExtractRequires_OAuthIncomplete(t *testing.T) {
	none := extractRequires(map[string]any{
		"requires": map[string]any{
			"oauth": map[string]any{"provider": "gemini"},
		},
	})
	require.Nil(t, none)
}

// TestMissingBins 缺失二进制的结构化输出：存在的二进制不报，不存在的精确报出。
func TestMissingBins(t *testing.T) {
	missing := MissingBins(&Requires{Bins: []string{"sh", "mindx-no-such-bin-xyz"}})
	require.Equal(t, []string{"mindx-no-such-bin-xyz"}, missing)
	require.Nil(t, MissingBins(nil))
}
