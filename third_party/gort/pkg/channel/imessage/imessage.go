// Package imessage provides a Channel adapter for Apple iMessage using AppleScript.
//
// This adapter integrates with macOS Messages.app via AppleScript to provide iMessage
// functionality including sending/receiving messages, file attachments, and chat history.
//
// Requirements:
//   - macOS 14+ with Messages.app signed in
//   - Full Disk Access for the terminal to read ~/Library/Messages/chat.db
//   - Automation permission for the terminal to control Messages.app
//     (automatically requested on first use)
//
// Features:
//   - Send and receive iMessage/SMS text messages
//   - Send file attachments
//   - Polling-based message watching
//   - Chat listing and history
//
// Architecture:
// The adapter uses AppleScript for sending messages (which properly triggers macOS
// permission dialogs) and reads from the Messages.app SQLite database for reading
// messages and chat history.
//
// Official Documentation:
//   - Apple Messages Framework: https://developer.apple.com/documentation/messages
package imessage

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/DotNetAge/gort/pkg/channel/imsg"
)

var (
	ErrNotMacOS           = errors.New("iMessage channel requires macOS")
	ErrIMsgNotInstalled   = errors.New("imsg CLI not installed. Install with: go install github.com/steipete/imsg/cmd/imsg@latest")
	ErrPermissionDenied   = errors.New("permission denied. Grant Full Disk Access and Automation permission in System Settings > Privacy & Security")
	ErrNoActiveChat       = errors.New("no active chat found for recipient")
	ErrInvalidPhoneNumber = errors.New("invalid phone number format")
)

// Config contains configuration for the iMessage channel.
type Config struct {
	// DefaultService is the default messaging service ("iMessage" or "SMS")
	DefaultService string

	// Region is the region code for phone number normalization (default: "US")
	Region string

	// EnableTypingIndicators enables typing indicator support (requires imsg v0.5.0+)
	EnableTypingIndicators bool

	// EnableReactions enables tapback reaction support (requires imsg v0.5.0+)
	EnableReactions bool

	// WatchAllChats watches all chats instead of specific ones
	WatchAllChats bool

	// IncludeReactions includes reaction events in message stream
	IncludeReactions bool
}

// Channel implements the channel.Channel interface for iMessage.
type Channel struct {
	*channel.BaseChannel

	config     Config
	client     *imsg.Client
	chats      map[string]*imsg.Chat // Map of handle/phone -> chat
	chatsMu    sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
	msgHandler channel.MessageHandler
}

// NewChannel creates a new iMessage channel.
func NewChannel(name string, config Config) (*Channel, error) {
	// Check if running on macOS
	if runtime.GOOS != "darwin" {
		return nil, ErrNotMacOS
	}

	// Set defaults
	if config.DefaultService == "" {
		config.DefaultService = "iMessage"
	}
	if config.Region == "" {
		config.Region = "US"
	}

	// Create imsg client
	client, err := imsg.NewClient()
	if err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeIMessage),
		config:      config,
		client:      client,
		chats:       make(map[string]*imsg.Chat),
		stopCh:      make(chan struct{}),
	}, nil
}

// Start starts the iMessage channel.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	if handler == nil {
		return errors.New("message handler is required")
	}

	c.msgHandler = handler

	// Start the imsg client
	if err := c.client.Start(ctx); err != nil {
		if errors.Is(err, imsg.ErrPermissionDenied) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("failed to start imsg client: %w", err)
	}

	// Load existing chats (skip if not supported in send-only mode)
	if err := c.loadChats(ctx); err != nil {
		if !errors.Is(err, imsg.ErrNotSupported) {
			return fmt.Errorf("failed to load chats: %w", err)
		}
		// Send-only mode doesn't support chat listing, which is OK
	}

	// Start watching for messages (skip if not supported in send-only mode)
	if c.config.WatchAllChats {
		c.wg.Add(1)
		go c.watchAllChats(ctx)
	} else {
		// Watch individual chats
		c.chatsMu.RLock()
		for _, chat := range c.chats {
			c.wg.Add(1)
			go c.watchChat(ctx, chat.ID)
		}
		c.chatsMu.RUnlock()
	}

	c.SetHandler(handler)
	c.SetStatus(channel.StatusRunning)

	return nil
}

