package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/core/teamstore"
)

// mustWriteTeamDoc 在 teams 目录写入一个 Team 实体（TEAM.md）。
func mustWriteTeamDoc(t *testing.T, teamsRoot, name, leader string, members []string, profile string) *teamstore.Team {
	t.Helper()
	store, _, err := teamstore.Load(teamsRoot)
	if err != nil {
		t.Fatalf("加载 teams 目录失败: %v", err)
	}
	team := &teamstore.Team{
		Meta: teamstore.TeamMeta{
			Name:        name,
			NickName:    "测队",
			Description: name + " 的描述",
			Leader:      leader,
			Members:     members,
		},
		Profile: profile,
	}
	if err := store.Save(team); err != nil {
		t.Fatalf("保存团队失败: %v", err)
	}
	return store.Get(name)
}

// TestExportInstallTeamRoundTrip 团队整包导出→安装 round-trip：
// 包结构（team/TEAM.md + agents/<n>/）与落位结果（teams/、agents/、技能分流）一致。
func TestExportInstallTeamRoundTrip(t *testing.T) {
	srcRoot, agents, skills := newTestStores(t)
	mustWriteSkill(t, filepath.Join(srcRoot, "skills"), "shared-skill", "共享技能", nil)
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载全局技能库失败: %v", err)
	}
	mustSaveAgent(t, agents, "lead-agent", "负责人职责", []string{"shared-skill"})
	mustSaveAgent(t, agents, "dev-agent", "成员职责", nil)
	team := mustWriteTeamDoc(t, filepath.Join(srcRoot, "teams"), "alpha", "lead-agent",
		[]string{"lead-agent", "dev-agent"}, "## 团队定位\n\n端到端交付。")

	// 导出
	outPath := filepath.Join(t.TempDir(), "alpha.mindpkg")
	manifest, warnings, err := ExportTeam(team, agents, skills, outPath)
	if err != nil {
		t.Fatalf("ExportTeam 失败: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("导出不应有告警: %v", warnings)
	}
	if manifest.Kind != KindTeam || manifest.Leader != "lead-agent" {
		t.Fatalf("清单类型/负责人不符: %+v", manifest)
	}
	if len(manifest.Members) != 2 || manifest.Members[0] != "dev-agent" {
		t.Fatalf("清单成员不符: %v", manifest.Members)
	}

	// 安装到全新环境
	dstRoot := t.TempDir()
	dstAgentsDir := filepath.Join(dstRoot, "agents")
	dstTeamsDir := filepath.Join(dstRoot, "teams")
	dstGlobalDir := filepath.Join(dstRoot, "skills")
	res, err := Install(outPath, InstallOptions{
		AgentsDir: dstAgentsDir,
		TeamsDir:  dstTeamsDir,
		GlobalDir: dstGlobalDir,
	})
	if err != nil {
		t.Fatalf("Install 失败: %v", err)
	}
	if res.Kind != KindTeam || res.Name != "alpha" {
		t.Fatalf("安装结果类型/名称不符: %+v", res)
	}
	if len(res.Members) != 2 {
		t.Fatalf("安装结果成员不符: %v", res.Members)
	}

	// 团队本体落位
	teamDoc, err := os.ReadFile(filepath.Join(dstTeamsDir, "alpha", "TEAM.md"))
	if err != nil {
		t.Fatalf("TEAM.md 未落位: %v", err)
	}
	if !strings.Contains(string(teamDoc), "leader: lead-agent") {
		t.Fatalf("TEAM.md frontmatter 不完整:\n%s", teamDoc)
	}
	// 成员 Agent 落位
	for _, name := range []string{"lead-agent", "dev-agent"} {
		if _, err := os.Stat(filepath.Join(dstAgentsDir, name, "IDENTITY.md")); err != nil {
			t.Fatalf("成员 %s 未落位: %v", name, err)
		}
	}
	// 技能分流：包内技能不在目标全局库 → 进全局库
	if len(res.SkillsGlobal) == 0 || res.SkillsGlobal[0] != "shared-skill" {
		t.Fatalf("技能落位结果不符: %v", res.SkillsGlobal)
	}
}

