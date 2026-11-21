// Package discord provides a Channel adapter for Discord.
//
// Official API Documentation:
// - URL: https://discord.com/developers/docs
// - API Version: Discord API v10
// - Last Updated: 2024
// - Authentication: Bot Token
//
// Supported Features:
// - Text messages with markdown formatting
// - Embeds (rich content)
// - File attachments
// - Reactions (emoji)
// - Thread management
// - Slash commands
// - Gateway events (real-time)
//
// Rate Limits:
// - Global: 50 requests per second
// - Per-route limits vary by endpoint
// - Rate limit headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
//
// Security:
// - Bot Token authentication
// - OAuth2 for user authorization
// - HTTPS required for all API calls
// - Gateway uses secure WebSocket (wss://)
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for Discord API.
var (
	// BaseURL is the Discord API base URL (can be overridden for testing)
	BaseURL = "https://discord.com/api/v10"
)

const (
	EndpointCreateMessage  = "/channels/{channel.id}/messages"
	EndpointEditMessage    = "/channels/{channel.id}/messages/{message.id}"
	EndpointDeleteMessage  = "/channels/{channel.id}/messages/{message.id}"
	EndpointCreateReaction = "/channels/{channel.id}/messages/{message.id}/reactions/{emoji}/@me"
	EndpointGetChannel     = "/channels/{channel.id}"
	EndpointGetGuild       = "/guilds/{guild.id}"
	EndpointGateway        = "/gateway"
	EndpointGatewayBot     = "/gateway/bot"
)

// Error definitions for Discord channel.
var (
	ErrTokenRequired   = errors.New("bot token is required")
	ErrInvalidToken    = errors.New("invalid bot token format")
	ErrChannelNotFound = errors.New("channel not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrForbidden       = errors.New("forbidden - check bot permissions")
	ErrRateLimited     = errors.New("rate limited by Discord API")
)

// Config contains the configuration for Discord channel.
type Config struct {
	// BotToken is the Discord bot token
	BotToken string

	// ApplicationID is the application ID for slash commands
	ApplicationID string

	// PublicKey is for verifying webhook requests (optional)
	PublicKey string

	// DefaultChannelID is the default channel to send messages to
	DefaultChannelID string

	// HTTPTimeout is the HTTP client timeout (default: 30s)
	HTTPTimeout time.Duration
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.BotToken == "" {
		return ErrTokenRequired
	}

	return nil
}

// Channel implements the channel.Channel interface for Discord.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewChannel creates a new Discord channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	timeout := config.HTTPTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeDiscord),
		config:      config,
		httpClient:  &http.Client{Timeout: timeout},
	}, nil
}

// NewChannelWithHTTPClient creates a new Discord channel with a custom HTTP client (for testing).
func NewChannelWithHTTPClient(name string, config Config, httpClient *http.Client) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeDiscord),
		config:      config,
		httpClient:  httpClient,
	}, nil
}

// Start starts the Discord channel.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	// Verify token is valid by making a test request
	if err := c.validateToken(ctx); err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	c.SetHandler(handler)
	c.SetStatus(channel.StatusRunning)
	return nil
}

// Stop stops the Discord channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.SetStatus(channel.StatusStopped)
	c.SetHandler(nil)
	return nil
}

// SendMessage sends a message to Discord.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	if !c.IsRunning() {
		return errors.New("channel is not running")
	}

	channelID := msg.To.ID
	if channelID == "" {
		channelID = c.config.DefaultChannelID
	}
	if channelID == "" {
		return errors.New("channel ID is required")
	}

	// Build message payload
	payload := c.buildMessagePayload(msg)

	return c.createMessage(ctx, channelID, payload)
}

// buildMessagePayload builds the message payload for Discord API.
func (c *Channel) buildMessagePayload(msg *channel.Message) map[string]interface{} {
	payload := map[string]interface{}{}

	switch msg.Type {
	case channel.MessageTypeText:
		payload["content"] = msg.Content

	case channel.MessageTypeMarkdown:
		// Discord uses markdown natively
		payload["content"] = msg.Content

	case channel.MessageTypeImage:
		// For images, create an embed
		if imageURL, ok := msg.GetMetadata("image_url"); ok {
			if url, ok := imageURL.(string); ok {
				payload["embeds"] = []map[string]interface{}{
					{
						"image": map[string]interface{}{
							"url": url,
						},
					},
				}
			}
		} else {
			payload["content"] = msg.Content
		}

	default:
		payload["content"] = msg.Content
	}

	// Add embeds if provided
	if embeds, ok := msg.GetMetadata("embeds"); ok {
		if e, ok := embeds.([]map[string]interface{}); ok {
			payload["embeds"] = e
		}
	}

	// Add reply reference if provided
	if replyTo, ok := msg.GetMetadata("reply_to_message_id"); ok {
		if msgID, ok := replyTo.(string); ok {
			payload["message_reference"] = map[string]interface{}{
				"message_id": msgID,
			}
		}
	}

	return payload
}

// createMessage sends a message to a Discord channel.
func (c *Channel) createMessage(ctx context.Context, channelID string, payload map[string]interface{}) error {
	endpoint := strings.Replace(EndpointCreateMessage, "{channel.id}", channelID, 1)
	return c.apiRequest(ctx, http.MethodPost, endpoint, payload, nil)
}

