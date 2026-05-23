package msg

import "time"

type ThinkingDeltaMsg struct {
	SessionID string
	Content   string
}

type ThinkingDoneMsg struct {
	SessionID   string
	Reasoning   string
	Decision    string
	IsFinal     bool
	ThoughtData map[string]any
}

type ActionStartMsg struct {
	SessionID    string
	ToolCount    int
	ToolNames    []string
	EstimatedTok int
}

type ActionProgressMsg struct {
	SessionID      string
	CompletedCount int
	TotalCount     int
	Status         string
}

type ToolExecStartMsg struct {
	SessionID    string
	ToolName     string
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
}

type ExecutionSummaryMsg struct {
	SessionID  string
	Duration   time.Duration
	TokensUsed int
	ToolCalls  int
}

type ActionEndMsg struct {
	SessionID    string
	TotalTools   int
	SuccessCount int
	FailedCount  int
	Summary      string
}

type FinalAnswerMsg struct {
	SessionID string
	Content   string
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

type ClearScreenMsg struct{}

type ExitMsg struct{}

type TickMsg struct {
	Time time.Time
}

type ChoiceSelectedMsg struct {
	Index int
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
	Options []string
	Prompt  string
}

type MouseScrollMsg struct {
	Lines int
}

// IterationMsg is sent at the end of each T-A-O cycle.
type IterationMsg struct {
	SessionID string
	Iteration int
}

// ExecutionCancelMsg is sent when the user presses ESC during T-A-O execution.
// The rootModel should call agent.Cancel() to interrupt the running loop.
type ExecutionCancelMsg struct{}

// PermissionRequestMsg carries a permission request from the reactor to the TUI.
// The TUI should display the question/options and let the user respond.
type PermissionRequestMsg struct {
	ToolName      string
	Reason        string
	SecurityLevel int
	Questions     []QuestionData
}

// QuestionData mirrors core.PermissionQuestion for TUI consumption.
type QuestionData struct {
	Question    string
	Header      string
	Options     []string
	MultiSelect bool
}
