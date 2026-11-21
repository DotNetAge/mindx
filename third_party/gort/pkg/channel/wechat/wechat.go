// Package wechat provides a Channel adapter for WeChat Official Accounts (公众号).
//
// Official API Documentation:
// - URL: https://developers.weixin.qq.com/doc/offiaccount/Getting_Started/Overview.html
// - API Version: 公众平台 API (服务端 API)
// - Last Updated: 2024
// - Authentication: access_token (AppID + AppSecret)
//
// Supported Features:
// - Text messages (文本消息)
// - Image messages (图片消息)
// - Voice messages (语音消息)
// - Video messages (视频消息)
// - File messages (文件消息)
// - Template messages (模板消息)
//
// Rate Limits:
// - access_token: Valid for 2 hours, refresh recommended every 1.5 hours
// - Template messages: Require user interaction within 48 hours
// - Customer service messages: 48-hour window after user interaction
//
// Security Requirements:
// - IP whitelist must be configured in WeChat Developer Console
// - Server URL must use HTTPS
// - Message signature verification required for webhooks
package wechat

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for WeChat Official Account API.
const (
	BaseURL = "https://api.weixin.qq.com/cgi-bin"

	EndpointToken        = "/token"
	EndpointSendMessage  = "/message/custom/send"
	EndpointSendTemplate = "/message/template/send"
	EndpointUploadMedia  = "/media/upload"
	EndpointGetMedia     = "/media/get"
)

// Error definitions for WeChat channel.
var (
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrTokenExpired         = errors.New("access token expired")
	ErrAppIDRequired        = errors.New("app_id is required")
	ErrAppSecretRequired    = errors.New("app_secret is required")
	ErrTokenRequired        = errors.New("token is required for webhook verification")
	ErrUserNotSubscribed    = errors.New("user not subscribed")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrMessageOutsideWindow = errors.New("message outside 48-hour window")
)

// Config contains the configuration for WeChat channel.
type Config struct {
	AppID     string
	AppSecret string
	Token     string
	AESKey    string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.AppID == "" {
		return ErrAppIDRequired
	}
	if c.AppSecret == "" {
		return ErrAppSecretRequired
	}
	if c.Token == "" {
		return ErrTokenRequired
	}
	return nil
}

// Channel implements the channel.Channel interface for WeChat.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	token      *accessToken
	tokenMu    sync.RWMutex
	cancel     context.CancelFunc
}

type accessToken struct {
	Value     string
	ExpiresAt time.Time
}

// NewChannel creates a new WeChat channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeWeChat),
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
		return fmt.Errorf("failed to get initial access token: %w", err)
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

// SendMessage sends a message to a WeChat user.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	token, err := c.getToken()
	if err != nil {
		return err
	}

	switch msg.Type {
	case channel.MessageTypeText:
		return c.sendTextMessage(ctx, token, msg)
	case channel.MessageTypeImage:
		return c.sendImageMessage(ctx, token, msg)
	case channel.MessageTypeFile:
		return c.sendFileMessage(ctx, token, msg)
	case channel.MessageTypeAudio:
		return c.sendVoiceMessage(ctx, token, msg)
	case channel.MessageTypeVideo:
		return c.sendVideoMessage(ctx, token, msg)
	default:
		return channel.ErrUnsupportedMessageType
	}
}

// HandleWebhook processes incoming webhook requests from WeChat.
// This implements the WebhookHandler interface.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	var req WebhookRequest
	if err := xml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse webhook data: %w", err)
	}

	msg := &channel.Message{
		ID:        req.MsgID,
		ChannelID: "wechat",
		Direction: channel.DirectionInbound,
		From: channel.UserInfo{
			ID:       req.FromUserName,
			Platform: "wechat",
		},
		To: channel.UserInfo{
			ID:       req.ToUserName,
			Platform: "wechat",
		},
		Timestamp: fmt.Sprintf("%d", req.CreateTime),
	}

	switch req.MsgType {
	case "text":
		msg.Type = channel.MessageTypeText
		msg.Content = req.Content
	case "image":
		msg.Type = channel.MessageTypeImage
		msg.Content = req.PicURL
		msg.SetMetadata("media_id", req.MediaID)
	case "voice":
		msg.Type = channel.MessageTypeAudio
		msg.SetMetadata("media_id", req.MediaID)
		msg.SetMetadata("format", req.Format)
	case "video":
		msg.Type = channel.MessageTypeVideo
		msg.SetMetadata("media_id", req.MediaID)
		msg.SetMetadata("thumb_media_id", req.ThumbMediaID)
	default:
		msg.Type = channel.MessageTypeEvent
		msg.SetMetadata("event_type", req.MsgType)
	}

	return msg, nil
}

// VerifySignature verifies the webhook signature from WeChat.
func (c *Channel) VerifySignature(signature, timestamp, nonce string) bool {
	arr := []string{c.config.Token, timestamp, nonce}
	sort.Strings(arr)
	combined := strings.Join(arr, "")

	h := sha1.New()
	h.Write([]byte(combined))
	calculated := hex.EncodeToString(h.Sum(nil))

	return calculated == signature
}

