package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func init() {
	Register("qq", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewQQChannel(&config.QQConfig{
			Port:         getIntFromConfig(cfg, "port", 8082),
			Path:         getStringFromConfigWithDefault(cfg, "path", "/qq/webhook"),
			AppID:        getStringFromConfig(cfg, "app_id"),
			AppSecret:    getStringFromConfig(cfg, "app_secret"),
			Token:        getStringFromConfig(cfg, "token"),
			WebSocketURL: getStringFromConfig(cfg, "websocket_url"),
			AccessToken:  getStringFromConfig(cfg, "access_token"),
		}), nil
	})
}

type QQChannel struct {
	config      *config.QQConfig
	server      *http.Server
	wsConn      *websocket.Conn
	onMessage   func(context.Context, *entity.IncomingMessage)
	mu          sync.RWMutex
	isRunning   bool
	startTime   time.Time
	totalMsg    int64
	lastMsgTime time.Time
	status      *entity.ChannelStatus
	logger      logging.Logger
	httpClient  *http.Client
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewQQChannel(cfg *config.QQConfig) *QQChannel {
	if cfg == nil {
		cfg = &config.QQConfig{
			Port: 8082,
			Path: "/qq/webhook",
		}
	}

	return &QQChannel{
		config: cfg,
		status: &entity.ChannelStatus{
			Name:        "qq",
			Type:        entity.ChannelTypeQQ,
			Description: "QQ 机器人 Channel (支持 OneBot 协议)",
			Running:     false,
		},
		logger: logging.GetSystemLogger().Named("channel.qq"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *QQChannel) Name() string {
	return "qq"
}

func (c *QQChannel) Type() entity.ChannelType {
	return entity.ChannelTypeQQ
}

func (c *QQChannel) Description() string {
	return "QQ 机器人 Channel (支持 OneBot 协议)"
}

func (c *QQChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return fmt.Errorf("QQ Channel 已运行")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.startTime = time.Now()
	c.isRunning = true
	c.status.Running = true
	c.status.StartTime = &c.startTime

	if c.config.WebSocketURL != "" {
		go c.connectOneBot()
	} else {
		mux := http.NewServeMux()
		mux.HandleFunc(c.config.Path, c.handleWebhook)

		c.server = &http.Server{
			Addr:         fmt.Sprintf(":%d", c.config.Port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		go func() {
			if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				c.logger.Error(i18n.T("adapter.http_server_error"), logging.Err(err))
			}
		}()
	}

	c.logger.Info(i18n.T("adapter.qq_started"),
		logging.String(i18n.T("adapter.mode"), c.getMode()),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
	)

	return nil
}

func (c *QQChannel) getMode() string {
	if c.config.WebSocketURL != "" {
		return "OneBot WebSocket"
	}
	return "Webhook"
}

func (c *QQChannel) connectOneBot() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.connectWebSocket(); err != nil {
				c.logger.Error(i18n.T("adapter.connect_onebot_failed"), logging.Err(err))
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}
}

func (c *QQChannel) connectWebSocket() error {
	headers := http.Header{}
	if c.config.AccessToken != "" {
		headers.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.AccessToken))
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.config.WebSocketURL, headers)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.mu.Lock()
	c.wsConn = conn
	c.mu.Unlock()

	c.logger.Info(i18n.T("adapter.onebot_connected"), logging.String(i18n.T("adapter.url"), c.config.WebSocketURL))

	defer func() {
		c.mu.Lock()
		if c.wsConn != nil {
			_ = c.wsConn.Close()
			c.wsConn = nil
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("read message failed: %w", err)
			}

			c.handleOneBotMessage(message)
		}
	}
}

