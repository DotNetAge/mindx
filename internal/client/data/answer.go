package data

import "time"

type AnswerStatus int

const (
	StatusThinking AnswerStatus = iota
	StatusExecuting
	StatusResponding
	StatusDone
	StatusError
)

type ActionStatus int

const (
	ActionExecuting ActionStatus = iota
	ActionDone
	ActionFailed
)

type ThinkingRound struct {
	Content   string    `json:"content"`
	TokensIn  int       `json:"tokens_in"`
	TokensOut int       `json:"tokens_out"`
	Timestamp time.Time `json:"timestamp"`
}

type ActionStep struct {
	ToolName      string         `json:"tool_name"`
	Status        ActionStatus   `json:"status"`
	EstimatedTok  int            `json:"estimated_tok"`
	Params        map[string]any `json:"params"`
	ProgressText  string         `json:"progress_text"`
	ResultText    string         `json:"result_text"`
	Collapsed     bool           `json:"collapsed"`
}

type ResultEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnswerData struct {
	SessionID         string          `json:"session_id"`
	AgentName         string          `json:"agent_name"`
	UserQuestion      string          `json:"user_question"`
	Status            AnswerStatus    `json:"status"`
	ThinkingLog       []ThinkingRound `json:"thinking_log"`
	PendingThink      string          `json:"pending_think"`
	Actions           []ActionStep    `json:"actions"`
	Results           []ResultEntry   `json:"results"`
	IsThinking        bool            `json:"is_thinking"`
	ThinkingCollapsed bool            `json:"thinking_collapsed"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Duration          time.Duration   `json:"duration"`
}
