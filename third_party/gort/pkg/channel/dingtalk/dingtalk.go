// Package dingtalk provides a Channel adapter for DingTalk (钉钉) robots.
//
// Official API Documentation:
// - URL: https://open.dingtalk.com/document/orgapp/overview-of-group-robots
// - API Version: 钉钉开放平台 API v2
// - Last Updated: 2024
// - Authentication: AppKey + AppSecret / Webhook URL with signature
//
// Supported Features:
// - Text messages (文本消息)
// - Markdown messages (Markdown消息)
// - Link messages (链接消息)
// - ActionCard messages (卡片消息)
// - Image messages (图片消息)
// - Voice messages (语音消息)
// - File messages (文件消息)
//
// Rate Limits:
// - Group robot: 20 messages per minute
// - Single chat: Requires user to initiate conversation first
//
// Security:
// - Webhook signature verification using HMAC-SHA256
// - IP whitelist can be configured
package dingtalk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for DingTalk API.
const (
	BaseURL = "https://oapi.dingtalk.com"

	EndpointRobotSend = "/robot/send"
	EndpointToken      = "/gettoken"
	EndpointUpload     = "/media/upload"
)

// Error definitions for DingTalk channel.
var (
	ErrWebhookURLRequired = errors.New("webhook_url is required")
	ErrInvalidWebhookURL  = errors.New("invalid webhook URL")
	ErrSignSecretRequired = errors.New("sign_secret is required when signature is enabled")
)

// Config contains the configuration for DingTalk channel.
type Config struct {
	WebhookURL string
	SignSecret string
	AppKey     string
	AppSecret  string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.WebhookURL == "" {
		return ErrWebhookURLRequired
	}

	if _, err := url.Parse(c.WebhookURL); err != nil {
		return ErrInvalidWebhookURL
	}

	return nil
}

// Channel implements the channel.Channel interface for DingTalk.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	cancel     context.CancelFunc
}

// NewChannel creates a new DingTalk channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeDingTalk),
		config:      config,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Start initializes the channel.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	if c.IsRunning() {
		return channel.ErrChannelAlreadyRunning
	}

	c.SetHandler(handler)
	c.SetStatus(channel.StatusRunning)
	return nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	c.SetStatus(channel.StatusStopped)
	return nil
}

// SendMessage sends a message to DingTalk.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	switch msg.Type {
	case channel.MessageTypeText:
		return c.sendTextMessage(ctx, msg)
	case channel.MessageTypeImage:
		return c.sendImageMessage(ctx, msg)
	case channel.MessageTypeFile:
		return c.sendFileMessage(ctx, msg)
	case channel.MessageTypeAudio:
		return c.sendVoiceMessage(ctx, msg)
	default:
		return c.sendMarkdownMessage(ctx, msg)
	}
}

// HandleWebhook processes incoming webhook requests from DingTalk.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	var req WebhookRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse webhook data: %w", err)
	}

	msg := &channel.Message{
		ID:        req.MsgID,
		ChannelID: "dingtalk",
		Direction: channel.DirectionInbound,
		From: channel.UserInfo{
			ID:       req.SenderID,
			Name:     req.SenderNick,
			Platform: "dingtalk",
		},
		Timestamp: fmt.Sprintf("%d", req.Timestamp),
	}

	switch req.MsgType {
	case "text":
		msg.Type = channel.MessageTypeText
		msg.Content = req.Content.Content
	case "picture":
		msg.Type = channel.MessageTypeImage
		msg.Content = req.Content.PicURL
	case "richText":
		msg.Type = channel.MessageTypeText
		msg.Content = req.Content.RichTextContent
	default:
		msg.Type = channel.MessageTypeEvent
		msg.SetMetadata("msg_type", req.MsgType)
	}

	return msg, nil
}

// WebhookRequest represents an incoming webhook request from DingTalk.
type WebhookRequest struct {
	MsgType    string `json:"msgtype"`
	MsgID      string `json:"msgId"`
	SenderID   string `json:"senderId"`
	SenderNick string `json:"senderNick"`
	Timestamp  int64  `json:"createAt"`
	Content    struct {
		Content         string `json:"content"`
		PicURL          string `json:"picURL"`
		RichTextContent string `json:"richTextContent"`
	} `json:"content"`
}

// TextMessage represents a text message to send.
type TextMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
		At      *AtInfo `json:"at,omitempty"`
	} `json:"text"`
}

// MarkdownMessage represents a markdown message to send.
type MarkdownMessage struct {
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		At    *AtInfo `json:"at,omitempty"`
	} `json:"markdown"`
}

// LinkMessage represents a link message to send.
type LinkMessage struct {
	MsgType string `json:"msgtype"`
	Link    struct {
		Title      string `json:"title"`
		Text       string `json:"text"`
		MessageURL string `json:"messageUrl"`
		PicURL     string `json:"picUrl,omitempty"`
	} `json:"link"`
}

// ActionCardMessage represents an action card message to send.
type ActionCardMessage struct {
	MsgType    string `json:"msgtype"`
	ActionCard struct {
		Title          string `json:"title"`
		Text           string `json:"text"`
		SingleTitle    string `json:"singleTitle,omitempty"`
		SingleURL      string `json:"singleURL,omitempty"`
		BtnOrientation string `json:"btnOrientation,omitempty"`
		Btns           []ActionButton `json:"btns,omitempty"`
	} `json:"actionCard"`
}

