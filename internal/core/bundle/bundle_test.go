package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// mustWriteSkill 在 skills 根目录写入一个最小技能目录（SKILL.md + 资源文件）。
func mustWriteSkill(t *testing.T, skillsRoot, name, description string, extra map[string]string) {
	t.Helper()
	dir := filepath.Join(skillsRoot, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建技能目录失败: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n指令正文：" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
	for rel, content := range extra {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("创建资源目录失败: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("写入资源 %s 失败: %v", rel, err)
		}
	}
}

// mustSaveAgent 将 Agent 写入 agents 目录（目录格式）。
func mustSaveAgent(t *testing.T, store *agentstore.AgentStore, name, soul string, skills []string) *agentstore.Agent {
	t.Helper()
	agent := &agentstore.Agent{
		Meta: agentstore.AgentMeta{
			Name:        name,
			Role:        "测试角色",
			Description: name + " 的描述",
			Skills:      skills,
		},
		Soul: soul,
	}
	if err := store.Save(agent); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	return store.Get(name)
}

// newTestStores 构造隔离的 agents + skills 存储（全部落在临时目录）。
func newTestStores(t *testing.T) (string, *agentstore.AgentStore, *skillstore.Store) {
	t.Helper()
	root := t.TempDir()
	agentsDir := filepath.Join(root, "agents")
	globalDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("创建 agents 目录失败: %v", err)
	}
	agents, _, err := agentstore.Load(agentsDir)
	if err != nil {
		t.Fatalf("加载 agents 失败: %v", err)
	}
	skills, err := skillstore.NewStore(globalDir, agentsDir)
	if err != nil {
		t.Fatalf("创建 skillstore 失败: %v", err)
	}
	return root, agents, skills
}

