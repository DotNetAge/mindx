package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadClientProviders_BuiltInFile 内置 client-provider.yml 全条目校验通过
func TestLoadClientProviders_BuiltInFile(t *testing.T) {
	providers, err := LoadClientProviders("../../runtime/settings/client-provider.yml")
	require.NoError(t, err)
	require.NotEmpty(t, providers)

	// 预置清单：minimax 完整预置，gemini/github/google/gitlab 未配置态；
	// qwen 已移除——Qwen OAuth 免费层于 2026-04-15 官方下线，禁止再预置死路条目
	registry := NewOAuthProviderRegistry(providers)
	_, qwenExists := registry.Provider("qwen")
	assert.False(t, qwenExists, "Qwen OAuth 已官方下线，不应再出现在注册表")
	assert.True(t, registry.Configured("minimax"))
	assert.False(t, registry.Configured("gemini"), "gemini 为机密客户端，client_id 留空")
	assert.False(t, registry.Configured("github"))
	assert.False(t, registry.Configured("google"))
	assert.False(t, registry.Configured("gitlab"))

	// kind 与 interaction 派生一致（minimax: user_code）
	minimax, ok := registry.Provider("minimax")
	require.True(t, ok)
	assert.Equal(t, OAuthKindUserCode, minimax.Kind)
	assert.Equal(t, OAuthInteractionUserCode, minimax.DeriveInteraction())

	// 公共客户端必须 PKCE 开启且无 secret
	assert.True(t, minimax.UsePKCE)
	assert.Empty(t, minimax.ClientSecret)

	// 回调监听只绑回环
	for _, name := range registry.Names() {
		p, _ := registry.Provider(name)
		if p.Kind == OAuthKindAuthorizationCode {
			assert.True(t, len(p.ListenAddr) >= 9 && p.ListenAddr[:9] == "127.0.0.1",
				"%s 的本地回调监听必须绑定 127.0.0.1", p.Name)
		}
	}
}

// TestLoadClientProviders_MissingFile 文件缺失返回 nil,nil（对齐 LoadProvidersFile 先例）
func TestLoadClientProviders_MissingFile(t *testing.T) {
	providers, err := LoadClientProviders(filepath.Join(t.TempDir(), "not-exist.yml"))
	require.NoError(t, err)
	assert.Nil(t, providers)
}

// writeTempYml 写入临时 yml 文件供加载校验用
func writeTempYml(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "client-provider.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

// TestLoadClientProviders_DuplicateName name 唯一性校验
func TestLoadClientProviders_DuplicateName(t *testing.T) {
	path := writeTempYml(t, `
oauth_providers:
  - name: dup
    kind: device_code
    device_url: https://d.example.com
    token_url: https://t.example.com
    client_id: cid
  - name: dup
    kind: device_code
    device_url: https://d2.example.com
    token_url: https://t2.example.com
    client_id: cid
`)
	_, err := LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate name")
}

// TestLoadClientProviders_UnknownKind kind 枚举校验
func TestLoadClientProviders_UnknownKind(t *testing.T) {
	path := writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: magic_flow
    token_url: https://t.example.com
    client_id: cid
`)
	_, err := LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown kind")
}

// TestLoadClientProviders_KindURLMismatch kind 与必填端点匹配校验
func TestLoadClientProviders_KindURLMismatch(t *testing.T) {
	// authorization_code 缺 authorize_url
	path := writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: authorization_code
    token_url: https://t.example.com
    client_id: cid
`)
	_, err := LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authorize_url")

	// device_code 缺 device_url
	path = writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: device_code
    token_url: https://t.example.com
    client_id: cid
`)
	_, err = LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device_url")

	// 缺 token_url（三类通用必填）
	path = writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: device_code
    device_url: https://d.example.com
    client_id: cid
`)
	_, err = LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token_url")
}

// TestLoadClientProviders_InteractionMismatch interaction 与 kind 派生一致性校验
func TestLoadClientProviders_InteractionMismatch(t *testing.T) {
	path := writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: device_code
    device_url: https://d.example.com
    token_url: https://t.example.com
    client_id: cid
    interaction: callback
`)
	_, err := LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatches")
}

// TestLoadClientProviders_SecretInYml client_secret 绝不预置（硬约束）
func TestLoadClientProviders_SecretInYml(t *testing.T) {
	path := writeTempYml(t, `
oauth_providers:
  - name: bad
    kind: authorization_code
    authorize_url: https://a.example.com
    token_url: https://t.example.com
    client_id: cid
    client_secret: leaked-secret
`)
	_, err := LoadClientProviders(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

// TestOAuthProviderRegistry_NewConfigured 查询 API 与未配置态判定
func TestOAuthProviderRegistry_NewConfigured(t *testing.T) {
	registry := NewOAuthProviderRegistry([]*OAuthProviderConfig{
		{Name: "full", Kind: OAuthKindDeviceCode, DeviceURL: "https://d", TokenURL: "https://t", ClientID: "cid"},
		{Name: "empty", Kind: OAuthKindDeviceCode, DeviceURL: "https://d", TokenURL: "https://t"},
	})

	_, ok := registry.Provider("full")
	assert.True(t, ok)
	_, ok = registry.Provider("missing")
	assert.False(t, ok)

	assert.True(t, registry.Configured("full"))
	assert.False(t, registry.Configured("empty"))
	assert.False(t, registry.Configured("missing"))

	assert.Equal(t, []string{"full", "empty"}, registry.Names())
}
