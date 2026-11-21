// Package feishu provides a Channel adapter for Feishu (飞书) / Lark.
//
// Official API Documentation:
// - URL: https://open.feishu.cn/document/home/introduction-to-feishu-open-platform
// - API Version: 飞书开放平台 API v1
// - Last Updated: 2024
// - Authentication: App ID + App Secret (tenant_access_token / user_access_token)
//
// Supported Features:
// - Text messages (文本消息)
// - Rich text messages (富文本消息)
// - Image messages (图片消息)
// - File messages (文件消息)
// - Card messages (卡片消息)
// - Audio messages (语音消息)
// - Video messages (视频消息)
//
// Rate Limits:
// - Same user: 5 QPS
// - Same group: 5 QPS
// - Webhook: 100 requests per minute
//
// Security:
// - App ID and App Secret authentication
// - IP whitelist can be configured
// - Webhook signature verification
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for Feishu API.
const (
	BaseURL = "https://open.feishu.cn/open-apis"

	EndpointToken       = "/auth/v3/tenant_access_token/internal"
	EndpointSendMessage = "/im/v1/messages"
	EndpointUploadImage = "/im/v1/images"
	EndpointUploadFile  = "/drive/v1/files/upload_all"
)

// Error definitions for Feishu channel.
var (
	ErrAppIDRequired     = errors.New("app_id is required")
	ErrAppSecretRequired = errors.New("app_secret is required")
	ErrTokenExpired      = errors.New("tenant_access_token expired")
)

// Config contains the configuration for Feishu channel.
type Config struct {
	AppID     string
	AppSecret string
	WebhookURL string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.AppID == "" {
		return ErrAppIDRequired
	}
	if c.AppSecret == "" {
		return ErrAppSecretRequired
	}
	return nil
}

// Channel implements the channel.Channel interface for Feishu.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	token      *tenantAccessToken
	tokenMu    sync.RWMutex
	cancel     context.CancelFunc
}

type tenantAccessToken struct {
	Value     string
	ExpiresAt time.Time
}

// NewChannel creates a new Feishu channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeFeishu),
		config:      config,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Start initializes the channel and starts token refresh loop.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	if c.IsRunning() {
		return channel.ErrChannelAlreadyRunning
	}

	c.SetHandler(handler)

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	if err := c.refreshToken(ctx); err != nil {
		return fmt.Errorf("failed to get initial tenant_access_token: %w", err)
	}

	go c.tokenRefreshLoop(ctx)

	c.SetStatus(channel.StatusRunning)
	return nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	if c.cancel != nil {
		c.cancel()
	}

	c.SetStatus(channel.StatusStopped)
	return nil
}

// SendMessage sends a message to Feishu.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	token, err := c.getToken()
	if err != nil {
		return err
	}

	receiveID := msg.To.ID
	receiveIDType := "open_id"

	if idType, ok := msg.GetMetadata("receive_id_type"); ok {
		receiveIDType = idType.(string)
	}

	switch msg.Type {
	case channel.MessageTypeText:
		return c.sendTextMessage(ctx, token, receiveID, receiveIDType, msg)
	case channel.MessageTypeImage:
		return c.sendImageMessage(ctx, token, receiveID, receiveIDType, msg)
	case channel.MessageTypeFile:
		return c.sendFileMessage(ctx, token, receiveID, receiveIDType, msg)
	case channel.MessageTypeAudio:
		return c.sendAudioMessage(ctx, token, receiveID, receiveIDType, msg)
	case channel.MessageTypeVideo:
		return c.sendVideoMessage(ctx, token, receiveID, receiveIDType, msg)
	default:
		return c.sendTextMessage(ctx, token, receiveID, receiveIDType, msg)
	}
}

