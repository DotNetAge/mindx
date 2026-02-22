package channels

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func init() {
	// 注册钉钉 Channel 工厂函数
	Register("dingtalk", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewDingTalkChannel(&config.DingTalkConfig{
			Port:          getIntFromConfig(cfg, "port", 8084),
			Path:          getStringFromConfigWithDefault(cfg, "path", "/dingtalk/webhook"),
			AppKey:        getStringFromConfig(cfg, "app_key"),
			AppSecret:     getStringFromConfig(cfg, "app_secret"),
			AgentID:       getStringFromConfig(cfg, "agent_id"),
			EncryptKey:    getStringFromConfig(cfg, "encrypt_key"),
			WebhookSecret: getStringFromConfig(cfg, "webhook_secret"),
		}), nil
	})
}

// DingTalkChannel 钉钉机器人 Channel
type DingTalkChannel struct {
	*WebhookChannel
	config       *config.DingTalkConfig
	accessToken  string
	tokenExpires time.Time
	tokenMutex   sync.RWMutex
	httpClient   *http.Client
}

// NewDingTalkChannel 创建钉钉 Channel
func NewDingTalkChannel(cfg *config.DingTalkConfig) *DingTalkChannel {
	if cfg == nil {
		cfg = &config.DingTalkConfig{
			Port: 8084,
			Path: "/dingtalk/webhook",
		}
	}

	baseChannel := NewWebhookChannel("dingtalk", entity.ChannelTypeDingTalk, cfg.Path, cfg)

	return &DingTalkChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Description 返回 Channel 描述
func (c *DingTalkChannel) Description() string {
	return "钉钉机器人 Webhook Channel"
}

// Start 启动钉钉 Channel (覆盖父类方法以使用自定义端口)
func (c *DingTalkChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("DingTalkChannel is not initialized")
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleDingTalkWebhook)

	c.WebhookChannel.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", c.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 调用父类的启动逻辑
	if err := c.WebhookChannel.Start(ctx); err != nil {
		return err
	}

	c.logger.Info(i18n.T("adapter.dingtalk_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
	)

	return nil
}

// SendMessage 发送消息到钉钉 Channel
func (c *DingTalkChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("dingtalk").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *DingTalkChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("DingTalkChannel is not running")
	}

	if c.config.WebhookSecret != "" {
		return c.sendViaWebhook(ctx, msg)
	}

	if c.config.AppKey != "" && c.config.AppSecret != "" {
		return c.sendViaAPI(ctx, msg)
	}

	return fmt.Errorf("DingTalk WebhookSecret or AppKey/AppSecret not configured")
}

// sendViaWebhook 通过Webhook发送消息
func (c *DingTalkChannel) sendViaWebhook(ctx context.Context, msg *entity.OutgoingMessage) error {
	webhookURL := c.config.WebhookSecret
	if !strings.HasPrefix(webhookURL, "http") {
		webhookURL = fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", webhookURL)
	}

	timestamp := time.Now().UnixMilli()

	if c.config.EncryptKey != "" {
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, c.config.EncryptKey)
		h := hmac.New(sha256.New, []byte(c.config.EncryptKey))
		h.Write([]byte(stringToSign))
		signData := h.Sum(nil)
		sign := base64.StdEncoding.EncodeToString(signData)
		sign = url.QueryEscape(sign)

		if strings.Contains(webhookURL, "?") {
			webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, sign)
		} else {
			webhookURL = fmt.Sprintf("%s?timestamp=%d&sign=%s", webhookURL, timestamp, sign)
		}
	}

	message := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": msg.Content,
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("DingTalk API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

// sendViaAPI 通过API发送消息
func (c *DingTalkChannel) sendViaAPI(ctx context.Context, msg *entity.OutgoingMessage) error {
	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", accessToken)

	message := map[string]interface{}{
		"agent_id":    c.config.AgentID,
		"userid_list": msg.SessionID,
		"msg": map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": msg.Content,
			},
		},
	}

	jsonData, err := json.Marshal(message)
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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("DingTalk API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

// getAccessToken 获取钉钉 access_token
func (c *DingTalkChannel) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMutex.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpires) {
		token := c.accessToken
		c.tokenMutex.RUnlock()
		return token, nil
	}
	c.tokenMutex.RUnlock()

	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpires) {
		return c.accessToken, nil
	}

	apiURL := fmt.Sprintf(
		"https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s",
		url.QueryEscape(c.config.AppKey),
		url.QueryEscape(c.config.AppSecret),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("DingTalk API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpires = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	c.logger.Info(i18n.T("adapter.get_access_token_success"),
		logging.Int("expires_in", result.ExpiresIn),
	)

	return c.accessToken, nil
}

