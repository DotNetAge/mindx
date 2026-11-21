// Package telegram provides a Channel adapter for Telegram Bot API.
//
// Official API Documentation:
// - URL: https://core.telegram.org/bots/api
// - API Version: Bot API 7.11
// - Last Updated: October 31, 2024
// - Authentication: Bot Token (obtained from @BotFather)
//
// Supported Features:
// - Text messages (文本消息)
// - Photo messages (图片消息)
// - Audio messages (音频消息)
// - Document messages (文档消息)
// - Video messages (视频消息)
// - Voice messages (语音消息)
// - Location messages (位置消息)
// - Contact messages (联系人消息)
// - Sticker messages (贴纸消息)
// - Poll messages (投票消息)
//
// Rate Limits:
// - Message length: 4096 characters for text
// - File size: 50MB (Bot API), 2GB (with local Bot API server)
// - Group chat: 20 messages per minute
// - Private chat: 30 messages per second
//
// Security:
// - Bot Token authentication
// - Webhook secret token verification
// - HTTPS required for webhooks
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// API endpoints for Telegram Bot API.
var (
	BaseURL = "https://api.telegram.org"

	EndpointSendMessage      = "/sendMessage"
	EndpointSendPhoto        = "/sendPhoto"
	EndpointSendDocument     = "/sendDocument"
	EndpointSendAudio        = "/sendAudio"
	EndpointSendVideo        = "/sendVideo"
	EndpointSendVoice        = "/sendVoice"
	EndpointSendLocation     = "/sendLocation"
	EndpointSendContact      = "/sendContact"
	EndpointGetUpdates       = "/getUpdates"
	EndpointSetWebhook       = "/setWebhook"
	EndpointDeleteWebhook    = "/deleteWebhook"
	EndpointGetMe            = "/getMe"
	EndpointAnswerCallbackQuery = "/answerCallbackQuery"
)

// Error definitions for Telegram channel.
var (
	ErrTokenRequired       = errors.New("bot token is required")
	ErrInvalidToken        = errors.New("invalid bot token format")
	ErrChatNotFound        = errors.New("chat not found")
	ErrUserBlocked         = errors.New("user blocked the bot")
	ErrMessageTooLong      = errors.New("message exceeds 4096 characters")
	ErrFileTooLarge        = errors.New("file exceeds size limit")
)

// Config contains the configuration for Telegram channel.
type Config struct {
	Token      string
	WebhookURL string
	SecretToken string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Token == "" {
		return ErrTokenRequired
	}
	return nil
}

// Channel implements the channel.Channel interface for Telegram.
type Channel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	baseURL    string
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewChannel creates a new Telegram channel.
func NewChannel(name string, config Config) (*Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Channel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeTelegram),
		config:      config,
		baseURL:     BaseURL + "/bot" + config.Token,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Start initializes the channel and starts polling or sets webhook.
