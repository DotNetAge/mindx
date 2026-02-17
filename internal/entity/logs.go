package entity

import "time"

// Message 对话消息结构体
type LogMessage struct {
	Content   string    `json:"content"`
	Sender    string    `json:"sender"` // "user" or "bot"
	Timestamp time.Time `json:"timestamp"`
}

// ConversationLog 对话日志结构体
type ConversationLog struct {
	ID        string       `json:"id"`
	Messages  []LogMessage `json:"messages"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Topic     string       `json:"topic,omitempty"`
}
