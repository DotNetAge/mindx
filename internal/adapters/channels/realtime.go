package channels

import (
	"context"
	"fmt"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RealTimeChannel 实时通道 (基于 WebSocket)
type RealTimeChannel struct {
	server          *http.ServeMux
	port            int
	upgrader        websocket.Upgrader
	clients         map[*websocket.Conn]*entity.WebClient
	mutex           sync.RWMutex
	running         bool
	runMutex        sync.RWMutex
	onMessage       func(ctx context.Context, msg *entity.IncomingMessage)
	messageCount    int64
	lastMessage     *time.Time
	startTime       *time.Time
	logger          logging.Logger
	onThinkingEvent func(sessionID string, event map[string]any) // 思考流事件回调
}

// NewRealTimeChannel 创建 RealTimeChannel
func NewRealTimeChannel(port int) *RealTimeChannel {
	return &RealTimeChannel{
		port:   port,
		server: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
		clients: make(map[*websocket.Conn]*entity.WebClient),
		logger:  logging.GetSystemLogger().Named("channel.realtime"),
	}
}

// Name 返回 Channel 名称
func (w *RealTimeChannel) Name() string {
	return "realtime"
}

// Type 返回 Channel 类型
func (w *RealTimeChannel) Type() entity.ChannelType {
	return entity.ChannelTypeRealTime
}

// Description 返回 Channel 描述
func (w *RealTimeChannel) Description() string {
	return "实时通道 (WebSocket, 支持 Web UI 和 Terminal UI)"
}

// Start 启动 Channel
func (w *RealTimeChannel) Start(ctx context.Context) error {
	w.runMutex.Lock()
	defer w.runMutex.Unlock()

	if w.running {
		return fmt.Errorf("RealTimeChannel is already running")
	}

	// 注册 WebSocket 处理函数
	w.server.HandleFunc("/ws", w.handleConnection)

	// 记录启动时间
	now := time.Now()
	w.startTime = &now

	// 启动 HTTP 服务器
	serverAddr := fmt.Sprintf(":%d", w.port)
	go func() {
		if err := http.ListenAndServe(serverAddr, w.server); err != nil {
			w.logger.Error(i18n.T("adapter.http_server_error"), logging.Err(err))
			w.running = false
		}
	}()

	w.running = true
	w.logger.Info(i18n.T("adapter.realtime_started"), logging.Int(i18n.T("adapter.port"), w.port))

	return nil
}

// Stop 停止 Channel
func (w *RealTimeChannel) Stop() error {
	w.runMutex.Lock()
	defer w.runMutex.Unlock()

	if !w.running {
		return nil
	}

	// 关闭所有客户端连接
	w.mutex.Lock()
	for conn := range w.clients {
		if err := conn.Close(); err != nil {
			w.logger.Error(i18n.T("adapter.close_conn_failed"), logging.Err(err))
		}
		delete(w.clients, conn)
	}
	w.mutex.Unlock()

	w.running = false
	w.logger.Info(i18n.T("adapter.realtime_stopped"))

	return nil
}

// IsRunning 返回 Channel 是否正在运行
func (w *RealTimeChannel) IsRunning() bool {
	w.runMutex.RLock()
	defer w.runMutex.RUnlock()
	return w.running
}

// SetOnMessage 设置消息接收回调
func (w *RealTimeChannel) SetOnMessage(callback func(ctx context.Context, msg *entity.IncomingMessage)) {
	w.onMessage = callback
}

// SetOnThinkingEvent 设置思考流事件回调
func (w *RealTimeChannel) SetOnThinkingEvent(callback func(sessionID string, event map[string]any)) {
	w.onThinkingEvent = callback
}

// SendThinkingEvent 发送思考流事件到指定会话
func (w *RealTimeChannel) SendThinkingEvent(sessionID string, event map[string]any) error {
	if !w.IsRunning() {
		w.logger.Error("[思考流] RealTimeChannel 未运行",
			logging.String("session_id", sessionID))
		return fmt.Errorf("RealTimeChannel is not running")
	}

	w.mutex.RLock()
	defer w.mutex.RUnlock()

	sent := false
	for conn, client := range w.clients {
		if client.SessionID == sessionID {
			response := map[string]any{
				"type":      "thinking",
				"event":     event,
				"timestamp": time.Now().Unix(),
			}

			w.logger.Info("[思考流] WebSocket 发送思考事件",
				logging.String("session_id", sessionID),
				logging.Any("event_type", event["type"]),
				logging.Any("event_content", event["content"]))

			if err := conn.WriteJSON(response); err != nil {
				w.logger.Error(i18n.T("adapter.send_think_event_failed"),
					logging.String(i18n.T("adapter.session_id"), client.SessionID),
					logging.Err(err),
				)
				continue
			}

			client.LastActiveTime = time.Now()
			sent = true
		}
	}

	if !sent {
		w.logger.Warn("[思考流] 未找到会话的 WebSocket 连接",
			logging.String("session_id", sessionID))
		return fmt.Errorf("no active connection for session %s", sessionID)
	}

	return nil
}

// SendMessage 发送消息到 Channel
func (w *RealTimeChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	if !w.IsRunning() {
		return fmt.Errorf("RealTimeChannel is not running")
	}

	w.mutex.RLock()
	defer w.mutex.RUnlock()

	// 查找目标会话的所有连接
	sent := false
	for conn, client := range w.clients {
		if client.SessionID == msg.SessionID {
			// 发送消息
			response := map[string]any{
				"type":      "message",
				"content":   msg.Content,
				"timestamp": time.Now().Unix(),
			}

			if err := conn.WriteJSON(response); err != nil {
				w.logger.Error(i18n.T("adapter.send_msg_failed"),
					logging.String(i18n.T("adapter.session_id"), client.SessionID),
					logging.Err(err),
				)
				continue
			}

			client.LastActiveTime = time.Now()
			sent = true
		}
	}

	if !sent {
		return fmt.Errorf("no active connection for session %s", msg.SessionID)
	}

	return nil
}

func (w *RealTimeChannel) forwardThinkingEvents(client *entity.WebClient) {
	for event := range client.EventChan {
		response := map[string]any{
			"type":      "thinking",
			"event":     event,
			"timestamp": time.Now().Unix(),
		}

		w.logger.Info("[思考流] WebSocket 发送思考事件",
			logging.String("session_id", client.SessionID),
			logging.String("event_type", string(event.Type)),
			logging.String("event_content", event.Content))

		if err := client.Conn.WriteJSON(response); err != nil {
			w.logger.Error(i18n.T("adapter.send_think_event_failed"),
				logging.String(i18n.T("adapter.session_id"), client.SessionID),
				logging.Err(err),
			)
			return
		}
	}
}

func (w *RealTimeChannel) GetEventChan(sessionID string) chan<- entity.ThinkingEvent {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	for _, client := range w.clients {
		if client.SessionID == sessionID {
			return client.EventChan
		}
	}
	return nil
}

// GetStatus 获取 Channel 状态
func (w *RealTimeChannel) GetStatus() *entity.ChannelStatus {
	w.runMutex.RLock()
	defer w.runMutex.RUnlock()

	status := &entity.ChannelStatus{
		Name:          w.Name(),
		Type:          w.Type(),
		Description:   w.Description(),
		Running:       w.running,
		TotalMessages: w.messageCount,
	}

	if w.startTime != nil {
		status.StartTime = w.startTime
	}

	if w.lastMessage != nil {
		status.LastMessageTime = w.lastMessage
	}

	// 健康检查
	healthCheck := &entity.HealthCheck{
		Status:        "healthy",
		Message:       "OK",
		LastCheckTime: time.Now(),
		Latency:       0,
	}

	if w.running {
		w.mutex.RLock()
		clientCount := len(w.clients)
		w.mutex.RUnlock()

		if clientCount == 0 {
			healthCheck.Status = "degraded"
			healthCheck.Message = "No active connections"
		} else {
			healthCheck.Message = fmt.Sprintf("%d active connections", clientCount)
		}
	} else {
		healthCheck.Status = "unhealthy"
		healthCheck.Message = "Channel is not running"
	}

	status.HealthCheck = healthCheck

	return status
}

// handleConnection 处理 WebSocket 连接
func (w *RealTimeChannel) handleConnection(connResp http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := w.upgrader.Upgrade(connResp, r, nil)
	if err != nil {
		w.logger.Error(i18n.T("adapter.upgrade_ws_failed"), logging.Err(err))
		return
	}

	// 生成会话 ID (使用 URL 参数或随机生成)
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = fmt.Sprintf("web_%d", time.Now().UnixNano())
	}

	// 创建客户端
	client := &entity.WebClient{
		Conn:           conn,
		SessionID:      sessionID,
		ChannelID:      w.Name(), // 使用实际的 channel 名称 "realtime"
		ClientID:       fmt.Sprintf("client_%d", time.Now().UnixNano()),
		SenderID:       fmt.Sprintf("user_%d", time.Now().UnixNano()),
		SenderName:     "用户",
		LastActiveTime: time.Now(),
		EventChan:      make(chan entity.ThinkingEvent, 100),
	}

	w.mutex.Lock()
	w.clients[conn] = client
	w.mutex.Unlock()

	go w.forwardThinkingEvents(client)

	w.logger.Info(i18n.T("adapter.new_ws_conn"),
		logging.String(i18n.T("adapter.session_id"), sessionID),
		logging.String("client_id", client.ClientID),
	)

	// 发送欢迎消息
	if err := conn.WriteJSON(map[string]any{
		"type":      "connected",
		"sessionID": sessionID,
		"message":   "Connected to RealTimeChannel",
		"timestamp": time.Now().Unix(),
	}); err != nil {
		w.logger.Warn(i18n.T("adapter.send_welcome_failed"),
			logging.String(i18n.T("adapter.session_id"), sessionID),
			logging.Err(err),
		)
	}

	// 处理消息
	go w.handleMessages(client)
}

