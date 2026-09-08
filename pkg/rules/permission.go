package rules

// RuleBehavior 定义权限规则匹配时可能的行为。
type RuleBehavior string

const (
	// RuleAllow 表示允许使用该工具。
	RuleAllow RuleBehavior = "allow"

	// RuleDeny 表示禁止使用该工具。
	RuleDeny RuleBehavior = "deny"

	// RuleAsk 表示使用该工具需要用户批准。
	RuleAsk RuleBehavior = "ask"
)

// PermissionRule 定义用于工具访问控制的单条权限规则。
type PermissionRule struct {
	Behavior       RuleBehavior `json:"behavior"`
	ToolName       string       `json:"tool_name"`
	ContentPattern string       `json:"content_pattern,omitempty"`
	Description    string       `json:"description,omitempty"`
	Source         string       `json:"source,omitempty"`
}

// PermissionRules 按行为类型组织的分类权限规则集合。
type PermissionRules struct {
	AlwaysAllow []PermissionRule `json:"always_allow"`
	AlwaysDeny  []PermissionRule `json:"always_deny"`
	AlwaysAsk   []PermissionRule `json:"always_ask"`
}

// PermissionRuleStore 定义加载和保存权限规则的接口。
type PermissionRuleStore interface {
	// Load 获取当前的权限规则集合。
	Load() (*PermissionRules, error)
	// Save 持久化给定的权限规则。
	Save(rules *PermissionRules) error
}
