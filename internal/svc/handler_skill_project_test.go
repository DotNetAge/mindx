package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// mustWriteProjectSkill 在项目 .skills 目录写入一个最小技能（SKILL.md）。
func mustWriteProjectSkill(t *testing.T, projectDir, name, description string) {
	t.Helper()
	dir := filepath.Join(projectDir, ".skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建技能目录失败: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n指令正文：" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
}

// TestSkillProjectDiscoverAndLoad 验收 P2：项目级库发现 + 批量确认一次性载入。
func TestSkillProjectDiscoverAndLoad(t *testing.T) {
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

	info, err := goharnesssession.CreateSession(context.Background(), d.app.SessDB(), "test-agent",
		goharnesssession.WithProjectDirOption(projectDir))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sid := info.SessionID

	// 1) 发现：项目库应发现 2 个技能，均未载入
	raw, err := d.handleSkillList(context.Background(), mustJSON(t, rpc.SkillListParams{
		ProjectDir: projectDir,
		SessionID:  sid,
	}))
	if err != nil {
		t.Fatalf("skill.list 失败: %v", err)
	}
	skills := extractSkillList(t, raw)
	if len(skills) != 2 {
		t.Fatalf("应发现 2 个项目技能, 实际 %d: %v", len(skills), skills)
	}
	for _, sk := range skills {
		if sk["loaded"] == true {
			t.Fatalf("载入前 loaded 应为 false: %v", sk)
		}
	}

	// 2) 批量确认一次性载入（names 为空 = 全部）
	rawLoad, err := d.handleSkillProjectLoad(context.Background(), mustJSON(t, rpc.SkillProjectLoadParams{
		SessionID: sid,
	}))
	if err != nil {
		t.Fatalf("skill.project_load 失败: %v", err)
	}
	m, _ := rawLoad.(map[string]any)
	if m == nil {
		t.Fatalf("project_load 返回结构不符: %T", rawLoad)
	}
	loaded, _ := m["loaded"].([]string)
	if len(loaded) != 2 {
		t.Fatalf("应载入 2 个技能, 实际 %v", m["loaded"])
	}

	// 3) 载入状态可见
	raw, err = d.handleSkillList(context.Background(), mustJSON(t, rpc.SkillListParams{
		ProjectDir: projectDir,
		SessionID:  sid,
	}))
	if err != nil {
		t.Fatalf("skill.list 失败: %v", err)
	}
	for _, sk := range extractSkillList(t, raw) {
		if sk["loaded"] != true {
			t.Fatalf("载入后 loaded 应为 true: %v", sk)
		}
	}

	// 4) 会话覆盖注册表已记录（daemon 每轮 Ask 重建会话时挂载）
	overlay := d.projectOverlayFor(sid)
	if overlay == nil {
		t.Fatalf("project_load 后应有会话覆盖注册表")
	}
	if _, err := overlay.GetSkill("proj-check"); err != nil {
		t.Fatalf("覆盖注册表应可检索项目技能: %v", err)
	}

	// 5) 按名单载入：仅载入指定技能
	sid2, err := goharnesssession.CreateSession(context.Background(), d.app.SessDB(), "test-agent",
		goharnesssession.WithProjectDirOption(projectDir))
	if err != nil {
		t.Fatalf("create session2: %v", err)
	}
	_, err = d.handleSkillProjectLoad(context.Background(), mustJSON(t, rpc.SkillProjectLoadParams{
		SessionID: sid2.SessionID,
		Names:     []string{"proj-refactor"},
	}))
	if err != nil {
		t.Fatalf("按名单载入失败: %v", err)
	}
	overlay2 := d.projectOverlayFor(sid2.SessionID)
	if _, err := overlay2.GetSkill("proj-refactor"); err != nil {
		t.Fatalf("名单内技能应已载入: %v", err)
	}
	if _, err := overlay2.GetSkill("proj-check"); err == nil {
		t.Fatalf("名单外技能不应被载入")
	}

	// 6) 名单中不存在的技能应报错
	if _, err := d.handleSkillProjectLoad(context.Background(), mustJSON(t, rpc.SkillProjectLoadParams{
		SessionID: sid2.SessionID,
		Names:     []string{"ghost"},
	})); err == nil {
		t.Fatalf("载入不存在的项目技能应报错")
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
	if !strings.Contains(sk.RootDir, "skills") || strings.Contains(sk.RootDir, ".skills") {
		t.Fatalf("晋升后 RootDir 应指向全局库, 实际: %s", sk.RootDir)
	}
}

// extractSkillList 从 skill.list 项目级返回中提取技能条目列表。
func extractSkillList(t *testing.T, raw any) []map[string]any {
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
			"loaded":      e.Loaded,
		})
	}
	return out
}
