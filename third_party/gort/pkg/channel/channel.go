// Package channel provides the Channel interface and implementations for various
// messaging platforms. Each adapter implements the Channel interface to enable
// unified message handling across different platforms.
//
// Basic Usage:
//
//	// Create a channel registry
//	registry := channel.NewRegistry()
//
//	// Register a channel
//	ch, err := dingtalk.NewChannel("my-dingtalk", dingtalk.Config{
//	    AppKey:    "your-app-key",
//	    AppSecret: "your-app-secret",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if err := registry.Register(ch); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start all channels
//	ctx := context.Background()
//	handler := func(ctx context.Context, msg *Message) error {
//	    fmt.Printf("Received: %s\n", msg.Content)
//	    return nil
//	}
//
//	if err := registry.StartAll(ctx, handler); err != nil {
//	    log.Fatal(err)
//	}
//
// Thread Safety:
//
// All types in this package are safe for concurrent use unless otherwise noted.
// The Channel interface implementations must be safe for concurrent use by multiple goroutines.
package channel

import (
	"context"
	"errors"
	"sync"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type MessageType string

const (
	MessageTypeText        MessageType = "text"
	MessageTypeImage       MessageType = "image"
	MessageTypeFile        MessageType = "file"
	MessageTypeAudio       MessageType = "audio"
	MessageTypeVideo       MessageType = "video"
	MessageTypeEvent       MessageType = "event"
	MessageTypeMarkdown    MessageType = "markdown"
	MessageTypeNews        MessageType = "news"
	MessageTypeVoice       MessageType = "voice"
	MessageTypeTemplateCard MessageType = "template_card"
)

type UserInfo struct {
	ID       string
	Name     string
	Avatar   string
	Language string
	Platform string
}

type Message struct {
	ID        string
	ChannelID string
	Type      MessageType
	Direction Direction
	From      UserInfo
	To        UserInfo
	Content   string
	Data      []byte
	Raw       []byte
	Timestamp string
	Metadata  map[string]interface{}
}

func (m *Message) GetMetadata(key string) (interface{}, bool) {
	if m.Metadata == nil {
		return nil, false
	}
	v, ok := m.Metadata[key]
	return v, ok
}

func (m *Message) SetMetadata(key string, value interface{}) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
}

func NewMessage(id, channelID string, direction Direction, from UserInfo, content string, msgType MessageType) *Message {
	return &Message{
		ID:        id,
		ChannelID: channelID,
		Type:      msgType,
		Direction: direction,
		From:      from,
		Content:   content,
		Metadata:  make(map[string]interface{}),
	}
}

// Error definitions for channel operations.
// These errors are returned by various channel methods to indicate specific failure conditions.
var (
	ErrChannelAlreadyRunning   = errors.New("channel is already running")
	ErrChannelNotRunning       = errors.New("channel is not running")
	ErrChannelNotFound         = errors.New("channel not found")
	ErrChannelAlreadyExists    = errors.New("channel already exists")
	ErrInvalidConfiguration    = errors.New("invalid channel configuration")
	ErrAuthenticationFailed    = errors.New("authentication failed")
	ErrRateLimited             = errors.New("rate limited")
	ErrMessageTooLarge         = errors.New("message too large")
	ErrUnsupportedMessageType  = errors.New("unsupported message type")
	ErrTokenExpired            = errors.New("token expired or not available")
	ErrRequiredField           = errors.New("required field is missing")
	ErrInvalidFormat           = errors.New("invalid format")
)

// Status represents the current operational status of a channel.
// It indicates whether the channel is stopped, running, or in an error state.
type Status string

const (
	// StatusStopped indicates the channel is not currently active.
	// This is the initial state and the state after Stop() is called.
	StatusStopped Status = "stopped"

	// StatusRunning indicates the channel is active and processing messages.
	// This state is entered after Start() is called successfully.
	StatusRunning Status = "running"

	// StatusError indicates the channel has encountered an error and is not operational.
	// The channel may need to be restarted or reconfigured.
	StatusError Status = "error"
)

// ChannelType represents the type of messaging platform.
// It is used to identify which platform a channel is connected to.
type ChannelType string