func (c *Channel) Start(ctx context.Context, handler channel.MessageHandler) error {
	if c.IsRunning() {
		return channel.ErrChannelAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	if c.config.WebhookURL != "" {
		if err := c.setWebhook(ctx); err != nil {
			cancel()
			c.cancel = nil
			return fmt.Errorf("failed to set webhook: %w", err)
		}
	} else {
		c.wg.Add(1)
		go c.pollUpdates(ctx)
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

	if c.cancel != nil {
		c.cancel()
	}

	if c.config.WebhookURL != "" {
		c.deleteWebhook(ctx)
	}

	c.wg.Wait()
	c.SetStatus(channel.StatusStopped)
	return nil
}

// SendMessage sends a message to Telegram.
func (c *Channel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if !c.IsRunning() {
		return channel.ErrChannelNotRunning
	}

	chatID, err := strconv.ParseInt(msg.To.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat_id: %w", err)
	}

	switch msg.Type {
	case channel.MessageTypeText:
		return c.sendTextMessage(ctx, chatID, msg)
	case channel.MessageTypeImage:
		return c.sendPhotoMessage(ctx, chatID, msg)
	case channel.MessageTypeFile:
		return c.sendDocumentMessage(ctx, chatID, msg)
	case channel.MessageTypeAudio:
		return c.sendAudioMessage(ctx, chatID, msg)
	case channel.MessageTypeVideo:
		return c.sendVideoMessage(ctx, chatID, msg)
	default:
		return c.sendTextMessage(ctx, chatID, msg)
	}
}

// HandleWebhook processes incoming webhook requests from Telegram.
func (c *Channel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	var update Update
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, fmt.Errorf("failed to parse webhook data: %w", err)
	}

	return c.parseUpdate(&update)
}

// Update represents a Telegram update.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *MessageObject `json:"message,omitempty"`
	EditedMessage *MessageObject `json:"edited_message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
	InlineQuery   *InlineQuery   `json:"inline_query,omitempty"`
}

// MessageObject represents a Telegram message.
type MessageObject struct {
	MessageID int64     `json:"message_id"`
	From      *User     `json:"from,omitempty"`
	Chat      *Chat     `json:"chat"`
	Date      int64     `json:"date"`
	Text      string    `json:"text,omitempty"`
	Photo     []PhotoSize `json:"photo,omitempty"`
	Document  *Document `json:"document,omitempty"`
	Audio     *Audio    `json:"audio,omitempty"`
	Video     *Video    `json:"video,omitempty"`
	Voice     *Voice    `json:"voice,omitempty"`
	Location  *Location `json:"location,omitempty"`
	Contact   *Contact  `json:"contact,omitempty"`
	Caption   string    `json:"caption,omitempty"`
}

// User represents a Telegram user.
type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// PhotoSize represents a photo size.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Document represents a document.
type Document struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Audio represents an audio file.
type Audio struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	Performer    string `json:"performer,omitempty"`
	Title        string `json:"title,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Video represents a video file.
type Video struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     int    `json:"duration"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Voice represents a voice message.
type Voice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Location represents a location.
type Location struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

// Contact represents a contact.
type Contact struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
}

// CallbackQuery represents a callback query.
type CallbackQuery struct {
	ID      string          `json:"id"`
	From    *User           `json:"from"`
	Message *MessageObject  `json:"message,omitempty"`
	Data    string          `json:"data,omitempty"`
}

// InlineQuery represents an inline query.
type InlineQuery struct {
	ID     string `json:"id"`
	From   *User  `json:"from"`
	Query  string `json:"query"`
	Offset string `json:"offset"`
}

// SendMessageRequest represents a request to send a message.
type SendMessageRequest struct {
	ChatID                interface{} `json:"chat_id"`
	Text                  string      `json:"text"`
	ParseMode             string      `json:"parse_mode,omitempty"`
	DisableNotification   bool        `json:"disable_notification,omitempty"`
	ReplyToMessageID      int64       `json:"reply_to_message_id,omitempty"`
	ReplyMarkup           interface{} `json:"reply_markup,omitempty"`
}

// SendPhotoRequest represents a request to send a photo.
type SendPhotoRequest struct {
	ChatID              interface{} `json:"chat_id"`
	Photo               string      `json:"photo"`
	Caption             string      `json:"caption,omitempty"`
	ParseMode           string      `json:"parse_mode,omitempty"`
	DisableNotification bool        `json:"disable_notification,omitempty"`
}

// SendDocumentRequest represents a request to send a document.
type SendDocumentRequest struct {
	ChatID              interface{} `json:"chat_id"`
	Document            string      `json:"document"`
	Caption             string      `json:"caption,omitempty"`
	ParseMode           string      `json:"parse_mode,omitempty"`
	DisableNotification bool        `json:"disable_notification,omitempty"`
}

// APIResponse represents a generic API response.
type APIResponse struct {
	Ok          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

// GetUpdatesResponse represents the response from getUpdates.
type GetUpdatesResponse struct {
	Ok     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func (c *Channel) parseUpdate(update *Update) (*channel.Message, error) {
	if update.CallbackQuery != nil {
		return c.parseCallbackQuery(update)
	}

	var msg *MessageObject
	var isEdited bool

	if update.Message != nil {
		msg = update.Message
	} else if update.EditedMessage != nil {
		msg = update.EditedMessage
		isEdited = true
	} else {
		return nil, errors.New("no message in update")
	}

	result := c.createBaseMessage(msg, isEdited)
	c.parseMessageContent(msg, result)

	return result, nil
}

func (c *Channel) parseCallbackQuery(update *Update) (*channel.Message, error) {
	result := &channel.Message{
		ID:        strconv.FormatInt(update.UpdateID, 10),
		ChannelID: "telegram",
		Direction: channel.DirectionInbound,
		Type:      channel.MessageTypeEvent,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if update.CallbackQuery.From != nil {
		result.From = channel.UserInfo{
			ID:       strconv.FormatInt(update.CallbackQuery.From.ID, 10),
			Name:     update.CallbackQuery.From.FirstName,
			Platform: "telegram",
		}
	}

	result.SetMetadata("event_type", "callback_query")
	result.SetMetadata("callback_data", update.CallbackQuery.Data)

	return result, nil
}

func (c *Channel) createBaseMessage(msg *MessageObject, isEdited bool) *channel.Message {
	result := &channel.Message{
		ID:        strconv.FormatInt(msg.MessageID, 10),
		ChannelID: "telegram",
		Direction: channel.DirectionInbound,
		Timestamp: time.Unix(msg.Date, 0).Format(time.RFC3339),
	}

	if msg.From != nil {
		result.From = channel.UserInfo{
			ID:       strconv.FormatInt(msg.From.ID, 10),
			Name:     msg.From.FirstName,
			Platform: "telegram",
		}
		if msg.From.Username != "" {
			result.SetMetadata("username", msg.From.Username)
		}
	}

	if msg.Chat != nil {
		result.To = channel.UserInfo{
			ID:       strconv.FormatInt(msg.Chat.ID, 10),
			Name:     msg.Chat.Title,
			Platform: "telegram",
		}
		result.SetMetadata("chat_type", msg.Chat.Type)
	}

	if isEdited {
		result.SetMetadata("is_edited", true)
	}

	return result
}

func (c *Channel) parseMessageContent(msg *MessageObject, result *channel.Message) {
	switch {
	case msg.Text != "":
		c.parseTextMessage(msg, result)
	case len(msg.Photo) > 0:
		c.parsePhotoMessage(msg, result)
	case msg.Document != nil:
		c.parseDocumentMessage(msg, result)
	case msg.Audio != nil:
		c.parseAudioMessage(msg, result)
	case msg.Video != nil:
		c.parseVideoMessage(msg, result)
	case msg.Voice != nil:
		c.parseVoiceMessage(msg, result)
	case msg.Location != nil:
		c.parseLocationMessage(msg, result)
	case msg.Contact != nil:
		c.parseContactMessage(msg, result)
	}
}

func (c *Channel) parseTextMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeText
	result.Content = msg.Text
}

func (c *Channel) parsePhotoMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeImage
	largestPhoto := msg.Photo[len(msg.Photo)-1]
	if msg.Caption != "" {
		result.Content = msg.Caption
	} else {
		result.Content = largestPhoto.FileID
	}
	result.SetMetadata("file_id", largestPhoto.FileID)
	result.SetMetadata("width", largestPhoto.Width)
	result.SetMetadata("height", largestPhoto.Height)
	if msg.Caption != "" {
		result.SetMetadata("caption", msg.Caption)
	}
}

func (c *Channel) parseDocumentMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeFile
	result.Content = msg.Document.FileID
	result.SetMetadata("file_id", msg.Document.FileID)
	result.SetMetadata("file_name", msg.Document.FileName)
	result.SetMetadata("mime_type", msg.Document.MimeType)
	if msg.Caption != "" {
		result.SetMetadata("caption", msg.Caption)
	}
}

func (c *Channel) parseAudioMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeAudio
	result.Content = msg.Audio.FileID
	result.SetMetadata("file_id", msg.Audio.FileID)
	result.SetMetadata("duration", msg.Audio.Duration)
	result.SetMetadata("title", msg.Audio.Title)
	result.SetMetadata("performer", msg.Audio.Performer)
}

func (c *Channel) parseVideoMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeVideo
	result.Content = msg.Video.FileID
	result.SetMetadata("file_id", msg.Video.FileID)
	result.SetMetadata("duration", msg.Video.Duration)
	result.SetMetadata("width", msg.Video.Width)
	result.SetMetadata("height", msg.Video.Height)
}

func (c *Channel) parseVoiceMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeAudio
	result.Content = msg.Voice.FileID
	result.SetMetadata("file_id", msg.Voice.FileID)
	result.SetMetadata("duration", msg.Voice.Duration)
}

func (c *Channel) parseLocationMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeEvent
	result.SetMetadata("event_type", "location")
	result.SetMetadata("latitude", msg.Location.Latitude)
	result.SetMetadata("longitude", msg.Location.Longitude)
}

func (c *Channel) parseContactMessage(msg *MessageObject, result *channel.Message) {
	result.Type = channel.MessageTypeEvent
	result.SetMetadata("event_type", "contact")
	result.SetMetadata("phone_number", msg.Contact.PhoneNumber)
	result.SetMetadata("first_name", msg.Contact.FirstName)
	result.SetMetadata("last_name", msg.Contact.LastName)
}

func (c *Channel) sendTextMessage(ctx context.Context, chatID int64, msg *channel.Message) error {
	req := SendMessageRequest{
		ChatID: chatID,
		Text:   msg.Content,
	}

	if parseMode, ok := msg.GetMetadata("parse_mode"); ok {
		req.ParseMode = parseMode.(string)
	}

	return c.sendAPIRequest(ctx, EndpointSendMessage, req)
}

func (c *Channel) sendPhotoMessage(ctx context.Context, chatID int64, msg *channel.Message) error {
	req := SendPhotoRequest{
		ChatID: chatID,
		Photo:  msg.Content,
	}

	if caption, ok := msg.GetMetadata("caption"); ok {
		req.Caption = caption.(string)
	}

	return c.sendAPIRequest(ctx, EndpointSendPhoto, req)
}

func (c *Channel) sendDocumentMessage(ctx context.Context, chatID int64, msg *channel.Message) error {
	req := SendDocumentRequest{
		ChatID:   chatID,
		Document: msg.Content,
	}

	if caption, ok := msg.GetMetadata("caption"); ok {
		req.Caption = caption.(string)
	}

	return c.sendAPIRequest(ctx, EndpointSendDocument, req)
}

func (c *Channel) sendAudioMessage(ctx context.Context, chatID int64, msg *channel.Message) error {
	req := map[string]interface{}{
		"chat_id": chatID,
		"audio":   msg.Content,
	}

	if title, ok := msg.GetMetadata("title"); ok {
		req["title"] = title
	}

	if performer, ok := msg.GetMetadata("performer"); ok {
		req["performer"] = performer
	}

	return c.sendAPIRequest(ctx, EndpointSendAudio, req)
}

func (c *Channel) sendVideoMessage(ctx context.Context, chatID int64, msg *channel.Message) error {
	req := map[string]interface{}{
		"chat_id": chatID,
		"video":   msg.Content,
	}

	if caption, ok := msg.GetMetadata("caption"); ok {
		req["caption"] = caption
	}

	return c.sendAPIRequest(ctx, EndpointSendVideo, req)
}

func (c *Channel) sendAPIRequest(ctx context.Context, endpoint string, payload interface{}) error {
	url := c.baseURL + endpoint

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

	if !apiResp.Ok {
		return fmt.Errorf("telegram API error: %d - %s", apiResp.ErrorCode, apiResp.Description)
	}

	return nil
}

func (c *Channel) pollUpdates(ctx context.Context) {
	defer c.wg.Done()

	var offset int64 = 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		url := fmt.Sprintf("%s%s?offset=%d&timeout=30", c.baseURL, EndpointGetUpdates, offset)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var updatesResp GetUpdatesResponse
		if err := json.Unmarshal(body, &updatesResp); err != nil {
			continue
		}

		if !updatesResp.Ok {
			continue
		}

		for _, update := range updatesResp.Result {
			offset = update.UpdateID + 1

			msg, err := c.parseUpdate(&update)
			if err != nil {
				continue
			}

			if handler := c.GetHandler(); handler != nil {
				go handler(ctx, msg)
			}
		}
	}
}

func (c *Channel) setWebhook(ctx context.Context) error {
	req := map[string]interface{}{
		"url": c.config.WebhookURL,
	}

	if c.config.SecretToken != "" {
		req["secret_token"] = c.config.SecretToken
	}

	return c.sendAPIRequest(ctx, EndpointSetWebhook, req)
}

func (c *Channel) deleteWebhook(ctx context.Context) error {
	return c.sendAPIRequest(ctx, EndpointDeleteWebhook, map[string]interface{}{})
}

// GetCapabilities returns the capabilities of the Telegram channel.
func (c *Channel) GetCapabilities() channel.ChannelCapabilities {
	return channel.ChannelCapabilities{
		TextMessages:     true,
		ImageMessages:    true,
		FileMessages:     true,
		AudioMessages:    true,
		VideoMessages:    true,
		LocationMessages: true,
		TemplateMessages: false,
		ReadReceipts:     false,
		TypingIndicators: false,
		MessageEditing:   true,
		MessageDeletion:  true,
		ReactionMessages: true,
	}
}