func (c *QQChannel) handleOneBotMessage(data []byte) {
	var event struct {
		PostType    string `json:"post_type"`
		MessageType string `json:"message_type"`
		SubType     string `json:"sub_type"`
		UserID      int64  `json:"user_id"`
		GroupID     int64  `json:"group_id"`
		Message     string `json:"message"`
		RawMessage  string `json:"raw_message"`
		MessageID   int32  `json:"message_id"`
		Sender      struct {
			UserID   int64  `json:"user_id"`
			Nickname string `json:"nickname"`
			Card     string `json:"card"`
		} `json:"sender"`
		Time int64 `json:"time"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		c.logger.Error(i18n.T("adapter.parse_onebot_failed"), logging.Err(err))
		return
	}

	if event.PostType != "message" {
		return
	}

	sessionID := fmt.Sprintf("%d", event.UserID)
	channelName := "QQ私聊"
	if event.MessageType == "group" {
		sessionID = fmt.Sprintf("group_%d", event.GroupID)
		channelName = "QQ群"
	}

	msg := &entity.IncomingMessage{
		ChannelID:   c.Name(),
		ChannelName: channelName,
		SessionID:   sessionID,
		MessageID:   fmt.Sprintf("qq_%d", event.MessageID),
		Sender: &entity.MessageSender{
			ID:   fmt.Sprintf("%d", event.Sender.UserID),
			Name: event.Sender.Nickname,
			Type: "user",
		},
		Content:     event.Message,
		ContentType: "text",
		Timestamp:   time.Unix(event.Time, 0),
		Metadata: map[string]interface{}{
			"user_id":      event.UserID,
			"group_id":     event.GroupID,
			"message_type": event.MessageType,
			"sub_type":     event.SubType,
		},
	}

	c.mu.Lock()
	c.totalMsg++
	c.lastMsgTime = time.Now()
	c.mu.Unlock()

	if c.onMessage != nil {
		c.onMessage(context.Background(), msg)
	}
}

func (c *QQChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event struct {
		PostType    string `json:"post_type"`
		MessageType string `json:"message_type"`
		UserID      int64  `json:"user_id"`
		GroupID     int64  `json:"group_id"`
		Message     string `json:"message"`
		MessageID   int32  `json:"message_id"`
		Sender      struct {
			UserID   int64  `json:"user_id"`
			Nickname string `json:"nickname"`
		} `json:"sender"`
		Time int64 `json:"time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		c.logger.Error(i18n.T("adapter.parse_webhook_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	sessionID := fmt.Sprintf("%d", event.UserID)
	channelName := "QQ私聊"
	if event.MessageType == "group" {
		sessionID = fmt.Sprintf("group_%d", event.GroupID)
		channelName = "QQ群"
	}

	msg := &entity.IncomingMessage{
		ChannelID:   c.Name(),
		ChannelName: channelName,
		SessionID:   sessionID,
		MessageID:   fmt.Sprintf("qq_%d", event.MessageID),
		Sender: &entity.MessageSender{
			ID:   fmt.Sprintf("%d", event.Sender.UserID),
			Name: event.Sender.Nickname,
			Type: "user",
		},
		Content:     event.Message,
		ContentType: "text",
		Timestamp:   time.Unix(event.Time, 0),
		Metadata: map[string]interface{}{
			"user_id":      event.UserID,
			"group_id":     event.GroupID,
			"message_type": event.MessageType,
		},
	}

	c.mu.Lock()
	c.totalMsg++
	c.lastMsgTime = time.Now()
	c.mu.Unlock()

	if c.onMessage != nil {
		c.onMessage(context.Background(), msg)
	}

	w.WriteHeader(http.StatusOK)
}

func (c *QQChannel) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	if c.wsConn != nil {
		_ = c.wsConn.Close()
		c.wsConn = nil
	}

	if c.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.server.Shutdown(ctx)
	}

	c.isRunning = false
	c.status.Running = false

	c.logger.Info(i18n.T("adapter.qq_stopped"))
	return nil
}

func (c *QQChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

func (c *QQChannel) SetOnMessage(callback func(context.Context, *entity.IncomingMessage)) {
	c.onMessage = callback
}

func (c *QQChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("qq").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *QQChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("QQ Channel is not running")
	}

	if c.config.WebSocketURL != "" && c.wsConn != nil {
		return c.sendViaWebSocket(ctx, msg)
	}

	return c.sendViaHTTP(ctx, msg)
}

func (c *QQChannel) sendViaWebSocket(ctx context.Context, msg *entity.OutgoingMessage) error {
	c.mu.RLock()
	conn := c.wsConn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket connection not established")
	}

	var apiCall map[string]interface{}
	if len(msg.SessionID) > 6 && msg.SessionID[:6] == "group_" {
		var groupID int64
		fmt.Sscanf(msg.SessionID, "group_%d", &groupID)
		apiCall = map[string]interface{}{
			"action": "send_group_msg",
			"params": map[string]interface{}{
				"group_id": groupID,
				"message":  msg.Content,
			},
		}
	} else {
		var userID int64
		fmt.Sscanf(msg.SessionID, "%d", &userID)
		apiCall = map[string]interface{}{
			"action": "send_private_msg",
			"params": map[string]interface{}{
				"user_id": userID,
				"message": msg.Content,
			},
		}
	}

	if err := conn.WriteJSON(apiCall); err != nil {
		return fmt.Errorf("send message via WebSocket failed: %w", err)
	}

	c.logger.Info(i18n.T("adapter.msg_send_ws_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

func (c *QQChannel) sendViaHTTP(ctx context.Context, msg *entity.OutgoingMessage) error {
	apiURL := fmt.Sprintf("http://localhost:%d", c.config.Port)

	var endpoint string
	var payload map[string]interface{}

	if len(msg.SessionID) > 6 && msg.SessionID[:6] == "group_" {
		var groupID int64
		fmt.Sscanf(msg.SessionID, "group_%d", &groupID)
		endpoint = "/send_group_msg"
		payload = map[string]interface{}{
			"group_id": groupID,
			"message":  msg.Content,
		}
	} else {
		var userID int64
		fmt.Sscanf(msg.SessionID, "%d", &userID)
		endpoint = "/send_private_msg"
		payload = map[string]interface{}{
			"user_id": userID,
			"message": msg.Content,
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.AccessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.AccessToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	c.logger.Info(i18n.T("adapter.msg_send_http_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

func (c *QQChannel) GetStatus() *entity.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.status.LastMessageTime = &c.lastMsgTime
	c.status.TotalMessages = c.totalMsg

	return c.status
}
