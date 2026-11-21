package svc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/stretchr/testify/require"
)

// failingCredStore 模拟钥匙串被锁定 / 凭据文件不可写。
type failingCredStore struct{}

func (failingCredStore) Get(string) (string, error) { return "", nil }
func (failingCredStore) Set(string, string) error   { return errors.New("keychain locked") }
func (failingCredStore) Delete(string) error        { return errors.New("keychain locked") }
func (failingCredStore) SetMeta(string, map[string]string) error {
	return errors.New("keychain locked")
}
func (failingCredStore) List(string) ([]core.CredentialEntry, error) {
	return nil, errors.New("keychain locked")
}

// withFailingCredStore 把凭据库构造替换成失败实现，返回还原函数。
func withFailingCredStore(t *testing.T) {
	t.Helper()
	orig := credentialStoreFor
	credentialStoreFor = func(*Daemon) core.CredentialStore { return failingCredStore{} }
	t.Cleanup(func() { credentialStoreFor = orig })
}

// TestUserConfigRejectsMalformedParams 畸形参数必须报错，不能静默当成只读请求。
//
// 回归点（P1 §4.14）：以前 `if err := json.Unmarshal(params, &updates); err == nil`
// 把解析失败包成「什么也没发生」，返回旧配置，调用方以为写入成功。
func TestUserConfigRejectsMalformedParams(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleUserConfig(context.Background(), json.RawMessage(`{"kb_addr":`))
	require.Error(t, err, "畸形参数应报错")
}

// TestUserConfigRollsBackWhenSaveFails 落盘失败必须报错并把内存值改回去。
//
// 回归点（P1 §4.14）：以前保存失败只记日志、照样返回「保存成功」，而
// cfg 是共享指针、内存已被改写 → 界面显示成功、重启回滚。
func TestUserConfigRollsBackWhenSaveFails(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	cfg := d.app.Config()
	// 把配置文件所在目录置为不可写，制造落盘失败
	cfgDir := filepath.Dir(cfg.Path())
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	require.NoError(t, os.Chmod(cfgDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(cfgDir, 0o700) })

	before := cfg.KBAddr
	_, err := d.handleUserConfig(context.Background(), json.RawMessage(`{"kb_addr":"http://example.invalid:9/kb"}`))
	require.Error(t, err, "落盘失败应报错")
	require.Equal(t, before, cfg.KBAddr, "内存值必须回滚，避免与磁盘分叉")
}

// TestProviderUpdateAbortsWhenCredentialStoreFails 凭据没落盘就不许改 YAML 引用。
//
// 回归点（P1 §4.15）：`_ = credStore.Set(...)` 吞掉失败后继续把 APIKey 改成
// 引用名 → 之后每次调用都拿空 Key 打上游，表现为「保存成功却一直 401」。
func TestProviderUpdateAbortsWhenCredentialStoreFails(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	ctx := context.Background()
	_, err := d.handleProviderCreate(ctx, json.RawMessage(`{"name":"p1","base_url":"https://api.example.com"}`))
	require.NoError(t, err)

	withFailingCredStore(t)

	_, err = d.handleProviderUpdate(ctx, json.RawMessage(`{"name":"p1","api_key":"sk-secret"}`))
	require.Error(t, err, "凭据写入失败时应中止更新")
	require.Contains(t, err.Error(), "没能存进本机的安全存储")

	// YAML 里的引用不得被改写
	for _, p := range d.app.ProviderConfigs() {
		if p.Name == "p1" {
			require.Empty(t, p.APIKey, "凭据写入失败后 YAML 不应指向不存在的密钥")
		}
	}
}
