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

// TestVersionAbsentKeepsFrontmatterClean 未写版本时不产生空字段（旧文件字节不受扰动）。
func TestVersionAbsentKeepsFrontmatterClean(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if err := store.Save(&Agent{Meta: AgentMeta{Name: "noversion", Role: "测试", Description: "无版本"}}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "noversion", "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if strings.Contains(string(raw), "version:") {
		t.Fatalf("未设置版本时不应写空字段：%s", string(raw))
	}
}
