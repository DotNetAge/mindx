package conv

import "time"

// NodeStatus 节点生命周期四态（对齐 Desktop NodeStatus）。
// success 不是高亮事件，视觉走中性灰；attention 是渲染层由类型+数据
// 推导的视觉态（静态 attention 类型 + 超阈值慢节点），不是 status 原语。
type NodeStatus string

const (
	NodeExecuting NodeStatus = "executing"
	NodeSuccess   NodeStatus = "success"
	NodeFailed    NodeStatus = "failed"
	NodeCancelled NodeStatus = "cancelled"
)

// NodeBase 通用壳——全类型共享字段（对齐 Desktop NodeBase）。
// - Offset: 轮内偏移（相对轮首 StartedAt），供 gap 标记和时长阈值化呈现；
// - Duration/Tokens: 执行元信息；
// - FoldDefault: 折叠策略的每类型静态声明（UI 折叠态不进数据）。
type NodeBase struct {
	ID          string
	Kind        string        // TreeNodeType: "content" | "thinking" | "group" | "tool.bash" | "task" | ...
	Status      NodeStatus
	Offset      time.Duration // 轮内偏移（相对轮首）
	Duration    time.Duration
	Tokens      int
	FoldDefault bool
}

// TreeNode 判别联合接口（用 Go interface + type switch 模拟 TS 判别联合）。
// Base 返回通用壳，所有具体类型必须满足此接口。
type TreeNode interface {
	Base() NodeBase
}

// ──────────────────────────────────────────────────────────────
// 族 1：执行内容（Content / Thinking / Group）
// ──────────────────────────────────────────────────────────────

// TurnUsage 单轮 LLM token 消耗口径（对齐 Desktop TurnUsage）。
type TurnUsage struct {
	PromptTokens   int
	CompletionTokens int
	CachedTokens   int
	ActualTokens   int // Prompt + Completion - Cached
	TotalTokens    int
	CallCount      int
	Cost           float64
}

// ContentNode 正文/过程段（对齐 Desktop ContentNode）。
// finishReason 区分过程段（推理区）与最终答案段：
//   stop     = 最终答案定稿（final_answer / task_summary / finish_reason=stop）
//   tool_calls = 过程段（每次 T-A-O 迭代的 content）
//   streaming = 流式中
type ContentNode struct {
	NodeBase
	Text         string
	Streaming    bool
	IsFinal      bool
	FinishReason string // "stop" | "tool_calls" | "streaming"
	TurnUsage    *TurnUsage
	Summary      string // task_summary 双语义归一（归一规则 8）
}

func (n *ContentNode) Base() NodeBase { return n.NodeBase }

// ThinkingNode 思考流（对齐 Desktop ThinkingNode）。
type ThinkingNode struct {
	NodeBase
	Text string
}

func (n *ThinkingNode) Base() NodeBase { return n.NodeBase }

// GroupNode 工具聚合组（对齐 Desktop GroupNode）。
// 相邻同 GroupKey 工具段被 builder 聚合为 GroupNode，树深 ≤2
// （成员必为 ToolNode，无更深层子树）。
type GroupNode struct {
	NodeBase
	GroupKey string // GroupKey 字符串值
	Children []*ToolNode
}

func (n *GroupNode) Base() NodeBase { return n.NodeBase }

// ──────────────────────────────────────────────────────────────
// 族 2：实体与协作（Task / Team / Subagent / Collect）
// ──────────────────────────────────────────────────────────────

// TaskTransition 任务状态流转记录（审计：每次 TaskUpdate 追加）。
type TaskTransition struct {
	Status string
	At     time.Duration // 轮内偏移 ms
}

// TaskSnapshotItem 清单快照条目（upsert 时刻任务看板的逐任务状态）。
type TaskSnapshotItem struct {
	TaskID     string
	Subject    string
	Status     string
	ActiveForm string
}

// TaskNode 任务实体卡（对齐 Desktop TaskNode）。
// 叶子状态卡，不挂执行子树——并行交错时工具流无法可靠归属任务。
type TaskNode struct {
	NodeBase
	TaskID     string
	Subject    string
	ActiveForm string
	TaskStatus string
	Checklist  []TaskSnapshotItem
	Transitions []TaskTransition
}

