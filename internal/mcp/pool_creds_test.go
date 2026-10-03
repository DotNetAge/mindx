package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeCredStoreForPool 连接池解析用的内存凭据库
type fakeCredStoreForPool struct {
	m map[string]string
}

func (f *fakeCredStoreForPool) Get(key string) (string, error) { return f.m[key], nil }
func (f *fakeCredStoreForPool) Set(key, value string) error {
	f.m[key] = value
	return nil
}

// TestResolveCreds_SymmetricKeys 对称键解析：env 声明中值为空的变量按
// <mcp_name>:<env_name> 从凭据库取值；有预设值的不覆盖；credential_ref 机制保留。
func TestResolveCreds_SymmetricKeys(t *testing.T) {
	store := &fakeCredStoreForPool{m: map[string]string{
		"my-mcp:API_KEY": "sym-value",
		"legacy_ref":     "ref-value",
	}}
	pool := NewConnectionPool(nil, store)

	cfg := ServerConfig{
		Name: "my-mcp",
		Env: map[string]string{
			"API_KEY": "",       // 空 → 从对称键解析
			"PRESET":  "inline", // 非空 → 保持原值不解析
			"ABSENT":  "",       // 空 + 无凭据 → 不注入（进程环境兜底）
		},
		CredentialRef: "legacy_ref",
	}

	creds, err := pool.resolveCreds(cfg)
	require.NoError(t, err)
	require.Equal(t, "sym-value", creds["API_KEY"])
	require.Equal(t, "ref-value", creds["legacy_ref"])
	require.NotContains(t, creds, "PRESET")
	require.NotContains(t, creds, "ABSENT")
}

// TestResolveCreds_NilStore 无凭据库时安全返回空映射。
func TestResolveCreds_NilStore(t *testing.T) {
	pool := NewConnectionPool(nil, nil)
	creds, err := pool.resolveCreds(ServerConfig{Name: "s", Env: map[string]string{"A": ""}, CredentialRef: "r"})
	require.NoError(t, err)
	require.Empty(t, creds)
}
