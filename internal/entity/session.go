package entity

import (
	"time"
)

// Message 表示一条对话消息
type Message struct {
	Role    string    `json:"role"`    // "user" 或 "assistant"
	Content string    `json:"content"` // 消息内容
	Time    time.Time `json:"time"`    // 消息时间
}

// Session 表示一个会话
type Session struct {
	ID         string    `json:"id"`          // 会话ID（仅内部使用，不对外暴露）
	Messages   []Message `json:"messages"`    // 所有消息记录（用于Web显示和Brain使用）
	TokensUsed int       `json:"tokens_used"` // Token消耗量（累积统计）
	IsEnded    bool      `json:"is_ended"`    // 会话是否已结束
	CreatedAt  time.Time `json:"created_at"`   // 创建时间
	EndedAt    time.Time `json:"ended_at"`    // 结束时间
}