const (
	// ChannelTypeWeChat represents the WeChat messaging platform.
	// API Documentation: https://developers.weixin.qq.com/doc/
	ChannelTypeWeChat ChannelType = "wechat"

	// ChannelTypeDingTalk represents the DingTalk (钉钉) messaging platform.
	// API Documentation: https://open.dingtalk.com/document/
	ChannelTypeDingTalk ChannelType = "dingtalk"

	// ChannelTypeFeishu represents the Feishu (飞书) messaging platform.
	// API Documentation: https://open.feishu.cn/document/
	ChannelTypeFeishu ChannelType = "feishu"

	// ChannelTypeTelegram represents the Telegram messaging platform.
	// API Documentation: https://core.telegram.org/bots/api
	ChannelTypeTelegram ChannelType = "telegram"

	// ChannelTypeMessenger represents the Facebook Messenger platform.
	// API Documentation: https://developers.facebook.com/docs/messenger-platform/
	ChannelTypeMessenger ChannelType = "messenger"

	// ChannelTypeWhatsApp represents the WhatsApp Business API.
	// API Documentation: https://developers.facebook.com/docs/whatsapp/
	ChannelTypeWhatsApp ChannelType = "whatsapp"

	// ChannelTypeIMessage represents the Apple iMessage platform.
	// Note: Requires special entitlements and hardware.
	ChannelTypeIMessage ChannelType = "imessage"

	// ChannelTypeWeCom represents the WeCom (企业微信) platform.
	// API Documentation: https://developer.work.weixin.qq.com/
	ChannelTypeWeCom ChannelType = "wecom"

	// ChannelTypeSlack represents the Slack platform.
	// API Documentation: https://api.slack.com/
	ChannelTypeSlack ChannelType = "slack"

	// ChannelTypeDiscord represents the Discord platform.
	// API Documentation: https://discord.com/developers/docs/
	ChannelTypeDiscord ChannelType = "discord"
)

// MessageHandler is the function type for handling incoming messages.
// Implementations receive the message context and the message itself.
//
// The handler should return nil on success, or an error if message processing fails.
// Returning an error may trigger retry logic depending on the channel configuration.
//
// Example:
//
//	handler := func(ctx context.Context, msg *Message) error {
//	    log.Printf("Received message from %s: %s", msg.From.Name, msg.Content)
//	    // Process the message...
//	    return nil
//	}
type MessageHandler func(ctx context.Context, msg *Message) error

// Channel is the core interface for all messaging platform adapters.
// Implementations must be safe for concurrent use.
//
// Each adapter must embed code comments documenting the official API source:
//   - API Documentation URL
//   - API Version
//   - Authentication Method
//
// Lifecycle:
//
//  1. Create a channel instance with platform-specific configuration
//  2. Register the channel with a Registry
//  3. Call Start() to begin receiving messages
//  4. Handle incoming messages via the MessageHandler
//  5. Call Stop() to gracefully shut down
//
// Example:
//
//	ch, _ := telegram.NewChannel("my-bot", telegram.Config{Token: "bot-token"})
//	ctx := context.Background()
//
//	err := ch.Start(ctx, func(ctx context.Context, msg *Message) error {
//	    fmt.Printf("From %s: %s\n", msg.From.Name, msg.Content)
//	    return nil
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Later...
//	ch.Stop(ctx)
type Channel interface {
	// Name returns the unique identifier for this channel instance.
	// This name is used to identify the channel in the registry and logs.
	Name() string

	// Type returns the channel type (e.g., "wechat", "telegram").
	// This identifies which messaging platform the channel connects to.
	Type() ChannelType

	// Start initializes the channel and begins listening for incoming messages.
	// The handler function will be called for each incoming message.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - handler: Function to call for each received message
	//
	// Returns an error if the channel is already running or fails to start.
	Start(ctx context.Context, handler MessageHandler) error

	// Stop gracefully shuts down the channel.
	// This should close any connections and clean up resources.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//
	// Returns an error if the channel is not running or fails to stop cleanly.
	Stop(ctx context.Context) error

	// IsRunning returns true if the channel is currently active.
	// This can be used to check the channel status before operations.
	IsRunning() bool

	// SendMessage sends a message through this channel.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - msg: The message to send
	//
	// Returns an error if the message cannot be sent.
	SendMessage(ctx context.Context, msg *Message) error

	// GetStatus returns the current status of the channel.
	// This provides more detailed status information than IsRunning().
	GetStatus() Status
}

