package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// mustWriteProjectSkill 在项目 .agents/skills 目录写入一个最小技能（SKILL.md）。
func mustWriteProjectSkill(t *testing.T, projectDir, name, description string) {
	t.Helper()
	dir := filepath.Join(projectDir, ".agents", "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建技能目录失败: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n指令正文：" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
}

// TestSkillProjectDiscover 验收：项目级库发现式扫描（晋升界面的数据来源）。
func TestSkillProjectDiscover(t *testing.T) {
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

	projectDir := t.TempDir()
	mustWriteProjectSkill(t, projectDir, "proj-check", "项目部署检查经验")
	mustWriteProjectSkill(t, projectDir, "proj-refactor", "项目重构经验")

	raw, err := d.handleSkillList(context.Background(), mustJSON(t, rpc.SkillListParams{
		ProjectDir: projectDir,
	}))
	if err != nil {
		t.Fatalf("skill.list 失败: %v", err)
	}
	skills := extractProjectSkillList(t, raw)
	if len(skills) != 2 {
		t.Fatalf("应发现 2 个项目技能, 实际 %d: %v", len(skills), skills)
	}
}

// TestSkillProjectResolve 验收：Skill 工具回退解析入口按名装载动态技能。
func TestSkillProjectResolve(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	projectDir := t.TempDir()
	mustWriteProjectSkill(t, projectDir, "proj-resolve", "按名解析经验")

	sk, err := d.app.Skills().ResolveProject(projectDir, "proj-resolve")
	if err != nil {
		t.Fatalf("ResolveProject 失败: %v", err)
	}
	if sk.Name != "proj-resolve" {
		t.Fatalf("技能名不符: %s", sk.Name)
	}
	// 名称含路径分隔符（目录穿越）必须拒绝
	if _, err := d.app.Skills().ResolveProject(projectDir, "../escape"); err == nil {
		t.Fatalf("含路径分隔符的名称应被拒绝")
	}
	// 不存在的技能返回未找到
	if _, err := d.app.Skills().ResolveProject(projectDir, "ghost"); err == nil {
		t.Fatalf("不存在的技能应返回未找到")
	}
}

// TestSkillPromoteRPC 验收 P2：晋升路径正确落库（项目级 → 全局级）。
func TestSkillPromoteRPC(t *testing.T) {
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

	projectDir := t.TempDir()
	mustWriteProjectSkill(t, projectDir, "promoted-skill", "项目级经验")

	// 晋升前全局库不可见
	if _, err := d.app.Skills().Global().GetSkill("promoted-skill"); err == nil {
		t.Fatalf("晋升前全局库不应包含项目技能")
	}

	raw, err := d.handleSkillPromote(context.Background(), mustJSON(t, rpc.SkillPromoteParams{
		Name:       "promoted-skill",
		From:       "project",
		To:         "global",
		ProjectDir: projectDir,
	}))
	if err != nil {
		t.Fatalf("skill.promote 失败: %v", err)
	}
	m, _ := raw.(map[string]string)
	if m == nil || m["status"] != "ok" {
		t.Fatalf("晋升返回结构不符: %v", raw)
	}

	// 晋升后全局库即时可见（Promote 内部重载全局注册表），且指向全局库目录
	sk, err := d.app.Skills().Global().GetSkill("promoted-skill")
	if err != nil {
		t.Fatalf("晋升后全局注册表应可检索: %v", err)
	}
	if !strings.Contains(sk.RootDir, "skills") || strings.Contains(sk.RootDir, ".agents") {
		t.Fatalf("晋升后 RootDir 应指向全局库, 实际: %s", sk.RootDir)
	}
}

// extractProjectSkillList 从 skill.list 项目级返回中提取技能条目列表。
func extractProjectSkillList(t *testing.T, raw any) []map[string]any {
	t.Helper()
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("skill.list 返回结构不符: %T", raw)
	}
	list, ok := m["skills"].([]projectSkillEntry)
	if !ok {
		t.Fatalf("skills 字段类型不符: %T", m["skills"])
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{
			"name":        e.Name,
			"description": e.Description,
		})
	}
	return out
}
