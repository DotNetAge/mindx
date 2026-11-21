// Package whatsapp provides a Channel adapter for WhatsApp Business API.
//
// Official API Documentation:
// - URL: https://developers.facebook.com/docs/whatsapp
// - API Version: Cloud API v19.0 / On-Premises API v2.57.2
// - Last Updated: 2024年7月
// - Authentication: WhatsApp Business Account + Phone Number ID + Access Token
//
// Supported Features:
// - Text messages
// - Image messages
// - Document messages
// - Audio messages
// - Video messages
// - Location messages
// - Template messages
//
// Rate Limits:
// - 24-hour customer service window after user message
// - Template messages require pre-approval
// - Pricing per conversation type
//
// Security:
// - Webhook signature verification (X-Hub-Signature-256)
// - HTTPS required for webhooks
// - App Secret required for signature validation
//
// Requirements:
// - WhatsApp Business Account
// - Phone Number ID
// - Permanent Access Token
// - Verified Business
package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

const (
	apiBaseURL = "https://graph.facebook.com/v19.0"
)

var (
	ErrInvalidSignature      = errors.New("invalid webhook signature")
	ErrPhoneNumberIDEmpty    = errors.New("phone number ID is empty")
	ErrAccessTokenEmpty      = errors.New("access token is empty")
	ErrTemplateNotApproved   = errors.New("template message not approved")
	ErrOutsideServiceWindow  = errors.New("outside 24-hour service window")
)

type Config struct {
	PhoneNumberID  string
	AccessToken    string
	AppSecret      string
	VerifyToken    string
	WebhookURL     string
	BusinessAccountID string
}

type WhatsAppChannel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewWhatsAppChannel(name string, config Config) (*WhatsAppChannel, error) {
	if config.PhoneNumberID == "" {
		return nil, ErrPhoneNumberIDEmpty
	}
	if config.AccessToken == "" {
		return nil, ErrAccessTokenEmpty
	}

	return &WhatsAppChannel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeWhatsApp),
		config:      config,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (w *WhatsAppChannel) Start(ctx context.Context, handler channel.MessageHandler) error {
	w.SetHandler(handler)
	w.SetStatus(channel.StatusRunning)
	return nil
}

func (w *WhatsAppChannel) Stop(ctx context.Context) error {
	w.SetStatus(channel.StatusStopped)
	w.SetHandler(nil)
	return nil
}

func (w *WhatsAppChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if msg == nil {
		return channel.ErrUnsupportedMessageType
	}

	recipientID := msg.To.ID
	if recipientID == "" {
		return fmt.Errorf("recipient ID is required")
	}

	payload := w.buildMessagePayload(recipientID, msg)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", apiBaseURL, w.config.PhoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", w.config.AccessToken))

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("whatsapp API error: %s", string(body))
	}

	return nil
}

func (w *WhatsAppChannel) buildMessagePayload(recipientID string, msg *channel.Message) map[string]interface{} {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                recipientID,
	}

	switch msg.Type {
	case channel.MessageTypeText:
		payload["type"] = "text"
		payload["text"] = map[string]interface{}{
			"body": msg.Content,
		}
	case channel.MessageTypeImage:
		payload["type"] = "image"
		payload["image"] = map[string]interface{}{
			"link": msg.Content,
		}
	case channel.MessageTypeAudio:
		payload["type"] = "audio"
		payload["audio"] = map[string]interface{}{
			"link": msg.Content,
		}
	case channel.MessageTypeVideo:
		payload["type"] = "video"
		payload["video"] = map[string]interface{}{
			"link": msg.Content,
		}
	case channel.MessageTypeFile:
		payload["type"] = "document"
		payload["document"] = map[string]interface{}{
			"link": msg.Content,
		}
	default:
		payload["type"] = "text"
		payload["text"] = map[string]interface{}{
			"body": msg.Content,
		}
	}

	return payload
}

func (w *WhatsAppChannel) SendTemplate(ctx context.Context, recipientID, templateName, languageCode string, components []map[string]interface{}) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                recipientID,
		"type":              "template",
		"template": map[string]interface{}{
			"name":       templateName,
			"language": map[string]string{
				"code": languageCode,
			},
			"components": components,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", apiBaseURL, w.config.PhoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", w.config.AccessToken))

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp API error: %s", string(body))
	}

	return nil
}

func (w *WhatsAppChannel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	return w.parseWebhookData(data)
}

