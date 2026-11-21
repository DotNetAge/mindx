package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DotNetAge/mindx/internal/core"
)

// 授权方式来自配置：settings 目录下的 client-provider.yml 决定界面能拿到哪些服务商，
// 代码里不再出现任何具体服务商分支。
func TestOAuthProviders_LoadFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `
oauth_providers:
  - name: acme
    title: Acme
    kind: device_code
    device_url: https://acme.example/device
    token_url: https://acme.example/token
    client_id: acme-public-id
    scopes: [read]
    interaction: device_code
  - name: acme-oauth
    title: Acme OAuth
    kind: authorization_code
    authorize_url: https://acme.example/authorize
    token_url: https://acme.example/token
    client_id: acme-id
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "client-provider.yml"), []byte(cfg), 0o600))

	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(dir, "mcp.json"), "")
	got := mgr.OAuthProviders()
	require.Len(t, got, 2, "内置 github + 配置里的 acme（authorization_code 不进自动授权清单）")

	byName := map[string]OAuthProviderInfo{}
	for _, p := range got {
		byName[p.Name] = p
	}
	require.Equal(t, "GitHub", byName["github"].Title)
	require.True(t, byName["github"].Configured)
	require.Equal(t, "Acme", byName["acme"].Title)
	require.True(t, byName["acme"].Configured)
	_, ok := byName["acme-oauth"]
	require.False(t, ok, "授权码方式不进设备码清单")
}

// 配置文件缺失时内置兜底仍可用，保证全新安装不必先写配置。
func TestOAuthProviders_BuiltinFallback(t *testing.T) {
	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(t.TempDir(), "mcp.json"), "")
	got := mgr.OAuthProviders()
	require.Len(t, got, 1)
	require.Equal(t, "github", got[0].Name)
	require.True(t, got[0].Configured)

	p, err := mgr.deviceProvider("github")
	require.NoError(t, err)
	require.Equal(t, core.OAuthKindDeviceCode, p.Kind)
	require.Equal(t, "https://github.com/login/device/code", p.DeviceURL)
}
