// Package messenger provides a Channel adapter for Facebook Messenger Platform.
//
// Official API Documentation:
// - URL: https://developers.facebook.com/docs/messenger-platform
// - API Version: Graph API v19.0
// - Last Updated: 2024年7月
// - Authentication: Page Access Token
//
// Supported Features:
// - Text messages
// - Image attachments
// - Audio attachments
// - Video attachments
// - File attachments
// - Button templates
// - Generic templates
//
// Rate Limits:
// - Messenger Profile API: 10 calls per 10 minutes
// - 24-hour message window after user interaction
// - User must initiate conversation first
//
// Security:
// - Webhook signature verification (X-Hub-Signature-256)
// - HTTPS required for webhooks
// - App Secret required for signature validation
package messenger

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
	ErrInvalidSignature    = errors.New("invalid webhook signature")
	ErrUserNotResponded    = errors.New("user has not responded within 24 hours")
	ErrPageAccessTokenEmpty = errors.New("page access token is empty")
)

type Config struct {
	PageID         string
	PageAccessToken string
	AppSecret      string
	VerifyToken    string
	WebhookURL     string
}

type MessengerChannel struct {
	*channel.BaseChannel

	config     Config
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewMessengerChannel(name string, config Config) (*MessengerChannel, error) {
	if config.PageAccessToken == "" {
		return nil, ErrPageAccessTokenEmpty
	}

	return &MessengerChannel{
		BaseChannel: channel.NewBaseChannel(name, channel.ChannelTypeMessenger),
		config:      config,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (m *MessengerChannel) Start(ctx context.Context, handler channel.MessageHandler) error {
	m.SetHandler(handler)
	m.SetStatus(channel.StatusRunning)
	return nil
}

func (m *MessengerChannel) Stop(ctx context.Context) error {
	m.SetStatus(channel.StatusStopped)
	m.SetHandler(nil)
	return nil
}

func (m *MessengerChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	if msg == nil {
		return channel.ErrUnsupportedMessageType
	}

	recipientID := msg.To.ID
	if recipientID == "" {
		return fmt.Errorf("recipient ID is required")
	}

	payload := m.buildMessagePayload(recipientID, msg)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/me/messages?access_token=%s", apiBaseURL, m.config.PageAccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("messenger API error: %s", string(body))
	}

	return nil
}

func (m *MessengerChannel) buildMessagePayload(recipientID string, msg *channel.Message) map[string]interface{} {
	payload := map[string]interface{}{
		"recipient": map[string]string{
			"id": recipientID,
		},
		"messaging_type": "MESSAGE_TAG",
		"tag":            "CONFIRMED_EVENT_UPDATE",
	}

	switch msg.Type {
	case channel.MessageTypeText:
		payload["message"] = map[string]interface{}{
			"text": msg.Content,
		}
	case channel.MessageTypeImage:
		payload["message"] = map[string]interface{}{
			"attachment": map[string]interface{}{
				"type": "image",
				"payload": map[string]interface{}{
					"url": msg.Content,
				},
			},
		}
	case channel.MessageTypeAudio:
		payload["message"] = map[string]interface{}{
			"attachment": map[string]interface{}{
				"type": "audio",
				"payload": map[string]interface{}{
					"url": msg.Content,
				},
			},
		}
	case channel.MessageTypeVideo:
		payload["message"] = map[string]interface{}{
			"attachment": map[string]interface{}{
				"type": "video",
				"payload": map[string]interface{}{
					"url": msg.Content,
				},
			},
		}
	case channel.MessageTypeFile:
		payload["message"] = map[string]interface{}{
			"attachment": map[string]interface{}{
				"type": "file",
				"payload": map[string]interface{}{
					"url": msg.Content,
				},
			},
		}
	default:
		payload["message"] = map[string]interface{}{
			"text": msg.Content,
		}
	}

	return payload
}

func (m *MessengerChannel) HandleWebhook(path string, data []byte) (*channel.Message, error) {
	if m.config.AppSecret != "" {
		return nil, fmt.Errorf("signature verification requires HTTP headers")
	}

	return m.parseWebhookData(data)
}

func (m *MessengerChannel) HandleWebhookWithSignature(data []byte, signature string) (*channel.Message, error) {
	if m.config.AppSecret != "" {
		if !m.verifySignature(data, signature) {
			return nil, ErrInvalidSignature
		}
	}

	return m.parseWebhookData(data)
}

func (m *MessengerChannel) verifySignature(data []byte, signature string) bool {
	if !bytes.HasPrefix([]byte(signature), []byte("sha256=")) {
		return false
	}

	expectedMAC, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(m.config.AppSecret))
	mac.Write(data)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(expectedMAC, actualMAC)
}

func (m *MessengerChannel) parseWebhookData(data []byte) (*channel.Message, error) {
	var webhook struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Time      int64  `json:"time"`
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Timestamp int64 `json:"timestamp"`
				Message   *struct {
					MID        string `json:"mid"`
					Text       string `json:"text"`
					Attachments []struct {
						Type    string `json:"type"`
						Payload struct {
							URL string `json:"url"`
						} `json:"payload"`
					} `json:"attachments"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(data, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	if webhook.Object != "page" || len(webhook.Entry) == 0 || len(webhook.Entry[0].Messaging) == 0 {
		return nil, nil
	}

	messaging := webhook.Entry[0].Messaging[0]
	if messaging.Message == nil {
		return nil, nil
	}

	msgType := channel.MessageTypeText
	content := messaging.Message.Text

	if len(messaging.Message.Attachments) > 0 {
		attachment := messaging.Message.Attachments[0]
		content = attachment.Payload.URL
		switch attachment.Type {
		case "image":
			msgType = channel.MessageTypeImage
		case "audio":
			msgType = channel.MessageTypeAudio
		case "video":
			msgType = channel.MessageTypeVideo
		case "file":
			msgType = channel.MessageTypeFile
		}
	}

	msg := channel.NewMessage(
		messaging.Message.MID,
		m.Name(),
		channel.DirectionInbound,
		channel.UserInfo{
			ID:       messaging.Sender.ID,
			Name:     "",
			Platform: "messenger",
		},
		content,
		msgType,
	)

	return msg, nil
}

func (m *MessengerChannel) VerifyWebhook(mode, token, challenge string) (string, error) {
	if mode != "subscribe" || token != m.config.VerifyToken {
		return "", fmt.Errorf("webhook verification failed")
	}
	return challenge, nil
}

func (m *MessengerChannel) SendTemplate(ctx context.Context, recipientID string, template map[string]interface{}) error {
	payload := map[string]interface{}{
		"recipient": map[string]string{
			"id": recipientID,
		},
		"message": map[string]interface{}{
			"attachment": template,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	url := fmt.Sprintf("%s/me/messages?access_token=%s", apiBaseURL, m.config.PageAccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("messenger API error: %s", string(body))
	}

	return nil
}
