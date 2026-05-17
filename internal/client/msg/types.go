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
	ToolName     string
	EstimatedTok int
	Params       map[string]any
}

type ActionProgressMsg struct {
	SessionID string
	ToolName  string
	Progress  string
}

type ActionResultMsg struct {
	SessionID string
	ToolName  string
	Success   bool
	Result    string
	Error     string
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
