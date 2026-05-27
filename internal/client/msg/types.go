package msg

import (
	"time"

	"github.com/DotNetAge/goreact/session"
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
	SessionID string
	Iteration int
}

// ExecutionCancelMsg is sent when the user presses ESC during T-A-O execution.
// The rootModel should call agent.Cancel() to interrupt the running loop.
type ExecutionCancelMsg struct{}

// AskUserEventMsg signals that an AskUserRequest event has arrived from the reactor.
// The rootModel should activate the dialog overlay and show the pending questions.
type AskUserEventMsg struct{}

// PermissionRequestMsg carries a permission request from the reactor to the TUI.
// The TUI should display the question/options and let the user respond.
type PermissionRequestMsg struct {
	ToolName      string
	Reason        string
	SecurityLevel int
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