// WebhookHandler is an optional interface for channels that support webhook-based
// message reception. Channels implementing this interface can receive messages
// via HTTP callbacks.
//
// Webhook endpoints should be secured and validated to prevent unauthorized access.
// Each platform has its own webhook validation mechanism.
type WebhookHandler interface {
	// HandleWebhook processes an incoming webhook request.
	//
	// Parameters:
	//   - path: the URL path of the webhook
	//   - data: the raw request body
	//
	// Returns the parsed message or an error if the webhook cannot be processed.
	//
	// Example:
	//
	//	handler := ch.(channel.WebhookHandler)
	//	msg, err := handler.HandleWebhook("/webhook/wechat", body)
	//	if err != nil {
	//	    http.Error(w, err.Error(), http.StatusBadRequest)
	//	    return
	//	}
	//	// Process msg...
	HandleWebhook(path string, data []byte) (*Message, error)
}

// OAuthConfig contains OAuth authentication configuration.
// This is used by channels that require OAuth 2.0 authentication.
type OAuthConfig struct {
	// ClientID is the OAuth client identifier.
	ClientID string

	// ClientSecret is the OAuth client secret.
	// This should be stored securely and not exposed in logs.
	ClientSecret string

	// RedirectURL is the URL to redirect to after OAuth authorization.
	RedirectURL string

	// Scopes are the OAuth scopes to request.
	Scopes []string
}

// RateLimitConfig contains rate limiting configuration.
// This controls how many requests can be made to the platform API.
type RateLimitConfig struct {
	// RequestsPerSecond is the maximum number of requests per second.
	RequestsPerSecond int

	// BurstSize is the maximum number of requests that can be made in a burst.
	BurstSize int
}

// ChannelCapabilities describes what features a channel supports.
// This can be used to determine which message types can be sent through a channel.
type ChannelCapabilities struct {
	// TextMessages indicates support for plain text messages.
	TextMessages bool

	// MarkdownMessages indicates support for markdown formatted messages.
	MarkdownMessages bool

	// ImageMessages indicates support for image messages.
	ImageMessages bool

	// FileMessages indicates support for file attachments.
	FileMessages bool

	// AudioMessages indicates support for audio messages.
	AudioMessages bool

	// VideoMessages indicates support for video messages.
	VideoMessages bool

	// VoiceMessages indicates support for voice messages.
	VoiceMessages bool

	// NewsMessages indicates support for news/article messages.
	NewsMessages bool

	// LocationMessages indicates support for location sharing.
	LocationMessages bool

	// TemplateMessages indicates support for template messages.
	TemplateMessages bool

	// TemplateCard indicates support for template card messages.
	TemplateCard bool

	// ReadReceipts indicates support for read receipts.
	ReadReceipts bool

	// TypingIndicators indicates support for typing indicators.
	TypingIndicators bool

	// MessageEditing indicates support for editing sent messages.
	MessageEditing bool

	// MessageDeletion indicates support for deleting sent messages.
	MessageDeletion bool

	// ReactionMessages indicates support for message reactions.
	ReactionMessages bool

	// BlockKit indicates support for Slack Block Kit messages.
	BlockKit bool

	// Interactive indicates support for interactive messages.
	Interactive bool

	// Threads indicates support for threaded messages.
	Threads bool
}

// BaseChannel provides common functionality for all channel implementations.
// It handles status management, handler registration, and thread-safe operations.
//
// Embed this struct in channel implementations to inherit common behavior:
//
//	type MyChannel struct {
//	    *channel.BaseChannel
//	    // additional fields...
//	}
//
// This type is safe for concurrent use.
type BaseChannel struct {
	name        string
	channelType ChannelType
	status      Status
	handler     MessageHandler
	sender      GatewaySender
	mu          sync.RWMutex
}