func (w *WhatsAppChannel) HandleWebhookWithSignature(data []byte, signature string) (*channel.Message, error) {
	if w.config.AppSecret != "" {
		if !w.verifySignature(data, signature) {
			return nil, ErrInvalidSignature
		}
	}

	return w.parseWebhookData(data)
}

func (w *WhatsAppChannel) verifySignature(data []byte, signature string) bool {
	if !bytes.HasPrefix([]byte(signature), []byte("sha256=")) {
		return false
	}

	expectedMAC, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(w.config.AppSecret))
	mac.Write(data)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(expectedMAC, actualMAC)
}

func (w *WhatsAppChannel) parseWebhookData(data []byte) (*channel.Message, error) {
	var webhook struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Changes   []struct {
				Value struct {
					MessagingProduct string `json:"messaging_product"`
					Metadata         struct {
						DisplayPhoneNumber string `json:"display_phone_number"`
						PhoneNumberID      string `json:"phone_number_id"`
					} `json:"metadata"`
					Contacts []struct {
						Profile struct {
							Name string `json:"name"`
						} `json:"profile"`
						WaID string `json:"wa_id"`
					} `json:"contacts"`
					Messages []struct {
						From      string `json:"from"`
						ID        string `json:"id"`
						Timestamp string `json:"timestamp"`
						Type      string `json:"type"`
						Text      *struct {
							Body string `json:"body"`
						} `json:"text"`
						Image *struct {
							ID       string `json:"id"`
							MimeType string `json:"mime_type"`
							Sha256   string `json:"sha256"`
						} `json:"image"`
						Audio *struct {
							ID       string `json:"id"`
							MimeType string `json:"mime_type"`
							Sha256   string `json:"sha256"`
						} `json:"audio"`
						Document *struct {
							ID       string `json:"id"`
							MimeType string `json:"mime_type"`
							Sha256   string `json:"sha256"`
							Filename string `json:"filename"`
						} `json:"document"`
						Video *struct {
							ID       string `json:"id"`
							MimeType string `json:"mime_type"`
							Sha256   string `json:"sha256"`
						} `json:"video"`
						Location *struct {
							Latitude  float64 `json:"latitude"`
							Longitude float64 `json:"longitude"`
						} `json:"location"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(data, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	if webhook.Object != "whatsapp_business_account" || len(webhook.Entry) == 0 || len(webhook.Entry[0].Changes) == 0 {
		return nil, nil
	}

	change := webhook.Entry[0].Changes[0].Value
	if len(change.Messages) == 0 {
		return nil, nil
	}

	msg := change.Messages[0]
	contactName := ""
	if len(change.Contacts) > 0 {
		contactName = change.Contacts[0].Profile.Name
	}

	msgType := channel.MessageTypeText
	content := ""

	switch msg.Type {
	case "text":
		if msg.Text != nil {
			content = msg.Text.Body
		}
	case "image":
		msgType = channel.MessageTypeImage
		if msg.Image != nil {
			content = msg.Image.ID
		}
	case "audio":
		msgType = channel.MessageTypeAudio
		if msg.Audio != nil {
			content = msg.Audio.ID
		}
	case "video":
		msgType = channel.MessageTypeVideo
		if msg.Video != nil {
			content = msg.Video.ID
		}
	case "document":
		msgType = channel.MessageTypeFile
		if msg.Document != nil {
			content = msg.Document.ID
		}
	case "location":
		msgType = channel.MessageTypeEvent
		if msg.Location != nil {
			content = fmt.Sprintf("%f,%f", msg.Location.Latitude, msg.Location.Longitude)
		}
	}

	result := channel.NewMessage(
		msg.ID,
		w.Name(),
		channel.DirectionInbound,
		channel.UserInfo{
			ID:       msg.From,
			Name:     contactName,
			Platform: "whatsapp",
		},
		content,
		msgType,
	)

	return result, nil
}

func (w *WhatsAppChannel) VerifyWebhook(mode, token, challenge string) (string, error) {
	if mode != "subscribe" || token != w.config.VerifyToken {
		return "", fmt.Errorf("webhook verification failed")
	}
	return challenge, nil
}

func (w *WhatsAppChannel) MarkAsRead(ctx context.Context, messageID string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", apiBaseURL, w.config.PhoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", w.config.AccessToken))

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to mark as read: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp API error: %s", string(body))
	}

	return nil
}
