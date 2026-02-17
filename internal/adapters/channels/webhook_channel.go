package channels

import (
	"context"
	"fmt"
	"io"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"sync"
	"time"
)

// WebhookChannel 基于 Webhook 的 Channel
// 适用于通过 HTTP Webhook 接收消息的平台(微信、飞书、钉钉等)
type WebhookChannel struct {
	platformName string
	platformType entity.ChannelType
	config       interface{} // 平台特定配置
	server       *http.Server
	webhookPath  string
	onMessage    func(context.Context, *entity.IncomingMessage)
	mu           sync.RWMutex
	isRunning    bool
	startTime    time.Time
	totalMsg     int64
	lastMsgTime  time.Time
	status       *entity.ChannelStatus
	logger       logging.Logger
}

// NewWebhookChannel 创建 Webhook Channel
func NewWebhookChannel(platformName string, platformType entity.ChannelType, webhookPath string, config interface{}) *WebhookChannel {
	return &WebhookChannel{
		platformName: platformName,
		platformType: platformType,
		webhookPath:  webhookPath,
		config:       config,
		status: &entity.ChannelStatus{
			Name:    platformName,
			Type:    platformType,
			Running: false,
		},
		logger: logging.GetSystemLogger().Named("channel." + platformName),
	}
}

// Name 返回 Channel 名称
func (c *WebhookChannel) Name() string {
	return c.platformName
}

// Type 返回 Channel 类型
func (c *WebhookChannel) Type() entity.ChannelType {
	return c.platformType
}

// Description 返回 Channel 描述
func (c *WebhookChannel) Description() string {
	return fmt.Sprintf("%s Webhook Channel", c.platformName)
}

// Start 启动 Channel
func (c *WebhookChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return fmt.Errorf("%s channel is already running", c.platformName)
	}

	// 如果 server 已经被子类设置，使用子类设置的 server
	if c.server == nil {
		// 创建 HTTP 服务器
		mux := http.NewServeMux()
		mux.HandleFunc(c.webhookPath, c.handleWebhook)

		c.server = &http.Server{
			Addr:         ":8080", // 应该从配置读取
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
	}

	// 在 goroutine 中启动服务器
	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.logger.Error(i18n.T("adapter.webhook_server_error"), logging.Err(err))
		}
	}()

	c.isRunning = true
	c.startTime = time.Now()
	c.status.Running = true
	c.status.StartTime = &c.startTime

	// 监听上下文取消
	go func() {
		<-ctx.Done()
		_ = c.Stop() // 停止失败不阻塞
	}()

	c.logger.Info(i18n.T("adapter.webhook_started"),
		logging.String("address", c.server.Addr),
		logging.String("path", c.webhookPath),
	)
	return nil
}

// Stop 停止 Channel
func (c *WebhookChannel) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return nil
	}

	if c.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.server.Shutdown(ctx) // 关闭失败不阻塞
	}

	c.isRunning = false
	c.status.Running = false

	c.logger.Info(i18n.T("adapter.webhook_stopped"))
	return nil
}

// IsRunning 返回 Channel 是否正在运行
func (c *WebhookChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

// SetOnMessage 设置消息接收回调
func (c *WebhookChannel) SetOnMessage(callback func(context.Context, *entity.IncomingMessage)) {
	c.onMessage = callback
}

// SendMessage 发送消息到 Channel
// Webhook Channel 通常不支持主动发送消息(因为需要使用平台 API)
func (c *WebhookChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	// 大多数 Webhook Channel 不支持直接发送消息
	// 需要通过平台 API 发送,这应该由专门的 Channel 实现
	return fmt.Errorf("%s Webhook channel does not support direct message sending", c.platformName)
}

// GetStatus 获取 Channel 状态
func (c *WebhookChannel) GetStatus() *entity.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.status.LastMessageTime = &time.Time{} // 从 lastMsgTime 更新
	c.status.TotalMessages = c.totalMsg

	return c.status
}

// handleWebhook 处理 Webhook 请求
func (c *WebhookChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// 验证请求方法
	if r.Method != "POST" && r.Method != "GET" {
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

	// 解析平台特定格式
	msg, err := c.parseWebhookMessage(body, r)
	if err != nil {
		c.logger.Error(i18n.T("adapter.parse_webhook_failed"), logging.Err(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 更新统计
	c.mu.Lock()
	c.totalMsg++
	c.lastMsgTime = time.Now()
	c.mu.Unlock()

	// 调用消息回调
	if c.onMessage != nil {
		ctx := context.Background()
		c.onMessage(ctx, msg)
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("success")); err != nil {
		c.logger.Warn(i18n.T("adapter.return_response_failed"), logging.Err(err))
	}
}

// parseWebhookMessage 解析 Webhook 消息
// 这是一个通用框架,具体实现应该由子类或特定平台处理
func (c *WebhookChannel) parseWebhookMessage(body []byte, r *http.Request) (*entity.IncomingMessage, error) {
	// 这里应该是平台特定的解析逻辑
	// 例如: 微信 XML 解析、飞书 JSON 解析等
	return nil, fmt.Errorf("parseWebhookMessage not implemented for platform %s", c.platformName)
}
