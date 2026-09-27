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
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// mustLeaderTestSession 构造挂在临时工作目录上的会话（Build 需要 Session 载体）。
func mustLeaderTestSession(t *testing.T, agentName string) *goharnesssession.Session {
	t.Helper()
	store, err := mindxses.NewRoutedSessionStore(t.TempDir() + "/data/session_dirs.json")
	if err != nil {
		t.Fatalf("NewRoutedSessionStore() error = %v", err)
	}
	info, err := goharnesssession.CreateSession(context.Background(), store, agentName,
		goharnesssession.WithProjectDirOption(t.TempDir()))
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := goharnesssession.Load(context.Background(), info.SessionID, agentName, store, logging.DefaultLogger())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return sess
}

// TestBuildTeamSection 单元验证负责人职责段的渲染规则：
// 带 members 注入职责段（TEAM.md 自定义内容优先于兜底文案，成员名单与
// filter 引导恒在段尾）；非 Leader / nil 返回空串。
func TestBuildTeamSection(t *testing.T) {
	leader := &agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:    "lead",
		Role:    "负责人",
		Team:    "产品研发",
		Members: []string{"writer", "coder"},
	}}

	// 无 TEAM.md：兜底文案（标题 + team 名 + 职责声明），成员名单与 filter 引导在段尾
	sec := buildLeaderSection(leader)
	if !strings.Contains(sec, "## 团队负责人职责") {
		t.Errorf("段落缺少标题:\n%s", sec)
	}
	if !strings.Contains(sec, "你是「产品研发」团队的负责人") {
		t.Errorf("段落缺少 team 名:\n%s", sec)
	}
	if !strings.Contains(sec, "团队成员：writer、coder") {
		t.Errorf("段落缺少成员名单:\n%s", sec)
	}
	if !strings.Contains(sec, "mindx agents info") {
		t.Errorf("段落缺少成员角色查询引导:\n%s", sec)
	}

	// 带 members 但未写 team：兜底文案不出现空队名
	noTeam := &agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:    "lead2",
		Members: []string{"writer"},
	}}
	sec2 := buildLeaderSection(noTeam)
	if !strings.Contains(sec2, "你是所在团队的负责人") {
		t.Errorf("无 team 名应使用兜底文案:\n%s", sec2)
	}
	if strings.Contains(sec2, "「」") {
		t.Errorf("不应渲染空队名:\n%s", sec2)
	}

	// 有 TEAM.md（TeamDuty 非空）：自定义内容整体替代兜底文案（与 IDENTITY/SOUL
	// 同模式，标题由文件自身处理），成员名单与 filter 引导仍保留在段尾
	custom := &agentstore.Agent{Meta: agentstore.AgentMeta{
		Name:    "lead3",
		Team:    "产品研发",
		Members: []string{"writer"},
	}, TeamDuty: "## 产品研发团队\n\n本团队负责 mindx 的产品设计与研发交付，成员按专业分工接单。"}
	sec3 := buildLeaderSection(custom)
	if !strings.Contains(sec3, "## 产品研发团队") {
		t.Errorf("TEAM.md 自定义内容应原样注入:\n%s", sec3)
	}
	if strings.Contains(sec3, "你是「产品研发」团队的负责人") {
		t.Errorf("有自定义内容时不应再出现兜底文案:\n%s", sec3)
	}
	if !strings.Contains(sec3, "团队成员：writer") {
		t.Errorf("自定义内容下成员名单仍应保留在段尾:\n%s", sec3)
	}
	if !strings.Contains(sec3, "mindx agents info") {
		t.Errorf("自定义内容下查询引导仍应保留:\n%s", sec3)
	}
	if strings.Index(sec3, "## 产品研发团队") > strings.Index(sec3, "团队成员：writer") {
		t.Errorf("自定义内容应插入在成员名单之前:\n%s", sec3)
	}

	// 非 Leader（Members 为空）与 nil agent：段落整体不出现
	if got := buildLeaderSection(&agentstore.Agent{Meta: agentstore.AgentMeta{Name: "solo"}}); got != "" {
		t.Errorf("非 Leader 应返回空串，got:\n%s", got)
	}
	if got := buildLeaderSection(nil); got != "" {
		t.Errorf("nil agent 应返回空串，got:\n%s", got)
	}
}

// TestPromptBuilderBuildWithLeader 集成验证 Build()：负责人 Agent 的系统提示词
// 含负责人职责段，普通 Agent 不含（TODO 定案：条件注入，非负责人无段落）。
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
		Team:        "产品研发",
		Members:     []string{"writer", "coder"},
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

	pb := NewPromptBuilder(store, nil, "", "", nil)

	// 负责人：Build() 输出含负责人职责段
	out := pb.Build("", mustLeaderTestSession(t, "lead"))
	if !strings.Contains(out, "## 团队负责人职责") {
		t.Errorf("负责人的系统提示词应含职责段:\n%s", out)
	}
	if !strings.Contains(out, "团队成员：writer、coder") {
		t.Errorf("职责段应含成员名单:\n%s", out)
	}

	// 普通成员：Build() 输出不含该段
	out2 := pb.Build("", mustLeaderTestSession(t, "solo"))
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
	pb := NewPromptBuilder(store, nil, prefs, "", nil)
	out := pb.Build("", mustLeaderTestSession(t, "solo"))
	if !strings.Contains(out, "测试准则条目") {
		t.Errorf("Build() 应注入用户目录 AGENTS.md 内容:\n%s", out)
	}

	// 文件缺失：整段跳过
	pbMissing := NewPromptBuilder(store, nil, t.TempDir(), "", nil)
	outMissing := pbMissing.Build("", mustLeaderTestSession(t, "solo"))
	if strings.Contains(outMissing, "测试准则条目") {
		t.Errorf("AGENTS.md 缺失时不应注入公共准则段:\n%s", outMissing)
	}

	// 用户目录未设置：整段跳过（适用于独立测试场景）
	pbNoPrefs := NewPromptBuilder(store, nil, "", "", nil)
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

	pb := NewPromptBuilder(store, nil, "", "", nil)
	out := pb.Build("", mustLeaderTestSession(t, "solo"))

	for _, want := range []string{"经验沉淀体系", "notes/", "skills/", "reports/", "MemorySearch", "TeamList"} {
		if !strings.Contains(out, want) {
			t.Errorf("配置环境段应含 %q:\n%s", want, out)
		}
	}
}
