package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// fakeCredentialStore 内存凭据存储桩（测试专用）。
type fakeCredentialStore struct {
	values map[string]string
}

func (f *fakeCredentialStore) backend() CredentialStore {
	store := NewMemoryCredentialStore()
	for k, v := range f.values {
		_ = store.Set(k, v)
	}
	return &seedOnSet{store: store, seed: f.values}
}

func (f *fakeCredentialStore) Get(key string) (string, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", os.ErrNotExist
}

func (f *fakeCredentialStore) Set(key, value string) error {
	f.values[key] = value
	return nil
}

func (f *fakeCredentialStore) Delete(key string) error {
	delete(f.values, key)
	return nil
}

func (f *fakeCredentialStore) SetMeta(key string, meta map[string]string) error { return nil }

func (f *fakeCredentialStore) List(prefix string) ([]CredentialEntry, error) { return nil, nil }

// seedOnSet 保留原始 values 映射，写入时同步回映射，便于测试断言。
type seedOnSet struct {
	store CredentialStore
	seed  map[string]string
}

func (s *seedOnSet) Get(key string) (string, error) { return s.store.Get(key) }
func (s *seedOnSet) Delete(key string) error        { return s.store.Delete(key) }
func (s *seedOnSet) SetMeta(key string, meta map[string]string) error {
	return s.store.SetMeta(key, meta)
}
func (s *seedOnSet) List(prefix string) ([]CredentialEntry, error) { return s.store.List(prefix) }
func (s *seedOnSet) Set(key, value string) error {
	s.seed[key] = value
	return s.store.Set(key, value)
}

// writeEnvSkill 在全局技能库写入一个技能目录（SKILL.md 内容由调用方给定）。
func writeEnvSkill(t *testing.T, globalDir, name, content string) {
	t.Helper()
	dir := filepath.Join(globalDir, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}

// TestSkillSubProcessEnvResolver 验证技能凭据注入解析器的选源语义：
// 对象形态条目经凭据库解析注入；字符串形态条目不注入（进程环境直通语义）；
// 凭据缺失的条目不产出键。
func TestSkillSubProcessEnvResolver(t *testing.T) {
	globalDir := t.TempDir()
	writeEnvSkill(t, globalDir, "skill-a", `---
name: skill-a
description: 测试技能 A（对象 + 字符串混合 envs）
metadata:
  requires:
    envs:
      - LARK_APP_ID
      - name: LARK_APP_SECRET
        prompt: 在飞书开放平台应用凭证页获取
        sensitive: true
---
正文
`)
	writeEnvSkill(t, globalDir, "skill-b", `---
name: skill-b
description: 测试技能 B（凭据缺失）
metadata:
  requires:
    envs:
      - name: MISSING_TOKEN
        prompt: 无凭据的令牌
---
正文
`)

	// NewStore 将"依赖未满足"类告警汇总返回（技能照常注册），测试环境
	// 无真实凭据必然触发，按生产语义忽略该错误仅使用注册表。
	store, _ := skillstore.NewStore(globalDir, filepath.Join(t.TempDir(), "agents"))
	require.NotNil(t, store)

	credStore := &fakeCredentialStore{values: map[string]string{
		SkillCredentialKey("skill-a", "LARK_APP_SECRET"): "secret-value",
	}}

	app := &App{skills: store, credStore: credStore}
	resolver := app.SkillSubProcessEnvResolver("tester", []string{"skill-a", "skill-b"})

	extras := resolver()
	require.NotNil(t, extras)
	require.Equal(t, "secret-value", extras["LARK_APP_SECRET"])
	require.NotContains(t, extras, "LARK_APP_ID")
	require.NotContains(t, extras, "MISSING_TOKEN")
	require.Len(t, extras, 1)
}

// TestSkillSubProcessEnvResolver_Empty 验证无可解析凭据时返回 nil，
// Bash 工具维持父进程环境继承行为（cmd.Env 不被显式设置）。
func TestSkillSubProcessEnvResolver_Empty(t *testing.T) {
	globalDir := t.TempDir()
	writeEnvSkill(t, globalDir, "skill-c", `---
name: skill-c
description: 测试技能 C（无凭据声明）
---
正文
`)

	store, err := skillstore.NewStore(globalDir, filepath.Join(t.TempDir(), "agents"))
	require.NoError(t, err)

	app := &App{
		skills:    store,
		credStore: &fakeCredentialStore{values: map[string]string{}},
	}

	resolver := app.SkillSubProcessEnvResolver("tester", []string{"skill-c"})
	require.Nil(t, resolver())
}
