package core

import (
	"github.com/DotNetAge/mindx/internal/core/agentstore"
)

// AgentIsHired 判断 Agent 是否已被雇佣（会话可用）。
// Hired 为 IDENTITY.md frontmatter 一级字段，缺省未雇佣（false）。
func AgentIsHired(a *agentstore.Agent) bool {
	return a != nil && a.Meta.Hired
}

// AgentCategory 返回 Agent 的业务分类（frontmatter 一级字段，中文），
// 由旧 domains 字段迁移而来；缺省为空串。
func AgentCategory(a *agentstore.Agent) string {
	if a == nil {
		return ""
	}
	return a.Meta.Category
}

// HiredAgents 返回当前 App 注册表的雇佣视图：
// 会话可用入口（TUI /agent 切换、@ 补全、默认 Agent 解析、会话/调度创建校验）
// 只应使用该视图；管理与浏览入口（agent.get/update/score、CLI 全量查询）
// 走全量注册表，保证未雇佣的 Agent 可见、可雇佣、可打分。
func (a *App) HiredAgents() []*agentstore.Agent {
	return a.agents.Hired()
}
