package channels

import (
	"context"
	"fmt"
	"sync"

	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// Gateway 网关
// 职责: 路由消息,处理 Channel 切换/转发,协调消息处理流程
type Gateway struct {
	manager           *ChannelManager
	channelContextMgr *ChannelContextManager
	defaultChan       string
	onMessage         func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error)
	embeddingSvc      *embedding.EmbeddingService
	channelVectors    map[string][]float64
	logger            logging.Logger
	convLogger        logging.Logger
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.RWMutex
	activeMessages    int
	shutdownWG        sync.WaitGroup
}

// NewGateway 创建网关
func NewGateway(defaultChannel string, embeddingSvc *embedding.EmbeddingService) *Gateway {
	if defaultChannel == "" {
		defaultChannel = "realtime"
	}

	ctx, cancel := context.WithCancel(context.Background())

	router := &Gateway{
		manager:           NewChannelManager(),
		channelVectors:    make(map[string][]float64),
		channelContextMgr: NewChannelContextManager(defaultChannel),
		defaultChan:       defaultChannel,
		embeddingSvc:      embeddingSvc,
		logger:            logging.GetSystemLogger().Named("channel_router"),
		convLogger:        logging.GetConversationLogger(),
		ctx:               ctx,
		cancel:            cancel,
	}

	if embeddingSvc != nil {
		router.precomputeChannelVectors()
	}

	return router
}

// Manager 获取 Channel 管理器
func (r *Gateway) Manager() *ChannelManager {
	return r.manager
}

// ChannelContextManager 获取 Channel 上下文管理器
func (r *Gateway) ChannelContextManager() *ChannelContextManager {
	return r.channelContextMgr
}

func (r *Gateway) SetOnMessage(callback func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error)) {
	r.onMessage = callback
}

