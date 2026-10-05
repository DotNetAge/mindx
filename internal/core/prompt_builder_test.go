package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/goharness/logging"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// mustLeaderTestSession 构造挂在临时工作目录上的会话（Build 需要 Session 载体）。
// teamName 非空时会话绑定团队（WithTeamNameOption，模拟团队会话）。
func mustLeaderTestSession(t *testing.T, agentName string, teamName ...string) *goharnesssession.Session {
	t.Helper()
	store, err := mindxses.NewRoutedSessionStore(t.TempDir() + "/data/session_dirs.json")
	if err != nil {
		t.Fatalf("NewRoutedSessionStore() error = %v", err)
	}
	opts := []goharnesssession.SessionOption{goharnesssession.WithProjectDirOption(t.TempDir())}
	if len(teamName) > 0 && teamName[0] != "" {
		opts = append(opts, goharnesssession.WithTeamNameOption(teamName[0]))
	}
	info, err := goharnesssession.CreateSession(context.Background(), store, agentName, opts...)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := goharnesssession.Load(context.Background(), info.SessionID, agentName, store, logging.DefaultLogger())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return sess
}

// mustTeamTestStore 构造含 alpha 团队的团队注册表（负责人 lead，成员 writer/coder）。
func mustTeamTestStore(t *testing.T, profile string) *teamstore.Store {
	t.Helper()
	dir := t.TempDir()
	store, _, err := teamstore.Load(dir)
	if err != nil {
		t.Fatalf("teamstore.Load() error = %v", err)
	}
	if err := store.Save(&teamstore.Team{
		Meta: teamstore.TeamMeta{
			Name:        "alpha",
			Description: "产品研发团队",
			Leader:      "lead",
			Members:     []string{"lead", "writer", "coder"},
		},
		Profile: profile,
	}); err != nil {
		t.Fatalf("Save team 失败: %v", err)
	}
	return store
}

// TestBuildTeamSection 单元验证负责人职责段的渲染规则（注入三重条件：
// 会话绑定团队 + 团队存在 + 当前 Agent 是负责人）：
// Profile 正文优先于兜底文案，成员名单与查询引导恒在段尾。
func TestBuildTeamSection(t *testing.T) {
	teams := mustTeamTestStore(t, "")
	pb := NewPromptBuilder(nil, nil, func() *teamstore.Store { return teams }, "", "", nil)
	leader := &agentstore.Agent{Meta: agentstore.AgentMeta{Name: "lead", Role: "负责人"}}
	teamSess := mustLeaderTestSession(t, "lead", "alpha")

	// 无 Profile：兜底文案（标题 + team 名 + 职责声明），成员名单与查询引导在段尾
	sec := pb.buildLeaderSection(leader, teamSess)
	if !strings.Contains(sec, "## 团队负责人职责") {
		t.Errorf("段落缺少标题:\n%s", sec)
	}
	if !strings.Contains(sec, "你是「alpha」团队的负责人") {
		t.Errorf("段落缺少 team 名:\n%s", sec)
	}
	if !strings.Contains(sec, "团队成员：lead、writer、coder") {
		t.Errorf("段落缺少成员名单:\n%s", sec)
	}
	if !strings.Contains(sec, "mindx agents info") {
		t.Errorf("段落缺少成员角色查询引导:\n%s", sec)
	}

	// 有 Profile：自定义内容整体替代兜底文案（与 IDENTITY/SOUL 同模式，
	// 标题由文件自身处理），成员名单与查询引导仍保留在段尾
	profile := "## 产品研发团队\n\n本团队负责 mindx 的产品设计与研发交付，成员按专业分工接单。"
	customTeams := mustTeamTestStore(t, profile)
	pbWithTeams := NewPromptBuilder(nil, nil, func() *teamstore.Store { return customTeams }, "", "", nil)
	sec3 := pbWithTeams.buildLeaderSection(leader, teamSess)
	if !strings.Contains(sec3, "## 产品研发团队") {
		t.Errorf("Team Profile 自定义内容应原样注入:\n%s", sec3)
	}
	if strings.Contains(sec3, "你是「alpha」团队的负责人") {
		t.Errorf("有自定义内容时不应再出现兜底文案:\n%s", sec3)
	}
	if !strings.Contains(sec3, "团队成员：lead、writer、coder") {
		t.Errorf("自定义内容下成员名单仍应保留在段尾:\n%s", sec3)
	}
	if strings.Index(sec3, "## 产品研发团队") > strings.Index(sec3, "团队成员：lead") {
		t.Errorf("自定义内容应插入在成员名单之前:\n%s", sec3)
	}

	// 容错降级：个人会话（未绑定团队）/ 团队被删 / nil agent —— 段落整体不出现
	if got := pbWithTeams.buildLeaderSection(leader, mustLeaderTestSession(t, "lead")); got != "" {
		t.Errorf("未绑定团队的会话不应注入团队段，got:\n%s", got)
	}
	emptyTeams := func() *teamstore.Store { return nil }
	pbNoTeams := NewPromptBuilder(nil, nil, emptyTeams, "", "", nil)
	if got := pbNoTeams.buildLeaderSection(leader, teamSess); got != "" {
		t.Errorf("团队注册表不可用应容错返回空串，got:\n%s", got)
	}
	if got := pb.buildLeaderSection(nil, teamSess); got != "" {
		t.Errorf("nil agent 应返回空串，got:\n%s", got)
	}
}

