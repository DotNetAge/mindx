package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// mustWriteGlobalSkill 在 daemon 测试环境的全局技能库写入一个最小技能。
func mustWriteGlobalSkill(t *testing.T, d *Daemon, name, description string) {
	t.Helper()
	dir := filepath.Join(d.app.Skills().GlobalDir(), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建技能目录失败: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n指令正文：" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
	if err := d.app.ReloadSkills(); err != nil {
		t.Fatalf("重载全局库失败: %v", err)
	}
}

// mustSaveTestAgent 直接经 agentstore 落库（不经过 agent.create RPC），
// 专注于分发包导出导入的验收范围。
func mustSaveTestAgent(t *testing.T, d *Daemon, name string, skills []string) {
	t.Helper()
	err := d.app.Agents().Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{
			Name:        name,
			Role:        "验收角色",
			Description: name + " 的描述",
			Skills:      skills,
		},
		Soul: name + " 行为规则",
	})
	if err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
}

// TestBundleExportImportRoundTrip 验收 P3：Agent / Skill 分发包导出 → 删除 → 导入恢复。
func TestBundleExportImportRoundTrip(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// ── Agent 包：创建（含引用技能）→ 导出 → 删除 → 导入恢复 ──
	mustWriteGlobalSkill(t, d, "roundtrip-skill", "回环验收技能")
	mustSaveTestAgent(t, d, "roundtrip-agent", []string{"roundtrip-skill"})

	outPath := filepath.Join(t.TempDir(), "roundtrip-agent.mindpkg")
	if _, err := d.handleAgentExport(context.Background(), mustJSON(t, rpc.AgentExportParams{
		Name:    "roundtrip-agent",
		OutPath: outPath,
	})); err != nil {
		t.Fatalf("agent.export 失败: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("分发包未生成: %v", err)
	}

	// 删除后导入恢复（同名冲突在删除后不存在，验证完整闭环）
	if err := d.app.Agents().Remove("roundtrip-agent"); err != nil {
		t.Fatalf("删除 agent 失败: %v", err)
	}
	res, err := d.handleAgentImport(context.Background(), mustJSON(t, rpc.AgentImportParams{Path: outPath}))
	if err != nil {
		t.Fatalf("agent.import 失败: %v", err)
	}
	if m, _ := res.(*bundle.InstallResult); m == nil || m.Name != "roundtrip-agent" {
		t.Fatalf("导入结果不符: %v", res)
	}
	if !d.app.Agents().Exists("roundtrip-agent") {
		t.Fatalf("导入后 agent 应已恢复")
	}

	// ── Skill 包：导出 → 删除 → 导入恢复 ──
	skillPkg := filepath.Join(t.TempDir(), "roundtrip-skill.mindpkg")
	if _, err := d.handleSkillExport(context.Background(), mustJSON(t, rpc.SkillExportParams{
		Name:    "roundtrip-skill",
		OutPath: skillPkg,
	})); err != nil {
		t.Fatalf("skill.export 失败: %v", err)
	}
	if _, err := d.handleSkillDelete(context.Background(), mustJSON(t, rpc.SkillDeleteParams{Name: "roundtrip-skill"})); err != nil {
		t.Fatalf("删除技能失败: %v", err)
	}
	if _, err := d.handleSkillImport(context.Background(), mustJSON(t, rpc.SkillImportParams{Path: skillPkg})); err != nil {
		t.Fatalf("skill.import 失败: %v", err)
	}
	if _, err := d.app.Skills().Global().GetSkill("roundtrip-skill"); err != nil {
		t.Fatalf("导入后技能应已恢复: %v", err)
	}
}

// TestBundleImportConflict 验收 P3：同名冲突默认拒绝，overwrite 确认后放行。
func TestBundleImportConflict(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	mustWriteGlobalSkill(t, d, "conflict-skill", "冲突验收技能")
	mustSaveTestAgent(t, d, "conflict-agent", []string{"conflict-skill"})

	agentPkg := filepath.Join(t.TempDir(), "conflict-agent.mindpkg")
	if _, err := d.handleAgentExport(context.Background(), mustJSON(t, rpc.AgentExportParams{
		Name:    "conflict-agent",
		OutPath: agentPkg,
	})); err != nil {
		t.Fatalf("agent.export 失败: %v", err)
	}

	// Agent 同名默认拒绝；overwrite 放行
	if _, err := d.handleAgentImport(context.Background(), mustJSON(t, rpc.AgentImportParams{Path: agentPkg})); err == nil {
		t.Fatalf("同名 agent 导入应默认拒绝")
	}
	if _, err := d.handleAgentImport(context.Background(), mustJSON(t, rpc.AgentImportParams{Path: agentPkg, Overwrite: true})); err != nil {
		t.Fatalf("确认覆盖后导入失败: %v", err)
	}

	// Skill 同名默认拒绝；overwrite 放行
	skillPkg := filepath.Join(t.TempDir(), "conflict-skill.mindpkg")
	if _, err := d.handleSkillExport(context.Background(), mustJSON(t, rpc.SkillExportParams{
		Name:    "conflict-skill",
		OutPath: skillPkg,
	})); err != nil {
		t.Fatalf("skill.export 失败: %v", err)
	}
	if _, err := d.handleSkillImport(context.Background(), mustJSON(t, rpc.SkillImportParams{Path: skillPkg})); err == nil {
		t.Fatalf("同名技能导入应默认拒绝")
	}
	if _, err := d.handleSkillImport(context.Background(), mustJSON(t, rpc.SkillImportParams{Path: skillPkg, Overwrite: true})); err != nil {
		t.Fatalf("确认覆盖后导入失败: %v", err)
	}
}

// TestMarketInstallUnknownPackage 验收：市场安装目标不存在时显式报错（不静默）。
func TestMarketInstallUnknownPackage(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()

	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 参数非法直接拒绝
	if _, err := d.handleMarketInstall(context.Background(), mustJSON(t, rpc.MarketInstallParams{Kind: "ghost", Name: "x"})); err == nil {
		t.Fatalf("非法 kind 应报错")
	}
	// 市场不可达（测试环境无网络依赖）：清单拉取与缓存均不可用 → 错误冒泡
	if _, err := d.handleMarketInstall(context.Background(), mustJSON(t, rpc.MarketInstallParams{Kind: "agent", Name: "anything"})); err == nil {
		t.Fatalf("清单不可用时应报错")
	}
}