// handleMessages 处理客户端消息
func (w *RealTimeChannel) handleMessages(client *entity.WebClient) {
	defer func() {
		// 清理连接
		if err := client.Conn.Close(); err != nil {
			w.logger.Debug(i18n.T("adapter.close_ws_conn"),
				logging.String(i18n.T("adapter.session_id"), client.SessionID),
				logging.Err(err),
			)
		}
		w.mutex.Lock()
		delete(w.clients, client.Conn)
		w.mutex.Unlock()
		w.logger.Info(i18n.T("adapter.ws_conn_closed"), logging.String(i18n.T("adapter.session_id"), client.SessionID))
	}()

	for {
		// 读取消息
		var msgData map[string]any
		if err := client.Conn.ReadJSON(&msgData); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				w.logger.Error(i18n.T("adapter.read_msg_failed"), logging.Err(err))
			}
			break
		}

		// 更新最后活跃时间
		client.LastActiveTime = time.Now()

		// 处理消息类型
		msgType, ok := msgData["type"].(string)
		if !ok {
			msgType = "message"
		}

		switch msgType {
		case "ping":
			// 响应 ping
			if err := client.Conn.WriteJSON(map[string]any{
				"type":      "pong",
				"timestamp": time.Now().Unix(),
			}); err != nil {
				w.logger.Warn(i18n.T("adapter.send_pong_failed"),
					logging.String(i18n.T("adapter.session_id"), client.SessionID),
					logging.Err(err),
				)
			}

		case "message":
			// 处理消息
			content, ok := msgData["content"].(string)
			if !ok {
				continue
			}

			// 构建 IncomingMessage
			msg := &entity.IncomingMessage{
				ChannelID:   client.ChannelID,
				ChannelName: w.Name(),
				SessionID:   client.SessionID,
				MessageID:   fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Sender: &entity.MessageSender{
					ID:   client.SenderID,
					Name: client.SenderName,
					Type: "user",
				},
				Content:     content,
				ContentType: "text",
				Timestamp:   time.Now(),
			}

			// 更新统计
			w.messageCount++
			now := time.Now()
			w.lastMessage = &now

			// 调用消息回调
			if w.onMessage != nil {
				w.onMessage(context.Background(), msg)
			}

		default:
			w.logger.Debug(i18n.T("adapter.unknown_msg_type"), logging.String(i18n.T("adapter.msg_type"), msgType))
		}
	}
}

// GetActiveConnections 获取活跃连接数
func (w *RealTimeChannel) GetActiveConnections() int {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return len(w.clients)
}
