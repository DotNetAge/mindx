package svc

import (
	"context"
	"encoding/json"
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

// TestEvaluatePreparation_EnvsCollectStarted envs 对象形态条目缺失 →
// collect_started（收集请求带回壳侧，条目含 prompt/url/sensitive 展示字段）。
func TestEvaluatePreparation_EnvsCollectStarted(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)
	detail := &skillstore.SkillDetail{
		Skill: &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{
			Env:  []string{"MINDX_TEST_SECRET_XYZ"},
			Envs: []skillstore.SkillEnvRequirement{{Name: "MINDX_TEST_SECRET_XYZ", Prompt: "到控制台申请", URL: "https://example.com", Sensitive: true}},
		},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareCollectStart, res.Status)
	require.Len(t, res.Collect, 1)
	require.Equal(t, "MINDX_TEST_SECRET_XYZ", res.Collect[0].Name)
	require.Equal(t, "到控制台申请", res.Collect[0].Prompt)
	require.True(t, res.Collect[0].Sensitive)
}

// TestEvaluatePreparation_EnvsCollectPresent 凭据已在（凭据库命中）→ ok，
// 不再返回收集请求。
func TestEvaluatePreparation_EnvsCollectPresent(t *testing.T) {
	store := newFakeCredStore()
	require.NoError(t, store.Set(core.SkillCredentialKey("my-skill", "MINDX_TEST_SECRET_XYZ"), "v"))
	d := prepareTestDaemon(store, nil)
	detail := &skillstore.SkillDetail{
		Skill: &skill.Skill{Name: "my-skill"},
		Requires: &skillstore.Requires{
			Env:  []string{"MINDX_TEST_SECRET_XYZ"},
			Envs: []skillstore.SkillEnvRequirement{{Name: "MINDX_TEST_SECRET_XYZ", Prompt: "到控制台申请"}},
		},
	}

	res := d.evaluatePreparation("my-skill", detail)
	require.Equal(t, prepareOK, res.Status)
}

// TestHandleSkillCredentialSet 收集提交链路：写凭据库（键 <skill>:<env>）+
// 响应确认落库（收尾推进由壳重调 skill.prepare）。
func TestHandleSkillCredentialSet(t *testing.T) {
	store := newFakeCredStore()
	d := prepareTestDaemon(store, nil)

	params := json.RawMessage(`{"name":"my-skill","envs":[{"env":"MINDX_TEST_SECRET_XYZ","value":"sv"}]}`)
	res, err := d.handleSkillCredentialSet(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"ok": true, "set": 1}, res)
	// 值已落凭据库（三层解析链第一层命中）
	v, err := store.Get(core.SkillCredentialKey("my-skill", "MINDX_TEST_SECRET_XYZ"))
	require.NoError(t, err)
	require.Equal(t, "sv", v)
}

// TestHandleSkillCredentialSet_ParamsGuard 参数守卫：缺 name / 缺 envs / 空值拒绝。
func TestHandleSkillCredentialSet_ParamsGuard(t *testing.T) {
	d := prepareTestDaemon(newFakeCredStore(), nil)

	_, err := d.handleSkillCredentialSet(context.Background(), json.RawMessage(`{"envs":[{"env":"A","value":"v"}]}`))
	require.ErrorContains(t, err, "缺少 name")

	_, err = d.handleSkillCredentialSet(context.Background(), json.RawMessage(`{"name":"my-skill"}`))
	require.ErrorContains(t, err, "缺少 envs")

	_, err = d.handleSkillCredentialSet(context.Background(), json.RawMessage(`{"name":"my-skill","envs":[{"env":"A","value":""}]}`))
	require.ErrorContains(t, err, "值不能为空")
}