// NewBaseChannel creates a new BaseChannel with the given name and type.
//
// Parameters:
//   - name: Unique identifier for this channel instance
//   - channelType: The messaging platform type
//
// Returns a new BaseChannel initialized with StatusStopped.
//
// Example:
//
//	base := channel.NewBaseChannel("my-wechat", channel.ChannelTypeWeChat)
//	fmt.Println(base.GetStatus()) // "stopped"
func NewBaseChannel(name string, channelType ChannelType) *BaseChannel {
	return &BaseChannel{
		name:        name,
		channelType: channelType,
		status:      StatusStopped,
	}
}

// Name returns the channel name.
// This is the unique identifier set during creation.
func (b *BaseChannel) Name() string {
	return b.name
}

// Type returns the channel type.
// This identifies which messaging platform the channel connects to.
func (b *BaseChannel) Type() ChannelType {
	return b.channelType
}

// GetStatus returns the current status.
// This method is thread-safe and can be called concurrently.
func (b *BaseChannel) GetStatus() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

// IsRunning returns true if the channel is running.
// This is a convenience method equivalent to checking GetStatus() == StatusRunning.
func (b *BaseChannel) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status == StatusRunning
}

// SetStatus updates the channel status.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - status: The new status to set
func (b *BaseChannel) SetStatus(status Status) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = status
}

// SetHandler sets the message handler.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - handler: The function to call for incoming messages
func (b *BaseChannel) SetHandler(handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handler = handler
}

// GetHandler returns the current message handler.
// This method is thread-safe and can be called concurrently.
// Returns nil if no handler has been set.
func (b *BaseChannel) GetHandler() MessageHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.handler
}

// HandleMessage calls the registered handler with the given message.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - ctx: Context for cancellation
//   - msg: The message to handle
//
// Returns ErrChannelNotRunning if no handler is registered.
func (b *BaseChannel) HandleMessage(ctx context.Context, msg *Message) error {
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler == nil {
		return ErrChannelNotRunning
	}
	return handler(ctx, msg)
}

type GatewaySender interface {
	Send(to string, message string)
	Broadcast(message string)
	SendJSON(to string, v interface{}) error
	BroadcastJSON(v interface{}) error
	ClientCount() int
}

func (b *BaseChannel) SetGatewaySender(s GatewaySender) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s != nil {
		b.sender = s
	}
}

func (b *BaseChannel) GetGatewaySender() GatewaySender {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sender
}

func (b *BaseChannel) SendToGateway(message string) {
	if s := b.GetGatewaySender(); s != nil {
		s.Broadcast(message)
	}
}

func (b *BaseChannel) SendToGatewayClient(clientID, message string) {
	if s := b.GetGatewaySender(); s != nil {
		s.Send(clientID, message)
	}
}

// Registry manages a collection of channels.
// It provides thread-safe operations for channel registration and lifecycle management.
//
// This type is safe for concurrent use.
type Registry struct {
	channels map[string]Channel
	mu       sync.RWMutex
}

// NewRegistry creates a new empty registry.
//
// Example:
//
//	registry := channel.NewRegistry()
//	fmt.Println(registry.Count()) // 0
func NewRegistry() *Registry {
	return &Registry{
		channels: make(map[string]Channel),
	}
}

// Register adds a channel to the registry.
//
// Parameters:
//   - channel: The channel to register
//
// Returns ErrChannelAlreadyExists if a channel with the same name already exists.
//
// Example:
//
//	ch, _ := telegram.NewChannel("my-bot", config)
//	if err := registry.Register(ch); err != nil {
//	    log.Fatal(err)
//	}
func (r *Registry) Register(channel Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := channel.Name()
	if _, exists := r.channels[name]; exists {
		return ErrChannelAlreadyExists
	}

	r.channels[name] = channel
	return nil
}

// Unregister removes a channel from the registry.
// This does not stop the channel if it is running.
//
// Parameters:
//   - name: The name of the channel to remove
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, name)
}

// Get retrieves a channel by name.
//
// Parameters:
//   - name: The name of the channel to retrieve
//
// Returns the channel and true if found, or nil and false if not found.
//
// Example:
//
//	if ch, ok := registry.Get("my-bot"); ok {
//	    fmt.Println(ch.Type())
//	}
func (r *Registry) Get(name string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	channel, ok := r.channels[name]
	return channel, ok
}

