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
	"time"
)

func init() {
	Register("facebook", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewFacebookChannel(&config.FacebookConfig{
			Port:            getIntFromConfig(cfg, "port", 8086),
			Path:            getStringFromConfigWithDefault(cfg, "path", "/facebook/webhook"),
			PageID:          getStringFromConfig(cfg, "page_id"),
			PageAccessToken: getStringFromConfig(cfg, "page_access_token"),
			AppSecret:       getStringFromConfig(cfg, "app_secret"),
			VerifyToken:     getStringFromConfig(cfg, "verify_token"),
		}), nil
	})
}

type FacebookChannel struct {
	*WebhookChannel
	config     *config.FacebookConfig
	httpClient *http.Client
}

func NewFacebookChannel(cfg *config.FacebookConfig) *FacebookChannel {
	if cfg == nil {
		cfg = &config.FacebookConfig{
			Port: 8086,
			Path: "/facebook/webhook",
		}
	}

	baseChannel := NewWebhookChannel("facebook", entity.ChannelTypeFacebook, cfg.Path, cfg)

	return &FacebookChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *FacebookChannel) Description() string {
	return "Facebook Messenger Platform Channel"
}

func (c *FacebookChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("FacebookChannel is not initialized")
	}

	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleFacebookWebhook)

	c.WebhookChannel.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", c.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := c.WebhookChannel.Start(ctx); err != nil {
		return err
	}

	c.logger.Info(i18n.T("adapter.facebook_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
	)

	return nil
}

func (c *FacebookChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("FacebookChannel is not running")
	}

	if c.config.PageAccessToken == "" {
		return fmt.Errorf("Facebook PageAccessToken not configured")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/me/messages?access_token=%s", c.config.PageAccessToken)

	payload := map[string]interface{}{
		"recipient": map[string]string{
			"id": msg.SessionID,
		},
		"message": map[string]string{
			"text": msg.Content,
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
		return fmt.Errorf("Facebook API error: %d - %s", result.Error.Code, result.Error.Message)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

func (c *FacebookChannel) handleFacebookWebhook(w http.ResponseWriter, r *http.Request) {
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

	msg, err := c.parseFacebookMessage(body)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_facebook_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if msg != nil && c.WebhookChannel.onMessage != nil {
		ctx := context.Background()
		c.WebhookChannel.onMessage(ctx, msg)
	}

	w.WriteHeader(http.StatusOK)
}

func (c *FacebookChannel) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == c.config.VerifyToken {
		c.logger.Info(i18n.T("adapter.facebook_verify_success"), logging.String("challenge", challenge))
		if _, err := w.Write([]byte(challenge)); err != nil {
			c.logger.Error(i18n.T("adapter.return_challenge_failed"), logging.Err(err))
		}
		return
	}

	c.logger.Warn(i18n.T("adapter.facebook_verify_failed"), logging.String("mode", mode), logging.String("token", token))
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func (c *FacebookChannel) parseFacebookMessage(body []byte) (*entity.IncomingMessage, error) {
	var webhookData struct {
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
				Message   struct {
					Mid  string `json:"mid"`
					Text string `json:"text"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &webhookData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook data: %w", err)
	}

	if webhookData.Object != "page" {
		return nil, nil
	}

	for _, entry := range webhookData.Entry {
		for _, messaging := range entry.Messaging {
			if messaging.Message.Mid != "" && messaging.Message.Text != "" {
				timestamp := time.Unix(messaging.Timestamp/1000, 0)

				msg := &entity.IncomingMessage{
					ChannelID:   "facebook",
					ChannelName: "Facebook",
					MessageID:   messaging.Message.Mid,
					Sender: &entity.MessageSender{
						ID:   messaging.Sender.ID,
						Name: "",
						Type: "user",
					},
					Content:     messaging.Message.Text,
					ContentType: "text",
					Timestamp:   timestamp,
					Metadata: map[string]interface{}{
						"page_id":      entry.ID,
						"recipient_id": messaging.Recipient.ID,
					},
				}

				msg.SessionID = messaging.Sender.ID
				return msg, nil
			}
		}
	}

	return nil, nil
}