// TestPromptBuilderBuildWithLeader 集成验证 Build()：团队会话中负责人的系统
// 提示词含负责人职责段；个人会话即使面对负责人也不注入（注入由 Session
// 绑定的 Team 驱动，与团队对话 = 与负责人对话只在团队会话成立）。
func TestPromptBuilderBuildWithLeader(t *testing.T) {
	dir := t.TempDir()
	store, _, err := agentstore.Load(dir)
	if err != nil {
		t.Fatalf("agentstore.Load() error = %v", err)
	}
	if err := store.Save(&agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:        "lead",
		Role:        "负责人",
		Description: "带团队",
	}}); err != nil {
		t.Fatalf("Save lead 失败: %v", err)
	}
	if err := store.Save(&agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:        "solo",
		Role:        "独立开发",
		Description: "单干",
	}}); err != nil {
		t.Fatalf("Save solo 失败: %v", err)
	}
	teams := mustTeamTestStore(t, "")

	pb := NewPromptBuilder(store, nil, func() *teamstore.Store { return teams }, "", "", nil)

	// 团队会话 + 负责人：Build() 输出含负责人职责段
	out := pb.Build("", mustLeaderTestSession(t, "lead", "alpha"))
	if !strings.Contains(out, "## 团队负责人职责") {
		t.Errorf("负责人的系统提示词应含职责段:\n%s", out)
	}
	if !strings.Contains(out, "团队成员：lead、writer、coder") {
		t.Errorf("职责段应含成员名单:\n%s", out)
	}

	// 个人会话 + 负责人：不注入（未绑定团队）
	outSoloSess := pb.Build("", mustLeaderTestSession(t, "lead"))
	if strings.Contains(outSoloSess, "团队负责人职责") {
		t.Errorf("个人会话不应注入团队段:\n%s", outSoloSess)
	}

	// 团队会话 + 非负责人 Agent：不注入
	out2 := pb.Build("", mustLeaderTestSession(t, "solo", "alpha"))
	if strings.Contains(out2, "团队负责人职责") {
		t.Errorf("非负责人的系统提示词不应含职责段:\n%s", out2)
	}
}

// TestPromptBuilderLoadsAgentsMD 验证 AGENTS.md 机制（公共行为准则外移）：
// 用户目录存在 AGENTS.md 时内容注入 Build() 输出；文件缺失或用户目录
// 未设置时整段跳过，不报错（容错优先）。
func TestPromptBuilderLoadsAgentsMD(t *testing.T) {
	dir := t.TempDir()
	store, _, err := agentstore.Load(dir)
	if err != nil {
		t.Fatalf("agentstore.Load() error = %v", err)
	}
	if err := store.Save(&agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:        "solo",
		Role:        "独立开发",
		Description: "单干",
	}}); err != nil {
		t.Fatalf("Save solo 失败: %v", err)
	}

	// 用户目录存在 AGENTS.md：内容原样注入
	prefs := t.TempDir()
	agentsMD := "## 必须遵守\n\n- 测试准则条目"
	if err := os.WriteFile(filepath.Join(prefs, agentsMDFileName), []byte(agentsMD), 0644); err != nil {
		t.Fatalf("写入 AGENTS.md 失败: %v", err)
	}
	pb := NewPromptBuilder(store, nil, nil, prefs, "", nil)
	out := pb.Build("", mustLeaderTestSession(t, "solo"))
	if !strings.Contains(out, "测试准则条目") {
		t.Errorf("Build() 应注入用户目录 AGENTS.md 内容:\n%s", out)
	}

	// 文件缺失：整段跳过
	pbMissing := NewPromptBuilder(store, nil, nil, t.TempDir(), "", nil)
	outMissing := pbMissing.Build("", mustLeaderTestSession(t, "solo"))
	if strings.Contains(outMissing, "测试准则条目") {
		t.Errorf("AGENTS.md 缺失时不应注入公共准则段:\n%s", outMissing)
	}

	// 用户目录未设置：整段跳过（适用于独立测试场景）
	pbNoPrefs := NewPromptBuilder(store, nil, nil, "", "", nil)
	outNoPrefs := pbNoPrefs.Build("", mustLeaderTestSession(t, "solo"))
	if strings.Contains(outNoPrefs, "测试准则条目") {
		t.Errorf("用户目录未设置时不应注入公共准则段:\n%s", outNoPrefs)
	}
}

// TestBuildEnvSectionContainsExperienceSystem 验证配置环境段注入经验沉淀体系
// 说明：.agents/ 四目录作用与 MemorySearch/TeamList 工具配合，逐字静态输出。
func TestBuildEnvSectionContainsExperienceSystem(t *testing.T) {
	dir := t.TempDir()
	store, _, err := agentstore.Load(dir)
	if err != nil {
		t.Fatalf("agentstore.Load() error = %v", err)
	}
	if err := store.Save(&agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:        "solo",
		Role:        "独立开发",
		Description: "单干",
	}}); err != nil {
		t.Fatalf("Save solo 失败: %v", err)
	}

	pb := NewPromptBuilder(store, nil, nil, "", "", nil)
	out := pb.Build("", mustLeaderTestSession(t, "solo"))

	for _, want := range []string{"经验沉淀体系", "notes/", "skills/", "reports/", "MemorySearch"} {
		if !strings.Contains(out, want) {
			t.Errorf("配置环境段应含 %q:\n%s", want, out)
		}
	}
	// TeamXXX 已从默认注册剥离，系统提示不得再引用 TeamList
	if strings.Contains(out, "TeamList") {
		t.Errorf("系统提示不应引用已剥离的 TeamList 工具:\n%s", out)
	}
}
