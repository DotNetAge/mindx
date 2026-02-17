package channels

import (
	"context"
	"sync"
	"time"

	"mindx/internal/entity"
)

// MockChannel 用于测试的模拟 Channel 实现
type MockChannel struct {
	name         string
	channelType  entity.ChannelType
	description  string
	running      bool
	onMessage    func(context.Context, *entity.IncomingMessage)
	sentMessages []*entity.OutgoingMessage
	startErr     error
	stopErr      error
	startCalled  bool
	stopCalled   bool
	startedChan  chan struct{} // 用于通知 Channel 已启动
	stoppedChan  chan struct{} // 用于通知 Channel 已停止
	messageChan  chan *entity.IncomingMessage
	mu           sync.RWMutex
}

// NewMockChannel 创建 Mock Channel
func NewMockChannel(name string, channelType entity.ChannelType, description string) *MockChannel {
	return &MockChannel{
		name:         name,
		channelType:  channelType,
		description:  description,
		running:      false,
		sentMessages: make([]*entity.OutgoingMessage, 0),
		startedChan:  make(chan struct{}, 1),
		stoppedChan:  make(chan struct{}, 1),
		messageChan:  make(chan *entity.IncomingMessage, 100),
	}
}

// Name 返回 Channel 名称
func (m *MockChannel) Name() string {
	return m.name
}

// Type 返回 Channel 类型
func (m *MockChannel) Type() entity.ChannelType {
	return m.channelType
}

// Description 返回 Channel 描述
func (m *MockChannel) Description() string {
	return m.description
}

// Start 启动 Channel
func (m *MockChannel) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	m.startCalled = true
	m.startErr = nil
	m.running = true

	// 重新创建 stoppedChan 如果已关闭
	select {
	case <-m.stoppedChan:
		m.stoppedChan = make(chan struct{}, 1)
	default:
	}

	// 模拟启动
	select {
	case m.startedChan <- struct{}{}:
	default:
	}

	// 启动消息接收循环
	go m.receiveLoop(ctx)

	return m.startErr
}

// receiveLoop 模拟接收消息的循环
func (m *MockChannel) receiveLoop(ctx context.Context) {
	for {
		select {
		case msg := <-m.messageChan:
			if m.onMessage != nil {
				m.onMessage(ctx, msg)
			}
		case <-ctx.Done():
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			close(m.stoppedChan)
			return
		}
	}
}

// Stop 停止 Channel
func (m *MockChannel) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.stopCalled = true
	m.running = false
	m.startErr = nil
	m.stopErr = nil

	return m.stopErr
}

// IsRunning 返回 Channel 是否正在运行
func (m *MockChannel) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// SetOnMessage 设置消息接收回调
func (m *MockChannel) SetOnMessage(callback func(context.Context, *entity.IncomingMessage)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessage = callback
}

// SendMessage 发送消息到 Channel
func (m *MockChannel) SendMessage(ctx context.Context, msg *entity.OutgoingMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return &ChannelError{
			Channel: m.name,
			Err:     "channel is not running",
		}
	}

	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

// GetStatus 获取 Channel 状态
func (m *MockChannel) GetStatus() *entity.ChannelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &entity.ChannelStatus{
		Name:          m.name,
		Type:          m.channelType,
		Description:   m.description,
		Running:       m.running,
		TotalMessages: int64(len(m.sentMessages)),
	}
}

// GetSentMessages 获取发送的消息列表（测试用）
func (m *MockChannel) GetSentMessages() []*entity.OutgoingMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	messages := make([]*entity.OutgoingMessage, len(m.sentMessages))
	copy(messages, m.sentMessages)
	return messages
}

// ClearSentMessages 清空发送的消息列表（测试用）
func (m *MockChannel) ClearSentMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = make([]*entity.OutgoingMessage, 0)
}

// SimulateIncomingMessage 模拟接收一条消息（测试用）
func (m *MockChannel) SimulateIncomingMessage(msg *entity.IncomingMessage) {
	m.messageChan <- msg
}

// SetStartError 设置启动错误（测试用）
func (m *MockChannel) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startErr = err
}

// SetStopError 设置停止错误（测试用）
func (m *MockChannel) SetStopError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopErr = err
}

// StartCalled 检查 Start 是否被调用（测试用）
func (m *MockChannel) StartCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.startCalled
}

// StopCalled 检查 Stop 是否被调用（测试用）
func (m *MockChannel) StopCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopCalled
}

// WaitUntilStarted 等待直到 Channel 启动（测试用）
func (m *MockChannel) WaitUntilStarted(timeout time.Duration) bool {
	select {
	case <-m.startedChan:
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitUntilStopped 等待直到 Channel 停止（测试用）
func (m *MockChannel) WaitUntilStopped(timeout time.Duration) bool {
	select {
	case <-m.stoppedChan:
		return true
	case <-time.After(timeout):
		return false
	}
}

// ChannelError Channel 错误
type ChannelError struct {
	Channel string
	Err     string
}

func (e *ChannelError) Error() string {
	return e.Channel + ": " + e.Err
}
