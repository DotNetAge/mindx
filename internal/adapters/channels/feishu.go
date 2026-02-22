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
	"sync"
	"time"
)

func init() {
	// 注册飞书 Channel 工厂函数
	Register("feishu", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewFeishuChannel(&config.FeishuConfig{
			Port:              getIntFromConfig(cfg, "port", 8080),
			Path:              getStringFromConfigWithDefault(cfg, "path", "/feishu/webhook"),
			AppID:             getStringFromConfig(cfg, "app_id"),
			AppSecret:         getStringFromConfig(cfg, "app_secret"),
			EncryptKey:        getStringFromConfig(cfg, "encrypt_key"),
			VerificationToken: getStringFromConfig(cfg, "verification_token"),
		}), nil
	})
}

// FeishuChannel 飞书机器人 Channel
type FeishuChannel struct {
	*WebhookChannel
	config       *config.FeishuConfig
	accessToken  string
	tokenExpires time.Time
	tokenMutex   sync.RWMutex
	httpClient   *http.Client
}

// NewFeishuChannel 创建飞书 Channel
func NewFeishuChannel(cfg *config.FeishuConfig) *FeishuChannel {
	if cfg == nil {
		cfg = &config.FeishuConfig{
			Port: 8080,
			Path: "/feishu/webhook",
		}
	}

	baseChannel := NewWebhookChannel("feishu", entity.ChannelTypeFeishu, cfg.Path, cfg)

	return &FeishuChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Description 返回 Channel 描述
func (c *FeishuChannel) Description() string {
	return "飞书机器人 Webhook Channel"
}

// Start 启动飞书 Channel (覆盖父类方法以使用自定义端口)
func (c *FeishuChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("FeishuChannel is not initialized")
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleFeishuWebhook)

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

	c.logger.Info(i18n.T("adapter.feishu_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
	)

	return nil
}

// SendMessage 发送消息到飞书 Channel
func (c *FeishuChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("feishu").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *FeishuChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("FeishuChannel is not running")
	}

	if c.config.AppID == "" || c.config.AppSecret == "" {
		return fmt.Errorf("Feishu AppID or AppSecret not configured")
	}

	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// 确定 receive_id_type，根据 metadata 或默认判断
	receiveIDType := "open_id" // 优先使用 open_id
	if msg.Metadata != nil {
		if chatType, ok := msg.Metadata["chat_type"].(string); ok && (chatType == "group" || chatType == "p2p") {
			receiveIDType = "chat_id"
		}
		if idType, ok := msg.Metadata["receive_id_type"].(string); ok {
			receiveIDType = idType
		}
	}

	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=%s", receiveIDType)

	message := map[string]interface{}{
		"receive_id": msg.SessionID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, msg.Content),
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

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
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.String("receive_id_type", receiveIDType),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

// getAccessToken 获取飞书 access_token
func (c *FeishuChannel) getAccessToken(ctx context.Context) (string, error) {
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

	apiURL := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"

	payload := map[string]string{
		"app_id":     c.config.AppID,
		"app_secret": c.config.AppSecret,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		AccessToken string `json:"tenant_access_token"`
		Expire      int    `json:"expire"`
		Code        int    `json:"code"`
		Msg         string `json:"msg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("Feishu API error: %d - %s", result.Code, result.Msg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpires = time.Now().Add(time.Duration(result.Expire-300) * time.Second)

	c.logger.Info(i18n.T("adapter.get_access_token_success"),
		logging.Int("expire", result.Expire),
	)

	return c.accessToken, nil
}

// parseWebhookMessage 解析飞书 Webhook 消息
func (c *FeishuChannel) parseWebhookMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	return c.parseFeishuMessage(body, r)
}

// handleFeishuWebhook 处理飞书 Webhook 请求
func (c *FeishuChannel) handleFeishuWebhook(w http.ResponseWriter, r *http.Request) {
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

	// 解析飞书消息
	msg, err := c.parseFeishuMessage(body, r)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_feishu_failed"), logging.Err(err))
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

// parseFeishuMessage 解析飞书消息
func (c *FeishuChannel) parseFeishuMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	// 验证签名
	timestamp := r.Header.Get("X-Lark-Request-Timestamp")
	nonce := r.Header.Get("X-Lark-Request-Nonce")
	signature := r.Header.Get("X-Lark-Signature")

	if !c.verifyFeishuSignature(string(body), timestamp, nonce, signature) {
		return nil, fmt.Errorf("飞书签名验证失败")
	}

	// 解析 JSON
	var feishuMsg FeishuMessage
	if err := json.Unmarshal(body, &feishuMsg); err != nil {
		return nil, fmt.Errorf("解析飞书 JSON 失败: %w", err)
	}

	// 解析消息内容
	content, ok := feishuMsg.Event.Content.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("消息内容格式错误")
	}

	text := ""
	if contentText, exists := content["text"]; exists {
		if textStr, ok := contentText.(string); ok {
			text = textStr
		}
	}

	// 获取发送者的各种 ID
	senderOpenID := feishuMsg.Event.Sender.SenderID.OpenID
	senderUserID := feishuMsg.Event.Sender.SenderID.UserID
	senderUnionID := feishuMsg.Event.Sender.SenderID.UnionID

	// 确定发送者 ID（优先使用 open_id）
	senderID := senderOpenID
	if senderID == "" {
		senderID = senderUserID
	}
	if senderID == "" {
		senderID = senderUnionID
	}

	// 确定会话 ID
	sessionID := senderID
	if feishuMsg.Event.ChatType == "group" || feishuMsg.Event.ChatType == "p2p" {
		if feishuMsg.Event.ChatID != "" {
			sessionID = feishuMsg.Event.ChatID
		}
	}

	// 转换为内部消息格式
	return &entity.IncomingMessage{
		ChannelID:   c.Name(),
		ChannelName: c.Name(),
		SessionID:   sessionID,
		MessageID:   feishuMsg.Event.MessageID,
		Sender: &entity.MessageSender{
			ID:   senderID,
			Name: senderID,
			Type: "user",
		},
		Content:     text,
		ContentType: "text",
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"timestamp":       timestamp,
			"nonce":           nonce,
			"chat_id":         feishuMsg.Event.ChatID,
			"chat_type":       feishuMsg.Event.ChatType,
			"sender_open_id":  senderOpenID,
			"sender_user_id":  senderUserID,
			"sender_union_id": senderUnionID,
		},
	}, nil
}

// verifyFeishuSignature 验证飞书签名
func (c *FeishuChannel) verifyFeishuSignature(body, timestamp, nonce, signature string) bool {
	if c.config.VerificationToken == "" {
		return true // 如果未配置 token,跳过验证
	}

	// 构造签名字符串
	signStr := fmt.Sprintf("%s\n%s\n%s", timestamp, nonce, body)

	// 计算签名
	h := hmac.New(sha256.New, []byte(c.config.VerificationToken))
	h.Write([]byte(signStr))
	signatureCalculated := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature == signatureCalculated
}

// FeishuMessage 飞书消息结构
type FeishuMessage struct {
	Header FeishuHeader       `json:"header"`
	Event  FeishuEventContent `json:"event"`
}

type FeishuHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	TenantKey  string `json:"tenant_key"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppType    string `json:"app_type"`
}

type FeishuEventContent struct {
	Sender    FeishuSender    `json:"sender"`
	MessageID string          `json:"message_id"`
	Content   interface{}     `json:"content"`
	ChatType  string          `json:"chat_type"`
	ChatID    string          `json:"chat_id"`
	Mention   []FeishuMention `json:"mention"`
}

type FeishuSender struct {
	SenderID   FeishuSenderID `json:"sender_id"`
	SenderType string         `json:"sender_type"`
	TenantKey  string         `json:"tenant_key"`
}

type FeishuSenderID struct {
	UnionID string `json:"union_id"`
	UserID  string `json:"user_id"`
	OpenID  string `json:"open_id"`
}

type FeishuMention struct {
	ID   string `string:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}