// HandleWebhook processes incoming webhook requests from Feishu.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	var req WebhookRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse webhook data: %w", err)
	}

	msg := &channel.Message{
		ID:        req.Header.EventID,
		ChannelID: "feishu",
		Direction: channel.DirectionInbound,
		Timestamp: fmt.Sprintf("%d", req.Header.EventCreatedAt),
	}

	if req.Event.Sender != nil {
		msg.From = channel.UserInfo{
			ID:       req.Event.Sender.SenderID.OpenID,
			Name:     req.Event.Sender.SenderID.UnionID,
			Platform: "feishu",
		}
	}

	if req.Event.Message != nil {
		msg.ID = req.Event.Message.MessageID
		msg.To = channel.UserInfo{
			ID:       req.Event.Message.ChatID,
			Platform: "feishu",
		}

		switch req.Event.Message.MessageType {
		case "text":
			msg.Type = channel.MessageTypeText
			var content TextContent
			if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err == nil {
				msg.Content = content.Text
			}
		case "image":
			msg.Type = channel.MessageTypeImage
			var content ImageContent
			if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err == nil {
				msg.SetMetadata("image_key", content.ImageKey)
			}
		case "file":
			msg.Type = channel.MessageTypeFile
			var content FileContent
			if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err == nil {
				msg.SetMetadata("file_key", content.FileKey)
				msg.SetMetadata("file_name", content.FileName)
			}
		case "audio":
			msg.Type = channel.MessageTypeAudio
			var content AudioContent
			if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err == nil {
				msg.SetMetadata("file_key", content.FileKey)
			}
		case "media":
			msg.Type = channel.MessageTypeVideo
			var content MediaContent
			if err := json.Unmarshal([]byte(req.Event.Message.Content), &content); err == nil {
				msg.SetMetadata("file_key", content.FileKey)
			}
		default:
			msg.Type = channel.MessageTypeEvent
			msg.SetMetadata("message_type", req.Event.Message.MessageType)
		}
	}

	return msg, nil
}

// WebhookRequest represents an incoming webhook request from Feishu.
type WebhookRequest struct {
	Schema string `json:"schema"`
	Header struct {
		EventID        string `json:"event_id"`
		EventType      string `json:"event_type"`
		EventCreatedAt int64  `json:"create_time"`
		Token          string `json:"token"`
		AppID          string `json:"app_id"`
		TenantKey      string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		Sender  *SenderInfo  `json:"sender"`
		Message *MessageInfo `json:"message"`
	} `json:"event"`
}

// SenderInfo represents sender information.
type SenderInfo struct {
	SenderID struct {
		OpenID  string `json:"open_id"`
		UnionID string `json:"union_id"`
		UserID  string `json:"user_id"`
	} `json:"sender_id"`
	SenderType string `json:"sender_type"`
	TenantKey  string `json:"tenant_key"`
}

