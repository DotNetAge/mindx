package skillstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/skill"
)

// testDeadline 为测试函数附加 10 秒超时看门狗（项目测试硬性要求）。
func testDeadline(t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()
}

// mustWriteSkill 以目录格式写入一个最小技能（SKILL.md）。
func mustWriteSkill(t *testing.T, libDir, name, description string) {
	t.Helper()
	dir := filepath.Join(libDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建技能目录 %s 失败: %v", dir, err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n这是技能 " + name + " 的指令正文。\n说明：" + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
}

// newTestStore 建立三级库测试环境，返回 Store 与各库目录。
func newTestStore(t *testing.T) (*Store, string, string, string) {
	t.Helper()
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global-skills")
	agentsDir := filepath.Join(tmp, "agents")
	projectDir := filepath.Join(tmp, "my-project")

	s, err := NewStore(globalDir, agentsDir)
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	return s, globalDir, agentsDir, projectDir
}

func TestRegistryFor_AgentLevelOverridesGlobal(t *testing.T) {
	testDeadline(t)

	s, globalDir, agentsDir, _ := newTestStore(t)

	// 全局库与 Agent 级库存在同名技能，内容不同（逻辑重写）
	mustWriteSkill(t, globalDir, "pdf", "全局版技能")
	mustWriteSkill(t, globalDir, "web-search", "全局独有技能")
	mustWriteSkill(t, filepath.Join(agentsDir, "writer", "skills"), "pdf", "Agent 级覆盖版本")

	// 全局库在 NewStore 时装载，测试先建库后写文件，需重载使其入内存
	if err := s.ReloadGlobal(); err != nil {
		t.Fatalf("ReloadGlobal 失败: %v", err)
	}

	reg := s.RegistryFor("writer")

	pdf, err := reg.GetSkill("pdf")
	if err != nil {
		t.Fatalf("同名技能 pdf 应可检索: %v", err)
	}
	if !strings.Contains(pdf.Instructions, "Agent 级覆盖版本") {
		t.Fatalf("同名技能应以 Agent 级版本为准, 实际: %s", pdf.Instructions)
	}
	if !strings.Contains(pdf.RootDir, filepath.Join("agents", "writer", "skills", "pdf")) {
		t.Fatalf("同名技能 RootDir 应指向 Agent 级目录, 实际: %s", pdf.RootDir)
	}

	if _, err := reg.GetSkill("web-search"); err != nil {
		t.Fatalf("全局独有技能应保留: %v", err)
	}
}

func TestDiscoverProject(t *testing.T) {
	testDeadline(t)

	s, _, _, projectDir := newTestStore(t)

	t.Run("目录不存在视为空库", func(t *testing.T) {
		reg, errs := s.DiscoverProject(projectDir)
		if len(errs) != 0 {
			t.Fatalf("不存在目录不应报错: %v", errs)
		}
		if len(reg.List()) != 0 {
			t.Fatalf("应返回空注册表, 实际: %v", reg.List())
		}
	})

	t.Run("发现项目技能", func(t *testing.T) {
		mustWriteSkill(t, s.ProjectSkillDir(projectDir), "refactor-helper", "项目重构经验")
		mustWriteSkill(t, s.ProjectSkillDir(projectDir), "deploy-check", "项目部署检查")

		reg, errs := s.DiscoverProject(projectDir)
		if len(errs) != 0 {
			t.Fatalf("发现不应报错: %v", errs)
		}
		got := reg.List()
		if len(got) != 2 {
			t.Fatalf("应发现 2 个项目技能, 实际 %d: %v", len(got), got)
		}
		if _, err := reg.GetSkill("refactor-helper"); err != nil {
			t.Fatalf("项目技能应可检索: %v", err)
		}
	})

	t.Run("项目库仅对指定项目可见", func(t *testing.T) {
		otherProject := filepath.Join(filepath.Dir(projectDir), "other-project")
		reg, errs := s.DiscoverProject(otherProject)
		if len(errs) != 0 || len(reg.List()) != 0 {
			t.Fatalf("其它项目应视为空库: %v %v", errs, reg.List())
		}
	})
}

func TestPromote(t *testing.T) {
	testDeadline(t)

	t.Run("项目级晋升到全局级", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, s.ProjectSkillDir(projectDir), "deploy-check", "项目部署检查")

		err := s.Promote(PromoteOptions{
			SkillName:  "deploy-check",
			From:       LevelProject,
			To:         LevelGlobal,
			ProjectDir: projectDir,
		})
		if err != nil {
			t.Fatalf("晋升失败: %v", err)
		}

		// 落库校验：目录复制 + 全局注册表即时可见
		copied := filepath.Join(globalDir, "deploy-check", "SKILL.md")
		if _, err := os.Stat(copied); err != nil {
			t.Fatalf("晋升后全局库应有技能目录: %v", err)
		}
		sk, err := s.Global().GetSkill("deploy-check")
		if err != nil {
			t.Fatalf("晋升后应可从全局注册表检索: %v", err)
		}
		if !strings.Contains(sk.Instructions, "项目部署检查") {
			t.Fatalf("晋升应保留指令正文, 实际: %s", sk.Instructions)
		}
	})

	t.Run("项目级晋升到 Agent 级", func(t *testing.T) {
		s, _, agentsDir, projectDir := newTestStore(t)
		mustWriteSkill(t, s.ProjectSkillDir(projectDir), "refactor-helper", "项目重构经验")

		err := s.Promote(PromoteOptions{
			SkillName:  "refactor-helper",
			From:       LevelProject,
			To:         LevelAgent,
			AgentName:  "writer",
			ProjectDir: projectDir,
		})
		if err != nil {
			t.Fatalf("晋升失败: %v", err)
		}

		// 落库校验：Agent 级目录出现，且 RegistryFor 中 Agent 级版本生效
		target := filepath.Join(agentsDir, "writer", "skills", "refactor-helper", "SKILL.md")
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("晋升后 Agent 级库应有技能目录: %v", err)
		}
		sk, err := s.RegistryFor("writer").GetSkill("refactor-helper")
		if err != nil {
			t.Fatalf("晋升后 Agent 运行时注册表应可检索: %v", err)
		}
		if !strings.Contains(sk.Instructions, "项目重构经验") {
			t.Fatalf("晋升应保留指令正文, 实际: %s", sk.Instructions)
		}
	})

	t.Run("Agent 级晋升到全局级", func(t *testing.T) {
		s, globalDir, agentsDir, _ := newTestStore(t)
		mustWriteSkill(t, filepath.Join(agentsDir, "writer", "skills"), "pdf", "Agent 私有经验")

		err := s.Promote(PromoteOptions{
			SkillName: "pdf",
			From:      LevelAgent,
			To:        LevelGlobal,
			AgentName: "writer",
		})
		if err != nil {
			t.Fatalf("晋升失败: %v", err)
		}

		copied := filepath.Join(globalDir, "pdf", "SKILL.md")
		if _, err := os.Stat(copied); err != nil {
			t.Fatalf("晋升后全局库应有技能目录: %v", err)
		}
		if _, err := s.Global().GetSkill("pdf"); err != nil {
			t.Fatalf("晋升后全局注册表应可检索: %v", err)
		}
	})

	t.Run("目标同名默认拒绝, 显式覆盖后替换", func(t *testing.T) {
		s, _, _, projectDir := newTestStore(t)
		mustWriteSkill(t, s.globalDir, "deploy-check", "全局旧版本")
		mustWriteSkill(t, s.ProjectSkillDir(projectDir), "deploy-check", "项目新版本")

		err := s.Promote(PromoteOptions{
			SkillName:  "deploy-check",
			From:       LevelProject,
			To:         LevelGlobal,
			ProjectDir: projectDir,
		})
		if err == nil {
			t.Fatalf("目标存在同名技能且未确认覆盖, 应拒绝")
		}

		err = s.Promote(PromoteOptions{
			SkillName:  "deploy-check",
			From:       LevelProject,
			To:         LevelGlobal,
			ProjectDir: projectDir,
			Overwrite:  true,
		})
		if err != nil {
			t.Fatalf("显式覆盖晋升失败: %v", err)
		}
		sk, _ := s.Global().GetSkill("deploy-check")
		if !strings.Contains(sk.Instructions, "项目新版本") {
			t.Fatalf("覆盖后应为项目版本, 实际: %s", sk.Instructions)
		}
	})

	t.Run("非法参数拒绝", func(t *testing.T) {
		s, _, agentsDir, projectDir := newTestStore(t)

		if err := s.Promote(PromoteOptions{SkillName: "a", From: LevelGlobal, To: LevelAgent, AgentName: "w"}); err == nil {
			t.Fatalf("全局库作为来源应拒绝")
		}
		if err := s.Promote(PromoteOptions{SkillName: "a", From: LevelAgent, To: LevelAgent, AgentName: "w"}); err == nil {
			t.Fatalf("同层级晋升应拒绝")
		}
		if err := s.Promote(PromoteOptions{SkillName: "a", From: LevelAgent, To: LevelGlobal}); err == nil {
			t.Fatalf("Agent 级来源缺 Agent 名应拒绝")
		}
		if err := s.Promote(PromoteOptions{SkillName: "a", From: LevelProject, To: LevelGlobal}); err == nil {
			t.Fatalf("项目级来源缺项目目录应拒绝")
		}
		_ = agentsDir
		_ = projectDir
	})

	t.Run("来源技能不存在拒绝", func(t *testing.T) {
		s, _, _, projectDir := newTestStore(t)
		err := s.Promote(PromoteOptions{
			SkillName:  "ghost",
			From:       LevelProject,
			To:         LevelGlobal,
			ProjectDir: projectDir,
		})
		if err == nil {
			t.Fatalf("来源技能不存在应拒绝")
		}
	})
}

// TestRegistryImplementsSPI 编译期保障：Registry 实现运行时检索 SPI。
func TestRegistryImplementsSPI(t *testing.T) {
	testDeadline(t)
	var _ skill.SkillRegistry = (*Registry)(nil)
}