// parseWebhookMessage 解析钉钉 Webhook 消息
func (c *DingTalkChannel) parseWebhookMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	return c.parseDingTalkMessage(body, r)
}

// handleDingTalkWebhook 处理钉钉 Webhook 请求
func (c *DingTalkChannel) handleDingTalkWebhook(w http.ResponseWriter, r *http.Request) {
	// 验证请求方法
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

	// 验证签名
	if c.config.EncryptKey != "" {
		timestamp := r.Header.Get("timestamp")
		sign := r.Header.Get("sign")
		if !c.verifyDingTalkSignature(timestamp, sign) {
			c.logger.Warn(i18n.T("adapter.dingtalk_sign_verify_failed"),
				logging.String("timestamp", timestamp),
				logging.String("sign", sign),
			)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// 解析钉钉消息
	msg, err := c.parseDingTalkMessage(body, r)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_dingtalk_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 更新统计
	c.WebhookChannel.mu.Lock()
	c.WebhookChannel.totalMsg++
	c.WebhookChannel.lastMsgTime = time.Now()
	c.WebhookChannel.mu.Unlock()

	// 调用消息回调
	if c.WebhookChannel.onMessage != nil {
		ctx := context.Background()
		c.WebhookChannel.onMessage(ctx, msg)
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
}

// verifyDingTalkSignature 验证钉钉签名
func (c *DingTalkChannel) verifyDingTalkSignature(timestamp, sign string) bool {
	if c.config.EncryptKey == "" {
		return true
	}

	stringToSign := fmt.Sprintf("%s\n%s", timestamp, c.config.EncryptKey)
	h := hmac.New(sha256.New, []byte(c.config.EncryptKey))
	h.Write([]byte(stringToSign))
	signData := h.Sum(nil)
	expectedSign := base64.StdEncoding.EncodeToString(signData)
	expectedSign = url.QueryEscape(expectedSign)

	return sign == expectedSign
}

// parseDingTalkMessage 解析钉钉消息
func (c *DingTalkChannel) parseDingTalkMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	var dingMsg DingTalkMessage
	if err := json.Unmarshal(body, &dingMsg); err != nil {
		return nil, fmt.Errorf("解析钉钉 JSON 失败: %w", err)
	}

	// 转换为内部消息格式
	return &entity.IncomingMessage{
		ChannelID:   c.Name(),
		ChannelName: c.Name(),
		SessionID:   dingMsg.SenderID,
		MessageID:   dingMsg.MsgID,
		Sender: &entity.MessageSender{
			ID:   dingMsg.SenderID,
			Name: dingMsg.SenderNick,
			Type: "user",
		},
		Content:     dingMsg.Text.Content,
		ContentType: "text",
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"chatbot_corpid": dingMsg.ChatbotCorpID,
			"chattype":       dingMsg.ChatType,
		},
	}, nil
}

// DingTalkMessage 钉钉消息结构
type DingTalkMessage struct {
	MsgID         string       `json:"msgId"`
	SenderID      string       `json:"senderId"`
	SenderNick    string       `json:"senderNick"`
	ChatbotCorpID string       `json:"chatbotCorpId"`
	ChatType      string       `json:"chatType"`
	ChatID        string       `json:"chatId"`
	Text          DingTalkText `json:"text"`
}

type DingTalkText struct {
	Content string `json:"content"`
}
