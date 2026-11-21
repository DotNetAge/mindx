package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DotNetAge/mindx/internal/core"
)

// 凭据元信息属于机制层：展示名与字段中文名随凭据一起存在 CredentialStore 里，
// 不依赖任何旁挂文件。这组用例锁住该行为，避免以后又冒出 credentials.json 之类。
func TestCredentialMeta_RoundTripThroughStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	store := core.NewMemoryCredentialStore()

	require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{"aly-dms":{"name":"aly-dms","title":"阿里云 DMS","enabled":true,"type":"http","url":"https://x","credential_group":"aliyun","env":{"ALIBABA_CLOUD_ACCESS_KEY_ID":"","ALIBABA_CLOUD_ACCESS_KEY_SECRET":""}}}}`), 0o600))

	mgr := NewManager(nil, store, path, "")
	require.NoError(t, store.Set("group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID", "ak"))
	require.NoError(t, mgr.SaveGroupMeta("aliyun", "阿里云访问密钥", map[string]string{
		"ALIBABA_CLOUD_ACCESS_KEY_ID":     "AccessKey ID",
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET": "AccessKey Secret",
	}))

	// 元信息必须落在凭据库里，而不是目录下的某个 json。
	require.NoFileExists(t, filepath.Join(dir, "credentials.json"))

	groups, err := mgr.ListCredentialGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "阿里云访问密钥", groups[0].Display)
	require.Equal(t, []string{"aly-dms"}, groups[0].UsedBy)

	byKey := map[string]CredentialFieldView{}
	for _, f := range groups[0].Fields {
		byKey[f.Key] = f
	}
	require.Equal(t, "AccessKey ID", byKey["ALIBABA_CLOUD_ACCESS_KEY_ID"].Label)
	require.True(t, byKey["ALIBABA_CLOUD_ACCESS_KEY_ID"].HasValue)
	require.False(t, byKey["ALIBABA_CLOUD_ACCESS_KEY_SECRET"].HasValue)
}

func TestCredentialMeta_DeleteRemovesEntry(t *testing.T) {
	store := core.NewMemoryCredentialStore()
	require.NoError(t, store.Set("group:g1:API_KEY", "v"))
	require.NoError(t, store.SetMeta("group:g1:API_KEY", map[string]string{"display": "G1", "label": "API Key"}))

	entries, err := store.List("group:")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "G1", entries[0].Meta["display"])

	require.NoError(t, store.Delete("group:g1:API_KEY"))
	entries, err = store.List("group:")
	require.NoError(t, err)
	require.Empty(t, entries)

	v, err := store.Get("group:g1:API_KEY")
	require.NoError(t, err)
	require.Empty(t, v)
}

func TestCredentialMeta_LegacyFileMigrates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(`{"groups":{"bocha":{"group":"bocha","display":"博查搜索 API Key","fields":{"BOCHA_API_KEY":"API Key"}}}}`), 0o600))

	store := core.NewMemoryCredentialStore()
	require.NoError(t, store.Set("group:bocha:BOCHA_API_KEY", "secret"))
	mgr := NewManager(nil, store, path, "")

	groups, err := mgr.ListCredentialGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "博查搜索 API Key", groups[0].Display)
	// 搬迁完成后旧文件改名为 .migrated，不再作为数据源。
	require.FileExists(t, filepath.Join(dir, "credentials.json.migrated"))
	require.NoFileExists(t, filepath.Join(dir, "credentials.json"))
}
