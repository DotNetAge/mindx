package svc

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/skill"
)

// NewSkillsPrompt 返回一个覆盖默认 buildSkillsCatalog 提示词的构建函数。
// 保持相同的头部和加载策略尾部，但只列出技能名称而非完整描述。
//
// 加载策略尾部包含使用 "mindx skill list -f" 查看特定技能详细描述的提示。
func NewSkillsPrompt() func([]*skill.Skill) string {
	return func(skills []*skill.Skill) string {
		if len(skills) == 0 {
			return ""
		}

		header := "## 能力（可用技能）\n" +
			"以下专业技能是否能完成用户要求的任务。如果技能匹配，使用 Skill 工具加载其指令，这将指导你完成特定领域的工作流程并提供额外的工具。\n\n" +
			"### 副作用规则\n" +
			"- Skill 工具的返回值代表该技能的完整知识。对于任何给定的技能名称，每个会话中最多只能调用 Skill 一次。之后，对该技能内容的所有引用必须依赖内存中已有的内容 — 不要使用任何工具（Bash、Read、Grep、Glob、WebFetch 等）重新读取其文件。\n\n" +
			"### 执行前自检\n" +
			"在调用 Bash、Read 或 Grep 访问文件或目录内容之前，必须先执行此检查：\n" +
			"1. 角色门控 (P0)：此任务是否在我的职责范围内？如果否 → 按行为准则委托，不要继续。\n" +
			"2. 如果在职责范围内：上述能力列表是否包含覆盖此任务的技能？\n" +
			"3. 如果是，我是否已通过 Skill 加载？\n" +
			"4. 输出你的推理和决策：\n" +
			"   - 推理：[职责检查结果 + 考虑了哪个技能]\n" +
			"   - 决策：委托（如果超出职责）| Skill（如果尚未加载）| 使用工具继续（如果已加载或无匹配技能）\n"

		footer := "\n### 加载策略\n" +
			"- 延迟加载技能：仅在即将执行需要它的任务时加载\n" +
			"- 加载前查看技能描述，运行：`mindx skill list -f \"<skill1>,<skill2>,...\"`\n" +
			"- 每个技能加载后即持久化 — 不要重复加载同一个技能\n"

		var nameBuilder strings.Builder
		for _, s := range skills {
			nameBuilder.WriteString(fmt.Sprintf("- %s\n", s.Name))
		}

		return header + nameBuilder.String() + footer
	}
}

// NewEnvironmentPrompt 返回一个覆盖默认 Environment 段落的构建函数。
// 在基础信息（ProjectDir、SessionDir）之上补充 SessionID、本地时间、用户配置目录和 Python 虚拟环境路径。
func NewEnvironmentPrompt(userPrefsDir, venvDir string) func(agents.EnvsParams) string {
	return func(params agents.EnvsParams) string {
		var sb strings.Builder
		sb.WriteString("## 配置环境\n")

		// ProjectDir：用户的工作目录（持久化，数据源头）。
		projectDir := params.ProjectDir
		if projectDir == "" {
			projectDir, _ = os.Getwd()
		}
		sb.WriteString(fmt.Sprintf("- **项目目录**: %s\n", projectDir))
		sb.WriteString(" 用户工作目录 — 文件在此永久保留，跨会话持续存在。\n")
		sb.WriteString(" 在此修改用户现有文件并创建长期使用的输出。\n")

		// SessionDir：限定于当前对话的临时工作区。
		sessionDir := params.SessionDir
		if sessionDir == "" {
			sessionDir = "（未设置 — 临时文件不会保留）"
		}
		sb.WriteString(fmt.Sprintf("- **会话目录**: %s\n", sessionDir))
		sb.WriteString("  当前对话的临时工作区。\n")
		sb.WriteString("  对话结束后内容将被删除 — 不要将重要工作放在此处。\n")

		// if userPrefsDir != "" {
		// 	sb.WriteString(fmt.Sprintf("- **用户配置**: %s\n", userPrefsDir))
		// 	sb.WriteString("  应用配置、技能和 Agent 定义。\n")
		// }
		if venvDir != "" {
			sb.WriteString(fmt.Sprintf("- **Python 虚拟环境**: %s\n", venvDir))
			sb.WriteString("  Python 脚本执行使用的虚拟环境。\n")
		}

		if params.SessionID != "" {
			sb.WriteString(fmt.Sprintf("- **会话ID**: %s\n", params.SessionID))
		}
		sb.WriteString(fmt.Sprintf("- **本地时间**: %s\n", time.Now().Format("2006-01-02")))

		return sb.String()
	}
}

// NewSearchStrategyPrompt 返回一个覆盖默认 Search Strategy 段落的构建函数。
// 指示 LLM 优先使用知识库工具（QuickSearch、QuickExplore、FindRelation）而非传统文件工具和网络搜索。
func NewSearchStrategyPrompt() func() string {
	return func() string {
		return "## 搜索策略\n\n" +
			"1. 对于本地搜索，优先使用 QuickSearch 没有数据才回退 Grep — " +
			"它通过语义搜索，而非仅靠文件名或文本模式匹配。\n" +
			"2. 当问题可能涉及当前项目时，也先尝试 QuickSearch 而非 WebSearch。\n" +
			"3. 对于外部话题或 QuickSearch 无结果时，回退到网络搜索（WebSearch）。\n" +
			"4. 浏览项目结构时，优先使用 QuickExplore 而非 LS 或 Glob — " +
			"它返回带有语义摘要的目录树。当 QuickExplore 不可用或不够用时，回退到 LS/Glob。\n" +
			"5. 对于依赖关系或关联性问题，使用 FindRelation — " +
			"它遍历知识图谱来展示实体之间的连接关系。"
	}
}