func (r *Gateway) HandleMessage(ctx context.Context, msg *entity.IncomingMessage) {
	// 检查是否正在关闭
	r.mu.RLock()
	isShuttingDown := r.ctx.Err() != nil
	r.mu.RUnlock()

	if isShuttingDown {
		r.logger.Debug("Router is shutting down, rejecting message",
			logging.String("session_id", msg.SessionID),
		)
		return
	}

	// 增加活跃消息计数
	r.mu.Lock()
	r.activeMessages++
	r.mu.Unlock()

	r.shutdownWG.Add(1)
	defer func() {
		r.mu.Lock()
		r.activeMessages--
		r.mu.Unlock()
		r.shutdownWG.Done()
	}()

	// 记录系统日志
	r.logger.Debug(i18n.T("adapter.handle_msg"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.String(i18n.T("adapter.channel_id"), msg.ChannelID),
		logging.String("content", msg.Content),
	)

	// 记录对话日志
	r.convLogger.Info("收到消息",
		logging.String("session_id", msg.SessionID),
		logging.String("message_id", msg.MessageID),
		logging.String("user_id", msg.Sender.ID),
		logging.String("user_name", msg.Sender.Name),
		logging.String("channel_id", msg.ChannelID),
		logging.String("channel_name", msg.ChannelName),
		logging.String("direction", "incoming"),
		logging.String("content", msg.Content),
		logging.String("content_type", msg.ContentType),
	)

	// 1. 确保 Channel 会话上下文存在
	r.channelContextMgr.Ensure(msg.SessionID, msg.ChannelID)

	// 2. 获取当前会话的 Channel
	currentChannel := r.channelContextMgr.CurrentChannel(msg.SessionID)

	// 3. 同步接收的消息到 RealTimeChannel (信息流畅性）
	// 如果消息不是来自 RealTimeChannel,则同步一份到 RealTimeChannel
	if msg.ChannelID != "realtime" {
		r.syncToRealTimeChannel(ctx, msg.ChannelID, msg.SessionID,
			fmt.Sprintf("%s: %s", msg.Sender.Name, msg.Content), "接收")
	}

	var eventChan chan<- entity.ThinkingEvent
	if realtime, err := r.manager.Get("realtime"); err == nil {
		if rtc, ok := realtime.(*RealTimeChannel); ok {
			eventChan = rtc.GetEventChan(msg.SessionID)
		}
	}

	answer, sendTo, err := r.onMessage(ctx, msg, eventChan)
	if err != nil {
		r.logger.Error(i18n.T("adapter.msg_process_failed"),
			logging.String(i18n.T("adapter.session_id"), msg.SessionID),
			logging.Err(err),
		)
		r.sendErrorResponse(ctx, msg, err)
		return
	}

	// 1. 发送响应到当前 Channel
	if answer != "" {
		// 同步到 RealTimeChannel（保持信息流畅性）
		// 如果当前 Channel 不是 RealTimeChannel，则同步消息
		if msg.ChannelID != "realtime" {
			r.syncToRealTimeChannel(ctx, msg.ChannelID, msg.SessionID, answer, "回复")
		}

		// 发送到当前 Channel
		if err := r.sendToChannel(ctx, msg.ChannelID, msg.SessionID, answer); err != nil {
			r.logger.Error(i18n.T("adapter.send_response_failed"),
				logging.String(i18n.T("adapter.channel_id"), msg.ChannelID),
				logging.String(i18n.T("adapter.session_id"), msg.SessionID),
				logging.Err(err),
			)
		} else {
			// 记录回复的对话日志
			r.convLogger.Info("发送回复",
				logging.String("session_id", msg.SessionID),
				logging.String("channel_id", msg.ChannelID),
				logging.String("direction", "outgoing"),
				logging.String("content", answer),
				logging.String("content_type", "text"),
			)
		}
	}

	// 2. 处理 SendTo 转发
	// 规则：SendTo 为空或无法匹配时，不进行转发
	if sendTo == "" {
		r.logger.Debug(i18n.T("adapter.sendto_empty_skip"),
			logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		)
		return
	}

	r.logger.Info(i18n.T("adapter.forward_intent"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.String("source_channel", msg.ChannelID),
		logging.String("send_to_raw", sendTo),
	)

	// 使用 EmbeddingService 语义匹配目标 Channel
	matchedChannel := r.matchChannelByVector(sendTo)
	if matchedChannel == "" {
		r.logger.Warn(i18n.T("adapter.target_channel_not_match"),
			logging.String("send_to", sendTo),
		)
		return
	}

	if matchedChannel == msg.ChannelID {
		r.logger.Info(i18n.T("adapter.same_channel_skip"))
		return
	}

	r.logger.Info(i18n.T("adapter.forward_to_target"),
		logging.String(i18n.T("adapter.session_id"), msg.SessionID),
		logging.String("from", msg.ChannelID),
		logging.String("to", matchedChannel),
		logging.String("content", answer),
	)

	// 发送转发消息到目标 Channel
	// 格式: [用户来源: ChannelName] answer
	forwardMessage := fmt.Sprintf("[转发消息 - 来自:%s 用户:%s] %s",
		msg.ChannelName,
		msg.Sender.Name,
		answer)

	// 同步转发消息到 RealTimeChannel（保持信息流畅性）
	if matchedChannel != "realtime" {
		r.syncToRealTimeChannel(ctx, matchedChannel, msg.SessionID, forwardMessage, "转发")
	}

	if err := r.sendToChannel(ctx, matchedChannel, msg.SessionID, forwardMessage); err != nil {
		r.logger.Error(i18n.T("adapter.forward_failed"),
			logging.String("to", matchedChannel),
			logging.String(i18n.T("adapter.session_id"), msg.SessionID),
			logging.Err(err),
		)
		// 发送失败提示到当前 Channel
		errorMsg := fmt.Sprintf("抱歉，转发到 %s 失败: %v", matchedChannel, err)
		if sendErr := r.sendToChannel(ctx, msg.ChannelID, msg.SessionID, errorMsg); sendErr != nil {
			r.logger.Error(i18n.T("adapter.send_error_failed"),
				logging.String(i18n.T("adapter.channel_id"), msg.ChannelID),
				logging.String(i18n.T("adapter.session_id"), msg.SessionID),
				logging.Err(sendErr),
			)
		}
	} else {
		// 记录转发的对话日志
		r.convLogger.Info("转发消息成功",
			logging.String("session_id", msg.SessionID),
			logging.String("from_channel", msg.ChannelID),
			logging.String("to_channel", matchedChannel),
			logging.String("content", forwardMessage),
			logging.String("direction", "forward"),
		)
	}

	// 3. 处理 Channel 切换：通过语义化匹配找出最相似的通道名称
	if answer != "" && r.embeddingSvc != nil {
		matchedChannel := r.matchChannelByVector(answer)
		if matchedChannel != "" && matchedChannel != currentChannel {
			if err := r.handleChannelSwitch(msg.SessionID, matchedChannel, "语义匹配切换"); err != nil {
				r.logger.Error(i18n.T("adapter.channel_switch_failed"),
					logging.String(i18n.T("adapter.session_id"), msg.SessionID),
					logging.String("from", currentChannel),
					logging.String("to", matchedChannel),
					logging.Err(err),
				)
			} else {
				r.logger.Info(i18n.T("adapter.channel_switch_success"),
					logging.String(i18n.T("adapter.session_id"), msg.SessionID),
					logging.String("from", currentChannel),
					logging.String("to", matchedChannel),
					logging.String("reason", "语义匹配"),
				)
			}
		}
	}
}

// handleChannelSwitch 处理 Channel 切换
func (r *Gateway) handleChannelSwitch(sessionID, targetChannel, reason string) error {
	switchInfo := &entity.ChannelSwitchInfo{
		Target: targetChannel,
		Reason: reason,
	}

	if !r.manager.Exists(switchInfo.Target) {
		return fmt.Errorf("目标 Channel %s 不存在", switchInfo.Target)
	}

	r.channelContextMgr.SetCurrentChannel(sessionID, switchInfo.Target, switchInfo.Reason)

	return nil
}

// syncToRealTimeChannel 同步消息到 RealTimeChannel
// 确保所有 Channel 的消息都能在 Web UI 和 Terminal UI 中看到
// messageType: 消息类型（"回复"、"转发"、"同步"）
func (r *Gateway) syncToRealTimeChannel(ctx context.Context, channelID, sessionID, content, messageType string) {
	rtChannel, err := r.manager.Get("realtime")
	if err != nil {
		r.logger.Debug(i18n.T("adapter.realtime_not_exist_skip"))
		return
	}

	if !rtChannel.IsRunning() {
		r.logger.Debug(i18n.T("adapter.realtime_not_running_skip"))
		return
	}

	// 获取源 Channel 的名称
	channel, err := r.manager.Get(channelID)
	if err != nil {
		r.logger.Debug(i18n.T("adapter.get_source_channel_failed"), logging.String(i18n.T("adapter.channel_id"), channelID))
		return
	}

	// 构建同步消息
	// 格式: [ChannelName - 类型] Content
	syncContent := fmt.Sprintf("[%s - %s] %s",
		channel.Name(),
		messageType,
		content)

	// 发送到 RealTimeChannel
	outMsg := &entity.OutgoingMessage{
		ChannelID:   "realtime",
		SessionID:   sessionID,
		Content:     syncContent,
		ContentType: "text",
	}

	if err := rtChannel.SendMessage(ctx, outMsg); err != nil {
		r.logger.Error(i18n.T("adapter.sync_realtime_failed"), logging.Err(err))
	}
}

// sendToChannel 发送消息到指定 Channel
func (r *Gateway) sendToChannel(ctx context.Context, channelID, sessionID, content string) error {
	channel, err := r.manager.Get(channelID)
	if err != nil {
		return err
	}

	if !channel.IsRunning() {
		return fmt.Errorf("Channel %s 未运行", channelID)
	}

	outMsg := &entity.OutgoingMessage{
		ChannelID:   channelID,
		SessionID:   sessionID,
		Content:     content,
		ContentType: "text",
	}

	return channel.SendMessage(ctx, outMsg)
}

// sendErrorResponse 发送错误响应
func (r *Gateway) sendErrorResponse(ctx context.Context, msg *entity.IncomingMessage, err error) {
	errorMsg := fmt.Sprintf("抱歉,处理您的请求时出错: %v", err)
	if sendErr := r.sendToChannel(ctx, msg.ChannelID, msg.SessionID, errorMsg); sendErr != nil {
		r.logger.Error(i18n.T("adapter.send_error_response_failed"),
			logging.String(i18n.T("adapter.channel_id"), msg.ChannelID),
			logging.String(i18n.T("adapter.session_id"), msg.SessionID),
			logging.Err(sendErr),
		)
	}
}

// Broadcast 向所有 Channel 广播消息
func (r *Gateway) Broadcast(ctx context.Context, content string) {
	channels := r.manager.List()
	successCount := 0
	for name, channel := range channels {
		if !channel.IsRunning() {
			continue
		}

		outMsg := &entity.OutgoingMessage{
			ChannelID:   name,
			Content:     content,
			ContentType: "text",
		}

		if err := channel.SendMessage(ctx, outMsg); err != nil {
			r.logger.Error(i18n.T("adapter.broadcast_failed"),
				logging.String("channel", name),
				logging.Err(err),
			)
		} else {
			successCount++
		}
	}

	r.logger.Debug(i18n.T("adapter.broadcast_complete"),
		logging.String("content", content),
		logging.Int(i18n.T("adapter.total"), len(channels)),
		logging.Int(i18n.T("adapter.success"), successCount),
	)
}

// precomputeChannelVectors 预计算所有Channel的向量
func (r *Gateway) precomputeChannelVectors() {
	channels := r.manager.List()
	for name, channel := range channels {
		text := fmt.Sprintf("%s %s", name, channel.Description())
		vector, err := r.embeddingSvc.GenerateEmbedding(text)
		if err != nil {
			r.logger.Warn(i18n.T("adapter.vectorize_channel_failed"),
				logging.String("channel", name),
				logging.Err(err),
			)
			continue
		}
		r.channelVectors[name] = vector
		r.logger.Debug(i18n.T("adapter.channel_vectorize_complete"),
			logging.String("channel", name),
			logging.Int("vector_dim", len(vector)),
		)
	}
}

// matchChannelByVector 通过向量相似度匹配最相似的Channel
func (r *Gateway) matchChannelByVector(target string) string {
	if r.embeddingSvc == nil {
		return ""
	}

	targetVec, err := r.embeddingSvc.GenerateEmbedding(target)
	if err != nil {
		r.logger.Debug(i18n.T("adapter.vectorize_text_failed"),
			logging.String("target", target),
			logging.Err(err),
		)
		return ""
	}

	bestMatch := ""
	bestScore := 0.7

	for channelName, channelVec := range r.channelVectors {
		score := utils.CalculateCosineSimilarity(targetVec, channelVec)
		if score > bestScore {
			bestScore = score
			bestMatch = channelName
		}
	}

	if bestMatch != "" {
		r.logger.Debug(i18n.T("adapter.channel_match_success"),
			logging.String("target", target),
			logging.String("matched", bestMatch),
			logging.Float64("score", bestScore),
		)
	}

	return bestMatch
}

// Shutdown 优雅关闭 ChannelRouter
// 等待所有正在处理的消息完成后再关闭
func (r *Gateway) Shutdown(ctx context.Context) error {
	r.logger.Info("ChannelRouter shutdown initiated...")

	// 1. 取消 context，停止接收新消息
	r.mu.Lock()
	r.cancel()
	r.mu.Unlock()

	// 2. 记录当前活跃消息数
	r.mu.RLock()
	activeCount := r.activeMessages
	r.mu.RUnlock()

	r.logger.Info("Waiting for active messages to complete",
		logging.Int("active_messages", activeCount),
	)

	// 3. 等待所有消息处理完成或超时
	done := make(chan struct{})
	go func() {
		r.shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("All messages processed gracefully")
		// 4. 停止所有 Channel
		if err := r.manager.StopAll(); err != nil {
			r.logger.Warn("Some channels failed to stop gracefully", logging.Err(err))
		}
		return nil
	case <-ctx.Done():
		r.logger.Warn("Shutdown timeout, forcing exit",
			logging.Int("remaining_messages", r.activeMessages),
		)
		return ctx.Err()
	}
}

// IsShuttingDown 检查是否正在关闭
func (r *Gateway) IsShuttingDown() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ctx.Err() != nil
}

// GetActiveMessageCount 获取当前正在处理的消息数
func (r *Gateway) GetActiveMessageCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeMessages
}