func (n *TaskNode) Base() NodeBase { return n.NodeBase }

// TeamMember 团队成员。
type TeamMember struct {
	Name string
	Role string // "leader" 或空
}

// TeamNode 团队创建卡（对齐 Desktop TeamNode）。
type TeamNode struct {
	NodeBase
	TeamName    string
	Description string
	Members     []TeamMember
}

func (n *TeamNode) Base() NodeBase { return n.NodeBase }

// SubagentNode 子任务卡（对齐 Desktop SubagentNode）。
// 全树最复杂节点：身份与状态来自双事件（SubAgent 工具 + subtask_spawned/completed）。
// 一阶段展开态沿用卡片不挂子树。restored=true 表示 localStorage 旁路补齐。
type SubagentNode struct {
	NodeBase
	AgentName   string
	TaskDigest  string
	SessionID   string
	ResultDigest string
	Restored    bool
}

func (n *SubagentNode) Base() NodeBase { return n.NodeBase }

// CollectNode 结果收集卡（对齐 Desktop CollectNode）。
// 按 SessionID 关联对应 SubagentNode，完成后回填其状态。
type CollectNode struct {
	NodeBase
	SessionIDs   []string
	ResultDigests []string
}

func (n *CollectNode) Base() NodeBase { return n.NodeBase }

// ──────────────────────────────────────────────────────────────
// 族 3：阻塞与系统（Permission / AskUser / Error / Compaction /
//       MaxTurns / LlmRetry / Cancelled）
// ──────────────────────────────────────────────────────────────

// PermissionNode 授权请求/决定（对齐 Desktop PermissionNode）。
// 阻塞授权的审计闭环：请求 + 决定 = 完整留痕。
type PermissionNode struct {
	NodeBase
	ToolName       string
	Reason         string
	SecurityLevel  string
	Decision       string // "pending" | "granted" | "denied"
}

func (n *PermissionNode) Base() NodeBase { return n.NodeBase }

// AskUserQuestion AskUser 问题条目。
type AskUserQuestion struct {
	Question   string
	Options    []string
	MultiSelect bool
}

// AskUserAnswer AskUser 回答条目。
type AskUserAnswer struct {
	Question string
	Answer   string
}

// AskUserNode AskUser 提问卡（对齐 Desktop AskUserNode）。
type AskUserNode struct {
	NodeBase
	Questions []AskUserQuestion
	Answers   []AskUserAnswer
}

func (n *AskUserNode) Base() NodeBase { return n.NodeBase }

// ErrorNode 错误卡（对齐 Desktop ErrorNode）。
// Source 归一 402 欠费 / 超时路径 / 其余运行时错误。
type ErrorNode struct {
	NodeBase
	Message string
	Source  string // "llm_timeout" | "provider_402" | "runtime"
}

func (n *ErrorNode) Base() NodeBase { return n.NodeBase }

// CompactionNode 上下文压缩卡（对齐 Desktop CompactionNode）。
type CompactionNode struct {
	NodeBase
	MessagesSlid   int
	RemainingAfter int
	WindowSize     int
}

func (n *CompactionNode) Base() NodeBase { return n.NodeBase }

// MaxTurnsNode 最大轮数卡（对齐 Desktop MaxTurnsNode）。
type MaxTurnsNode struct {
	NodeBase
	TurnsCompleted int
	MaxTurns       int
	Suggestion     string
}

func (n *MaxTurnsNode) Base() NodeBase { return n.NodeBase }

// LlmRetryNode 建流重试卡（对齐 Desktop LlmRetryNode）。
type LlmRetryNode struct {
	NodeBase
	Provider    string
	Model       string
	StatusCode  int
	Attempt     int
	MaxAttempts int
	RetryAfter  time.Duration
}

func (n *LlmRetryNode) Base() NodeBase { return n.NodeBase }

// CancelledNode 用户中断卡（对齐 Desktop CancelledNode）。
type CancelledNode struct {
	NodeBase
	Elapsed time.Duration
}

func (n *CancelledNode) Base() NodeBase { return n.NodeBase }

// ──────────────────────────────────────────────────────────────
// 族 4：工具（18 种统一 ToolNode，按 Kind 区分）
// ──────────────────────────────────────────────────────────────