// ActionButton represents a button in action card.
type ActionButton struct {
	Title     string `json:"title"`
	ActionURL string `json:"actionURL"`
}

// ImageMessage represents an image message to send.
type ImageMessage struct {
	MsgType string `json:"msgtype"`
	Image   struct {
		MediaID string `json:"media_id"`
	} `json:"image"`
}

// VoiceMessage represents a voice message to send.
type VoiceMessage struct {
	MsgType string `json:"msgtype"`
	Voice   struct {
		MediaID  string `json:"media_id"`
		Duration int    `json:"duration,omitempty"`
	} `json:"voice"`
}

// FileMessage represents a file message to send.
type FileMessage struct {
	MsgType string `json:"msgtype"`
	File    struct {
		MediaID string `json:"media_id"`
	} `json:"file"`
}

// AtInfo represents @ information.
type AtInfo struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll,omitempty"`
}

// APIResponse represents a generic API response.
type APIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (c *Channel) sendTextMessage(ctx context.Context, msg *channel.Message) error {
	textMsg := TextMessage{
		MsgType: "text",
	}
	textMsg.Text.Content = msg.Content

	if atMobiles, ok := msg.GetMetadata("at_mobiles"); ok {
		if atMobilesSlice, ok := atMobiles.([]string); ok {
			if textMsg.Text.At == nil {
				textMsg.Text.At = &AtInfo{}
			}
			textMsg.Text.At.AtMobiles = atMobilesSlice
		}
	}

	if atUserIDs, ok := msg.GetMetadata("at_user_ids"); ok {
		if atUserIDsSlice, ok := atUserIDs.([]string); ok {
			if textMsg.Text.At == nil {
				textMsg.Text.At = &AtInfo{}
			}
			textMsg.Text.At.AtUserIds = atUserIDsSlice
		}
	}

	if isAtAll, ok := msg.GetMetadata("is_at_all"); ok {
		if isAtAllBool, ok := isAtAll.(bool); ok {
			if textMsg.Text.At == nil {
				textMsg.Text.At = &AtInfo{}
			}
			textMsg.Text.At.IsAtAll = isAtAllBool
		}
	}

	return c.sendAPIRequest(ctx, textMsg)
}

func (c *Channel) sendMarkdownMessage(ctx context.Context, msg *channel.Message) error {
	mdMsg := MarkdownMessage{
		MsgType: "markdown",
	}

	title, ok := msg.GetMetadata("title")
	if ok {
		if titleStr, ok := title.(string); ok {
			mdMsg.Markdown.Title = titleStr
		} else {
			mdMsg.Markdown.Title = msg.Content[:min(50, len(msg.Content))]
		}
	} else {
		mdMsg.Markdown.Title = msg.Content[:min(50, len(msg.Content))]
	}
	mdMsg.Markdown.Text = msg.Content

	return c.sendAPIRequest(ctx, mdMsg)
}

func (c *Channel) sendImageMessage(ctx context.Context, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for image messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	imageMsg := ImageMessage{
		MsgType: "image",
	}
	imageMsg.Image.MediaID = mediaIDStr

	return c.sendAPIRequest(ctx, imageMsg)
}

func (c *Channel) sendVoiceMessage(ctx context.Context, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for voice messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	voiceMsg := VoiceMessage{
		MsgType: "voice",
	}
	voiceMsg.Voice.MediaID = mediaIDStr

	if duration, ok := msg.GetMetadata("duration"); ok {
		if durationInt, ok := duration.(int); ok {
			voiceMsg.Voice.Duration = durationInt
		}
	}

	return c.sendAPIRequest(ctx, voiceMsg)
}

func (c *Channel) sendFileMessage(ctx context.Context, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for file messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	fileMsg := FileMessage{
		MsgType: "file",
	}
	fileMsg.File.MediaID = mediaIDStr

	return c.sendAPIRequest(ctx, fileMsg)
}

func (c *Channel) sendAPIRequest(ctx context.Context, payload interface{}) error {
	webhookURL := c.config.WebhookURL

	if c.config.SignSecret != "" {
		timestamp := time.Now().UnixMilli()
		sign := c.generateSignature(timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, sign)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return err
	}

	if apiResp.ErrCode != 0 {
		return fmt.Errorf("dingtalk API error: %d - %s", apiResp.ErrCode, apiResp.ErrMsg)
	}

	return nil
}

func (c *Channel) generateSignature(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, c.config.SignSecret)

	h := hmac.New(sha256.New, []byte(c.config.SignSecret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return url.QueryEscape(signature)
}

// GetCapabilities returns the capabilities of the DingTalk channel.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		ImageMessages:    true,
		FileMessages:     true,
		AudioMessages:    true,
		VideoMessages:    false,
		TemplateMessages: false,
		ReadReceipts:     false,
		TypingIndicators: false,
		MessageEditing:   false,
		MessageDeletion:  false,
		ReactionMessages: false,
	}
}
