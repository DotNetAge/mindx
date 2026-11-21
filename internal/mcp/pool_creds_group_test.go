package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResolveCreds_AccountGroup 账号级凭据组优先于连接器自身：
// 阿里云一套 AK 顶着 15 个产品，组里只存一份（group:<组>:<键>）。
func TestResolveCreds_AccountGroup(t *testing.T) {
	store := &fakeCredStoreForPool{m: map[string]string{
		"group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID":     "group-ak",
		"group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_SECRET": "group-sk",
		"rds:ALIBABA_CLOUD_ACCESS_KEY_ID":              "own-ak",
	}}
	pool := NewConnectionPool(nil, store)

	cfg := ServerConfig{
		Name:            "rds",
		CredentialGroup: "aliyun",
		Env: map[string]string{
			"ALIBABA_CLOUD_ACCESS_KEY_ID":     "<YOUR_AK>", // 占位即未填，走组
			"ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<YOUR_SK>",
		},
	}

	creds, err := pool.resolveCreds(cfg)
	require.NoError(t, err)
	// 组优先：同组产品共用一份，避免 15 处拷贝与轮换漏改
	require.Equal(t, "group-ak", creds["ALIBABA_CLOUD_ACCESS_KEY_ID"])
	require.Equal(t, "group-sk", creds["ALIBABA_CLOUD_ACCESS_KEY_SECRET"])
}

// TestResolveCreds_PlaceholderIsUnset 占位值不是真值：必须能被凭据库覆盖，
// 否则用户在界面上填了密钥也永远解析不出来。
func TestResolveCreds_PlaceholderIsUnset(t *testing.T) {
	for _, placeholder := range []string{"<YOUR_AMAP_KEY>", "your_token_here", "", "xxx"} {
		t.Run(placeholder, func(t *testing.T) {
			store := &fakeCredStoreForPool{m: map[string]string{"amap:AMAP_MAPS_API_KEY": "real-key"}}
			pool := NewConnectionPool(nil, store)
			creds, err := pool.resolveCreds(ServerConfig{
				Name: "amap",
				Env:  map[string]string{"AMAP_MAPS_API_KEY": placeholder},
			})
			require.NoError(t, err)
			require.Equal(t, "real-key", creds["AMAP_MAPS_API_KEY"])
		})
	}
}

// TestResolveCreds_HeaderKey HTTP 类连接器的 header 也要能从凭据库取。
func TestResolveCreds_HeaderKey(t *testing.T) {
	store := &fakeCredStoreForPool{m: map[string]string{
		"group:tencent:Authorization": "group-token",
	}}
	pool := NewConnectionPool(nil, store)
	creds, err := pool.resolveCreds(ServerConfig{
		Name:            "tencent-docs",
		CredentialGroup: "tencent",
		Headers:         map[string]string{"Authorization": "Bearer <YOUR_TENCENT_DOCS_TOKEN>"},
	})
	require.NoError(t, err)
	// 模板里的 Bearer 前缀要保留，用户只贴裸 token
	require.Equal(t, "Bearer group-token", creds["Authorization"])
}
