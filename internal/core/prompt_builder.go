package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
)

// 本文件是 mindx 侧的基础系统提示词组装器（PR-PROMPTS 第一节定案）：
//
//   - 生成逻辑独立成类、独立成文件维护；
//   - 拼接顺序固定为 IDENTITY → SOUL → Skill summary list → AGENTS.md（用户目录文件）→ Env → 扩展规则，
//     满足"静态在前、动态在后"，保证 KV 缓存前缀稳定；
//   - goharness 不再追加任何文案段，输出即本组装器返回的单条 system 消息；
//     Memory 注入仍由 goharness Hook 承载（另一条动态路径，不在此处）。

// agentsMDFileName 是全体 Agent 公共行为准则文件名（AGENTS.md 机制）：
// 源文件随应用发布（runtime/AGENTS.md），安装时释放至用户目录，用户可自行编辑。
const agentsMDFileName = "AGENTS.md"

// agentDiscoveryIntro 是 Agent 发现引导文案（扩展规则段固定条目，
// 原实现注册进规则注册表，P4 起直接渲染，不再写入用户规则文件）。
const agentDiscoveryIntro = "Agent 发现：当需要查找或列出可用 Agent 时，运行 'mindx agent list'（或 'mindx agent list --json' 获取结构化输出）。列表显示 Agent 名称、角色、描述及其技能。用于查找合适的 Agent 并通过 SubAgent 进行委托。"

// experienceSystemNote 是经验沉淀体系说明（配置环境段固定条目）：
// 说明项目目录 .agents/ 体系的作用与配合工具，行为判据（随手记、只读共享、
// 公理引用、复盘触发）由 runtime/AGENTS.md 公共规则承载，此处只交代环境事实。
// 记忆加载规则（2026-10-06 双系统记忆理论）：压缩产物为多条独立记忆条目
// （每条一个 markdown 文件），Agent 以文件工具自助加载，无注入 Hook 与检索工具。
const experienceSystemNote = "- **经验沉淀体系**: 项目目录下的 .agents/ 目录承载跨会话经验 —— AGENTS.md 为项目军规与工作目标；notes/ 记录随手笔记；memory/ 存放会话压缩产生的记忆条目（每条一个 markdown 文件）；skills/ 存放可复用技能；references/ 存放权威资料公理（原文只读，引用标注 [ref:ID]）；reports/ 存放复盘日报。会话开始处理实质性任务前或上下文压缩后恢复工作时，先列出 .agents/memory/ 目录并读取时间上最相近的相关记忆条目了解历史上下文；需要回忆更早的决策时用 Grep 按关键词检索记忆与笔记目录。"

// PromptBuilder 组装 mindx 侧的基础系统提示词。
// 依赖注入 AgentStore（身份/行为/Skills 声明）与 SkillStore（技能摘要目录），
// 环境参数在构造时固化（它们是 mindx 的领地，不随会话变化）；
// rulesSection 为扩展规则段渲染回调（App.BuildRulesSection，P4 规则收口）。
// teams 为团队注册表闭包回调（App 层 func(){ return a.teams }）：Build 每次
// Ask 实时解引用，ReloadTeams 原子换指针后即刻生效，promptBuilder 与
// runtimeCache 均无需失效（Runtime 的方法值隐式持有本实例，直接持指针会使
// 换指针后已建 Runtime 仍读旧库，闭包是关键）。
type PromptBuilder struct {
	agents       *agentstore.AgentStore
	skills       *skillstore.Store
	teamsFn      func() *teamstore.Store
	userPrefs    string // ~/.mindx 用户配置目录（Env 段）
	venvDir      string // Python 虚拟环境目录（Env 段）
	rulesSection func() string
}

// NewPromptBuilder 创建提示词组装器。
func NewPromptBuilder(agents *agentstore.AgentStore, skills *skillstore.Store, teamsFn func() *teamstore.Store, userPrefs, venvDir string, rulesSection func() string) *PromptBuilder {
	return &PromptBuilder{
		agents:       agents,
		skills:       skills,
		teamsFn:      teamsFn,
		userPrefs:    userPrefs,
		venvDir:      venvDir,
		rulesSection: rulesSection,
	}
}

