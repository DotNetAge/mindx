package msg

import (
	"time"

	"github.com/DotNetAge/goharness/session"
)

type ThinkingDeltaMsg struct {
	SessionID string
	Content   string
}

type ThinkingDoneMsg struct {
	SessionID string
	Content   string
	IsFinal   bool
}

type ToolExecStartMsg struct {
	SessionID    string
	ToolName     string
	ToolCallID   string // 精确关联 tool_use_delta 占位与 tool_exec_end 结果
	Params       map[string]any
	EstimatedTok int
}

type ToolExecEndMsg struct {
	SessionID  string
	ToolName   string
	ToolCallID string
	Success    bool
	Result     string
	Error      string
	Duration   time.Duration
	DiffText   string // unified diff for file-modifying tools
	DiffAdds   int    // lines added
	DiffDels   int    // lines removed
	DiffFile   string // file path changed

	// 工具所在轮次 LLM 调用的实际 token 消耗（随 tool_exec_end 下发）。
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
}

type ExecutionSummaryMsg struct {
	SessionID  string
	Duration   time.Duration
	TokensUsed session.TokenUsage
	ToolCalls  int
}

type FinalAnswerMsg struct {
	SessionID string
	Content   string
}

// ContentDeltaMsg is a streaming text content fragment from the LLM response.
// Used to progressively build the final output before FinalAnswer arrives.
type ContentDeltaMsg struct {
	SessionID string
	Content   string
}

// ToolUseDeltaMsg is a streaming tool call argument fragment from the LLM response.
// Used to show tool call arguments being generated in real-time.
type ToolUseDeltaMsg struct {
	SessionID string
	Index     int
	ID        string
	Name      string
	Arguments string
}

type AgentErrorMsg struct {
	SessionID string
	Error     error
}

type LLMTimeoutMsg struct {
	SessionID string
	Timeout   time.Duration
	Elapsed   time.Duration
	Error     string
}

// LLMCancelledMsg signals that the LLM call was cancelled by the user.
// Unlike LLMTimeoutMsg (actual timeout), this is a user-initiated interruption.
// The UI should display this as an informational notice, not an error.
type LLMCancelledMsg struct {
	SessionID string
	Elapsed   time.Duration
}

// LLMRetryMsg 表示 LLM 建流请求失败后进入退避重试（服务商限流 429 / 5xx 等
// 可预知错误）。这是可预知的等待，必须冒泡给用户而非静默处理——否则 TUI
// 会一直停留在「思考中」，用户无从得知服务端正在重试、还要等多久。
//
// Phase 为 "retry"（即将退避等待后重试，显示/原地更新等待提示）
// 或 "recovered"（重试后成功建流，UI 应自动移除等待提示）。
// 重试耗尽仍失败时走 AgentErrorMsg 收尾。
type LLMRetryMsg struct {
	SessionID   string
	Provider    string
	Model       string
	StatusCode  int
	Attempt     int
	MaxAttempts int
	RetryAfter  time.Duration
	Error       string
	Phase       string
}

// LLM 重试事件的阶段取值，与 events.LLMRetryPhase* 对应。
const (
	// LLMRetryPhaseRetry 表示即将退避等待后重试。
	LLMRetryPhaseRetry = "retry"

	// LLMRetryPhaseRecovered 表示重试后已成功建流，等待提示应移除。
	LLMRetryPhaseRecovered = "recovered"
)

// MaxTurnsReachedMsg signals that the Think-Act loop reached MaxTurns
// without producing a final answer.
// This is NOT an error - it's a normal boundary condition.
// The UI should display this as an informational notice with a friendly suggestion.
type MaxTurnsReachedMsg struct {
	SessionID      string
	TurnsCompleted int
	MaxTurns       int
	Suggestion     string
}

type SessionDoneMsg struct {
	SessionID string
}

type UserSendMsg struct {
	Text string
}

type AgentSwitchMsg struct {
	AgentName string
}

type SlashCommandMsg struct {
	Name string
	Args []string
}

type CollapseToggleMsg struct {
	SessionID   string
	ActionIndex int
}

type ThinkCollapseMsg struct {
	SessionID string
}

// ToggleToolsFoldMsg 切换当前会话流的工具调用折叠摘要（ctrl+o 触发，
// SessionID 由 client 层补齐后转发给 conv 组件）。
type ToggleToolsFoldMsg struct {
	SessionID string
}

type ClearScreenMsg struct{}

type ExitMsg struct{}

type TickMsg struct {
	Time time.Time
}

type ChoiceSelectedMsg struct {
	Index      int
	Indices    []int
	CustomText string
}

type NotifTimeoutMsg struct {
	ID string
}

type SessionLoadedMsg struct {
	AgentName string
	SessionID string
}

type WindowResizeMsg struct {
	Width  int
	Height int
}

type ShowChoicesMsg struct {
	Options        []string
	Prompt         string
	MultiSelect    bool
	AllowTextInput bool
}

type MouseScrollMsg struct {
	Lines int
}

// IterationMsg is sent at the end of each T-A-O cycle.
type IterationMsg struct {
	SessionID         string
	Iteration         int
	TerminationReason string
	Duration          time.Duration
}

