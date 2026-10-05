package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
)

// nopMigrationLogger 静默日志（测试不需要输出）。
type nopMigrationLogger struct{}

func (nopMigrationLogger) Warn(string, ...any) {}
func (nopMigrationLogger) Info(string, ...any) {}

// writeAgentFixture 在 agents 目录构造一个 Agent（IDENTITY.md 必需，SOUL/TEAM 可选）。
func writeAgentFixture(t *testing.T, agentsDir, name, frontmatter, teamDuty string) {
	t.Helper()
	dir := filepath.Join(agentsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	identity := "---\n" + frontmatter + "---\n我是 " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte(identity), 0644); err != nil {
		t.Fatal(err)
	}
	if teamDuty != "" {
		if err := os.WriteFile(filepath.Join(dir, "TEAM.md"), []byte(teamDuty), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// setupMigration 构造 agents/teams 双目录（仅目录，fixture 由各测试先写后加载）。
func setupMigration(t *testing.T) (agentsDir, teamsDir string) {
	t.Helper()
	root := t.TempDir()
	agentsDir = filepath.Join(root, "agents")
	teamsDir = filepath.Join(root, "teams")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	return agentsDir, teamsDir
}

// loadRegistries 在 fixture 写好后加载两个注册表。
func loadRegistries(t *testing.T, agentsDir, teamsDir string) (*agentstore.AgentStore, *teamstore.Store) {
	t.Helper()
	agentsReg, _, err := agentstore.Load(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	teamsReg, _, err := teamstore.Load(teamsDir)
	if err != nil {
		t.Fatal(err)
	}
	return agentsReg, teamsReg
}

// 负责人迁移：members 非空 → 建队（Profile 继承 TEAM.md 正文）+ 清键 + TEAM.md 留档。
func TestMigrateLegacyTeamsLeader(t *testing.T) {
	agentsDir, teamsDir := setupMigration(t)
	writeAgentFixture(t, agentsDir, "architect",
		"name: architect\nrole: 架构师\ndescription: 专职架构设计\nteam: alpha\nmembers:\n  - flutter-dev\n  - backend-dev\n",
		"## 团队职责\n\n协调端到端交付。")
	agentsReg, teamsReg := loadRegistries(t, agentsDir, teamsDir)

	if n := MigrateLegacyTeams(agentsReg, teamsReg, nopMigrationLogger{}); n != 1 {
		t.Fatalf("迁移数 = %d, 期望 1", n)
	}

	team := teamsReg.Get("alpha")
	if team == nil {
		t.Fatal("团队 alpha 未创建")
	}
	if team.Meta.Leader != "architect" {
		t.Fatalf("Leader = %q, 期望 architect", team.Meta.Leader)
	}
	// 成员集 = 负责人 + 原 members（去重保序）
	if len(team.Meta.Members) != 3 || team.Meta.Members[0] != "architect" {
		t.Fatalf("Members = %v, 期望 [architect flutter-dev backend-dev]", team.Meta.Members)
	}
	if team.Profile == "" || team.Meta.Hired {
		t.Fatalf("Profile 应继承 TEAM.md 正文、Hired 继承负责人（未雇佣=false）: %+v", team.Meta)
	}

	// Agent 侧键清空 + TEAM.md 留档
	fresh, _, err := agentstore.Load(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	a := fresh.Get("architect")
	if a.Meta.Team != "" || len(a.Meta.Members) != 0 {
		t.Fatalf("退役键未清除: team=%q members=%v", a.Meta.Team, a.Meta.Members)
	}
	if _, err := os.Stat(filepath.Join(a.Dir, "TEAM.md.bak")); err != nil {
		t.Fatalf("TEAM.md 未留档: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.Dir, "TEAM.md")); !os.IsNotExist(err) {
		t.Fatal("原 TEAM.md 应已改名")
	}
}

// 无 team 名的负责人：以负责人名兜底建队。
func TestMigrateLegacyTeamsLeaderWithoutTeamName(t *testing.T) {
	agentsDir, teamsDir := setupMigration(t)
	writeAgentFixture(t, agentsDir, "pm",
		"name: pm\nrole: 项目经理\nmembers:\n  - dev1\n", "")
	agentsReg, teamsReg := loadRegistries(t, agentsDir, teamsDir)

	if n := MigrateLegacyTeams(agentsReg, teamsReg, nopMigrationLogger{}); n != 1 {
		t.Fatalf("迁移数 = %d, 期望 1", n)
	}
	if teamsReg.Get("pm") == nil {
		t.Fatal("应以负责人名 pm 兜底建队")
	}
}

// 多负责人声明同一团队：首个建队者为 Leader，后续并入成员并降级。
func TestMigrateLegacyTeamsDuplicateLeaders(t *testing.T) {
	agentsDir, teamsDir := setupMigration(t)
	writeAgentFixture(t, agentsDir, "aaa",
		"name: aaa\nrole: A\nteam: shared\nmembers:\n  - m1\n", "")
	writeAgentFixture(t, agentsDir, "bbb",
		"name: bbb\nrole: B\nteam: shared\nmembers:\n  - m2\n", "")
	agentsReg, teamsReg := loadRegistries(t, agentsDir, teamsDir)

	MigrateLegacyTeams(agentsReg, teamsReg, nopMigrationLogger{})

	team := teamsReg.Get("shared")
	if team == nil {
		t.Fatal("团队 shared 未创建")
	}
	if team.Meta.Leader != "aaa" {
		t.Fatalf("Leader = %q, 期望首个建队者 aaa", team.Meta.Leader)
	}
	// bbb 及其 members 并入成员集
	for _, want := range []string{"aaa", "bbb", "m1", "m2"} {
		found := false
		for _, m := range team.Meta.Members {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("成员 %s 未并入: %v", want, team.Meta.Members)
		}
	}
}

// 成员的 team 声明：指向已建团队并入；指向不存在团队仅清键。
func TestMigrateLegacyTeamsMemberDeclarations(t *testing.T) {
	agentsDir, teamsDir := setupMigration(t)
	// 预置已建团队（模拟负责人先迁移）
	writeAgentFixture(t, agentsDir, "leader1",
		"name: leader1\nrole: L\nteam: alpha\nmembers:\n  - m0\n", "")
	// 成员1 指向已建的 alpha；成员2 指向不存在的 ghost
	writeAgentFixture(t, agentsDir, "member1",
		"name: member1\nrole: M\nteam: alpha\n", "")
	writeAgentFixture(t, agentsDir, "member2",
		"name: member2\nrole: M\nteam: ghost\n", "")
	agentsReg, teamsReg := loadRegistries(t, agentsDir, teamsDir)

	MigrateLegacyTeams(agentsReg, teamsReg, nopMigrationLogger{})

	team := teamsReg.Get("alpha")
	if team == nil {
		t.Fatal("团队 alpha 未创建")
	}
	if len(team.Meta.Members) != 3 {
		t.Fatalf("Members = %v, 期望 [leader1 m0 member1]", team.Meta.Members)
	}
	if teamsReg.Get("ghost") != nil {
		t.Fatal("不应为成员的悬空声明建空队")
	}

	fresh, _, err := agentstore.Load(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	if a := fresh.Get("member2"); a.Meta.Team != "" {
		t.Fatalf("悬空 team 键未清除: %q", a.Meta.Team)
	}
}

// 幂等：跑两遍结果一致，第二遍零迁移，.bak 不再搬运。
func TestMigrateLegacyTeamsIdempotent(t *testing.T) {
	agentsDir, teamsDir := setupMigration(t)
	writeAgentFixture(t, agentsDir, "architect",
		"name: architect\nrole: 架构师\nteam: alpha\nmembers:\n  - dev\n", "职责正文")
	agentsReg, teamsReg := loadRegistries(t, agentsDir, teamsDir)

	if n := MigrateLegacyTeams(agentsReg, teamsReg, nopMigrationLogger{}); n != 1 {
		t.Fatalf("第一次迁移数 = %d, 期望 1", n)
	}

	// 重新加载（模拟重启）
	freshAgents, _, err := agentstore.Load(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	freshTeams, _, err := teamstore.Load(teamsDir)
	if err != nil {
		t.Fatal(err)
	}
	if n := MigrateLegacyTeams(freshAgents, freshTeams, nopMigrationLogger{}); n != 0 {
		t.Fatalf("第二次迁移数 = %d, 期望 0（幂等）", n)
	}

	team := freshTeams.Get("alpha")
	if team == nil || team.Meta.Leader != "architect" || len(team.Meta.Members) != 2 {
		t.Fatalf("第二次迁移后团队状态漂移: %+v", team)
	}
}
