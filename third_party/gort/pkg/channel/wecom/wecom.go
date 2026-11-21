// Package wecom provides a Channel adapter for WeCom (企业微信) webhook robots.
//
// Official API Documentation:
// - URL: https://developer.work.weixin.qq.com/document/path/91770
// - API Version: WeCom API v3
// - Last Updated: 2024
// - Authentication: Webhook URL with key parameter
//
// Supported Features:
// - Text messages (文本消息)
// - Markdown messages (Markdown消息)
// - Markdown v2 messages (Markdown_v2消息)
// - Image messages (图片消息)
// - News messages (图文消息)
// - File messages (文件消息)
// - Voice messages (语音消息)
// - Template card messages (模板卡片消息)
//
// Rate Limits:
// - 20 messages per minute per robot
// - Message length limits vary by type
//
// Security:
// - Webhook URL contains secret key parameter
// - IP whitelist can be configured in WeCom admin console
// - HTTPS required for all API calls
package wecom

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for WeCom API.
const (
	BaseURL = "https://qyapi.weixin.qq.com"

	EndpointWebhookSend = "/cgi-bin/webhook/send"
	EndpointMediaUpload = "/cgi-bin/media/upload"
)

// Error definitions for WeCom channel.
var (
	ErrWebhookURLRequired = errors.New("webhook_url is required")
	ErrInvalidWebhookURL  = errors.New("invalid webhook URL")
	ErrKeyRequired        = errors.New("webhook key is required")
	ErrMessageTooLong     = errors.New("message content exceeds maximum length")
	ErrInvalidMessageType = errors.New("invalid message type")
)

// Config contains the configuration for WeCom channel.
type Config struct {
	// WebhookURL is the full webhook URL including the key parameter
	// Example: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=693a91f6-7xxx-4bc4-97a0-0ec2sifa5aaa
	WebhookURL string

	// Key is the webhook key (extracted from WebhookURL if not provided)
	Key string

	// HTTPTimeout is the HTTP client timeout (default: 30s)
	HTTPTimeout time.Duration
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.WebhookURL == "" && c.Key == "" {
		return ErrWebhookURLRequired
	}

	if c.WebhookURL != "" {
		if _, err := url.Parse(c.WebhookURL); err != nil {
			return ErrInvalidWebhookURL
		}
	}

	return nil
}

// Channel implements the channel.Channel interface for WeCom.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	key        string
	mu         sync.RWMutex
}

// NewChannel creates a new WeCom channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Extract key from webhook URL if not provided
	key := config.Key
	if key == "" && config.WebhookURL != "" {
		parsedURL, err := url.Parse(config.WebhookURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse webhook URL: %w", err)
		}
		key = parsedURL.Query().Get("key")
		if key == "" {
			return nil, ErrKeyRequired
		}
	}

	timeout := config.HTTPTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeWeCom),
		config:      config,
		httpClient:  &http.Client{Timeout: timeout},
		key:         key,
	}, nil
}

// Start starts the WeCom channel.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	c.SetHandler(handler)
	c.SetStatus(channel.StatusRunning)
	return nil
}

// Stop stops the WeCom channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.SetStatus(channel.StatusStopped)
	c.SetHandler(nil)
	return nil
}

// SendMessage sends a message via WeCom webhook.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	if !c.IsRunning() {
		return errors.New("channel is not running")
	}

	payload, err := c.buildPayload(msg)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	return c.sendWebhookRequest(ctx, payload)
}

// buildPayload builds the webhook payload based on message type.
func (c *Channel) buildPayload(msg *channel.Message) (map[string]interface{}, error) {
	switch msg.Type {
	case channel.MessageTypeText:
		return c.buildTextPayload(msg)
	case channel.MessageTypeMarkdown:
		return c.buildMarkdownPayload(msg)
	case channel.MessageTypeImage:
		return c.buildImagePayload(msg)
	case channel.MessageTypeNews:
		return c.buildNewsPayload(msg)
	case channel.MessageTypeFile:
		return c.buildFilePayload(msg)
	case channel.MessageTypeVoice:
		return c.buildVoicePayload(msg)
	case channel.MessageTypeTemplateCard:
		return c.buildTemplateCardPayload(msg)
	default:
		// Default to text message
		return c.buildTextPayload(msg)
	}
}

// buildTextPayload builds a text message payload.
func (c *Channel) buildTextPayload(msg *channel.Message) (map[string]interface{}, error) {
	content := msg.Content
	if len(content) > 2048 {
		return nil, ErrMessageTooLong
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": content,
		},
	}

	// Add mentioned list if provided
	if mentionedList, ok := msg.GetMetadata("mentioned_list"); ok {
		if list, ok := mentionedList.([]string); ok {
			payload["text"].(map[string]interface{})["mentioned_list"] = list
		}
	}

	// Add mentioned mobile list if provided
	if mentionedMobileList, ok := msg.GetMetadata("mentioned_mobile_list"); ok {
		if list, ok := mentionedMobileList.([]string); ok {
			payload["text"].(map[string]interface{})["mentioned_mobile_list"] = list
		}
	}

	return payload, nil
}

// buildMarkdownPayload builds a markdown message payload.
func (c *Channel) buildMarkdownPayload(msg *channel.Message) (map[string]interface{}, error) {
	content := msg.Content
	if len(content) > 4096 {
		return nil, ErrMessageTooLong
	}

	// Check if markdown_v2 is requested
	if version, ok := msg.GetMetadata("markdown_version"); ok {
		if v, ok := version.(string); ok && v == "v2" {
			return map[string]interface{}{
				"msgtype": "markdown_v2",
				"markdown_v2": map[string]interface{}{
					"content": content,
				},
			}, nil
		}
	}

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": content,
		},
	}, nil
}

