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

// TestExtractRequires_EnvsObjectForm envs 对象形态 {name, prompt, url, sensitive}：
// 对象条目进 Envs 且 name 并入 Env（MissingEnvs / verifyDependencies 出口兼容）；
// 字符串条目只进 Env；name 缺失整条忽略（不产生半残声明）。
func TestExtractRequires_EnvsObjectForm(t *testing.T) {
	r := extractRequires(map[string]any{
		"requires": map[string]any{
			"envs": []any{
				"LARK_APP_ID",
				map[string]any{
					"name":      "LARK_APP_SECRET",
					"prompt":    "在飞书开放平台的应用凭证页获取",
					"url":       "https://open.feishu.cn/app",
					"sensitive": true,
				},
				map[string]any{"prompt": "缺少 name 的条目应整条忽略"},
				map[string]any{"name": "  "},
			},
		},
	})
	require.NotNil(t, r)
	require.Len(t, r.Envs, 1)
	require.Equal(t, "LARK_APP_SECRET", r.Envs[0].Name)
	require.Equal(t, "在飞书开放平台的应用凭证页获取", r.Envs[0].Prompt)
	require.Equal(t, "https://open.feishu.cn/app", r.Envs[0].URL)
	require.True(t, r.Envs[0].Sensitive)
	// 双出口：字符串条目 + 对象条目 name 都并入 Env
	require.Equal(t, []string{"LARK_APP_ID", "LARK_APP_SECRET"}, r.Env)
}

// TestExtractRequires_EnvsLegacyStringOnly 存量纯字符串名单解析后 Envs 为空（不影响既有调用方）。
func TestExtractRequires_EnvsLegacyStringOnly(t *testing.T) {
	r := extractRequires(map[string]any{
		"requires": map[string]any{
			"envs": []any{"MINDX_API_KEY"},
		},
	})
	require.NotNil(t, r)
	require.Equal(t, []string{"MINDX_API_KEY"}, r.Env)
	require.Empty(t, r.Envs)
}