// EditMessage edits an existing message.
func (c *Channel) EditMessage(ctx context.Context, channelID, messageID string, msg *channel.Message) error {
	payload := c.buildMessagePayload(msg)
	endpoint := strings.Replace(EndpointEditMessage, "{channel.id}", channelID, 1)
	endpoint = strings.Replace(endpoint, "{message.id}", messageID, 1)

	return c.apiRequest(ctx, http.MethodPatch, endpoint, payload, nil)
}

// DeleteMessage deletes a message.
func (c *Channel) DeleteMessage(ctx context.Context, channelID, messageID string) error {
	endpoint := strings.Replace(EndpointDeleteMessage, "{channel.id}", channelID, 1)
	endpoint = strings.Replace(endpoint, "{message.id}", messageID, 1)

	return c.apiRequest(ctx, http.MethodDelete, endpoint, nil, nil)
}

// AddReaction adds a reaction (emoji) to a message.
func (c *Channel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	// URL encode the emoji
	emoji = urlEncodeEmoji(emoji)

	endpoint := strings.Replace(EndpointCreateReaction, "{channel.id}", channelID, 1)
	endpoint = strings.Replace(endpoint, "{message.id}", messageID, 1)
	endpoint = strings.Replace(endpoint, "{emoji}", emoji, 1)

	return c.apiRequest(ctx, http.MethodPut, endpoint, nil, nil)
}

// UploadFile uploads a file to Discord.
func (c *Channel) UploadFile(ctx context.Context, channelID string, fileData []byte, filename, content string) error {
	// Build multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	// Add content if provided
	if content != "" {
		if err := writer.WriteField("content", content); err != nil {
			return fmt.Errorf("failed to write content field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	endpoint := strings.Replace(EndpointCreateMessage, "{channel.id}", channelID, 1)
	reqURL := BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+c.config.BotToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, nil)
}

// validateToken validates the bot token.
func (c *Channel) validateToken(ctx context.Context) error {
	return c.apiRequest(ctx, http.MethodGet, EndpointGateway, nil, nil)
}

// apiRequest makes a request to the Discord API.
func (c *Channel) apiRequest(ctx context.Context, method, endpoint string, payload map[string]interface{}, result interface{}) error {
	reqURL := BaseURL + endpoint

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+c.config.BotToken)
	req.Header.Set("User-Agent", "GortBot/1.0")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, result)
}

// parseResponse parses the Discord API response.
func (c *Channel) parseResponse(resp *http.Response, result interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}

	// Handle specific status codes
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		// Success
	case http.StatusNotFound:
		return ErrChannelNotFound
	case http.StatusForbidden:
		return ErrForbidden
	default:
		if resp.StatusCode >= 400 {
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// HandleWebhook handles incoming webhook requests from Discord.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	// Verify signature if public key is configured
	if c.config.PublicKey != "" {
		// Signature verification should be done at the HTTP handler level
		// This method assumes the request has already been verified
	}

	var event struct {
		Type int `json:"type"`
		Data struct {
			ID        string `json:"id"`
			ChannelID string `json:"channel_id"`
			GuildID   string `json:"guild_id"`
			Author    struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			} `json:"author"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
		} `json:"d"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook data: %w", err)
	}

	// Type 0 is Dispatch (regular message)
	if event.Type != 0 {
		return nil, nil
	}

	// Ignore bot messages
	if event.Data.Author.Bot {
		return nil, nil
	}

	msg := channel.NewMessage(
		event.Data.ID,
		c.Name(),
		channel.DirectionInbound,
		channel.UserInfo{
			ID:       event.Data.Author.ID,
			Name:     event.Data.Author.Username,
			Platform: "discord",
		},
		event.Data.Content,
		channel.MessageTypeText,
	)

	msg.To = channel.UserInfo{
		ID:       event.Data.ChannelID,
		Platform: "discord",
	}

	return msg, nil
}

// GetCapabilities returns the channel capabilities.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		MarkdownMessages: true,
		ImageMessages:    true,
		FileMessages:     true,
		AudioMessages:    true,
		VideoMessages:    true,
		ReactionMessages: true,
		MessageEditing:   true,
		MessageDeletion:  true,
		Threads:          true,
		LocationMessages: false,
		ReadReceipts:     false,
		TypingIndicators: false,
	}
}

// BuildEmbed builds a Discord embed.
func BuildEmbed(title, description, url string, color int) map[string]interface{} {
	embed := map[string]interface{}{
		"title":       title,
		"description": description,
		"color":       color,
	}

	if url != "" {
		embed["url"] = url
	}

	return embed
}

// BuildEmbedField builds an embed field.
func BuildEmbedField(name, value string, inline bool) map[string]interface{} {
	return map[string]interface{}{
		"name":   name,
		"value":  value,
		"inline": inline,
	}
}

// BuildEmbedFooter builds an embed footer.
func BuildEmbedFooter(text, iconURL string) map[string]interface{} {
	footer := map[string]interface{}{
		"text": text,
	}

	if iconURL != "" {
		footer["icon_url"] = iconURL
	}

	return footer
}

// urlEncodeEmoji URL encodes an emoji for use in API endpoints.
func urlEncodeEmoji(emoji string) string {
	if strings.Contains(emoji, ":") {
		return emoji
	}
	return url.PathEscape(emoji)
}
