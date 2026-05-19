package data

import "time"

type SessionMeta struct {
	SessionID string    `json:"session_id"`
	AgentName string    `json:"agent_name"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatSession struct {
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id"`
}

type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connecting
	Authenticated
	Connected
)

type NotificationLevel int

const (
	NotifInfo NotificationLevel = iota
	NotifSuccess
	NotifError
	NotifWarning
)

type Notification struct {
	ID        string
	Level     NotificationLevel
	Message   string
	CreatedAt time.Time
	Duration  time.Duration
}

type Shortcut struct {
	Key         string
	Description string
}

type SearchState struct {
	Query        string
	CurrentIndex int
	TotalMatches int
}