// TestExportInstallSkill 验收 P3：Skill 包导出 → 安装落全局库 → 同名拒绝 → 覆盖替换。
func TestExportInstallSkill(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	root, _, skills := newTestStores(t)
	mustWriteSkill(t, filepath.Join(root, "skills"), "deploy-check", "部署检查经验", map[string]string{
		"references/runbook.md": "执行手册内容",
	})
	// 磁盘写入后重载内存注册表（Store 构造早于技能落盘）
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载全局库失败: %v", err)
	}

	pkg := filepath.Join(root, "deploy-check.mindpkg")
	if _, err := ExportSkill(skills, "deploy-check", pkg); err != nil {
		t.Fatalf("导出 Skill 包失败: %v", err)
	}

	// 安装到空全局库（用第二个根目录模拟目标环境）
	targetRoot := t.TempDir()
	targetGlobal := filepath.Join(targetRoot, "skills")
	res, err := Install(pkg, InstallOptions{GlobalDir: targetGlobal})
	if err != nil {
		t.Fatalf("安装 Skill 包失败: %v", err)
	}
	if res.Kind != KindSkill || res.Name != "deploy-check" {
		t.Fatalf("安装结果不符: %+v", res)
	}
	if len(res.SkillsGlobal) != 1 || res.SkillsGlobal[0] != "deploy-check" {
		t.Fatalf("应落入全局库 1 个技能, 实际: %v", res.SkillsGlobal)
	}
	// 资源文件完整落地
	for _, p := range []string{"SKILL.md", "references/runbook.md"} {
		data, err := os.ReadFile(filepath.Join(targetGlobal, "deploy-check", filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("安装后缺少文件 %s: %v", p, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			t.Fatalf("文件 %s 内容为空", p)
		}
	}

	// 同名默认拒绝
	if _, err := Install(pkg, InstallOptions{GlobalDir: targetGlobal}); err == nil {
		t.Fatalf("全局库同名技能应默认拒绝安装")
	}
	// 显式覆盖后替换成功
	if _, err := Install(pkg, InstallOptions{GlobalDir: targetGlobal, Overwrite: true}); err != nil {
		t.Fatalf("覆盖安装失败: %v", err)
	}
}

// TestExportInstallAgent 验收 P3：Agent 包导出（保真）→ 安装展开（同名技能落
// Agent 级实现逻辑重写，不同名进全局库）→ 同名 Agent 拒绝 → 覆盖安装。
func TestExportInstallAgent(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	root, agents, skills := newTestStores(t)
	globalDir := filepath.Join(root, "skills")

	// 全局库技能 shared-tip（源环境全局引用）+ agent-local（Agent 级同名覆盖版本）
	mustWriteSkill(t, globalDir, "shared-tip", "全局版本提示", nil)
	mustWriteSkill(t, globalDir, "agent-local", "全局旧版本", nil)
	// Agent 级覆盖版本（逻辑重写）
	mustWriteSkill(t, filepath.Join(root, "agents", "porter", "skills"), "agent-local", "Agent 级覆盖版本", nil)

	mustSaveAgent(t, agents, "porter", "搬运工行为规则", []string{"shared-tip", "agent-local", "ghost-missing"})

	// 磁盘写入后重载内存注册表（Store 构造早于技能落盘）
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载全局库失败: %v", err)
	}

	pkg := filepath.Join(root, "porter.mindpkg")
	manifest, warnings, err := ExportAgent(agents.Get("porter"), skills, pkg)
	if err != nil {
		t.Fatalf("导出 Agent 包失败: %v", err)
	}
	if manifest.Kind != KindAgent || manifest.Name != "porter" {
		t.Fatalf("包清单不符: %+v", manifest)
	}
	// 保真：Agent 级同名覆盖版本优先打入包内
	found := false
	for _, s := range manifest.Skills {
		if s == "agent-local" {
			found = true
		}
	}
	if !found {
		t.Fatalf("包内应包含 agent-local 技能: %v", manifest.Skills)
	}
	// ghost-missing 引用缺失 → 告警但不阻断
	if len(warnings) != 1 || !strings.Contains(warnings[0], "ghost-missing") {
		t.Fatalf("引用缺失应产生告警, 实际: %v", warnings)
	}

	// 安装到全新目标环境（无 shared-tip / agent-local 全局技能）
	targetRoot := t.TempDir()
	targetAgents := filepath.Join(targetRoot, "agents")
	targetGlobal := filepath.Join(targetRoot, "skills")
	if err := os.MkdirAll(targetAgents, 0755); err != nil {
		t.Fatalf("创建目标 agents 目录失败: %v", err)
	}

	res, err := Install(pkg, InstallOptions{AgentsDir: targetAgents, GlobalDir: targetGlobal})
	if err != nil {
		t.Fatalf("安装 Agent 包失败: %v", err)
	}
	if res.Kind != KindAgent || res.Name != "porter" {
		t.Fatalf("安装结果不符: %+v", res)
	}

	// 展开规则：包内技能与目标全局库均不同名 → 全部进全局库
	if len(res.SkillsGlobal) != 2 {
		t.Fatalf("包内 2 个技能应进全局库, 实际 global=%v agent=%v", res.SkillsGlobal, res.SkillsAgent)
	}
	for _, name := range []string{"shared-tip", "agent-local"} {
		if _, err := os.Stat(filepath.Join(targetGlobal, name, "SKILL.md")); err != nil {
			t.Fatalf("全局库应包含技能 %s: %v", name, err)
		}
	}
	// Agent 本体落地
	if _, err := os.Stat(filepath.Join(targetAgents, "porter", "IDENTITY.md")); err != nil {
		t.Fatalf("安装后缺少 IDENTITY.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetAgents, "porter", "SOUL.md")); err != nil {
		t.Fatalf("安装后缺少 SOUL.md: %v", err)
	}

	// 目标环境全局库已有同名 shared-tip → 安装时该技能应落 Agent 级（逻辑重写）
	targetAgents2 := filepath.Join(t.TempDir(), "agents")
	targetGlobal2 := filepath.Join(t.TempDir(), "skills")
	mustWriteSkill(t, targetGlobal2, "shared-tip", "目标环境已有版本", nil)
	res2, err := Install(pkg, InstallOptions{AgentsDir: targetAgents2, GlobalDir: targetGlobal2})
	if err != nil {
		t.Fatalf("二次安装失败: %v", err)
	}
	if len(res2.SkillsAgent) != 1 || res2.SkillsAgent[0] != "shared-tip" {
		t.Fatalf("同名技能应落 Agent 级, 实际 agent=%v global=%v", res2.SkillsAgent, res2.SkillsGlobal)
	}
	if _, err := os.Stat(filepath.Join(targetAgents2, "porter", "skills", "shared-tip", "SKILL.md")); err != nil {
		t.Fatalf("同名技能应落 Agent 级目录: %v", err)
	}

	// 同名 Agent 默认拒绝；覆盖安装成功
	if _, err := Install(pkg, InstallOptions{AgentsDir: targetAgents2, GlobalDir: targetGlobal2}); err == nil {
		t.Fatalf("同名智能体应默认拒绝安装")
	}
	if _, err := Install(pkg, InstallOptions{AgentsDir: targetAgents2, GlobalDir: targetGlobal2, Overwrite: true}); err != nil {
		t.Fatalf("覆盖安装失败: %v", err)
	}
}

// TestInstallRejectsZipSlip 验收：包内路径穿越条目被拒绝（zip slip 防护）。
func TestInstallRejectsZipSlip(t *testing.T) {
	evil := filepath.Join(t.TempDir(), "evil.mindpkg")
	if err := buildZip(evil, map[string][]byte{
		"manifest.json":               []byte(`{"format":1,"kind":"skill","name":"evil"}`),
		"skills/evil/../../escape.md": []byte("越权内容"),
	}); err != nil {
		t.Fatalf("构造恶意包失败: %v", err)
	}
	if _, err := Install(evil, InstallOptions{GlobalDir: t.TempDir()}); err == nil {
		t.Fatalf("路径穿越条目应被拒绝")
	}
}

// TestMarketManifestRoundTrip 验收：市场清单缓存降级路径（远程失败 → 缓存命中）。
func TestMarketManifestRoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewMarketClient("http://127.0.0.1:1/manifest.json", cacheDir) // 不可达端口

	// 预置缓存：模拟"以前拉取成功过"
	pre := &MarketManifest{
		Version:   1,
		UpdatedAt: "2026-09-06T00:00:00Z",
		Packages: []MarketPackage{{
			Kind:        KindSkill,
			Name:        "cached-skill",
			Description: "缓存中的技能",
			File:        "packages/cached-skill.mindpkg",
			Sha256:      strings.Repeat("a", 64),
		}},
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}
	data, _ := jsonMarshal(pre)
	if err := os.WriteFile(filepath.Join(cacheDir, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("写缓存清单失败: %v", err)
	}

	res, err := client.List()
	if err != nil {
		t.Fatalf("清单降级失败: %v", err)
	}
	if res.Source != "cache" {
		t.Fatalf("应降级为缓存, 实际: %s", res.Source)
	}
	if res.Warning == "" {
		t.Fatalf("降级原因应随结果返回供前端提示")
	}
	if len(res.Manifest.Packages) != 1 || res.Manifest.Packages[0].Name != "cached-skill" {
		t.Fatalf("缓存清单内容不符: %+v", res.Manifest)
	}

	// 远程与缓存均不可用 → 错误冒泡（不静默）
	if _, err := NewMarketClient("http://127.0.0.1:1/manifest.json", t.TempDir()).List(); err == nil {
		t.Fatalf("无缓存可降级时应返回错误")
	}
}
