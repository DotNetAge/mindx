package agentstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain 为整个测试包提供 10 秒硬超时保护，防止 IO 卡死拖垮测试进程。
func TestMain(m *testing.M) {
	timer := time.AfterFunc(10*time.Second, func() {
		panic("agentstore 测试整体超时（10s）")
	})
	defer timer.Stop()
	os.Exit(m.Run())
}

const legacyAgent = `---
name: tester
role: 测试助手
description: 用于迁移测试
model: gpt-test
exclude_tools:
  - Bash
meta:
  icon: robot
  domains:
    - code
    - test
  hired: true
  custom: keep-me
---
你好，我是测试助手。`

func TestMigrateLegacyAndLoad(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "tester.md")
	if err := os.WriteFile(legacyPath, []byte(legacyAgent), 0644); err != nil {
		t.Fatalf("写入旧格式文件失败: %v", err)
	}

	store, report, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if report["tester.md"] != nil {
		t.Fatalf("迁移报告包含错误: %v", report["tester.md"])
	}

	// 旧文件已备份
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("旧文件应已改名为 .bak，stat err = %v", err)
	}
	if _, err := os.Stat(legacyPath + ".bak"); err != nil {
		t.Fatalf("备份文件不存在: %v", err)
	}

	// 字段提升验证
	a := store.Get("tester")
	if a == nil {
		t.Fatal("迁移后的 tester 未加载")
	}
	if a.Meta.Icon != "robot" {
		t.Errorf("icon 未从 meta 提升，got %q", a.Meta.Icon)
	}
	if !a.Meta.Hired {
		t.Error("hired 未从 meta 提升")
	}
	if a.Meta.Category != "code" {
		t.Errorf("domains→category 提升错误（应取首个非空值）: got %q", a.Meta.Category)
	}
	if a.Meta.ExcludeTools == nil || len(a.Meta.ExcludeTools) != 1 || a.Meta.ExcludeTools[0] != "Bash" {
		t.Errorf("exclude_tools 迁移错误: %v", a.Meta.ExcludeTools)
	}
	if a.Meta.Meta["custom"] != "keep-me" {
		t.Errorf("自由扩展字段应保留在 Meta，got %v", a.Meta.Meta)
	}
	// 旧单文件「正文即行为规则」：正文应迁入 SOUL.md（而非 introduction）
	if a.Meta.Introduction != "" {
		t.Errorf("IDENTITY.md 正文应留空（角色定义走兜底生成），got %q", a.Meta.Introduction)
	}
	if a.Soul != "你好，我是测试助手。" {
		t.Errorf("SOUL.md 应承载旧文件正文（行为规则），got %q", a.Soul)
	}

	// 幂等：重复 Load 不再迁移、不报错
	store2, _, err := Load(dir)
	if err != nil {
		t.Fatalf("二次加载失败: %v", err)
	}
	if store2.Get("tester") == nil {
		t.Fatal("二次加载后 tester 丢失")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	agent := &Agent{
		Meta: AgentMeta{
			Name:        "writer",
			Role:        "写手",
			Description: "写测试数据",
			Icon:        "pen",
			Category:    "文档创作",
			Hired:       true,
			Skills:      []string{"summarize"},
		},
		Soul: "行为规则：简洁。",
	}
	if err := store.Save(agent); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 磁盘文件存在性
	if _, err := os.Stat(filepath.Join(dir, "writer", "IDENTITY.md")); err != nil {
		t.Fatalf("IDENTITY.md 未生成: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "writer", "SOUL.md")); err != nil {
		t.Fatalf("SOUL.md 未生成: %v", err)
	}

	// 重载 round-trip
	fresh, _, err := Load(dir)
	if err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	got := fresh.Get("writer")
	if got == nil {
		t.Fatal("重载后 writer 丢失")
	}
	if got.Meta.Icon != "pen" || !got.Meta.Hired || got.Soul != "行为规则：简洁。" {
		t.Errorf("round-trip 数据不一致: %+v / soul=%q", got.Meta, got.Soul)
	}
}

// TestLoadMigratesDomainsToCategory 验证目录格式 Agent 的旧 frontmatter（domains 键）
// 在加载时一次性迁移为 category 并写回 IDENTITY.md，消除双格式。
func TestLoadMigratesDomainsToCategory(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "legacy-agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("创建 Agent 目录失败: %v", err)
	}
	identity := "---\nname: legacy-agent\nrole: 迁移测试\ndescription: 旧业务领域字段\ndomains:\n  - 数据分析\n---\n\n身份正文。\n"
	identityPath := filepath.Join(agentDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte(identity), 0644); err != nil {
		t.Fatalf("写入 IDENTITY.md 失败: %v", err)
	}

	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	a := store.Get("legacy-agent")
	if a == nil {
		t.Fatal("legacy-agent 未加载")
	}
	if a.Meta.Category != "数据分析" {
		t.Errorf("category 迁移错误: got %q", a.Meta.Category)
	}

	// 写回验证：文件中不再含 domains 键，category 就位
	data, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatalf("读取写回后的 IDENTITY.md 失败: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "domains") {
		t.Errorf("写回后文件仍含 domains 键:\n%s", content)
	}
	if !strings.Contains(content, "category: 数据分析") {
		t.Errorf("写回后文件缺少 category 字段:\n%s", content)
	}

	// 幂等：二次加载不再触发写回（无迁移报告即无变更）
	if _, _, err := Load(dir); err != nil {
		t.Fatalf("二次加载失败: %v", err)
	}
}

