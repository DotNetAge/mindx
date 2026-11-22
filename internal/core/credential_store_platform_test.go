package core

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Linux/Docker 走 encryptedFileStore。这组用例在任意平台跑的是同一份代码，
// 覆盖元信息、枚举、删除、重开实例后的持久性，以及中文标签（不依赖钥匙串）。
func TestEncryptedFileStore_MetaListDelete(t *testing.T) {
	dir := t.TempDir()
	newStore := func() CredentialStore {
		return &encryptedFileStore{path: filepath.Join(dir, "settings", ".credentials")}
	}
	store := newStore()

	require.NoError(t, store.Set("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID", "ak"))
	require.NoError(t, store.SetMeta("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID", map[string]string{
		"display": "阿里云访问密钥",
		"label":   "AccessKey ID",
	}))
	require.NoError(t, store.Set("ai:deepseek", "sk-legacy"))

	entries, err := store.List("group:")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "阿里云访问密钥", entries[0].Meta["display"])

	// 重开实例：值、元信息、索引都要能从密文文件里读回来。
	reopened := newStore()
	v, err := reopened.Get("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID")
	require.NoError(t, err)
	require.Equal(t, "ak", v)
	entries, err = reopened.List("group:")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// 老键路径不受影响：普通键照旧可取。
	legacy, err := reopened.Get("ai:deepseek")
	require.NoError(t, err)
	require.Equal(t, "sk-legacy", legacy)

	require.NoError(t, reopened.Delete("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID"))
	v, err = reopened.Get("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID")
	require.NoError(t, err)
	require.Empty(t, v)
	entries, err = reopened.List("group:")
	require.NoError(t, err)
	require.Empty(t, entries)
	legacy, err = reopened.Get("ai:deepseek")
	require.NoError(t, err)
	require.Equal(t, "sk-legacy", legacy)
}

// 索引坏掉不得连坐：索引被写坏时按空索引继续，并能自愈重写。
func TestEncryptedFileStore_CorruptIndexDoesNotBreakGetSet(t *testing.T) {
	dir := t.TempDir()
	store := &encryptedFileStore{path: filepath.Join(dir, "settings", ".credentials")}
	require.NoError(t, store.Set(credentialIndexKey, "not-json"))

	require.NoError(t, store.Set("ai:deepseek", "sk-1"))
	v, err := store.Get("ai:deepseek")
	require.NoError(t, err)
	require.Equal(t, "sk-1", v)

	// 索引解析失败按空索引处理：不报错，也不假装有残留条目。
	entries, err := store.List("group:")
	require.NoError(t, err)
	require.Empty(t, entries)

	// 自愈：坏索引在同一次 Set 里被重写成合法 JSON，重开后能正常枚举。
	reopened := &encryptedFileStore{path: store.path}
	entries, err = reopened.List("ai:")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "ai:deepseek", entries[0].Key)
}

// MINDX_CREDENTIALS_DIR 优先级最高：容器里把凭据落到挂载卷上靠的就是它。
func TestNewCredentialStore_EnvDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINDX_CREDENTIALS_DIR", dir)

	store := NewCredentialStore(t.TempDir())
	require.NoError(t, store.Set("ai:deepseek", "sk-env"))
	v, err := store.Get("ai:deepseek")
	require.NoError(t, err)
	require.Equal(t, "sk-env", v)
	require.FileExists(t, filepath.Join(dir, ".credentials"))
	require.FileExists(t, filepath.Join(dir, ".credentials.key"))
}