// ExecutionCancelMsg is sent when the user presses ESC during T-A-O execution.
// The rootModel should call agent.Cancel() to interrupt the running loop.
type ExecutionCancelMsg struct{}

// AskUserEventMsg signals that an AskUserRequest event has arrived from the reactor.
// The rootModel should activate the dialog overlay and show the pending questions.
type AskUserEventMsg struct{}

// PermissionRequestMsg carries a permission request from the reactor to the TUI.
// The TUI should display the question/options and let the user respond.
// SessionID 携带发起授权请求的会话 ID：子智能体授权冒泡时为主会话 ID 之外的
// 子会话 ID，用户决策后 TUI 据此发送带目标魔法词精确路由；主会话自身的授权
// 请求为空（无目标，走先到先服务）。
type PermissionRequestMsg struct {
	ToolName      string
	Reason        string
	SecurityLevel int
	SessionID     string
}

type DaemonConnStatus int

const (
	DaemonUnknown DaemonConnStatus = iota
	DaemonConnected
	DaemonDisconnected
)

// DaemonStatusMsg reports WebSocket connectivity to the daemon.
type DaemonStatusMsg struct {
	Status DaemonConnStatus
}

// ── svc 服务端事件对齐（daemon → client 推送，见 rpc.go 事件注册）──────────

// CompactionMsg signals the session context window was compacted (compaction).
type CompactionMsg struct {
	SessionID      string
	MessagesSlid   int
	RemainingAfter int
	WindowSize     int
}

// SubtaskSpawnedMsg signals a sub-agent task was spawned (subtask_spawned).
// SessionID 为父会话 ID；SubSessionID 为发起执行的子会话 ID。
type SubtaskSpawnedMsg struct {
	SessionID    string
	SubSessionID string
	AgentName    string
	Description  string
	TimeoutSec   int
}

// SubtaskCompletedMsg signals a sub-agent task has finished (subtask_completed).
type SubtaskCompletedMsg struct {
	SessionID    string
	SubSessionID string
	AgentName    string
	Success      bool
	Answer       string
	Error        string
	Description  string
}

// TaskSummaryMsg carries the natural-language task summary markdown (task_summary)。
// TokenUsage 取自 envelope meta（prompt/completion/cached/reasoning/total）。
type TaskSummaryMsg struct {
	SessionID  string
	Content    string
	TokenUsage session.TokenUsage
}

// TokenUsageRecordedMsg reports one recorded LLM token usage record
// (token_usage_recorded)，字段与 session.TokenUsageRecord 一致。
type TokenUsageRecordedMsg struct {
	Record session.TokenUsageRecord
}

// UserMessageSavedMsg 回传后端持久化用户消息的 Timestamp（user_message_saved），
// 用于刷新前即可用「回收本轮」（session.delete_round）。
type UserMessageSavedMsg struct {
	SessionID string
	Timestamp int64
}

// MessageQueuedMsg signals the user message entered the per-session serial queue
// and waits for the previous execution to finish (message_queued).
type MessageQueuedMsg struct {
	SessionID string
	Timestamp int64
}

// MessageProcessingMsg signals a queued user message started executing
// (message_processing)。
type MessageProcessingMsg struct {
	SessionID string
	Timestamp int64
}

// PermissionDeniedMsg signals a tool execution was denied (permission_denied)。
// Reason 为拒绝原因文本。
type PermissionDeniedMsg struct {
	SessionID string
	Reason    string
}

// ScheduleJobMsg 广播调度任务生命周期事件
// （schedule.job_started / job_completed / job_failed / job_missed）。
type ScheduleJobMsg struct {
	EntryID    string
	RunID      string
	Agent      string
	SessionID  string
	Status     string // "started" | "completed" | "failed" | "missed"
	Content    string
	ProjectDir string
	Error      string
}

// DaemonUpdateMsg 自动升级广播（update_started / update_installed）。
type DaemonUpdateMsg struct {
	Phase   string // "started" 或 "installed"
	Version string
}

// ContextCompactionMsg 上下文压缩广播
// （compact_start / compact_done / micro_compact_start / micro_compact_done）。
type ContextCompactionMsg struct {
	SessionID    string
	Micro        bool
	Phase        string // "start" 或 "done"
	WindowTokens int
	MessagesSlid int
	Compressed   int // micro_compact_done：压缩消息数
	Deduped      int // micro_compact_done：去重消息数
	Ratio        float64
}

// ContextUsageMsg 每轮 LLM 请求后的上下文窗口占用广播（context_usage）。
type ContextUsageMsg struct {
	SessionID          string
	WindowTokens       float64
	MaxWindowSize      float64
	UsageRatio         float64
	MessageCount       int
	Cursor             int
	ActiveMessageCount int
	TotalActualTokens  float64
	TotalCost          float64
}

// TerminalOutputMsg 推送终端会话的输出片段（terminal.output）。
// SessionID 为 terminal.start 返回的终端会话 ID。
type TerminalOutputMsg struct {
	SessionID string
	Data      string
}

// TerminalExitMsg 推送终端进程退出（terminal.exit），ExitCode 为退出码。
type TerminalExitMsg struct {
	SessionID string
	ExitCode  int
}
