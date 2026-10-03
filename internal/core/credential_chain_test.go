package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveSkillCredential_PrefersCredentialStore 第一层：凭据存储优先
func TestResolveSkillCredential_PrefersCredentialStore(t *testing.T) {
	dir := t.TempDir()
	store := NewCredentialStore(dir)
	require.NoError(t, store.Set(SkillCredentialKey("my-skill", "GEMINI_TOKEN"), `{"access":"from-store"}`))

	t.Setenv("GEMINI_TOKEN", "from-env")
	value, ok := ResolveSkillCredential(store, "my-skill", "GEMINI_TOKEN")
	require.True(t, ok)
	assert.Equal(t, `{"access":"from-store"}`, value)
}

// TestResolveSkillCredential_FallsBackToEnv 第二层：凭据存储缺失回退进程环境变量
func TestResolveSkillCredential_FallsBackToEnv(t *testing.T) {
	store := NewCredentialStore(t.TempDir())
	t.Setenv("MINDX_TEST_ENV_CRED", "from-env")

	value, ok := ResolveSkillCredential(store, "my-skill", "MINDX_TEST_ENV_CRED")
	require.True(t, ok)
	assert.Equal(t, "from-env", value)
}

// TestResolveSkillCredential_Missing 第三层：均缺失返回缺失信号
func TestResolveSkillCredential_Missing(t *testing.T) {
	store := NewCredentialStore(t.TempDir())
	value, ok := ResolveSkillCredential(store, "my-skill", "MINDX_TEST_MISSING_CRED")
	assert.False(t, ok)
	assert.Empty(t, value)
}

// TestResolveSkillCredential_NilStore store 为 nil 时安全回退环境变量
func TestResolveSkillCredential_NilStore(t *testing.T) {
	t.Setenv("MINDX_TEST_ENV_CRED", "from-env")
	value, ok := ResolveSkillCredential(nil, "my-skill", "MINDX_TEST_ENV_CRED")
	require.True(t, ok)
	assert.Equal(t, "from-env", value)
}

// TestSkillCredentialKeyIsolation 技能名前缀隔离：不同技能同名 env 互不可见
func TestSkillCredentialKeyIsolation(t *testing.T) {
	dir := t.TempDir()
	store := NewCredentialStore(dir)
	require.NoError(t, store.Set(SkillCredentialKey("skill-a", "TOKEN"), "a"))
	require.NoError(t, store.Set(SkillCredentialKey("skill-b", "TOKEN"), "b"))

	a, ok := ResolveSkillCredential(store, "skill-a", "TOKEN")
	require.True(t, ok)
	assert.Equal(t, "a", a)

	b, ok := ResolveSkillCredential(store, "skill-b", "TOKEN")
	require.True(t, ok)
	assert.Equal(t, "b", b)

	// 第三技能未授权
	_, ok = ResolveSkillCredential(store, "skill-c", "TOKEN")
	assert.False(t, ok)
}

// TestEncryptedFileStorePersistence 加密文件存储持久化回路（OAuth 令牌 JSON 值）。
// 直接构造 encryptedFileStore——NewCredentialStore 在 darwin 上返回 Keychain 后端。
func TestEncryptedFileStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store := &encryptedFileStore{path: filepath.Join(dir, "settings", ".credentials")}

	tokenJSON := `{"access":"a","refresh":"r","expires":123}`
	require.NoError(t, store.Set(SkillCredentialKey("my-skill", "TOKEN"), tokenJSON))

	// 值确实落在 settings/.credentials 且非明文
	data, err := os.ReadFile(filepath.Join(dir, "settings", ".credentials"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"access":"a"`, "凭据文件不得明文存储令牌")

	// 重建 store 后可读回
	store2 := &encryptedFileStore{path: filepath.Join(dir, "settings", ".credentials")}
	got, err := store2.Get(SkillCredentialKey("my-skill", "TOKEN"))
	require.NoError(t, err)
	assert.Equal(t, tokenJSON, got)
}