// ToolNode 工具节点统一载体（对齐 Desktop 18 种 ToolNode 判别联合的公共壳 + 专有芯）。
// 通过 Kind 字段（如 "tool.bash"）区分具体类型；各工具专属 payload 字段全部内聚在此。
//
// Desktop 的做法是把 18 种工具各自定义独立接口（ToolBashNode / ToolReadNode …），
// 每种都有自己的字段。TUI 端 Go 没有判别联合的原生支持，用 Kind + 公共壳 + 全部
// payload 字段的扁平结构等效实现（渲染层通过 Kind 判断消费哪些字段）。
type ToolNode struct {
	NodeBase
	ToolName string // 原始工具名 "Bash"/"Write"/"Read"/...

	// ── 18 种工具的 payload 字段（按 registry/summary.ts 的 object/badges 提取所需） ──

	// Bash / RunScript
	BashCmd      string
	BashExitCode int
	RunSkillName string
	RunScriptArgs string

	// Read / Write / Edit
	FilePath   string
	ReadLines  int
	WriteBytes int
	EditAdds   int
	EditDels   int

	// Ls / Glob
	LsPath     string
	LsRecursive bool
	LsEntries  int
	GlobPattern string
	GlobMatches int

	// Grep
	GrepPattern string
	GrepHits    int
	GrepInclude string

	// WebFetch / WebSearch
	WebFetchURL   string
	WebFetchTitle string
	WebFetchBytes int
	WebSearchQuery string
	WebSearchCount int
	WebSearchCached bool

	// KB Search / Memory Search
	KBSearchQuery  string
	KBSearchHits   int
	MemorySearchQuery string
	MemorySearchHits  int

	// Skill
	SkillName string

	// Sleep
	SleepDuration time.Duration

	// TaskQuery
	TaskQueryResult string

	// TeamOps
	TeamOpsAction  string
	TeamOpsTeam    string

	// Cron
	CronAction string
	CronID     string
	CronAgent  string
	CronExpr   string
	CronEnabled *bool

	// Notify
	NotifyTitle   string
	NotifyMessage string

	// 通用展开态原始输出
	ResultText  string
	ResultTail   string // 长截断（对齐 Desktop outputTail 20KB）
}

func (n *ToolNode) Base() NodeBase { return n.NodeBase }

// ──────────────────────────────────────────────────────────────
// 辅助：类型守卫（对齐 Desktop isGroupNode / isToolNode）
// ──────────────────────────────────────────────────────────────

// IsGroupNode 类型守卫：节点是否为 GroupNode。
func IsGroupNode(n TreeNode) bool {
	_, ok := n.(*GroupNode)
	return ok
}

// IsToolNode 类型守卫：节点是否为工具节点（含 GroupNode.Children）。
// 与 Desktop 对齐：Kind 以 "tool." 前缀判定。
func IsToolNode(n TreeNode) bool {
	return len(n.Base().Kind) >= 5 && n.Base().Kind[:5] == "tool."
}

// TreeNodeTypes 全量节点类型名（与 Desktop TreeNodeType 联合派生的枚举对齐）。
// 用于断言构建器不私加未登记类型。
var TreeNodeTypes = map[string]bool{
	"content":      true,
	"thinking":     true,
	"group":        true,
	"task":         true,
	"team":         true,
	"subagent":     true,
	"collect":      true,
	"permission":   true,
	"ask_user":     true,
	"error":        true,
	"compaction":   true,
	"max_turns":    true,
	"llm_retry":    true,
	"cancelled":    true,
	// 工具 18 种
	"tool.read":        true,
	"tool.write":       true,
	"tool.edit":        true,
	"tool.ls":          true,
	"tool.glob":        true,
	"tool.grep":        true,
	"tool.bash":        true,
	"tool.run_script":  true,
	"tool.web_fetch":   true,
	"tool.web_search":  true,
	"tool.kb_search":   true,
	"tool.memory_search": true,
	"tool.skill":       true,
	"tool.sleep":       true,
	"tool.task_query":  true,
	"tool.team_ops":    true,
	"tool.cron":        true,
	"tool.notify":      true,
}
