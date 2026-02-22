package channels

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

func init() {
	// 注册微信 Channel 工厂函数
	Register("wechat", func(cfg map[string]interface{}) (core.Channel, error) {
		return NewWeChatChannel(&config.WeChatConfig{
			Port:           getIntFromConfig(cfg, "port", 8081),
			Path:           getStringFromConfigWithDefault(cfg, "path", "/wechat/webhook"),
			Type:           "mp",
			AppID:          getStringFromConfig(cfg, "app_id"),
			AppSecret:      getStringFromConfig(cfg, "app_secret"),
			Token:          getStringFromConfig(cfg, "token"),
			EncodingAESKey: getStringFromConfig(cfg, "encoding_aes_key"),
		}), nil
	})
}

// WeChatMessage 微信消息结构
type WeChatMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        int64    `xml:"MsgId"`
	Event        string   `xml:"Event"`
}

// WeChatChannel 微信公众号/企业微信 Channel
type WeChatChannel struct {
	*WebhookChannel
	config       *config.WeChatConfig
	accessToken  string
	tokenExpires time.Time
	tokenMutex   sync.RWMutex
	httpClient   *http.Client
}

// NewWeChatChannel 创建微信 Channel
func NewWeChatChannel(cfg *config.WeChatConfig) *WeChatChannel {
	if cfg == nil {
		cfg = &config.WeChatConfig{
			Port: 8081,
			Path: "/wechat/webhook",
			Type: "mp",
		}
	}

	baseChannel := NewWebhookChannel("wechat", entity.ChannelTypeWeChat, cfg.Path, cfg)

	return &WeChatChannel{
		WebhookChannel: baseChannel,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Description 返回 Channel 描述
func (c *WeChatChannel) Description() string {
	return "微信公众号/企业微信 Webhook Channel"
}

// Start 启动微信 Channel (覆盖父类方法以使用自定义端口)
func (c *WeChatChannel) Start(ctx context.Context) error {
	if c == nil || c.WebhookChannel == nil {
		return fmt.Errorf("WeChatChannel is not initialized")
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleRequest)

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

	c.logger.Info(i18n.T("adapter.wechat_started"),
		logging.Int(i18n.T("adapter.port"), c.config.Port),
		logging.String("path", c.config.Path),
		logging.String("type", c.config.Type),
	)

	return nil
}

// SendMessage 发送消息到微信 Channel
func (c *WeChatChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	return getBreaker("wechat").Execute(func() error {
		return c.doSendMessage(ctx, msg)
	})
}

func (c *WeChatChannel) doSendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("WeChatChannel is not running")
	}

	if c.config.AppID == "" || c.config.AppSecret == "" {
		return fmt.Errorf("WeChat AppID or AppSecret not configured")
	}

	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s", accessToken)

	message := map[string]interface{}{
		"touser":  msg.SessionID,
		"msgtype": "text",
		"text": map[string]string{
			"content": msg.Content,
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
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.logger.Info(i18n.T("adapter.msg_send_success"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.Int("content_length", len(msg.Content)),
	)

	return nil
}

// getAccessToken 获取微信 access_token
func (c *WeChatChannel) getAccessToken(ctx context.Context) (string, error) {
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
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		url.QueryEscape(c.config.AppID),
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
		return "", fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpires = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	c.logger.Info(i18n.T("adapter.get_access_token_success"),
		logging.Int("expires_in", result.ExpiresIn),
	)

	return c.accessToken, nil
}

// parseWebhookMessage 解析微信 Webhook 消息
func (c *WeChatChannel) parseWebhookMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	// 检查是否是验证请求
	if r.Method == "GET" {
		return c.handleVerificationRequest(r)
	}

	// 解析 XML 消息
	var wechatMsg WeChatMessage
	if err := xml.Unmarshal(body, &wechatMsg); err != nil {
		return nil, fmt.Errorf("解析微信消息失败: %w", err)
	}

	// 构建消息
	msg := &entity.IncomingMessage{
		ChannelID:   "wechat",
		ChannelName: "WeChat",
		MessageID:   fmt.Sprintf("wechat_%d", wechatMsg.MsgID),
		Sender: &entity.MessageSender{
			ID:   wechatMsg.FromUserName,
			Name: wechatMsg.FromUserName,
			Type: "user",
		},
		Content:     wechatMsg.Content,
		ContentType: "text",
		Timestamp:   time.Unix(wechatMsg.CreateTime, 0),
		Metadata: map[string]interface{}{
			"to_user":      wechatMsg.ToUserName,
			"message_type": wechatMsg.MsgType,
			"event":        wechatMsg.Event,
		},
	}

	// 生成会话 ID (使用发送者 ID)
	msg.SessionID = wechatMsg.FromUserName

	return msg, nil
}

// handleVerificationRequest 处理微信验证请求
func (c *WeChatChannel) handleVerificationRequest(r *http.Request) (*entity.IncomingMessage, error) {
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echostr := r.URL.Query().Get("echostr")

	// 验证签名
	params := []string{c.config.Token, timestamp, nonce}
	sort.Strings(params)
	sortStr := strings.Join(params, "")
	hash := sha1.Sum([]byte(sortStr))
	signatureCalculated := hex.EncodeToString(hash[:])

	if signature != signatureCalculated {
		c.logger.Warn(i18n.T("adapter.wechat_sign_verify_failed"))
		return nil, fmt.Errorf("invalid signature")
	}

	c.logger.Info(i18n.T("adapter.wechat_sign_verify_success"), logging.String("echostr", echostr))

	// 返回一个特殊的消息表示验证成功
	return &entity.IncomingMessage{
		ChannelID:   "wechat",
		ChannelName: "WeChat",
		MessageID:   "verification",
		SessionID:   "verification",
		Sender: &entity.MessageSender{
			ID:   "wechat_server",
			Name: "微信服务器",
			Type: "system",
		},
		Content:     echostr,
		ContentType: "text",
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"type":      "verification",
			"signature": signature,
			"timestamp": timestamp,
			"nonce":     nonce,
			"echostr":   echostr,
		},
	}, nil
}

// handleRequest 处理微信 Webhook 请求
func (c *WeChatChannel) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 更新统计
	c.WebhookChannel.mu.Lock()
	c.WebhookChannel.totalMsg++
	c.WebhookChannel.lastMsgTime = time.Now()
	c.WebhookChannel.mu.Unlock()

	// 验证请求方法
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error(i18n.T("adapter.read_body_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 解析消息
	msg, err := c.parseWebhookMessage(body, r)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_wechat_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 如果是验证请求，直接返回 echostr
	if r.Method == "GET" {
		if echostr, ok := msg.Metadata["echostr"].(string); ok {
			if _, err := w.Write([]byte(echostr)); err != nil {
				c.logger.Error(i18n.T("adapter.return_echostr_failed"), logging.Err(err))
			}
			return
		}
	}

	// 调用消息回调
	if c.WebhookChannel.onMessage != nil {
		ctx := context.Background()
		c.WebhookChannel.onMessage(ctx, msg)
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		c.logger.Error(i18n.T("adapter.return_response_failed"), logging.Err(err))
	}
}