// GetAll returns all registered channels.
// The returned slice is a copy and safe to modify.
//
// Returns a slice containing all registered channels.
func (r *Registry) GetAll() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channels := make([]Channel, 0, len(r.channels))
	for _, c := range r.channels {
		channels = append(channels, c)
	}
	return channels
}

// Count returns the number of registered channels.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.channels)
}

// StartAll starts all registered channels.
// If any channel fails to start, the operation stops and returns the error.
// Channels that started successfully before the error are not automatically stopped.
//
// Parameters:
//   - ctx: Context for cancellation
//   - handler: Message handler to use for all channels
//
// Returns an error if any channel fails to start.
func (r *Registry) StartAll(ctx context.Context, handler MessageHandler) error {
	r.mu.RLock()
	channels := make([]Channel, 0, len(r.channels))
	for _, c := range r.channels {
		channels = append(channels, c)
	}
	r.mu.RUnlock()

	for _, c := range channels {
		if err := c.Start(ctx, handler); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all registered channels.
// Attempts to stop all channels even if some fail.
// Returns the last error encountered, if any.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns the last error encountered during shutdown, or nil if all channels stopped successfully.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	channels := make([]Channel, 0, len(r.channels))
	for _, c := range r.channels {
		channels = append(channels, c)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, c := range channels {
		if err := c.Stop(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// MockChannel is a mock implementation of the Channel interface for testing.
// It records all sent messages and can simulate errors.
//
// This type is safe for concurrent use.
type MockChannel struct {
	*BaseChannel
	messages  []*Message
	sendError error
	started   bool
	mu        sync.Mutex
}

// NewMockChannel creates a new MockChannel for testing.
//
// Parameters:
//   - name: Unique identifier for this mock channel
//   - channelType: The channel type to simulate
//
// Example:
//
//	mock := channel.NewMockChannel("test", channel.ChannelTypeTelegram)
//	err := mock.Start(ctx, handler)
func NewMockChannel(name string, channelType ChannelType) *MockChannel {
	return &MockChannel{
		BaseChannel: NewBaseChannel(name, channelType),
		messages:    make([]*Message, 0),
	}
}

// Start starts the mock channel.
//
// Parameters:
//   - ctx: Context for cancellation
//   - handler: Message handler (stored but not used in mock)
//
// Returns ErrChannelAlreadyRunning if already started.
func (m *MockChannel) Start(ctx context.Context, handler MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return ErrChannelAlreadyRunning
	}

	m.started = true
	m.SetHandler(handler)
	m.SetStatus(StatusRunning)
	return nil
}

// Stop stops the mock channel.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns ErrChannelNotRunning if not started.
func (m *MockChannel) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return ErrChannelNotRunning
	}

	m.started = false
	m.SetStatus(StatusStopped)
	m.SetHandler(nil)
	return nil
}

// SendMessage simulates sending a message.
// The message is recorded in the messages slice.
// If sendError is set, it returns that error instead.
//
// Parameters:
//   - ctx: Context for cancellation
//   - msg: The message to "send"
//
// Returns sendError if set, otherwise nil.
func (m *MockChannel) SendMessage(ctx context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendError != nil {
		return m.sendError
	}

	m.messages = append(m.messages, msg)
	return nil
}

// GetMessages returns all recorded messages.
// This is useful for verifying that messages were "sent".
//
// Returns a slice of all messages passed to SendMessage.
func (m *MockChannel) GetMessages() []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// SetSendError sets an error to be returned by SendMessage.
// Use this to simulate send failures in tests.
//
// Parameters:
//   - err: The error to return, or nil for success
func (m *MockChannel) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendError = err
}

// ClearMessages removes all recorded messages.
// This is useful for resetting state between tests.
func (m *MockChannel) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = m.messages[:0]
}

// SimulateMessage simulates receiving a message.
// This is useful for testing message handling without a real connection.
//
// Parameters:
//   - ctx: Context for cancellation
//   - msg: The message to simulate receiving
//
// Returns ErrChannelNotRunning if the channel is not started.
func (m *MockChannel) SimulateMessage(ctx context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return ErrChannelNotRunning
	}

	handler := m.GetHandler()
	if handler != nil {
		return handler(ctx, msg)
	}
	return nil
}
