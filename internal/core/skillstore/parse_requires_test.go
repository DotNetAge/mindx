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

// TestLoadSkillDetail_RequiresOAuthAndInstall frontmatter 全量解析：
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

// TestExtractRequires_EnvKeyCompat 存量 env 键兼容（envs 优先，env 兜底）。
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

// TestValidBinName 白名单形态判定。
func TestValidBinName(t *testing.T) {
	valid := []string{"node", "python3", "docker-compose", "g++", "go1.26", "pnpm", "a"}
	for _, name := range valid {
		require.True(t, ValidBinName(name), "%q 应被接受", name)
	}

	invalid := []string{
		"", " node", "node ", "node;rm -rf /", "sh -c whoami",
		"$(whoami)", "`id`", "node|curl", "node&&ls", "a'b", `a"b`,
		"node>/tmp/x", "../../bin/sh", "/bin/sh", "-rf", "node\nrm",
	}
	for _, name := range invalid {
		require.False(t, ValidBinName(name), "%q 应被拒绝", name)
	}
}

// TestResolveBinRejectsInjection requires.bins 是第三方 SKILL.md 声明的字符串，
// 会被拼进 `zsh -lic "which <name>"` 在登录 shell 里执行（登录 shell 会 source
// ~/.zprofile、~/.zshrc）。非白名单名字必须在进入 shell 之前被拒绝。
//
// 判定依据是拒绝原因而非副作用：名字带分号/替换语法时，第一层 PATH 查找会
// 失败、第二层 shell 解析可能因登录 shell 启动超时（2s）而未真正执行命令，
// 只靠「命令有没有跑」会得到与时序相关、甚至空过的结论。这里要求错误文案
// 明确指出名称不合法——校验一旦被移除，返回的就会是「未找到」或 shell 原错。
// 探针文件仅作附加断言：一旦真的执行了命令，立即暴露。
func TestResolveBinRejectsInjection(t *testing.T) {
	dir := t.TempDir()
	canary := filepath.Join(dir, "pwned")

	cases := []string{
		"sh -c 'touch " + canary + "'",
		"x; touch " + canary,
		"x&&touch " + canary,
		"x|tee " + canary,
		"$(touch " + canary + ")",
		"`touch " + canary + "`",
		"../../bin/sh",
		"/bin/sh",
	}
	for _, name := range cases {
		path, err := resolveBin(name)
		if err == nil {
			t.Fatalf("恶意依赖名 %q 应被拒绝，实际解析为 %q", name, path)
		}
		require.Contains(t, err.Error(), "不合法", "恶意依赖名 %q 必须在进入 shell 前被拒", name)
		if _, statErr := os.Stat(canary); statErr == nil {
			t.Fatalf("依赖名 %q 触发了命令执行", name)
		}
	}

	// 合法名字仍走正常三层查找（Layer 1 即可命中，不依赖登录 shell）。
	if _, err := resolveBin("sh"); err != nil {
		t.Fatalf("合法依赖名 sh 应能解析: %v", err)
	}
}

// TestMissingBinsReportsInvalidName 非法依赖名以专用文案进入警告，而非笼统「未找到」。
func TestMissingBinsReportsInvalidName(t *testing.T) {
	missing := MissingBins(&Requires{Bins: []string{"node;rm -rf /"}})
	require.Equal(t, []string{"node;rm -rf /"}, missing)

	warnings := verifyDependencies(&Requires{Bins: []string{"node;rm -rf /"}})
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "不合法")
}
