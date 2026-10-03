package svc

import (
	"testing"

	"github.com/DotNetAge/goharness/skill"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
	"github.com/stretchr/testify/require"
)

// prepareTestDaemon 构造带 OAuth 注册表/凭据库/流程管理器的最小 Daemon。
func prepareTestDaemon(store *fakeCredStore, providers []*core.OAuthProviderConfig) *Daemon {
	return &Daemon{
		oauthCredStore: store,
		oauthRegistry:  core.NewOAuthProviderRegistry(providers),
		oauthFlows:     newOAuthFlowManager(),
	}
}

func prepareTestProvider(clientID string) *core.OAuthProviderConfig {
	return &core.OAuthProviderConfig{
		Name: "pv", Title: "Provider X", Kind: core.OAuthKindDeviceCode,
		TokenURL: "https://example.com/token", DeviceURL: "https://example.com/device",
		Interaction: core.OAuthInteractionDeviceCode,
		ClientID:    clientID,
	}
}

func prepareTestDetail(oauth *skillstore.OAuthRequirement) *skillstore.SkillDetail {
	return &skillstore.SkillDetail{
		Skill: &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{
			OAuth: oauth,
			Env:   []string{"MINDX_TEST_MISSING_ENV_XYZ"},
		},
	}
}

// TestEvaluatePreparation_OAuthStarted 凭据缺失 + 提供商已配置 → 启动授权流程。
func TestEvaluatePreparation_OAuthStarted(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), []*core.OAuthProviderConfig{prepareTestProvider("cid")})
	detail := prepareTestDetail(&skillstore.OAuthRequirement{Provider: "pv", Env: "GEMINI_TOKEN"})

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareOAuthStarted, res.Status)
	require.Equal(t, "pv", res.Provider)
	require.Equal(t, "Provider X", res.ProviderTitle)
	require.Equal(t, "GEMINI_TOKEN", res.Env)

	// 重入幂等：流程进行中不重复启动，仍返回 oauth_started
	res2 := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareOAuthStarted, res2.Status)
}

// TestEvaluatePreparation_UnconfiguredProvider 未配置态：同步报「去设置填 Client ID」。
func TestEvaluatePreparation_UnconfiguredProvider(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), []*core.OAuthProviderConfig{prepareTestProvider("")})
	detail := prepareTestDetail(&skillstore.OAuthRequirement{Provider: "pv", Env: "GEMINI_TOKEN"})

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareBlocked, res.Status)
	require.Contains(t, res.Message, "尚未配置 Client ID")
	require.False(t, d.oauthFlows.running("pv"))
}

// TestEvaluatePreparation_UnknownProvider 未知提供商阻塞。
func TestEvaluatePreparation_UnknownProvider(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := prepareTestDetail(&skillstore.OAuthRequirement{Provider: "nope", Env: "GEMINI_TOKEN"})

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareBlocked, res.Status)
	require.Contains(t, res.Message, "未知的授权提供商")
}

// TestEvaluatePreparation_CredentialPresent 凭据已在（三层解析链第一层命中）→ ok，
// 不启动任何授权流程。
func TestEvaluatePreparation_CredentialPresent(t *testing.T) {
	store := newFakeCredStore()
	require.NoError(t, store.Set(core.SkillCredentialKey("my-skill", "GEMINI_TOKEN"), `{"access":"x"}`))
	d := prepareTestDaemon(store, []*core.OAuthProviderConfig{prepareTestProvider("cid")})
	detail := &skillstore.SkillDetail{
		Skill:    &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{OAuth: &skillstore.OAuthRequirement{Provider: "pv", Env: "GEMINI_TOKEN"}},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareOK, res.Status)
	require.False(t, d.oauthFlows.running("pv"))
}

// TestEvaluatePreparation_MissingEnvBlocked 非 oauth 落点的环境变量缺失 → 环境不满足。
func TestEvaluatePreparation_MissingEnvBlocked(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := &skillstore.SkillDetail{
		Skill:    &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{Env: []string{"MINDX_TEST_MISSING_ENV_XYZ"}},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareBlocked, res.Status)
	require.Contains(t, res.Message, "MINDX_TEST_MISSING_ENV_XYZ")
}

// TestEvaluatePreparation_BinsNeedsConfirm 二进制缺失且有本平台 install 命令块
// → needs_confirm（命令清单带回壳侧展示确认）。
func TestEvaluatePreparation_BinsNeedsConfirm(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := &skillstore.SkillDetail{
		Skill:           &skill.Skill{Name: "my-skill"},
		Requires:        &skillstore.Requires{Bins: []string{"mindx-no-such-bin-xyz"}},
		InstallCommands: map[string][]string{"darwin": {"true"}, "linux": {"true"}},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareNeedsConfirm, res.Status)
	require.NotEmpty(t, res.Commands)
	require.NotEmpty(t, res.Platform)
}

// TestEvaluatePreparation_BinsBlockedNoCommands 二进制缺失且无 install 命令块 → 环境不满足。
func TestEvaluatePreparation_BinsBlockedNoCommands(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := &skillstore.SkillDetail{
		Skill:    &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{Bins: []string{"mindx-no-such-bin-xyz"}},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareBlocked, res.Status)
	require.Contains(t, res.Message, "mindx-no-such-bin-xyz")
}

// TestEvaluatePreparation_NoRequirements 无任何依赖声明 → ok（存量技能不受影响）。
func TestEvaluatePreparation_NoRequirements(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := &skillstore.SkillDetail{Skill: &skill.Skill{Name: "my-skill"}}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareOK, res.Status)
}
