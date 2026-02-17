package entity

import (
	"time"

	"github.com/gorilla/websocket"
)

type ThinkingEventType string

const (
	ThinkingEventStart      ThinkingEventType = "start"
	ThinkingEventProgress   ThinkingEventType = "progress"
	ThinkingEventChunk      ThinkingEventType = "chunk"
	ThinkingEventToolCall   ThinkingEventType = "tool_call"
	ThinkingEventToolResult ThinkingEventType = "tool_result"
	ThinkingEventComplete   ThinkingEventType = "complete"
	ThinkingEventError      ThinkingEventType = "error"
)

type ThinkingEvent struct {
	Type      ThinkingEventType `json:"type"`
	Content   string            `json:"content"`
	Progress  float64           `json:"progress"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]any    `json:"metadata"`
}

type WebClient struct {
	Conn           *websocket.Conn
	SessionID      string
	ChannelID      string
	ClientID       string
	SenderID       string
	SenderName     string
	LastActiveTime time.Time
	EventChan      chan ThinkingEvent `json:"-"`
}