// TestInstallTeamConflict 团队目录或成员 Agent 已存在时默认拒绝，Overwrite 放行。
func TestInstallTeamConflict(t *testing.T) {
	_, agents, skills := newTestStores(t)
	mustSaveAgent(t, agents, "lead-agent", "职责", nil)
	team := mustWriteTeamDoc(t, filepath.Join(t.TempDir(), "teams"), "alpha", "lead-agent",
		[]string{"lead-agent"}, "正文")
	outPath := filepath.Join(t.TempDir(), "alpha.mindpkg")
	if _, _, err := ExportTeam(team, agents, skills, outPath); err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	dstRoot := t.TempDir()
	dstAgents := filepath.Join(dstRoot, "agents")
	dstTeams := filepath.Join(dstRoot, "teams")
	dstSkills := filepath.Join(dstRoot, "skills")
	for _, d := range []string{dstAgents, dstTeams, dstSkills} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// 预置同名团队目录 → 默认拒绝
	if err := os.MkdirAll(filepath.Join(dstTeams, "alpha"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(outPath, InstallOptions{AgentsDir: dstAgents, TeamsDir: dstTeams, GlobalDir: dstSkills}); err == nil {
		t.Fatal("同名团队已存在应默认拒绝")
	} else if !strings.Contains(err.Error(), "团队") {
		t.Fatalf("冲突信息应指明团队: %v", err)
	}
	// 预置同名成员 Agent → 默认拒绝
	if err := os.RemoveAll(filepath.Join(dstTeams, "alpha")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dstAgents, "lead-agent"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(outPath, InstallOptions{AgentsDir: dstAgents, TeamsDir: dstTeams, GlobalDir: dstSkills}); err == nil {
		t.Fatal("同名成员 Agent 已存在应默认拒绝")
	}
	// Overwrite 放行
	if _, err := Install(outPath, InstallOptions{AgentsDir: dstAgents, TeamsDir: dstTeams, GlobalDir: dstSkills, Overwrite: true}); err != nil {
		t.Fatalf("Overwrite 应放行: %v", err)
	}
}

// TestExportTeamMissingMembers 成员在本地缺失时告警不阻断；全部缺失时报错。
func TestExportTeamMissingMembers(t *testing.T) {
	_, agents, skills := newTestStores(t)
	mustSaveAgent(t, agents, "lead-agent", "职责", nil)
	// ghost 未创建
	team := mustWriteTeamDoc(t, filepath.Join(t.TempDir(), "teams"), "alpha", "lead-agent",
		[]string{"lead-agent", "ghost"}, "正文")

	outPath := filepath.Join(t.TempDir(), "alpha.mindpkg")
	manifest, warnings, err := ExportTeam(team, agents, skills, outPath)
	if err != nil {
		t.Fatalf("部分成员缺失不应阻断导出: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "ghost") {
		t.Fatalf("缺失成员应告警: %v", warnings)
	}
	if len(manifest.Members) != 1 || manifest.Members[0] != "lead-agent" {
		t.Fatalf("包内成员应为 lead-agent: %v", manifest.Members)
	}

	// 全部缺失（仅引用不存在的成员）
	empty := mustWriteTeamDoc(t, filepath.Join(t.TempDir(), "teams"), "beta", "ghost",
		[]string{"ghost"}, "正文")
	if _, _, err := ExportTeam(empty, agents, skills, filepath.Join(t.TempDir(), "beta.mindpkg")); err == nil {
		t.Fatal("全部成员缺失应报错")
	}
}

// TestTeamManifestRoundTrip 清单 Leader/Members 字段 JSON 序列化口径（市场透传契约）。
func TestTeamManifestRoundTrip(t *testing.T) {
	data, err := jsonMarshal(Manifest{Format: PackageFormat, Kind: KindTeam, Name: "alpha",
		Leader: "lead-agent", Members: []string{"lead-agent", "dev-agent"}})
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Kind != KindTeam || m.Leader != "lead-agent" || len(m.Members) != 2 {
		t.Fatalf("清单 round-trip 不一致: %+v", m)
	}
	raw := map[string]any{}
	_ = json.Unmarshal(data, &raw)
	if raw["leader"] != "lead-agent" {
		t.Fatalf("leader 字段 JSON 键应为 snake_case 直映射: %v", raw)
	}
}
