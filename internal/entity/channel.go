package entity

import (
	"time"
)

// ChannelType Channel 类型
type ChannelType string

const (
	ChannelTypeRealTime ChannelType = "realtime" // 实时通道 (WebSocket)
	ChannelTypeFeishu   ChannelType = "feishu"   // 飞书机器人
	ChannelTypeWeChat   ChannelType = "wechat"   // 微信
	ChannelTypeQQ       ChannelType = "qq"       // QQ 机器人
	ChannelTypeDoubao   ChannelType = "doubao"   // 豆包机器人
	ChannelTypeDingTalk ChannelType = "dingtalk" // 钉钉机器人
	ChannelTypeWhatsApp ChannelType = "whatsapp" // WhatsApp 机器人
	ChannelTypeFacebook ChannelType = "facebook" // Facebook 机器人
	ChannelTypeTelegram ChannelType = "telegram" // Telegram 机器人
	ChannelTypeIMessage ChannelType = "imessage" // iMessage 机器人
)

// IncomingMessage 进入的消息 (从外部进入系统)
type IncomingMessage struct {
	// ChannelID 消息来源 Channel ID
	ChannelID string `json:"channel_id"`

	// ChannelName 消息来源 Channel 名称
	ChannelName string `json:"channel_name"`

	// SessionID 会话 ID
	// 同一个对话使用相同的 ID,用于上下文管理
	SessionID string `json:"session_id"`

	// MessageID 消息在原平台的唯一 ID
	MessageID string `json:"message_id"`

	// Sender 消息发送者信息
	Sender *MessageSender `json:"sender"`

	// Content 消息内容
	Content string `json:"content"`

	// ContentType 内容类型 (text, image, audio, video, file 等)
	ContentType string `json:"content_type"`

	// Attachments 附件列表
	Attachments []*Attachment `json:"attachments,omitempty"`

	// Metadata 元数据,可存储平台特定的额外信息
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamp 消息时间戳
	Timestamp time.Time `json:"timestamp"`

	// ReplyTo 如果是回复消息,记录被回复的消息 ID
	ReplyTo string `json:"reply_to,omitempty"`

	// HookID Hook标识符
	HookID string `json:"hook_id,omitempty"`

	// HookName Hook名称
	HookName string `json:"hook_name,omitempty"`

	// Source 消息来源
	Source string `json:"source,omitempty"`

	// ConversationID 会话ID（用于回复消息）
	ConversationID string `json:"conversation_id,omitempty"`
}

// OutgoingMessage 发出的消息 (从系统发送到外部)
type OutgoingMessage struct {
	// ChannelID 目标 Channel ID
	ChannelID string `json:"channel_id"`

	// SessionID 目标会话 ID
	SessionID string `json:"session_id"`

	// Content 消息内容
	Content string `json:"content"`

	// ContentType 内容类型
	ContentType string `json:"content_type"`

	// Attachments 附件
	Attachments []*Attachment `json:"attachments,omitempty"`

	// ReplyTo 回复的消息 ID
	ReplyTo string `json:"reply_to,omitempty"`

	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// ConversationID 会话ID（用于回复消息）
	ConversationID string `json:"conversation_id,omitempty"`
}

// MessageSender 消息发送者信息
type MessageSender struct {
	// ID 发送者在原平台的唯一 ID
	ID string `json:"id"`

	// Name 发送者显示名称
	Name string `json:"name"`

	// Avatar 头像 URL
	Avatar string `json:"avatar,omitempty"`

	// Type 发送者类型 (user, group, system 等)
	Type string `json:"type"`

	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Attachment 附件信息
type Attachment struct {
	// Type 附件类型 (image, audio, video, file 等)
	Type string `json:"type"`

	// URL 附件访问 URL
	URL string `json:"url"`

	// Name 附件名称
	Name string `json:"name,omitempty"`

	// Size 附件大小 (字节)
	Size int64 `json:"size,omitempty"`

	// MIMEType MIME 类型
	MIMEType string `json:"mime_type,omitempty"`

	// Duration 媒体时长 (秒,音频/视频)
	Duration int `json:"duration,omitempty"`

	// Thumbnail 缩略图 URL
	Thumbnail string `json:"thumbnail,omitempty"`
}

// ChannelStatus Channel 状态
type ChannelStatus struct {
	// Name Channel 名称
	Name string `json:"name"`

	// Type Channel 类型
	Type ChannelType `json:"type"`

	// Description Channel 描述
	Description string `json:"description"`

	// Running 是否正在运行
	Running bool `json:"running"`

	// StartTime 启动时间
	StartTime *time.Time `json:"start_time,omitempty"`

	// LastMessageTime 最后一条消息接收时间
	LastMessageTime *time.Time `json:"last_message_time,omitempty"`

	// TotalMessages 总接收消息数
	TotalMessages int64 `json:"total_messages"`

	// Error 错误信息 (如果有)
	Error string `json:"error,omitempty"`

	// HealthCheck 健康检查结果
	HealthCheck *HealthCheck `json:"health_check,omitempty"`
}

// HealthCheck 健康检查结果
type HealthCheck struct {
	// Status 状态 (healthy, degraded, unhealthy)
	Status string `json:"status"`

	// Message 描述信息
	Message string `json:"message"`

	// LastCheckTime 最后检查时间
	LastCheckTime time.Time `json:"last_check_time"`

	// Latency 延迟 (毫秒)
	Latency int64 `json:"latency"`
}

// ChannelSwitchInfo Channel 切换信息
type ChannelSwitchInfo struct {
	// Target 目标 Channel 名称
	Target string `json:"target"`

	// Reason 切换原因
	Reason string `json:"reason,omitempty"`
}

// ChannelForwardInfo Channel 转发信息
type ChannelForwardInfo struct {
	// Target 目标 Channel 名称
	Target string `json:"target"`

	// Message 要转发的消息内容
	Message string `json:"message"`
}