func TestSetHiredPersists(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if err := store.Save(&Agent{Meta: AgentMeta{
		Name: "a1", Role: "r", Description: "d",
		ExcludeTools: []string{"Bash"}, Icon: "x",
	}}); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	if err := store.SetHired("a1", true); err != nil {
		t.Fatalf("SetHired 失败: %v", err)
	}
	if len(store.Hired()) != 1 {
		t.Fatalf("雇佣视图应包含 a1，got %v", store.Hired())
	}

	// 持久化验证：新实例读取后 hired 仍为 true，且 exclude_tools / icon 不丢
	fresh, _, err := Load(dir)
	if err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	a := fresh.Get("a1")
	if a == nil || !a.Meta.Hired {
		t.Fatal("hired 未持久化")
	}
	if a.Meta.Icon != "x" || len(a.Meta.ExcludeTools) != 1 {
		t.Errorf("强类型重写丢失字段: %+v", a.Meta)
	}

	if err := store.SetHired("no-such", true); err == nil {
		t.Error("对不存在的 agent 设置雇佣应报错")
	}
}

// TestSaveTeamRoundTrip 验证组队属性（team/members）的序列化回读与 IsLeader 派生：
// 原始属性落盘 frontmatter，IsLeader 为派生值不落盘，重载后派生结果一致。
func TestSaveTeamRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	if err := store.Save(&Agent{Meta: AgentMeta{
		Name:        "lead",
		Role:        "团队负责人",
		Description: "带团队",
		Team:        "产品研发",
		Members:     []string{"writer", "coder"},
	}, TeamDuty: "## 产品研发团队\n\n负责 mindx 的产品设计与研发交付。",
	}); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 磁盘验证：TEAM.md 独立落盘为负责人自定义团队职责
	teamData, err := os.ReadFile(filepath.Join(dir, "lead", "TEAM.md"))
	if err != nil {
		t.Fatalf("读取 TEAM.md 失败: %v", err)
	}
	if !strings.Contains(string(teamData), "产品设计与研发交付") {
		t.Errorf("TEAM.md 内容不一致:\n%s", teamData)
	}

	// 磁盘验证：team/members 原始属性落盘，派生值 IsLeader 不落盘
	data, err := os.ReadFile(filepath.Join(dir, "lead", "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取 IDENTITY.md 失败: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "team: 产品研发") {
		t.Errorf("frontmatter 缺少 team 字段:\n%s", content)
	}
	if !strings.Contains(content, "members:") || !strings.Contains(content, "writer") {
		t.Errorf("frontmatter 缺少 members 字段:\n%s", content)
	}
	if strings.Contains(content, "is_leader") {
		t.Errorf("派生值 IsLeader 不应落盘:\n%s", content)
	}

	// 重载 round-trip：原始属性回读一致，IsLeader 正确派生
	fresh, _, err := Load(dir)
	if err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	got := fresh.Get("lead")
	if got == nil {
		t.Fatal("重载后 lead 丢失")
	}
	if got.Meta.Team != "产品研发" {
		t.Errorf("team 回读不一致: got %q", got.Meta.Team)
	}
	if len(got.Meta.Members) != 2 || got.Meta.Members[0] != "writer" || got.Meta.Members[1] != "coder" {
		t.Errorf("members 回读不一致: got %v", got.Meta.Members)
	}
	if !strings.Contains(got.TeamDuty, "产品设计与研发交付") {
		t.Errorf("TEAM.md 回读不一致: got %q", got.TeamDuty)
	}
	if !got.IsLeader() {
		t.Error("Members 非空应派生为负责人")
	}

	// 成员侧：仅带 team 无 members → 非负责人
	if err := store.Save(&Agent{Meta: AgentMeta{
		Name: "member", Role: "成员", Description: "被协调", Team: "产品研发",
	}}); err != nil {
		t.Fatalf("Save 成员失败: %v", err)
	}
	fresh2, _, err := Load(dir)
	if err != nil {
		t.Fatalf("二次重载失败: %v", err)
	}
	m := fresh2.Get("member")
	if m == nil {
		t.Fatal("重载后 member 丢失")
	}
	if m.IsLeader() {
		t.Error("仅带 team 无 members 不应派生为负责人")
	}
}

// TestLoadLegacyWithoutTeamKeys 验证无 team/members 键的旧 IDENTITY.md 加载不受影响。
func TestLoadLegacyWithoutTeamKeys(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "solo")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("创建 Agent 目录失败: %v", err)
	}
	identity := "---\nname: solo\nrole: 独立开发者\ndescription: 无组队属性\n---\n\n身份正文。\n"
	if err := os.WriteFile(filepath.Join(agentDir, "IDENTITY.md"), []byte(identity), 0644); err != nil {
		t.Fatalf("写入 IDENTITY.md 失败: %v", err)
	}

	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	a := store.Get("solo")
	if a == nil {
		t.Fatal("solo 未加载")
	}
	if a.Meta.Team != "" || len(a.Meta.Members) != 0 {
		t.Errorf("未写组队属性时应保持零值，got team=%q members=%v", a.Meta.Team, a.Meta.Members)
	}
	if a.IsLeader() {
		t.Error("无 members 不应派生为负责人")
	}
	if a.Soul != "" {
		t.Errorf("无 SOUL.md 时 Soul 应为空，got %q", a.Soul)
	}
}

func TestLoadEmptyAndInvalid(t *testing.T) {
	if _, _, err := Load(""); err == nil {
		t.Error("空目录参数应报错")
	}

	// 不存在的目录 → 空注册表
	store, _, err := Load(filepath.Join(t.TempDir(), "none"))
	if err != nil {
		t.Fatalf("不存在目录应返回空注册表而非错误: %v", err)
	}
	if len(store.List()) != 0 {
		t.Error("空注册表应有 0 个 Agent")
	}
}
