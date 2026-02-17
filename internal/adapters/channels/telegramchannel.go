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
	Register("telegram", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewTelegramChannel(&config.TelegramConfig{
			Port:        getIntFromConfig(cfg, "port", 8087),
			Path:        getStringFromConfigWithDefault(cfg, "path", "/telegram/webhook"),
			BotToken:    getStringFromConfig(cfg, "bot_token"),
			WebhookURL:  getStringFromConfig(cfg, "webhook_url"),
			SecretToken: getStringFromConfig(cfg, "secret_token"),
			UseWebhook:  getBoolFromConfig(cfg, "use_webhook", true),
		}), nil
	})
}

type TelegramChannel struct {
	*WebhookChannel
	config     *config.TelegramConfig
	httpClient *http.Client
}

func NewTelegramChannel(cfg *config.TelegramConfig) *TelegramChannel {
	if cfg == nil {
		cfg = &config.TelegramConfig{
			Port: 8087,
			Path: "/telegram/webhook",
		}
	}

	baseChannel := NewWebhookChannel("telegram", entity.ChannelTypeTelegram, cfg.Path, cfg)

	return &TelegramChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *TelegramChannel) Description() string {
	return "Telegram Bot API Channel"
}

func (c *TelegramChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("TelegramChannel is not initialized")
	}

	if c.config.BotToken == "" {
		return fmt.Errorf("Telegram BotToken not configured")
	}

	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleTelegramWebhook)

	c.WebhookChannel.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", c.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := c.WebhookChannel.Start(ctx); err != nil {
		return err
	}

	if c.config.WebhookURL != "" {
		if err := c.setWebhook(); err != nil {
			c.logger.Warn(i18n.T("adapter.telegram_set_webhook_failed"), logging.Err(err))
		}
	}

	c.logger.Info(i18n.T("adapter.telegram_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
	)

	return nil
}

func (c *TelegramChannel) Stop() error {
	return c.WebhookChannel.Stop()
}

func (c *TelegramChannel) setWebhook() error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", c.config.BotToken)

	payload := map[string]interface{}{
		"url": c.config.WebhookURL,
	}

	if c.config.SecretToken != "" {
		payload["secret_token"] = c.config.SecretToken
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Ok {
		return fmt.Errorf("failed to set webhook: %s", result.Description)
	}

	c.logger.Info(i18n.T("adapter.telegram_set_webhook_success"))
	return nil
}

func (c *TelegramChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("TelegramChannel is not running")
	}

	if c.config.BotToken == "" {
		return fmt.Errorf("Telegram BotToken not configured")
	}

	chatID, err := strconv.ParseInt(msg.SessionID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.config.BotToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    msg.Content,
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
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Ok {
		return fmt.Errorf("Telegram API error: %s", result.Description)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

func (c *TelegramChannel) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	c.WebhookChannel.mu.Lock()
	c.WebhookChannel.totalMsg++
	c.WebhookChannel.lastMsgTime = time.Now()
	c.WebhookChannel.mu.Unlock()

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if c.config.SecretToken != "" {
		secretToken := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if secretToken != c.config.SecretToken {
			c.logger.Warn(i18n.T("adapter.telegram_verify_failed"))
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error(i18n.T("adapter.read_body_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		c.logger.Error(i18n.T("adapter.parse_telegram_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	msg := c.parseTelegramUpdate(update)
	if msg != nil && c.WebhookChannel.onMessage != nil {
		ctx := context.Background()
		c.WebhookChannel.onMessage(ctx, msg)
	}

	w.WriteHeader(http.StatusOK)
}

func (c *TelegramChannel) parseTelegramUpdate(update TelegramUpdate) *entity.IncomingMessage {
	if update.Message == nil {
		return nil
	}

	message := update.Message
	if message.Text == "" {
		return nil
	}

	senderName := ""
	if message.From != nil {
		senderName = message.From.FirstName
		if message.From.LastName != "" {
			senderName += " " + message.From.LastName
		}
	}

	msg := &entity.IncomingMessage{
		ChannelID:   "telegram",
		ChannelName: "Telegram",
		MessageID:   strconv.FormatInt(int64(message.MessageID), 10),
		Sender: &entity.MessageSender{
			ID:   strconv.FormatInt(message.From.ID, 10),
			Name: senderName,
			Type: "user",
		},
		Content:     message.Text,
		ContentType: "text",
		Timestamp:   time.Unix(int64(message.Date), 0),
		Metadata: map[string]interface{}{
			"chat_id":   message.Chat.ID,
			"chat_type": message.Chat.Type,
		},
	}

	msg.SessionID = strconv.FormatInt(message.Chat.ID, 10)
	return msg
}

type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	MessageID int           `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      TelegramChat  `json:"chat"`
	Date      int           `json:"date"`
	Text      string        `json:"text"`
}

type TelegramUser struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

type TelegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}