// Build 构造指定会话的基础系统提示词，作为 WithBaseSystemPrompt 的 builder。
// agentName 取自会话归属；Agent 不存在或 AgentStore 不可用时返回空字符串
// （goharness 会跳过基础段，输出空 system 消息，适用于独立测试场景）。
func (b *PromptBuilder) Build(_ string, s *session.Session) string {
	if b == nil || b.agents == nil || s == nil {
		return ""
	}
	agent := b.agents.Get(s.AgentName())
	if agent == nil {
		return ""
	}

	var sections []string

	// 1. IDENTITY（身份段，格式与旧版保持一致）
	if sec := buildIdentitySection(agent); sec != "" {
		sections = append(sections, sec)
	}

	// 2. SOUL（行为规则与负责范围）：基于文件的段不注入标题——标题由
	// SOUL.md 自身处理（含空判断，标题不会空悬）；作者可用任意标题结构
	if soul := strings.TrimSpace(agent.Soul); soul != "" {
		sections = append(sections, soul)
	}

	// 3. 团队负责人职责（条件段：会话绑定了团队且当前 Agent 是该团队负责人时
	// 注入；团队 Profile 正文优先，缺失走兜底文案）；静态段紧随身份/行为段，
	// 维持「静态在前、动态在后」的 KV 缓存前缀稳定。
	// 生成根基：团队会话的 System Prompt 仍由 Agent（负责人）生成，本段只是
	// buildLeaderSection 的内容来源更换，其余段落与结构零改动。
	if sec := b.buildLeaderSection(agent, s); sec != "" {
		sections = append(sections, sec)
	}

	// 4. Skill summary list（技能摘要目录；仅静态注册表条目——工作目录内的
	// 动态技能绝不进入系统提示词，否则 KV 缓存前缀全部失效）
	if sec := b.buildSkillsSection(agent); sec != "" {
		sections = append(sections, sec)
	}

	// 5. AGENTS.md（全体 Agent 公共规则，加载自用户目录）
	if sec := b.loadAgentsMD(); sec != "" {
		sections = append(sections, sec)
	}

	// 6. Env（环境信息 + 搜索策略）
	if sec := b.buildEnvSection(s); sec != "" {
		sections = append(sections, sec)
	}

	// 7. 扩展规则（权限规则 + Agent 发现引导 + 用户规则，App.BuildRulesSection 渲染；
	// P4 定案由 mindx 拼装，goharness 侧 ruleReg 已退役）
	if b.rulesSection != nil {
		if sec := b.rulesSection(); sec != "" {
			sections = append(sections, sec)
		}
	}

	return strings.Join(sections, "\n\n")
}

// buildIdentitySection 渲染身份段（评审定案）：
//   - IDENTITY.md 带正文（frontmatter 之外）时，优先取正文作为整个 Agent 的
//     「人设定义」，原样拼接（标题由定义文件自身处理）；
//   - 正文为空时才启用兜底生成规则：由 frontmatter 的 name/role/description
//     拼出标准角色定义（此段为 mindx 动态生成，标题由 mindx 注入）。
func buildIdentitySection(agent *agentstore.Agent) string {
	meta := agent.Meta
	if intro := strings.TrimSpace(meta.Introduction); intro != "" {
		return intro
	}
	return fmt.Sprintf("## 角色定义 \n 我叫 %s 是一名 %s, %s",
		meta.Name, meta.Role, meta.Description)
}

