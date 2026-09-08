package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- FileRuleRegistry ----

func TestFileRuleRegistry_NewWithValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "rules.yaml")

	yamlContent := `
rules:
  - id: rule-1
    intro: Always be polite
    scope: global
    priority: 100
    enabled: true
  - id: rule-2
    intro: Never delete production data
    scope: local
    priority: 200
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试 YAML 文件失败: %v", err)
	}

	reg, err := NewFileRuleRegistry(yamlPath)
	if err != nil {
		t.Fatalf("创建注册表不应报错: %v", err)
	}
	if all := reg.All(); len(all) != 2 {
		t.Errorf("期望 2 条规则，实际 %d 条", len(all))
	}
}

func TestFileRuleRegistry_NonExistentFileReturnsEmpty(t *testing.T) {
	reg, err := NewFileRuleRegistry(filepath.Join(t.TempDir(), "not-exist.yaml"))
	if err != nil {
		t.Fatalf("文件不存在应返回空注册表而非报错: %v", err)
	}
	if len(reg.All()) != 0 {
		t.Errorf("期望空注册表，实际 %d 条", len(reg.All()))
	}
}

func TestFileRuleRegistry_InvalidYAML(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(yamlPath, []byte("{invalid: yaml: content["), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if _, err := NewFileRuleRegistry(yamlPath); err == nil {
		t.Error("非法 YAML 应返回错误")
	}
}

func TestFileRuleRegistry_MissingID(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "missing-id.yaml")
	yamlContent := `
rules:
  - intro: Missing ID rule
    scope: global
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if _, err := NewFileRuleRegistry(yamlPath); err == nil {
		t.Error("缺少 ID 的规则应返回错误")
	}
}

func TestFileRuleRegistry_MissingIntro(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "missing-intro.yaml")
	yamlContent := `
rules:
  - id: no-intro-rule
    scope: global
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if _, err := NewFileRuleRegistry(yamlPath); err == nil {
		t.Error("缺少介绍文本的规则应返回错误")
	}
}

func TestFileRuleRegistry_RegisterAndPersist(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "rules.yaml")

	reg, err := NewFileRuleRegistry(yamlPath)
	if err != nil {
		t.Fatalf("创建注册表失败: %v", err)
	}

	// 注册新规则并写盘
	err = reg.Register(Rule{ID: "new-rule", Intro: "A new rule", Scope: ScopeLocal, Priority: 50, Enabled: true})
	if err != nil {
		t.Fatalf("注册规则不应报错: %v", err)
	}

	// 同名注册覆盖（更新语义）
	err = reg.Register(Rule{ID: "new-rule", Intro: "Updated", Scope: ScopeLocal, Priority: 60, Enabled: true})
	if err != nil {
		t.Fatalf("覆盖注册不应报错: %v", err)
	}
	got, ok := reg.Get("new-rule")
	if !ok || got.Intro != "Updated" {
		t.Fatalf("同名注册应覆盖原规则，实际 %+v", got)
	}

	// 重新打开文件验证持久化生效
	reg2, err := NewFileRuleRegistry(yamlPath)
	if err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	if _, ok := reg2.Get("new-rule"); !ok {
		t.Error("注册的规则应已持久化到文件")
	}
}

func TestFileRuleRegistry_Unregister(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "rules.yaml")
	yamlContent := `rules:
  - id: to-remove
    intro: Will be removed
    scope: global
    enabled: true
  - id: keep-this
    intro: Keep this one
    scope: global
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg, _ := NewFileRuleRegistry(yamlPath)
	reg.Unregister("to-remove")
	reg.Unregister("nonexistent") // 不存在的规则为空操作

	if _, found := reg.Get("to-remove"); found {
		t.Error("已移除的规则不应再存在")
	}
	if _, found := reg.Get("keep-this"); !found {
		t.Error("未移除的规则应保留")
	}
}

func TestFileRuleRegistry_AllReturnsCopy(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "rules.yaml")
	yamlContent := `rules:
  - id: original
    intro: Original rule
    scope: global
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg, _ := NewFileRuleRegistry(yamlPath)
	all := reg.All()
	all[0].Intro = "Modified in copy"

	original, _ := reg.Get("original")
	if original.Intro == "Modified in copy" {
		t.Error("All() 应返回副本，修改副本不应影响注册表")
	}
}

func TestFileRuleRegistry_GetByScope(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "rules.yaml")
	yamlContent := `rules:
  - id: global-1
    intro: Global rule 1
    scope: global
    enabled: true
  - id: local-1
    intro: Local rule 1
    scope: local
    enabled: true
  - id: conversation-1
    intro: Conversation rule 1
    scope: conversation
    enabled: true
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg, _ := NewFileRuleRegistry(yamlPath)
	if got := len(reg.GetByScope(ScopeGlobal)); got != 1 {
		t.Errorf("期望 1 条 global 规则，实际 %d 条", got)
	}
	if got := len(reg.GetByScope(ScopeLocal)); got != 1 {
		t.Errorf("期望 1 条 local 规则，实际 %d 条", got)
	}
	if got := len(reg.GetByScope(ScopeConversation)); got != 1 {
		t.Errorf("期望 1 条 conversation 规则，实际 %d 条", got)
	}
}

func TestFileRuleRegistry_FormatPromptSection(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "rules.yaml")
	yamlContent := `rules:
  - id: enabled-1
    intro: Be helpful and concise
    scope: global
    enabled: true
  - id: disabled-1
    intro: This should not appear
    scope: global
    enabled: false
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg, _ := NewFileRuleRegistry(yamlPath)
	prompt := reg.FormatPromptSection()

	if !strings.Contains(prompt, "Be helpful and concise") {
		t.Errorf("渲染结果应包含启用规则的介绍文本，实际 %q", prompt)
	}
	if strings.Contains(prompt, "This should not appear") {
		t.Errorf("禁用规则不应出现在渲染结果中，实际 %q", prompt)
	}
	if !strings.HasPrefix(prompt, "- ") {
		t.Errorf("渲染条目应以 '- ' 开头（Markdown 列表格式），实际 %q", prompt)
	}
}

// ---- MemRuleRegistry ----

func TestMemRuleRegistry_RegisterAndFormat(t *testing.T) {
	reg := NewMemRuleRegistry()
	if err := reg.Register(Rule{ID: "a", Intro: "Rule A", Scope: ScopeGlobal, Enabled: true}); err != nil {
		t.Fatalf("注册规则不应报错: %v", err)
	}
	// 禁用规则不参与渲染
	if err := reg.Register(Rule{ID: "b", Intro: "Rule B", Scope: ScopeGlobal, Enabled: false}); err != nil {
		t.Fatalf("注册规则不应报错: %v", err)
	}

	prompt := reg.FormatPromptSection()
	if !strings.Contains(prompt, "Rule A") || strings.Contains(prompt, "Rule B") {
		t.Errorf("渲染结果应只包含启用规则，实际 %q", prompt)
	}

	reg.Unregister("a")
	if prompt := reg.FormatPromptSection(); prompt != "" {
		t.Errorf("移除全部启用规则后应渲染为空，实际 %q", prompt)
	}
}