// WebhookRequest represents an incoming webhook request from WeChat.
type WebhookRequest struct {
	ToUserName   string  `xml:"ToUserName"`
	FromUserName string  `xml:"FromUserName"`
	CreateTime   int64   `xml:"CreateTime"`
	MsgType      string  `xml:"MsgType"`
	Content      string  `xml:"Content"`
	MsgID        string  `xml:"MsgId"`
	PicURL       string  `xml:"PicUrl"`
	MediaID      string  `xml:"MediaId"`
	Format       string  `xml:"Format"`
	ThumbMediaID string  `xml:"ThumbMediaId"`
	Location_X   float64 `xml:"Location_X"`
	Location_Y   float64 `xml:"Location_Y"`
	Scale        int     `xml:"Scale"`
	Label        string  `xml:"Label"`
	Event        string  `xml:"Event"`
	EventKey     string  `xml:"EventKey"`
}

// TextMessage represents a text message to send.
type TextMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

// ImageMessage represents an image message to send.
type ImageMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Image   struct {
		MediaID string `json:"media_id"`
	} `json:"image"`
}

// VoiceMessage represents a voice message to send.
type VoiceMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Voice   struct {
		MediaID string `json:"media_id"`
	} `json:"voice"`
}

// VideoMessage represents a video message to send.
type VideoMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Video   struct {
		MediaID      string `json:"media_id"`
		ThumbMediaID string `json:"thumb_media_id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
	} `json:"video"`
}

// FileMessage represents a file message to send.
type FileMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	File    struct {
		MediaID string `json:"media_id"`
	} `json:"file"`
}

// APIResponse represents a generic API response.
type APIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// TokenResponse represents the response from the token API.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
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
	url := BaseURL + EndpointToken
	payload := map[string]string{
		"grant_type": "client_credential",
		"appid":      c.config.AppID,
		"secret":     c.config.AppSecret,
	}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	if tokenResp.ErrCode != 0 {
		return fmt.Errorf("wechat API error: %d - %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	c.tokenMu.Lock()
	c.token = &accessToken{
		Value:     tokenResp.AccessToken,
		ExpiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second),
	}
	c.tokenMu.Unlock()

	return nil
}

func (c *Channel) tokenRefreshLoop(ctx context.Context) {
	const tokenRefreshInterval = 90 * time.Minute
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

func (c *Channel) sendTextMessage(ctx context.Context, token string, msg *channel.Message) error {
	textMsg := TextMessage{
		ToUser:  msg.To.ID,
		MsgType: "text",
	}
	textMsg.Text.Content = msg.Content

	return c.sendAPIRequest(ctx, token, EndpointSendMessage, textMsg)
}

func (c *Channel) sendImageMessage(ctx context.Context, token string, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for image messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	imageMsg := ImageMessage{
		ToUser:  msg.To.ID,
		MsgType: "image",
	}
	imageMsg.Image.MediaID = mediaIDStr

	return c.sendAPIRequest(ctx, token, EndpointSendMessage, imageMsg)
}

func (c *Channel) sendVoiceMessage(ctx context.Context, token string, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for voice messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	voiceMsg := VoiceMessage{
		ToUser:  msg.To.ID,
		MsgType: "voice",
	}
	voiceMsg.Voice.MediaID = mediaIDStr

	return c.sendAPIRequest(ctx, token, EndpointSendMessage, voiceMsg)
}

func (c *Channel) sendVideoMessage(ctx context.Context, token string, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for video messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	thumbMediaID, _ := msg.GetMetadata("thumb_media_id")
	title, _ := msg.GetMetadata("title")
	description, _ := msg.GetMetadata("description")

	videoMsg := VideoMessage{
		ToUser:  msg.To.ID,
		MsgType: "video",
	}
	videoMsg.Video.MediaID = mediaIDStr
	if thumbMediaID != nil {
		if s, ok := thumbMediaID.(string); ok {
			videoMsg.Video.ThumbMediaID = s
		}
	}
	if title != nil {
		if s, ok := title.(string); ok {
			videoMsg.Video.Title = s
		}
	}
	if description != nil {
		if s, ok := description.(string); ok {
			videoMsg.Video.Description = s
		}
	}

	return c.sendAPIRequest(ctx, token, EndpointSendMessage, videoMsg)
}

func (c *Channel) sendFileMessage(ctx context.Context, token string, msg *channel.Message) error {
	mediaID, ok := msg.GetMetadata("media_id")
	if !ok {
		return errors.New("media_id is required for file messages")
	}

	mediaIDStr, ok := mediaID.(string)
	if !ok {
		return errors.New("media_id must be a string")
	}

	fileMsg := FileMessage{
		ToUser:  msg.To.ID,
		MsgType: "file",
	}
	fileMsg.File.MediaID = mediaIDStr

	return c.sendAPIRequest(ctx, token, EndpointSendMessage, fileMsg)
}

func (c *Channel) sendAPIRequest(ctx context.Context, token, endpoint string, payload interface{}) error {
	url := fmt.Sprintf("%s%s?access_token=%s", BaseURL, endpoint, token)

	body, err := json.Marshal(payload)
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

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return err
	}

	if apiResp.ErrCode != 0 {
		return fmt.Errorf("wechat API error: %d - %s", apiResp.ErrCode, apiResp.ErrMsg)
	}

	return nil
}

// GetCapabilities returns the capabilities of the WeChat channel.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		ImageMessages:    true,
		FileMessages:     true,
		AudioMessages:    true,
		VideoMessages:    true,
		TemplateMessages: true,
		ReadReceipts:     false,
		TypingIndicators: false,
		MessageEditing:   false,
		MessageDeletion:  false,
		ReactionMessages: false,
	}
}
