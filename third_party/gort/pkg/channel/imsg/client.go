// Package imsg provides a Go client for iMessage using pure AppleScript.
// This client communicates with macOS Messages.app via AppleScript only,
// with no dependency on imsg CLI or Full Disk Access.
//
// Requirements:
//   - macOS 14+ with Messages.app signed in
//   - Automation permission for the terminal to control Messages.app
//     (automatically requested on first use)
//
// Features:
//   - Send text messages
//   - Send file attachments
//
// Architecture:
// The client uses AppleScript exclusively for all operations.
// No database access, no imsg CLI dependency, no Full Disk Access required.
// macOS will automatically prompt for Automation permission on first use.
package imsg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrPermissionDenied   = errors.New("permission denied. Please allow automation permission in System Settings > Privacy & Security > Automation")
	ErrMessagesNotRunning = errors.New("Messages.app is not running")
	ErrNotSupported       = errors.New("this feature is not supported in send-only mode")
)

// Client represents a client for iMessage via AppleScript.
type Client struct {
	mu           sync.Mutex
	running      bool
	stopCh       chan struct{}
	capabilities Capabilities
}

// Capabilities represents the features supported by the client.
type Capabilities struct {
	HasTypingIndicators bool
	HasReactions        bool
	HasRPC              bool
}

// Chat represents an iMessage conversation.
type Chat struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Identifier    string   `json:"identifier"`
	Service       string   `json:"service"`
	LastMessageAt string   `json:"last_message_at"`
	IsGroup       bool     `json:"is_group"`
	Participants  []string `json:"participants,omitempty"`
}

// Message represents an iMessage/SMS message.
type Message struct {
	ID                   int64        `json:"id"`
	ChatID               int64        `json:"chat_id"`
	GUID                 string       `json:"guid"`
	ReplyToGUID          string       `json:"reply_to_guid,omitempty"`
	DestinationCallerID  string       `json:"destination_caller_id,omitempty"`
	Sender               string       `json:"sender"`
	IsFromMe             bool         `json:"is_from_me"`
	Text                 string       `json:"text"`
	CreatedAt            string       `json:"created_at"`
	Attachments          []Attachment `json:"attachments,omitempty"`
	IsReaction           bool         `json:"is_reaction,omitempty"`
	ReactionType         string       `json:"reaction_type,omitempty"`
	ReactionEmoji        string       `json:"reaction_emoji,omitempty"`
	IsReactionAdd        bool         `json:"is_reaction_add,omitempty"`
	ReactedToGUID        string       `json:"reacted_to_guid,omitempty"`
	ThreadOriginatorGUID string       `json:"thread_originator_guid,omitempty"`
}

// Attachment represents a message attachment.
type Attachment struct {
	Filename     string `json:"filename"`
	TransferName string `json:"transfer_name"`
	UTI          string `json:"uti"`
	MIMEType     string `json:"mime_type"`
	TotalBytes   int64  `json:"total_bytes"`
	IsSticker    bool   `json:"is_sticker"`
	OriginalPath string `json:"original_path"`
	Missing      bool   `json:"missing"`
}

// NewClient creates a new iMessage client.
func NewClient() (*Client, error) {
	return &Client{
		stopCh: make(chan struct{}),
		capabilities: Capabilities{
			HasRPC:              true,
			HasTypingIndicators: false,
			HasReactions:        false,
		},
	}, nil
}

// Start initializes the iMessage client.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	// Ensure Messages.app is running (will launch if not running)
	if err := c.ensureMessagesApp(ctx); err != nil {
		return fmt.Errorf("failed to start Messages.app: %w", err)
	}

	// Test AppleScript permission (this will trigger permission dialog if needed)
	if err := c.testPermission(ctx); err != nil {
		return fmt.Errorf("AppleScript permission denied: %w", err)
	}

	c.running = true

	return nil
}

// Stop stops the client.
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	close(c.stopCh)
	c.running = false

	return nil
}

// IsRunning returns true if the client is running.
func (c *Client) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// GetCapabilities returns the capabilities of the client.
func (c *Client) GetCapabilities() Capabilities {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.capabilities
}

// ListChats returns a list of recent chats.
// Note: Not supported in send-only mode (requires Full Disk Access).
func (c *Client) ListChats(ctx context.Context, limit int) ([]Chat, error) {
	return nil, ErrNotSupported
}

