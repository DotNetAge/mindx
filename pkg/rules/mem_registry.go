package rules

import (
	"strings"
	"sync"
)

// MemRuleRegistry 基于内存切片的 RuleRegistry 实现。
// 不做持久化，适用于运行时临时聚合规则的场景
// （如权限规则转渲染条目的聚合器）。
type MemRuleRegistry struct {
	mu    sync.RWMutex
	rules []Rule
}

// 编译期接口检查
var _ RuleRegistry = (*MemRuleRegistry)(nil)

// NewMemRuleRegistry 创建一个空的内存规则注册表。
func NewMemRuleRegistry() *MemRuleRegistry {
	return &MemRuleRegistry{}
}

// Register 在注册表中添加或更新一条规则。
// 若存在相同 ID 的规则，则更新该规则。
func (r *MemRuleRegistry) Register(rule Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.rules {
		if r.rules[i].ID == rule.ID {
			r.rules[i] = rule
			return nil
		}
	}
	r.rules = append(r.rules, rule)
	return nil
}

// Unregister 按 ID 从注册表中移除一条规则。
// 若规则不存在则为空操作。
func (r *MemRuleRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return
		}
	}
}

// Get 按 ID 检索一条规则。找到时返回该规则和 true，否则返回 nil 和 false。
func (r *MemRuleRegistry) Get(id string) (*Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := range r.rules {
		if r.rules[i].ID == id {
			return &r.rules[i], true
		}
	}
	return nil, false
}

// All 返回注册表中所有规则的副本。
func (r *MemRuleRegistry) All() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Rule, len(r.rules))
	copy(out, r.rules)
	return out
}

// GetByScope 返回匹配给定范围的所有规则。
func (r *MemRuleRegistry) GetByScope(scope RuleScope) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var filtered []Rule
	for _, rule := range r.rules {
		if rule.Scope == scope {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

// FormatPromptSection 将已启用的规则格式化为 Markdown 列表，以便嵌入到 system prompts 中。
// 若未定义任何规则或所有规则都被禁用，则返回空字符串。
func (r *MemRuleRegistry) FormatPromptSection() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.rules) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, rule := range r.rules {
		if rule.Enabled {
			sb.WriteString("- ")
			sb.WriteString(rule.Intro)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