// Stop stops the iMessage channel.
func (c *Channel) Stop(ctx context.Context) error {
	close(c.stopCh)

	// Stop watching
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines stopped
	case <-ctx.Done():
		// Timeout waiting for goroutines
	}

	// Stop the imsg client
	if err := c.client.Stop(); err != nil {
		return fmt.Errorf("failed to stop imsg client: %w", err)
	}

	c.SetStatus(channel.StatusStopped)
	c.msgHandler = nil

	return nil
}

// SendMessage sends a message via iMessage.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	if !c.IsRunning() {
		return errors.New("channel is not running")
	}

	recipient := msg.To.ID
	if recipient == "" {
		return errors.New("recipient is required")
	}

	// Normalize phone number if needed
	recipient = c.normalizeHandle(recipient)

	// Send typing indicator if enabled
	if c.config.EnableTypingIndicators && c.client.GetCapabilities().HasTypingIndicators {
		// Get chat ID for recipient
		chatID := c.getChatIDForHandle(recipient)
		if chatID > 0 {
			_ = c.client.SendTyping(ctx, chatID, true)
			// Stop typing after a short delay
			go func() {
				time.Sleep(2 * time.Second)
				_ = c.client.SendTyping(context.Background(), chatID, false)
			}()
		}
	}

	// Send the message
	service := c.config.DefaultService
	if svc, ok := msg.GetMetadata("service"); ok {
		if svcStr, ok := svc.(string); ok {
			service = svcStr
		}
	}

	// Check if there's a file attachment
	if filePath, ok := msg.GetMetadata("file_path"); ok {
		if filePathStr, ok := filePath.(string); ok && filePathStr != "" {
			return c.client.SendFile(ctx, recipient, filePathStr, msg.Content, service)
		}
	}

	return c.client.SendMessage(ctx, recipient, msg.Content, service)
}

// SendReaction sends a tapback reaction to a message.
func (c *Channel) SendReaction(ctx context.Context, chatID int64, messageGUID, reactionType string) error {
	if !c.config.EnableReactions {
		return errors.New("reactions are not enabled")
	}

	if !c.client.GetCapabilities().HasReactions {
		return errors.New("imsg version does not support reactions (requires v0.5.0+)")
	}

	return c.client.SendReaction(ctx, chatID, messageGUID, reactionType)
}

// GetChats returns a list of available chats.
func (c *Channel) GetChats(ctx context.Context) ([]imsg.Chat, error) {
	if !c.IsRunning() {
		return nil, errors.New("channel is not running")
	}

	return c.client.ListChats(ctx, 100)
}

// GetHistory returns message history for a chat.
func (c *Channel) GetHistory(ctx context.Context, chatID int64, limit int) ([]imsg.Message, error) {
	if !c.IsRunning() {
		return nil, errors.New("channel is not running")
	}

	return c.client.GetHistory(ctx, chatID, limit)
}

// GetCapabilities returns the channel capabilities.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	caps := channel.ChannelCapabilities{
		TextMessages:     true,
		ImageMessages:    true,
		AudioMessages:    true,
		VideoMessages:    true,
		FileMessages:     true,
		LocationMessages: false,
		TemplateMessages: false,
		ReadReceipts:     false,
		TypingIndicators: c.config.EnableTypingIndicators && c.client.GetCapabilities().HasTypingIndicators,
	}

	return caps
}

// loadChats loads the list of available chats.
func (c *Channel) loadChats(ctx context.Context) error {
	chats, err := c.client.ListChats(ctx, 100)
	if err != nil {
		return err
	}

	c.chatsMu.Lock()
	defer c.chatsMu.Unlock()

	for i := range chats {
		chat := &chats[i]
		// Index by identifier
		c.chats[chat.Identifier] = chat

		// Also index by participants
		for _, participant := range chat.Participants {
			normalized := c.normalizeHandle(participant)
			c.chats[normalized] = chat
		}
	}

	return nil
}

// watchAllChats watches all chats for new messages.
func (c *Channel) watchAllChats(ctx context.Context) {
	defer c.wg.Done()

	// Get all chat IDs
	c.chatsMu.RLock()
	chatIDs := make(map[int64]bool)
	for _, chat := range c.chats {
		chatIDs[chat.ID] = true
	}
	c.chatsMu.RUnlock()

	// Watch each chat
	for chatID := range chatIDs {
		c.wg.Add(1)
		go c.watchChat(ctx, chatID)
	}
}

