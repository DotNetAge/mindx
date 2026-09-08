package skillstore

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/goharness/skill"
)

// TestLiveRegistry_GlobalSwapImmediate 核心回归：全局库原子替换后，
// 同一 LiveRegistry 实例（即同一 Runtime，无需重建）即刻检索到新技能集。
// 这是 PR-PROMPTS 第二节"热加载需实时生效"的验收场景。
func TestLiveRegistry_GlobalSwapImmediate(t *testing.T) {
	testDeadline(t)

	s, globalDir, _, _ := newTestStore(t)
	live := s.LiveRegistryFor("writer")

	// 运行时已持有 live 视图；此刻向全局库新增技能并重载（模拟 skill.add RPC / watcher）
	if _, err := live.GetSkill("late-skill"); !errors.Is(err, skill.ErrSkillNotFound) {
		t.Fatalf("装载前应检索不到 late-skill, 实际: %v", err)
	}
	mustWriteSkill(t, globalDir, "late-skill", "运行中新增的技能")
	if err := s.ReloadGlobal(); err != nil {
		t.Fatalf("ReloadGlobal 失败: %v", err)
	}

	// 不换 live 实例（不重建 Runtime），检索即刻生效
	sk, err := live.GetSkill("late-skill")
	if err != nil {
		t.Fatalf("原子替换后应即刻检索到 late-skill: %v", err)
	}
	if sk.Description != "运行中新增的技能" {
		t.Errorf("description = %q, want %q", sk.Description, "运行中新增的技能")
	}
}

// TestLiveRegistry_AgentLevelOverrideAndLiveReload Agent 级同名覆盖全局级，
// 且 Agent 级技能落盘后无需任何重载即刻可检索（实时读盘）。
func TestLiveRegistry_AgentLevelOverrideAndLiveReload(t *testing.T) {
	testDeadline(t)

	s, globalDir, agentsDir, _ := newTestStore(t)
	mustWriteSkill(t, globalDir, "pdf", "全局版技能")
	if err := s.ReloadGlobal(); err != nil {
		t.Fatalf("ReloadGlobal 失败: %v", err)
	}

	live := s.LiveRegistryFor("writer")
	writerSkills := filepath.Join(agentsDir, "writer", "skills")

	// 1. 全局库技能可检索
	if _, err := live.GetSkill("pdf"); err != nil {
		t.Fatalf("全局库技能应可检索: %v", err)
	}

	// 2. Agent 级落盘同名技能（逻辑重写）：不调用任何 Reload，即刻覆盖
	mustWriteSkill(t, writerSkills, "pdf", "Agent 级覆盖版本")
	sk, err := live.GetSkill("pdf")
	if err != nil {
		t.Fatalf("Agent 级覆盖后检索失败: %v", err)
	}
	if sk.Description != "Agent 级覆盖版本" {
		t.Errorf("同名技能应以 Agent 级版本为准, 实际: %q", sk.Description)
	}

	// 3. 其它 Agent 不受影响（Agent 级库按 agentName 隔离）
	other, err := s.LiveRegistryFor("coder").GetSkill("pdf")
	if err != nil {
		t.Fatalf("其它 Agent 应回退全局库: %v", err)
	}
	if other.Description != "全局版技能" {
		t.Errorf("其它 Agent 应看到全局版, 实际: %q", other.Description)
	}
}

// TestLiveRegistry_InvalidNameRejected 非法技能名（路径穿越、空名、非法字符）
// 一律按未找到处理，且不得触碰磁盘。
func TestLiveRegistry_InvalidNameRejected(t *testing.T) {
	testDeadline(t)

	s, _, _, _ := newTestStore(t)
	live := s.LiveRegistryFor("writer")

	for _, name := range []string{
		"",                       // 空名
		"../evil",                // 路径穿越
		"../../etc/passwd",       // 深层穿越
		"UPPER-case",             // 大写非法
		"with space",             // 空格非法
		"with/slash",             // 分隔符非法
		string(make([]byte, 65)), // 超长
	} {
		if _, err := live.GetSkill(name); !errors.Is(err, skill.ErrSkillNotFound) {
			t.Errorf("非法技能名 %q 应返回 ErrSkillNotFound, 实际: %v", name, err)
		}
	}
}