// buildLeaderSection 渲染团队负责人职责段。
// 注入条件（三重，任一不满足整段跳过）：
//  1. 会话绑定了团队（s.TeamName() 非空——会话级事实，个人会话即使面对
//     负责人也不注入，与团队对话 = 与负责人对话只在团队会话成立）；
//  2. 团队注册表中存在该团队（Team 中途被删则容错降级为普通个体，log 不可用
//     静默跳过）；
//  3. 当前 Agent 是该团队的负责人（team.Leader == agent.Name，Leader 被换后
//     旧负责人的团队会话容错降级）。
//
// 内容优先级与 IDENTITY/SOUL 同模式：Team 实体 Profile 正文（teams/<team>/TEAM.md
// 正文）原样拼接（标题由文件自身处理），缺失时走兜底文案（标题由本函数注入）。
// 「团队成员」名单区为机械部分，两种形态下都保留在段尾：按注册表实时解析
// 实际雇佣成员（存在且 hired）并附一句概览式角色描述，供 LLM 把正文以「角色」
// 指代的成员对号到实际名单；无任何就绪成员时降级为空缺文案。
func (b *PromptBuilder) buildLeaderSection(agent *agentstore.Agent, s *session.Session) string {
	if agent == nil || s == nil {
		return ""
	}
	teamName := s.TeamName()
	if teamName == "" {
		return ""
	}
	if b == nil || b.teamsFn == nil {
		return ""
	}
	teams := b.teamsFn()
	if teams == nil {
		return ""
	}
	team := teams.Get(teamName)
	if team == nil || team.Meta.Leader != agent.Name() {
		return ""
	}

	var sb strings.Builder
	if profile := strings.TrimSpace(team.Profile); profile != "" {
		sb.WriteString(profile)
	} else {
		sb.WriteString("## 团队负责人职责\n\n")
		fmt.Fprintf(&sb, "你是「%s」团队的负责人。", teamName)
		sb.WriteString("作为负责人，你对团队的工作结果负最终责任，协调与分派团队成员的工作是你的重要职责之一；" +
			"需要成员配合时，把任务委托给对应的成员并跟进交付结果。")
	}
	sb.WriteString("\n\n")
	// 成员名单区：把 TEAM.md 正文中以「角色」指代的成员对号到实际雇佣名单——
	// 仅列出注册表中存在且已雇佣的成员，附一句概览式角色描述；
	// 未雇佣或未注册的成员不列入（负责人不能委托不在编制内的成员）。
	lines := make([]string, 0, len(team.Meta.Members))
	for _, name := range team.Meta.Members {
		if line := b.memberRosterLine(name); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		sb.WriteString("团队成员：（暂无已雇佣成员，相关职责由你直接承担，并提示用户补充编制）\n")
		return sb.String()
	}
	sb.WriteString("团队成员：\n")
	sb.WriteString(strings.Join(lines, "\n"))
	sb.WriteString("\n\n如果不了解成员的职责，可以运行 `mindx agents info <成员名...>` 查看成员的角色与职责，再决定委托对象。")
	return sb.String()
}

// memberRosterLine 渲染成员名单中的一行：「- 成员名（一句话角色描述）」。
// 成员未注册或未雇佣时返回空串（不列入名单）；角色描述取 description 概览，
// 压缩空白（含换行）后截取第一句，超长截断到 60 字符，保证名单紧凑可扫读。
func (b *PromptBuilder) memberRosterLine(name string) string {
	if b.agents == nil || name == "" {
		return ""
	}
	a := b.agents.Get(name)
	if a == nil || !a.Meta.Hired {
		return ""
	}
	desc := strings.Join(strings.Fields(a.Meta.Description), " ")
	if idx := strings.IndexAny(desc, "。！？；；"); idx >= 0 {
		desc = strings.TrimSpace(desc[:idx])
	}
	if runes := []rune(desc); len(runes) > 60 {
		desc = string(runes[:60])
	}
	if desc == "" {
		return fmt.Sprintf("- %s", name)
	}
	return fmt.Sprintf("- %s（%s）", name, desc)
}

