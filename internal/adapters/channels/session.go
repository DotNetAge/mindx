package channels

import (
	"sync"
	"time"

	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// ChannelContextManager Channel 上下文管理器
// 职责: 管理会话的 Channel 状态,记录每个会话当前使用的 Channel
// 注意: 这与 usecase.MessageHistoryManager 不同,后者管理对话消息历史
type ChannelContextManager struct {
	sessions      map[string]*SessionContext
	sessionsMutex sync.RWMutex
	defaultChan   string
	logger        logging.Logger
}

// SessionContext 会话上下文
type SessionContext struct {
	SessionID      string    // 会话 ID
	CurrentChannel string    // 当前 Channel 名称
	CreatedAt      time.Time // 创建时间
	UpdatedAt      time.Time // 最后更新时间
}

// NewChannelContextManager 创建 Channel 上下文管理器
func NewChannelContextManager(defaultChannel string) *ChannelContextManager {
	return &ChannelContextManager{
		sessions:    make(map[string]*SessionContext),
		defaultChan: defaultChannel,
		logger:      logging.GetSystemLogger().Named("channel_session"),
	}
}

// SetDefaultChannel 设置默认 Channel
func (ccm *ChannelContextManager) SetDefaultChannel(name string) {
	ccm.defaultChan = name
}

// Get 获取会话上下文
func (ccm *ChannelContextManager) Get(sessionID string) *SessionContext {
	ccm.sessionsMutex.RLock()
	defer ccm.sessionsMutex.RUnlock()

	if ctx, exists := ccm.sessions[sessionID]; exists {
		return ctx
	}

	// 如果不存在,返回默认上下文
	return &SessionContext{
		SessionID:      sessionID,
		CurrentChannel: ccm.defaultChan,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// Set 设置会话上下文
func (ccm *ChannelContextManager) Set(sessionID, channelID string) {
	ccm.sessionsMutex.Lock()
	defer ccm.sessionsMutex.Unlock()

	now := time.Now()
	if ctx, exists := ccm.sessions[sessionID]; exists {
		oldChannel := ctx.CurrentChannel
		ctx.CurrentChannel = channelID
		ctx.UpdatedAt = now
		ccm.logger.Info(i18n.T("adapter.session_switch"),
			logging.String(i18n.T("adapter.session_id"), sessionID),
			logging.String("from", oldChannel),
			logging.String("to", channelID),
		)
	} else {
		ccm.sessions[sessionID] = &SessionContext{
			SessionID:      sessionID,
			CurrentChannel: channelID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		ccm.logger.Info(i18n.T("adapter.session_new"),
			logging.String(i18n.T("adapter.session_id"), sessionID),
			logging.String("channel", channelID),
		)
	}
}

// Ensure 确保会话上下文存在
func (ccm *ChannelContextManager) Ensure(sessionID, channelID string) *SessionContext {
	ccm.sessionsMutex.Lock()
	defer ccm.sessionsMutex.Unlock()

	now := time.Now()
	if ctx, exists := ccm.sessions[sessionID]; exists {
		return ctx
	}

	ctx := &SessionContext{
		SessionID:      sessionID,
		CurrentChannel: channelID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	ccm.sessions[sessionID] = ctx
	ccm.logger.Info(i18n.T("adapter.session_ensure"),
		logging.String(i18n.T("adapter.session_id"), sessionID),
		logging.String("channel", channelID),
	)
	return ctx
}

// Delete 删除会话上下文
func (ccm *ChannelContextManager) Delete(sessionID string) {
	ccm.sessionsMutex.Lock()
	defer ccm.sessionsMutex.Unlock()

	delete(ccm.sessions, sessionID)
	ccm.logger.Info(i18n.T("adapter.session_delete"),
		logging.String(i18n.T("adapter.session_id"), sessionID),
	)
}

// CurrentChannel 获取会话当前使用的 Channel
func (ccm *ChannelContextManager) CurrentChannel(sessionID string) string {
	ctx := ccm.Get(sessionID)
	return ctx.CurrentChannel
}

// SetCurrentChannel 设置会话当前使用的 Channel
func (ccm *ChannelContextManager) SetCurrentChannel(sessionID, channelID string, reason string) {
	oldChannel := ccm.CurrentChannel(sessionID)
	ccm.Set(sessionID, channelID)
	if reason != "" {
		ccm.logger.Info(i18n.T("adapter.session_switch"),
			logging.String(i18n.T("adapter.session_id"), sessionID),
			logging.String("from", oldChannel),
			logging.String("to", channelID),
			logging.String("reason", reason),
		)
	}
}

// List 列出所有会话
func (ccm *ChannelContextManager) List() []*SessionContext {
	ccm.sessionsMutex.RLock()
	defer ccm.sessionsMutex.RUnlock()

	sessions := make([]*SessionContext, 0, len(ccm.sessions))
	for _, ctx := range ccm.sessions {
		sessions = append(sessions, ctx)
	}

	return sessions
}

// Count 获取会话数量
func (ccm *ChannelContextManager) Count() int {
	ccm.sessionsMutex.RLock()
	defer ccm.sessionsMutex.RUnlock()

	return len(ccm.sessions)
}

// Clear 清空所有会话
func (ccm *ChannelContextManager) Clear() {
	ccm.sessionsMutex.Lock()
	defer ccm.sessionsMutex.Unlock()

	count := len(ccm.sessions)
	ccm.sessions = make(map[string]*SessionContext)
	ccm.logger.Info(i18n.T("adapter.session_clear"),
		logging.Int("count", count),
	)
}
