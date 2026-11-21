package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/DotNetAge/mindx/internal/core"
)

// fakeProvider 把设备码流程指向假端点：授权方式由配置描述，测试只需给一份配置。
func fakeProvider(name string, srv *httptest.Server) *core.OAuthProviderConfig {
	return &core.OAuthProviderConfig{
		Name:          name,
		Title:         "GitHub",
		Kind:          core.OAuthKindDeviceCode,
		DeviceURL:     srv.URL + "/device/code",
		TokenURL:      srv.URL + "/token",
		ClientID:      "test-client",
		Scopes:        []string{"repo", "read:org"},
		UserInfoURL:   srv.URL + "/user",
		UserNameField: "login",
	}
}

// 设备码授权是一条闭环：申请短码 → 用户确认 → 轮询拿到 token → 落库 → 连接器生效。
// 这里用假端点把整条链路跑一遍，避免真去 GitHub 授权（也无法自动完成人工确认）。
func TestDeviceFlow_HappyPath(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device/code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "dev-123", "user_code": "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900, "interval": 1,
			})
		case "/token":
			polls++
			if polls < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_test", "token_type": "bearer", "scope": "repo"})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{"github":{"name":"github","title":"GitHub","enabled":false,"type":"stdio","command":"npx","args":["-y","x"],"env":{"GITHUB_PERSONAL_ACCESS_TOKEN":"<YOUR_TOKEN>"}}}}`), 0o600))

	store := core.NewMemoryCredentialStore()
	mgr := NewManager(nil, store, path, "")
	mgr.SetOAuthProviders([]*core.OAuthProviderConfig{fakeProvider("github", srv)})

	start, err := mgr.StartDeviceFlow(DeviceFlowParams{
		OAuthProvider: "github",
		Group:         "github",
		Field:         "GITHUB_PERSONAL_ACCESS_TOKEN",
		Display:       "GitHub 授权",
		Servers:       []string{"github"},
	})
	require.NoError(t, err)
	require.Equal(t, "ABCD-1234", start.UserCode)
	require.NotEmpty(t, start.Session)

	pending, err := mgr.PollDeviceFlow(start.Session)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status)

	done, err := mgr.PollDeviceFlow(start.Session)
	require.NoError(t, err)
	require.Equal(t, "ok", done.Status)
	require.Equal(t, "octocat", done.Login)

	token, err := store.Get("group:github:GITHUB_PERSONAL_ACCESS_TOKEN")
	require.NoError(t, err)
	require.Equal(t, "gho_test", token)

	entries, err := store.List("group:github:")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "GitHub 授权", entries[0].Meta["display"])

	servers, err := LoadServers(path)
	require.NoError(t, err)
	var gh *ServerConfig
	for i := range servers {
		if servers[i].Name == "github" {
			gh = &servers[i]
		}
	}
	require.NotNil(t, gh)
	require.Equal(t, "github", gh.CredentialGroup)
	require.True(t, gh.Enabled)
	require.Equal(t, "", gh.Env["GITHUB_PERSONAL_ACCESS_TOKEN"])
}

// 环境没开设备码授权时，必须给出可执行的下一步，而不是抛一串英文错误。
func TestDeviceFlow_DeviceFlowDisabledMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "device_flow_disabled", "error_description": "Device Flow must be explicitly enabled",
		})
	}))
	defer srv.Close()

	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(t.TempDir(), "mcp.json"), "")
	mgr.SetOAuthProviders([]*core.OAuthProviderConfig{fakeProvider("github", srv)})
	_, err := mgr.StartDeviceFlow(DeviceFlowParams{OAuthProvider: "github"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "设备码")
}

// 未注册或未配置的服务商要明确拒绝，不能静默失败。
func TestDeviceFlow_UnknownProvider(t *testing.T) {
	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(t.TempDir(), "mcp.json"), "")
	p := &core.OAuthProviderConfig{Name: "gitee", Kind: core.OAuthKindAuthorizationCode, ClientID: "cid"}
	mgr.SetOAuthProviders([]*core.OAuthProviderConfig{p})

	_, err := mgr.StartDeviceFlow(DeviceFlowParams{OAuthProvider: "gitee"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "手动填写")

	_, err = mgr.StartDeviceFlow(DeviceFlowParams{OAuthProvider: "nope"})
	require.Error(t, err)
}

// 旧字段 provider 仍然可用（历史界面传的是 provider）。
func TestDeviceFlow_LegacyProviderField(t *testing.T) {
	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(t.TempDir(), "mcp.json"), "")
	p := fakeProvider("github", httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "keep-going"})
	})))
	mgr.SetOAuthProviders([]*core.OAuthProviderConfig{p})
	_, err := mgr.StartDeviceFlow(DeviceFlowParams{Provider: "github"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "缺少授权服务商")
}

// 会话过期后不再反复打服务商。
func TestDeviceFlow_ExpiredSession(t *testing.T) {
	mgr := NewManager(nil, core.NewMemoryCredentialStore(), filepath.Join(t.TempDir(), "mcp.json"), "")
	mgr.SetOAuthProviders([]*core.OAuthProviderConfig{{Name: "github", Kind: core.OAuthKindDeviceCode, ClientID: "cid"}})
	mgr.deviceFlow.sessions = map[string]*DeviceFlowSession{
		"s1": {oauthProvider: "github", expiresAt: time.Now().Add(-time.Minute)},
	}
	r, err := mgr.PollDeviceFlow("s1")
	require.NoError(t, err)
	require.Equal(t, "expired", r.Status)

	_, err = mgr.PollDeviceFlow("s1")
	require.Error(t, err, "过期会话应被清掉")
}
