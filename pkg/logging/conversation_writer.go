package logging

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap/zapcore"
)

// ConversationEntry 对话日志条目
type ConversationEntry struct {
	SessionID       string                 `json:"session_id"`
	MessageID       string                 `json:"message_id"`
	UserID          string                 `json:"user_id"`
	UserName        string                 `json:"user_name"`
	ChannelID       string                 `json:"channel_id"`
	ChannelName     string                 `json:"channel_name"`
	Direction       string                 `json:"direction"` // "incoming" 或 "outgoing"
	Content         string                 `json:"content"`
	ContentType     string                 `json:"content_type"`
	Timestamp       time.Time              `json:"timestamp"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	ReplyTo         string                 `json:"reply_to,omitempty"`
	ConversationID  string                 `json:"conversation_id,omitempty"`
}

// ConversationWriter 对话日志写入接口
type ConversationWriter interface {
	// Write 写入对话日志
	Write(entry *ConversationEntry) error
	// WriteBatch 批量写入
	WriteBatch(entries []*ConversationEntry) error
	// Close 关闭写入器
	Close() error
}

// DBConversationWriter 数据库对话日志写入器
type DBConversationWriter struct {
	db Database
}

// Database 数据库接口
type Database interface {
	SaveConversationLog(ctx context.Context, entry *ConversationEntry) error
}

// NewDBConversationWriter 创建数据库对话日志写入器
func NewDBConversationWriter(db Database) *DBConversationWriter {
	return &DBConversationWriter{
		db: db,
	}
}

// Write 写入对话日志
func (w *DBConversationWriter) Write(entry *ConversationEntry) error {
	ctx := context.Background()
	return w.db.SaveConversationLog(ctx, entry)
}

// WriteBatch 批量写入
func (w *DBConversationWriter) WriteBatch(entries []*ConversationEntry) error {
	ctx := context.Background()
	for _, entry := range entries {
		if err := w.db.SaveConversationLog(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭写入器
func (w *DBConversationWriter) Close() error {
	return nil
}

// conversationWriteSyncer Zap WriteSyncer 适配器
type conversationWriteSyncer struct {
	writer ConversationWriter
}

// NewConversationWriteSyncer 创建 Zap WriteSyncer 适配器
func NewConversationWriteSyncer(writer ConversationWriter) zapcore.WriteSyncer {
	return &conversationWriteSyncer{
		writer: writer,
	}
}

// Write 实现 WriteSyncer 接口
func (s *conversationWriteSyncer) Write(p []byte) (n int, err error) {
	entry := &ConversationEntry{}
	if err := json.Unmarshal(p, entry); err != nil {
		return 0, err
	}
	if err := s.writer.Write(entry); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Sync 实现 Sync 接口
func (s *conversationWriteSyncer) Sync() error {
	return nil
}