// GetHistory returns message history for a chat.
// Note: Not supported in send-only mode (requires Full Disk Access).
func (c *Client) GetHistory(ctx context.Context, chatID int64, limit int) ([]Message, error) {
	return nil, ErrNotSupported
}

// SendMessage sends a text message.
func (c *Client) SendMessage(ctx context.Context, to, text, service string) error {
	if !c.IsRunning() {
		return ErrMessagesNotRunning
	}

	return c.sendMessageViaAppleScript(ctx, to, text, service)
}

// SendFile sends a file attachment.
func (c *Client) SendFile(ctx context.Context, to, filePath, text, service string) error {
	if !c.IsRunning() {
		return ErrMessagesNotRunning
	}

	return c.sendFileViaAppleScript(ctx, to, filePath, text, service)
}

// SendReaction sends a tapback reaction.
// Note: Not supported in AppleScript-only mode.
func (c *Client) SendReaction(ctx context.Context, chatID int64, messageGUID, reactionType string) error {
	return ErrNotSupported
}

// SendTyping sends a typing indicator.
// Note: Not supported in AppleScript-only mode.
func (c *Client) SendTyping(ctx context.Context, chatID int64, typing bool) error {
	return ErrNotSupported
}

// Watch starts watching for new messages.
// Note: Not supported in send-only mode (requires Full Disk Access).
func (c *Client) Watch(ctx context.Context, chatID int64, sinceRowID int64, includeReactions bool) (<-chan Message, error) {
	return nil, ErrNotSupported
}

// AppleScript helper methods

func (c *Client) testPermission(ctx context.Context) error {
	script := `
tell application "Messages"
	return name
end tell
`
	_, err := c.runAppleScript(ctx, script)
	if err != nil {
		if strings.Contains(err.Error(), "not authorized") ||
			strings.Contains(err.Error(), "permission denied") {
			return ErrPermissionDenied
		}
		return err
	}
	return nil
}

func (c *Client) isMessagesRunning() bool {
	cmd := exec.Command("pgrep", "-x", "Messages")
	return cmd.Run() == nil
}

func (c *Client) ensureMessagesApp(ctx context.Context) error {
	if c.isMessagesRunning() {
		return nil
	}

	script := `
tell application "Messages"
	activate
end tell
`
	_, err := c.runAppleScript(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to launch Messages.app: %w", err)
	}

	// Wait for Messages.app to start (up to 10 seconds)
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if c.isMessagesRunning() {
			return nil
		}
	}

	return ErrMessagesNotRunning
}

func (c *Client) sendMessageViaAppleScript(ctx context.Context, to, text, service string) error {
	// Escape special characters in text
	escapedText := escapeAppleScript(text)

	script := fmt.Sprintf(`
tell application "Messages"
	set targetService to service "%s"
	
	try
		set theChat to chat "%s" of targetService
		send "%s" to theChat
	on error
		// Chat doesn't exist, create new conversation
		set theBuddy to buddy "%s" of targetService
		send "%s" to theBuddy
	end try
end tell
`, service, to, escapedText, to, escapedText)

	_, err := c.runAppleScript(ctx, script)
	if err != nil {
		if strings.Contains(err.Error(), "not authorized") {
			return ErrPermissionDenied
		}
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *Client) sendFileViaAppleScript(ctx context.Context, to, filePath, text, service string) error {
	escapedText := escapeAppleScript(text)
	escapedPath := escapeAppleScript(filePath)

	script := fmt.Sprintf(`
tell application "Messages"
	set targetService to service "%s"
	set theFile to POSIX file "%s"
	
	try
		set theChat to chat "%s" of targetService
		send theFile to theChat
		if "%s" is not "" then
			send "%s" to theChat
		end if
	on error
		set theBuddy to buddy "%s" of targetService
		send theFile to theBuddy
		if "%s" is not "" then
			send "%s" to theBuddy
		end if
	end try
end tell
`, service, escapedPath, to, text, escapedText, to, text, escapedText)

	_, err := c.runAppleScript(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	return nil
}

func (c *Client) runAppleScript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return "", fmt.Errorf("AppleScript error: %s", errMsg)
		}
		return "", err
	}

	return stdout.String(), nil
}

// Utility functions

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
