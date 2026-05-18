package msg

type ThinkingDeltaMsg struct {
	SessionID string
	Content   string
}

type ThinkingDoneMsg struct {
	SessionID string
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
	SessionID string
	ToolName  string
	Success   bool
	Result    string
	Error     string
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
	AnswerIndex int
	ActionIndex int
}

type ThinkCollapseMsg struct {
	AnswerIndex int
}

type ClearScreenMsg struct{}

type ExitMsg struct{}

type TickMsg struct{}

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