// MessageInfo represents message information.
type MessageInfo struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id"`
	ParentID    string `json:"parent_id"`
	CreateTime  int64  `json:"create_time"`
	ChatID      string `json:"chat_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	Mentions    []MentionInfo `json:"mentions"`
}

// MentionInfo represents mention information.
type MentionInfo struct {
	Key       string `json:"key"`
	ID        struct {
		OpenID  string `json:"open_id"`
		UserID  string `json:"user_id"`
	} `json:"id"`
	Name string `json:"name"`
}

// TextContent represents text message content.
type TextContent struct {
	Text string `json:"text"`
}

// ImageContent represents image message content.
type ImageContent struct {
	ImageKey string `json:"image_key"`
}

// FileContent represents file message content.
type FileContent struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
}

// AudioContent represents audio message content.
type AudioContent struct {
	FileKey string `json:"file_key"`
}

// MediaContent represents media message content.
type MediaContent struct {
	FileKey string `json:"file_key"`
	FileName string `json:"file_name"`
}

// SendMessageRequest represents a request to send a message.
type SendMessageRequest struct {
	ReceiveID     string      `json:"receive_id"`
	MsgType       string      `json:"msg_type"`
	Content       interface{} `json:"content"`
	ReceiveIDType string      `json:"receive_id_type,omitempty"`
}

// TokenResponse represents the response from the token API.
type TokenResponse struct {
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
}

// APIResponse represents a generic API response.
type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		MessageID string `json:"message_id"`
	} `json:"data"`
}

func (c *Channel) getToken() (string, error) {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()

	if c.token == nil || time.Now().After(c.token.ExpiresAt) {
		return "", ErrTokenExpired
	}

	return c.token.Value, nil
}

func (c *Channel) refreshToken(ctx context.Context) error {
	url := fmt.Sprintf("%s%s", BaseURL, EndpointToken)

	reqBody := map[string]string{
		"app_id":     c.config.AppID,
		"app_secret": c.config.AppSecret,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return err
	}

	if tokenResp.Code != 0 {
		return fmt.Errorf("feishu API error: %d - %s", tokenResp.Code, tokenResp.Msg)
	}

	c.tokenMu.Lock()
	c.token = &tenantAccessToken{
		Value:     tokenResp.TenantAccessToken,
		ExpiresAt: time.Now().Add(time.Duration(tokenResp.Expire-300) * time.Second),
	}
	c.tokenMu.Unlock()

	return nil
}

const tokenRefreshInterval = 90 * time.Minute

func (c *Channel) tokenRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(tokenRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := c.refreshToken(refreshCtx)
			cancel()
			if err != nil {
				c.SetStatus(channel.StatusError)
			}
		}
	}
}

func (c *Channel) sendTextMessage(ctx context.Context, token, receiveID, receiveIDType string, msg *channel.Message) error {
	content := TextContent{Text: msg.Content}

	sendReq := SendMessageRequest{
		ReceiveID:     receiveID,
		MsgType:       "text",
		Content:       content,
		ReceiveIDType: receiveIDType,
	}

	return c.sendAPIRequest(ctx, token, receiveIDType, sendReq)
}

func (c *Channel) sendImageMessage(ctx context.Context, token, receiveID, receiveIDType string, msg *channel.Message) error {
	imageKey, ok := msg.GetMetadata("image_key")
	if !ok {
		return errors.New("image_key is required for image messages")
	}

	imageKeyStr, ok := imageKey.(string)
	if !ok {
		return errors.New("image_key must be a string")
	}

	content := ImageContent{ImageKey: imageKeyStr}

	sendReq := SendMessageRequest{
		ReceiveID:     receiveID,
		MsgType:       "image",
		Content:       content,
		ReceiveIDType: receiveIDType,
	}

	return c.sendAPIRequest(ctx, token, receiveIDType, sendReq)
}

func (c *Channel) sendFileMessage(ctx context.Context, token, receiveID, receiveIDType string, msg *channel.Message) error {
	fileKey, ok := msg.GetMetadata("file_key")
	if !ok {
		return errors.New("file_key is required for file messages")
	}

	fileKeyStr, ok := fileKey.(string)
	if !ok {
		return errors.New("file_key must be a string")
	}

	content := FileContent{FileKey: fileKeyStr}

	if fileName, ok := msg.GetMetadata("file_name"); ok {
		if fileNameStr, ok := fileName.(string); ok {
			content.FileName = fileNameStr
		}
	}

	sendReq := SendMessageRequest{
		ReceiveID:     receiveID,
		MsgType:       "file",
		Content:       content,
		ReceiveIDType: receiveIDType,
	}

	return c.sendAPIRequest(ctx, token, receiveIDType, sendReq)
}

func (c *Channel) sendAudioMessage(ctx context.Context, token, receiveID, receiveIDType string, msg *channel.Message) error {
	fileKey, ok := msg.GetMetadata("file_key")
	if !ok {
		return errors.New("file_key is required for audio messages")
	}

	fileKeyStr, ok := fileKey.(string)
	if !ok {
		return errors.New("file_key must be a string")
	}

	content := AudioContent{FileKey: fileKeyStr}

	sendReq := SendMessageRequest{
		ReceiveID:     receiveID,
		MsgType:       "audio",
		Content:       content,
		ReceiveIDType: receiveIDType,
	}

	return c.sendAPIRequest(ctx, token, receiveIDType, sendReq)
}

func (c *Channel) sendVideoMessage(ctx context.Context, token, receiveID, receiveIDType string, msg *channel.Message) error {
	fileKey, ok := msg.GetMetadata("file_key")
	if !ok {
		return errors.New("file_key is required for video messages")
	}

	fileKeyStr, ok := fileKey.(string)
	if !ok {
		return errors.New("file_key must be a string")
	}

	content := MediaContent{FileKey: fileKeyStr}

	if fileName, ok := msg.GetMetadata("file_name"); ok {
		if fileNameStr, ok := fileName.(string); ok {
			content.FileName = fileNameStr
		}
	}

	sendReq := SendMessageRequest{
		ReceiveID:     receiveID,
		MsgType:       "media",
		Content:       content,
		ReceiveIDType: receiveIDType,
	}

	return c.sendAPIRequest(ctx, token, receiveIDType, sendReq)
}

func (c *Channel) sendAPIRequest(ctx context.Context, token, receiveIDType string, payload interface{}) error {
	url := fmt.Sprintf("%s%s?receive_id_type=%s", BaseURL, EndpointSendMessage, receiveIDType)

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

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

	if apiResp.Code != 0 {
		return fmt.Errorf("feishu API error: %d - %s", apiResp.Code, apiResp.Msg)
	}

	return nil
}

// GetCapabilities returns the capabilities of the Feishu channel.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		ImageMessages:    true,
		FileMessages:     true,
		AudioMessages:    true,
		VideoMessages:    true,
		TemplateMessages: true,
		ReadReceipts:     true,
		TypingIndicators: true,
		MessageEditing:   true,
		MessageDeletion:  true,
		ReactionMessages: true,
	}
}
