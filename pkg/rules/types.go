package rules

// RuleScope 定义行为规则的适用范围。
type RuleScope string

const (
	// ScopeGlobal 适用于所有会话中的所有 agent。
	ScopeGlobal RuleScope = "global"

	// ScopeLocal 仅适用于注册该规则的 agent。
	// 对于该 agent，在会话之间持久保留。
	ScopeLocal RuleScope = "local"

	// ScopeConversation 仅适用于当前会话/对话。
	// 在会话结束或 agent 切换身份时清除。
	ScopeConversation RuleScope = "conversation"
)

// Rule 定义 AI agent 的单条行为约束。
// 规则作为必须遵守的行为规范注入到 System Prompt 中。
//
// 示例:
//
//	rule := Rule{
//	    ID:       "no-delete-prod",
//	    Intro:    "Never delete production data files. Any modification must be backed up first.",
//	    Scope:    ScopeGlobal,
//	    Priority: 100,
//	    Enabled:  true,
//	}
type Rule struct {
	ID       string    `json:"id" yaml:"id"`
	Intro    string    `json:"intro" yaml:"intro"`
	Scope    RuleScope `json:"scope" yaml:"scope"`
	Priority int       `json:"priority" yaml:"priority"`
	Enabled  bool      `json:"enabled" yaml:"enabled"`
}

// RuleRegistry 管理 agent 的行为规则。
// 规则在每次 LLM 调用前渲染到 System Prompt 的扩展规则段落中，
// 从而实现动态行为控制。
//
// 规则是静态约束（例如"执行破坏性命令前必须先询问"），
// 无论当前 Intent 或上下文如何都适用——规则定义"应该/必须"。
type RuleRegistry interface {
	Register(rule Rule) error
	Unregister(id string)
	Get(id string) (*Rule, bool)
	All() []Rule
	GetByScope(scope RuleScope) []Rule
	FormatPromptSection() string
}