// watchChat watches a specific chat for new messages.
func (c *Channel) watchChat(ctx context.Context, chatID int64) {
	defer c.wg.Done()

	msgCh, err := c.client.Watch(ctx, chatID, 0, c.config.IncludeReactions)
	if err != nil {
		return
	}

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case imsgMsg, ok := <-msgCh:
			if !ok {
				return
			}

			// Convert imsg message to gort message
			msg := c.convertMessage(&imsgMsg)
			if msg == nil {
				continue
			}

			// Call handler
			if c.msgHandler != nil {
				handlerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = c.msgHandler(handlerCtx, msg)
				cancel()
			}
		}
	}
}

// convertMessage converts an imsg.Message to a gort message.Message.
func (c *Channel) convertMessage(imsgMsg *imsg.Message) *channel.Message {
	if imsgMsg == nil {
		return nil
	}

	msgType := channel.MessageTypeText
	if imsgMsg.IsReaction {
		msgType = channel.MessageTypeEvent
	} else if len(imsgMsg.Attachments) > 0 {
		// Determine type from first attachment
		attach := imsgMsg.Attachments[0]
		switch {
		case strings.HasPrefix(attach.MIMEType, "image/"):
			msgType = channel.MessageTypeImage
		case strings.HasPrefix(attach.MIMEType, "video/"):
			msgType = channel.MessageTypeVideo
		case strings.HasPrefix(attach.MIMEType, "audio/"):
			msgType = channel.MessageTypeAudio
		default:
			msgType = channel.MessageTypeFile
		}
	}

	msg := channel.NewMessage(
		strconv.FormatInt(imsgMsg.ID, 10),
		c.Name(),
		channel.DirectionInbound,
		channel.UserInfo{
			ID:       imsgMsg.Sender,
			Name:     imsgMsg.Sender,
			Platform: "imessage",
		},
		imsgMsg.Text,
		msgType,
	)

	// Set recipient
	msg.To = channel.UserInfo{
		ID:       strconv.FormatInt(imsgMsg.ChatID, 10),
		Platform: "imessage",
	}

	// Parse timestamp
	if imsgMsg.CreatedAt != "" {
		if ts, err := time.Parse(time.RFC3339, imsgMsg.CreatedAt); err == nil {
			msg.Timestamp = ts.Format(time.RFC3339)
		}
	}

	// Add metadata
	msg.SetMetadata("guid", imsgMsg.GUID)
	msg.SetMetadata("chat_id", imsgMsg.ChatID)
	msg.SetMetadata("is_from_me", imsgMsg.IsFromMe)

	if imsgMsg.IsReaction {
		msg.SetMetadata("is_reaction", true)
		msg.SetMetadata("reaction_type", imsgMsg.ReactionType)
		msg.SetMetadata("reaction_emoji", imsgMsg.ReactionEmoji)
		msg.SetMetadata("is_reaction_add", imsgMsg.IsReactionAdd)
		msg.SetMetadata("reacted_to_guid", imsgMsg.ReactedToGUID)
	}

	if imsgMsg.ReplyToGUID != "" {
		msg.SetMetadata("reply_to_guid", imsgMsg.ReplyToGUID)
	}

	if len(imsgMsg.Attachments) > 0 {
		attach := imsgMsg.Attachments[0]
		msg.SetMetadata("attachment_filename", attach.Filename)
		msg.SetMetadata("attachment_path", attach.OriginalPath)
		msg.SetMetadata("attachment_mime_type", attach.MIMEType)
	}

	return msg
}

// getChatIDForHandle returns the chat ID for a given handle/phone number.
func (c *Channel) getChatIDForHandle(handle string) int64 {
	normalized := c.normalizeHandle(handle)

	c.chatsMu.RLock()
	defer c.chatsMu.RUnlock()

	if chat, ok := c.chats[normalized]; ok {
		return chat.ID
	}

	return 0
}

// normalizeHandle normalizes a phone number or handle.
func (c *Channel) normalizeHandle(handle string) string {
	// Remove common prefixes and whitespace
	handle = strings.TrimSpace(handle)
	handle = strings.TrimPrefix(handle, "tel:")
	handle = strings.TrimPrefix(handle, "sms:")

	// If it looks like a phone number, normalize it
	if strings.HasPrefix(handle, "+") || isNumeric(handle) {
		// Remove all non-numeric characters except +
		var result strings.Builder
		for _, r := range handle {
			if r == '+' || (r >= '0' && r <= '9') {
				result.WriteRune(r)
			}
		}
		return result.String()
	}

	// Return as-is for email addresses
	return handle
}

// isNumeric checks if a string contains only digits.
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
