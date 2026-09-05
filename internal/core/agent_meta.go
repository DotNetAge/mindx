package core

import (
	"strings"

	goharnessconfig "github.com/DotNetAge/goharness/config"
)

// AgentMetaIsHired 从 meta 映射中读取雇佣标记（键 "hired"）。
// 缺省视为未雇佣（false）；容忍 bool 与字符串写法（"true"/"false"，大小写不敏感），
// 避免手改 frontmatter 时类型写法不一致导致判定漂移。
func AgentMetaIsHired(meta map[string]any) bool {
	if meta == nil {
		return false
	}
	switch v := meta["hired"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

// AgentMetaDomains 从 meta 映射中读取领域标签（键 "domains"）。
// 容忍 []any（YAML 解析产物）与 []string 两种形态，元素统一转小写、去空白、去重，
// 忽略非字符串与空项；缺省返回 nil。
func AgentMetaDomains(meta map[string]any) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta["domains"]
	if !ok {
		return nil
	}
	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case []string:
		for _, s := range v {
			items = append(items, s)
		}
	default:
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	var domains []string
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			continue
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		domains = append(domains, s)
	}
	return domains
}

// AgentIsHired 判断 Agent 是否已被雇佣（会话可用）。缺省未雇佣。
func AgentIsHired(cfg *goharnessconfig.AgentConfig) bool {
	return cfg != nil && AgentMetaIsHired(cfg.Meta)
}

// AgentDomains 返回 Agent 的领域标签（已规范化），缺省为 nil。
func AgentDomains(cfg *goharnessconfig.AgentConfig) []string {
	if cfg == nil {
		return nil
	}
	return AgentMetaDomains(cfg.Meta)
}

// HiredAgentsOf 返回雇佣视图：注册表中已被雇佣的 Agent 列表。
// 会话可用入口（TUI /agent 切换、@ 补全、默认 Agent 解析、会话/调度创建校验）
// 只应使用该视图；管理与浏览入口（agent.get/update/score、CLI 全量查询）
// 走全量注册表，保证未雇佣的 Agent 可见、可雇佣、可打分。
func HiredAgentsOf(registry *goharnessconfig.AgentRegistry) []*goharnessconfig.AgentConfig {
	if registry == nil {
		return nil
	}
	var out []*goharnessconfig.AgentConfig
	for _, cfg := range registry.List() {
		if AgentIsHired(cfg) {
			out = append(out, cfg)
		}
	}
	return out
}

// HiredAgents 返回当前 App 注册表的雇佣视图（见 HiredAgentsOf）。
func (a *App) HiredAgents() []*goharnessconfig.AgentConfig {
	return HiredAgentsOf(a.agents)
}
