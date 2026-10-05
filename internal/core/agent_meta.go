package core

import (
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
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

// AgentIsLeader 判断 Agent 是否为某团队负责人（配置态派生值）：
// 团队注册表中存在 Leader == 该 Agent 名的团队。引用单向（Team→Agent），
// Agent 对团队零感知；仅用于展示投影（agent.list/get 的 is_leader、
// 选择器去重），不驱动注入——注入由 Session 绑定的 TeamName 驱动。
func AgentIsLeader(teams *teamstore.Store, a *agentstore.Agent) bool {
	if a == nil || teams == nil {
		return false
	}
	return teams.FindByLeader(a.Name()) != nil
}

// HiredAgents 返回当前 App 注册表的雇佣视图：
// 会话可用入口（TUI /agent 切换、@ 补全、默认 Agent 解析、会话/调度创建校验）
// 只应使用该视图；管理与浏览入口（agent.get/update/score、CLI 全量查询）
// 走全量注册表，保证未雇佣的 Agent 可见、可雇佣、可打分。
func (a *App) HiredAgents() []*agentstore.Agent {
	return a.agents.Hired()
}

// HiredTeams 返回当前 App 团队注册表的雇佣视图：
// 团队条目还需 Leader 在雇佣视图中（团队会话 resolve 到 Leader，
// Leader 未雇佣的团队无法建会话，不呈现）。
func (a *App) HiredTeams() []*teamstore.Team {
	teams := a.teams.Hired()
	if len(teams) == 0 {
		return nil
	}
	hired := make(map[string]bool, len(a.agents.Hired()))
	for _, ag := range a.agents.Hired() {
		hired[ag.Name()] = true
	}
	out := make([]*teamstore.Team, 0, len(teams))
	for _, t := range teams {
		if hired[t.Meta.Leader] {
			out = append(out, t)
		}
	}
	return out
}
