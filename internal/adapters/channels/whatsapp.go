package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"strconv"
	"time"
)

func init() {
	Register("whatsapp", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewWhatsAppChannel(&config.WhatsAppConfig{
			Port:          getIntFromConfig(cfg, "port", 8085),
			Path:          getStringFromConfigWithDefault(cfg, "path", "/whatsapp/webhook"),
			PhoneNumberID: getStringFromConfig(cfg, "phone_number_id"),
			BusinessID:    getStringFromConfig(cfg, "business_id"),
			AccessToken:   getStringFromConfig(cfg, "access_token"),
			VerifyToken:   getStringFromConfig(cfg, "verify_token"),
		}), nil
	})
}

type WhatsAppChannel struct {
	*WebhookChannel
	config     *config.WhatsAppConfig
	httpClient *http.Client
}

func NewWhatsAppChannel(cfg *config.WhatsAppConfig) *WhatsAppChannel {
	if cfg == nil {
		cfg = &config.WhatsAppConfig{
			Port: 8085,
			Path: "/whatsapp/webhook",
		}
	}

	baseChannel := NewWebhookChannel("whatsapp", entity.ChannelTypeWhatsApp, cfg.Path, cfg)

	return &WhatsAppChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *WhatsAppChannel) Description() string {
	return "WhatsApp Business Cloud API Channel"
}

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("WhatsAppChannel is not initialized")
	}

	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleWhatsAppWebhook)

	c.WebhookChannel.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", c.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := c.WebhookChannel.Start(ctx); err != nil {
		return err
	}

	c.logger.Info(i18n.T("adapter.whatsapp_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
	)

	return nil
}

func (c *WhatsAppChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("whatsapp").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *WhatsAppChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("WhatsAppChannel is not running")
	}

	if c.config.PhoneNumberID == "" || c.config.AccessToken == "" {
		return fmt.Errorf("WhatsApp PhoneNumberID or AccessToken not configured")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/messages", c.config.PhoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                msg.SessionID,
		"type":              "text",
		"text": map[string]string{
			"body": msg.Content,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.AccessToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      int    `json:"code"`
			ErrorData struct {
				Details string `json:"details"`
			} `json:"error_data"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error.Code != 0 {
		return fmt.Errorf("WhatsApp API error: %d - %s", result.Error.Code, result.Error.Message)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

func (c *WhatsAppChannel) handleWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	c.WebhookChannel.mu.Lock()
	c.WebhookChannel.totalMsg++
	c.WebhookChannel.lastMsgTime = time.Now()
	c.WebhookChannel.mu.Unlock()

	if r.Method == "GET" {
		c.handleVerification(w, r)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error(i18n.T("adapter.read_body_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	msg, err := c.parseWhatsAppMessage(body)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_whatsapp_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if msg != nil && c.WebhookChannel.onMessage != nil {
		ctx := context.Background()
		c.WebhookChannel.onMessage(ctx, msg)
	}

	w.WriteHeader(http.StatusOK)
}

func (c *WhatsAppChannel) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == c.config.VerifyToken {
		c.logger.Info(i18n.T("adapter.whatsapp_verify_success"), logging.String("challenge", challenge))
		if _, err := w.Write([]byte(challenge)); err != nil {
			c.logger.Error(i18n.T("adapter.return_challenge_failed"), logging.Err(err))
		}
		return
	}

	c.logger.Warn(i18n.T("adapter.whatsapp_verify_failed"), logging.String("mode", mode), logging.String("token", "***"))
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func (c *WhatsAppChannel) parseWhatsAppMessage(body []byte) (*entity.IncomingMessage, error) {
	var webhookData struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string `json:"id"`
			Changes []struct {
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
						Text      struct {
							Body string `json:"body"`
						} `json:"text"`
						Type string `json:"type"`
					} `json:"messages"`
				} `json:"value"`
				Field string `json:"field"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &webhookData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook data: %w", err)
	}

	if webhookData.Object != "whatsapp_business_account" {
		return nil, nil
	}

	for _, entry := range webhookData.Entry {
		for _, change := range entry.Changes {
			if change.Field == "messages" && len(change.Value.Messages) > 0 {
				message := change.Value.Messages[0]
				if message.Type != "text" {
					continue
				}

				senderName := ""
				if len(change.Value.Contacts) > 0 {
					senderName = change.Value.Contacts[0].Profile.Name
				}

				timestamp, _ := strconv.ParseInt(message.Timestamp, 10, 64)

				msg := &entity.IncomingMessage{
					ChannelID:   "whatsapp",
					ChannelName: "WhatsApp",
					MessageID:   message.ID,
					Sender: &entity.MessageSender{
						ID:   message.From,
						Name: senderName,
						Type: "user",
					},
					Content:     message.Text.Body,
					ContentType: "text",
					Timestamp:   time.Unix(timestamp, 0),
					Metadata: map[string]interface{}{
						"phone_number_id":      change.Value.Metadata.PhoneNumberID,
						"display_phone_number": change.Value.Metadata.DisplayPhoneNumber,
					},
				}

				msg.SessionID = message.From
				return msg, nil
			}
		}
	}

	return nil, nil
}
