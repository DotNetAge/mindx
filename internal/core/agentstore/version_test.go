package agentstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionRoundTrip 定义版本必须随 IDENTITY.md 往返：写盘带 version，重新加载仍在。
// 血统只有这一份事实源，序列化丢字段就等于升级链断在本地。
func TestVersionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	agent := &Agent{
		Meta: AgentMeta{
			Name:        "versioned",
			Role:        "测试",
			Description: "带版本的 Agent",
			Version:     "1.4.2",
		},
		Soul: "身份正文",
	}
	if err := store.Save(agent); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "versioned", "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取 IDENTITY.md 失败: %v", err)
	}
	if !strings.Contains(string(raw), "version: 1.4.2") {
		t.Fatalf("frontmatter 未写入版本：%s", string(raw))
	}

	reloaded, _, err := Load(dir)
	if err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	got := reloaded.Get("versioned")
	if got == nil {
		t.Fatal("重载后找不到 Agent")
	}
	if got.Meta.Version != "1.4.2" {
		t.Fatalf("版本丢失：%q", got.Meta.Version)
	}
}

// TestNewAgentGetsDefaultVersion 新建即给版本起点（与种子、技能模板同口径）。
func TestNewAgentGetsDefaultVersion(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if err := store.Save(&Agent{Meta: AgentMeta{Name: "fresh", Role: "测试", Description: "新建"}}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "fresh", "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if !strings.Contains(string(raw), "version: "+DefaultVersion) {
		t.Fatalf("新建 Agent 应带默认版本：%s", string(raw))
	}
}

// TestExistingUnversionedKeepsNoVersion 已存在的老实体不被补版本：
// 目录已存在时保存不写 version（旧安装的版本只能靠纳管或重装确认，不替它编）。
func TestExistingUnversionedKeepsNoVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "legacy"), 0755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "legacy", "IDENTITY.md"),
		[]byte("---\nname: legacy\nrole: 测试\n---\n\n正文\n"), 0644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	agent := store.Get("legacy")
	if agent == nil {
		t.Fatal("应能载入老实体")
	}
	if err := store.SetHired("legacy", true); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "legacy", "IDENTITY.md"))
	if strings.Contains(string(raw), "version:") {
		t.Fatalf("已存在的老实体不应被补版本：%s", string(raw))
	}
}