// loadAgentsMD 读取用户目录下的 AGENTS.md（全体 Agent 公共行为准则）。
// 文件由安装释放（ExtractWorkspace）与版本同步（SyncRuntimeAssets）维护，
// 用户可自行编辑——每次 Build 现场读取，编辑即时生效。
// 用户目录未设置、文件缺失或为空时返回空串，整段跳过（容错优先，
// 不阻塞其余段落的组装，适用于独立测试场景）。
func (b *PromptBuilder) loadAgentsMD() string {
	if b == nil || b.userPrefs == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(b.userPrefs, agentsMDFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// buildSkillsSection 渲染技能摘要目录段（沿用 mindx 既有目录格式）。
// 仅渲染静态注册表（全局库 + Agent 级）条目；工作目录内的动态技能不经此
// 段注入（军规：动态技能经 mindx skills discovery 发现、Skill 工具按需加载，
// 会话中途改写 system prompt 会使 KV 缓存前缀全部失效）。
func (b *PromptBuilder) buildSkillsSection(agent *agentstore.Agent) string {
	if b.skills == nil {
		return ""
	}
	catalog := b.skills.Catalog(agent.Meta.Name, agent.Meta.Skills)

	if len(catalog) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 可用技能\n" +
		"以下专业技能是否能完成用户要求的任务。如果技能匹配，使用 Skill 工具加载其指令，这将指导你完成特定领域的工作流程并提供额外的工具。\n\n")
	for _, sk := range catalog {
		sb.WriteString(fmt.Sprintf("- %s - %s\n", sk.Name, sk.Description))
	}
	sb.WriteString("### 执行前自检\n" +
		"在调用 Bash、Read 或 Grep 访问文件或目录内容之前，必须先执行此检查：\n" +
		"1. 上述能力列表是否包含覆盖此任务的技能？\n" +
		"2. 如果是，我是否已通过 Skill 加载？\n" +
		"3. 输出你的推理和决策：\n" +
		"   - 推理：[考虑了哪个技能]\n" +
		"   - 决策：Skill（如果尚未加载）| 使用工具继续（如果已加载或无匹配技能）\n")
	return sb.String()
}

// buildEnvSection 渲染环境信息段（含搜索策略，沿用 mindx 既有格式）。
func (b *PromptBuilder) buildEnvSection(s *session.Session) string {
	var sb strings.Builder
	sb.WriteString("## 配置环境\n")

	// ProjectDir：用户的工作目录（持久化，数据源头）。
	projectDir := s.ProjectDir()
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	sb.WriteString(fmt.Sprintf("- **项目目录**: %s\n", projectDir))
	sb.WriteString(" 用户工作目录 — 文件在此永久保留，跨会话持续存在。\n")
	sb.WriteString(" 在此修改用户现有文件并创建长期使用的输出。\n")

	// 经验沉淀体系：紧随项目目录，说明 .agents/ 各目录作用与配合工具。
	sb.WriteString(experienceSystemNote + "\n")

	// SessionDir：限定于当前对话的临时工作区。
	sessionDir := s.SessionDir()
	if sessionDir == "" {
		sessionDir = "（未设置 — 临时文件不会保留）"
	}
	sb.WriteString(fmt.Sprintf("- **会话目录**: %s\n", sessionDir))
	sb.WriteString("  当前对话的临时工作区。\n")
	sb.WriteString("  对话结束后内容将被删除 — 不要将重要工作放在此处。\n")

	if b.venvDir != "" {
		sb.WriteString(fmt.Sprintf("- **Python 虚拟环境**: %s\n", b.venvDir))
		sb.WriteString("  Python 脚本执行使用的虚拟环境。\n")
	}

	if id := s.ID(); id != "" {
		sb.WriteString(fmt.Sprintf("- **会话ID**: %s\n", id))
	}
	if s.Sponsor() != "" {
		sb.WriteString(fmt.Sprintf("- **发起会话的Agent**: %s\n", s.Sponsor()))
	}
	sb.WriteString(fmt.Sprintf("- **本地时间**: %s\n", time.Now().Format("2006-01-02")))

	// 搜索策略
	sb.WriteString("\n## 搜索策略\n\n" +
		"1. 对于本地搜索，优先使用 Grep（grep/ripgrep）搜索项目文件内容，或使用语义搜索工具（如可用）。\n" +
		"2. 对于外部话题或本地搜索无结果时，回退到网络搜索（WebSearch）。\n" +
		"3. 浏览项目结构时，使用内置文件工具（LS、Glob）。")

	return sb.String()
}