// buildImagePayload builds an image message payload.
func (c *Channel) buildImagePayload(msg *channel.Message) (map[string]interface{}, error) {
	// Get image data from metadata
	imageData, ok := msg.GetMetadata("image_data")
	if !ok {
		// Try to get image path
		imagePath, ok := msg.GetMetadata("image_path")
		if !ok {
			return nil, errors.New("image_data or image_path is required for image messages")
		}
		// Read image file
		data, err := c.readImageFile(imagePath.(string))
		if err != nil {
			return nil, fmt.Errorf("failed to read image file: %w", err)
		}
		imageData = data
	}

	var imageBytes []byte
	switch v := imageData.(type) {
	case []byte:
		imageBytes = v
	case string:
		// Assume base64 encoded
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 image: %w", err)
		}
		imageBytes = decoded
	default:
		return nil, errors.New("image_data must be []byte or base64 string")
	}

	// Calculate MD5
	md5Hash := md5.Sum(imageBytes)
	md5Str := hex.EncodeToString(md5Hash[:])

	// Base64 encode
	base64Data := base64.StdEncoding.EncodeToString(imageBytes)

	return map[string]interface{}{
		"msgtype": "image",
		"image": map[string]interface{}{
			"base64": base64Data,
			"md5":    md5Str,
		},
	}, nil
}

// buildNewsPayload builds a news (图文) message payload.
func (c *Channel) buildNewsPayload(msg *channel.Message) (map[string]interface{}, error) {
	articles := []map[string]interface{}{}

	// Get articles from metadata
	if articlesData, ok := msg.GetMetadata("articles"); ok {
		if data, ok := articlesData.([]map[string]interface{}); ok {
			for _, article := range data {
				articles = append(articles, article)
			}
		}
	} else {
		// Build single article from message
		title := msg.Content
		if t, ok := msg.GetMetadata("title"); ok {
			if titleStr, ok := t.(string); ok {
				title = titleStr
			}
		}

		description := ""
		if d, ok := msg.GetMetadata("description"); ok {
			if descStr, ok := d.(string); ok {
				description = descStr
			}
		}

		url := ""
		if u, ok := msg.GetMetadata("url"); ok {
			if urlStr, ok := u.(string); ok {
				url = urlStr
			}
		}

		picURL := ""
		if p, ok := msg.GetMetadata("picurl"); ok {
			if picStr, ok := p.(string); ok {
				picURL = picStr
			}
		}

		articles = append(articles, map[string]interface{}{
			"title":       title,
			"description": description,
			"url":         url,
			"picurl":      picURL,
		})
	}

	return map[string]interface{}{
		"msgtype": "news",
		"news": map[string]interface{}{
			"articles": articles,
		},
	}, nil
}

// buildFilePayload builds a file message payload.
func (c *Channel) buildFilePayload(msg *channel.Message) (map[string]interface{}, error) {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return nil, errors.New("media_id is required for file messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return nil, errors.New("media_id must be a string")
	}

	return map[string]interface{}{
		"msgtype": "file",
		"file": map[string]interface{}{
			"media_id": mediaIDStr,
		},
	}, nil
}

// buildVoicePayload builds a voice message payload.
func (c *Channel) buildVoicePayload(msg *channel.Message) (map[string]interface{}, error) {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return nil, errors.New("media_id is required for voice messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return nil, errors.New("media_id must be a string")
	}

	return map[string]interface{}{
		"msgtype": "voice",
		"voice": map[string]interface{}{
			"media_id": mediaIDStr,
		},
	}, nil
}

// buildTemplateCardPayload builds a template card message payload.
func (c *Channel) buildTemplateCardPayload(msg *channel.Message) (map[string]interface{}, error) {
	// Get template card data from metadata
	cardData, ok := msg.GetMetadata("template_card")
	if !ok {
		return nil, errors.New("template_card data is required for template card messages")
	}

	card, ok := cardData.(map[string]interface{})
	if !ok {
		return nil, errors.New("template_card must be a map")
	}

	return map[string]interface{}{
		"msgtype":        "template_card",
		"template_card":  card,
	}, nil
}

// sendWebhookRequest sends a request to the WeCom webhook.
func (c *Channel) sendWebhookRequest(ctx context.Context, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	webhookURL := c.config.WebhookURL
	if webhookURL == "" {
		webhookURL = fmt.Sprintf("%s%s?key=%s", BaseURL, EndpointWebhookSend, c.key)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("wecom API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// readImageFile reads an image file and returns its contents.
func (c *Channel) readImageFile(path string) ([]byte, error) {
	// This is a placeholder - in production, implement actual file reading
	return nil, errors.New("file reading not implemented")
}

// GetCapabilities returns the channel capabilities.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		MarkdownMessages: true,
		ImageMessages:    true,
		NewsMessages:     true,
		FileMessages:     true,
		VoiceMessages:    true,
		TemplateCard:     true,
		LocationMessages: false,
		ReadReceipts:     false,
		TypingIndicators: false,
	}
}

// HandleWebhook handles incoming webhook requests from WeCom.
// Note: WeCom webhook robots are outbound only, so this method returns an error.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	return nil, errors.New("wecom webhook robots do not support inbound messages")
}

// UploadMedia uploads a media file to WeCom and returns the media_id.
func (c *Channel) UploadMedia(ctx context.Context, mediaType string, fileData []byte, filename string) (string, error) {
	// This would require access token and more complex implementation
	// For webhook robots, media should be uploaded via the WeCom admin console
	return "", errors.New("media upload not supported for webhook robots")
}
