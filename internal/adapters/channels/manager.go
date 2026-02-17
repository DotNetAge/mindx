package channels

import (
	"context"
	"fmt"
	"sync"

	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// ChannelManager Channel 管理器
// 职责: Channel 的生命周期管理(启动、停止、查询)
type ChannelManager struct {
	channels map[string]core.Channel
	mutex    sync.RWMutex
	logger   logging.Logger
}

// NewChannelManager 创建 Channel 管理器
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]core.Channel),
		logger:   logging.GetSystemLogger().Named("channel_manager"),
	}
}

// Get 获取 Channel
func (m *ChannelManager) Get(name string) (core.Channel, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	channel, exists := m.channels[name]
	if !exists {
		return nil, fmt.Errorf("channel %s not found", name)
	}

	return channel, nil
}

// List 列出所有 Channel
func (m *ChannelManager) List() map[string]core.Channel {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	result := make(map[string]core.Channel, len(m.channels))
	for k, v := range m.channels {
		result[k] = v
	}

	return result
}

// StopAll 停止所有 Channel
func (m *ChannelManager) StopAll() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	successCount := 0
	for name := range m.channels {
		channel := m.channels[name]
		if channel.IsRunning() {
			if err := channel.Stop(); err != nil {
				m.logger.Error(i18n.T("adapter.stop_channel_failed"),
					logging.String(i18n.T("adapter.name"), name),
					logging.Err(err),
				)
			} else {
				successCount++
			}
		}
	}

	m.logger.Info(i18n.T("adapter.batch_stop_complete"),
		logging.Int(i18n.T("adapter.total"), len(m.channels)),
		logging.Int(i18n.T("adapter.success"), successCount),
	)
	return nil
}

// Exists 检查 Channel 是否存在
func (m *ChannelManager) Exists(name string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.channels[name]
	return exists
}

// Count 获取 Channel 数量
func (m *ChannelManager) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.channels)
}

// AddChannel 添加 Channel
func (m *ChannelManager) AddChannel(channel core.Channel) {
	if channel == nil {
		return
	}

	name := channel.Name()
	if name == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.channels[name] = channel
	m.logger.Info(i18n.T("adapter.channel_added"),
		logging.String(i18n.T("adapter.name"), name),
		logging.String(i18n.T("adapter.type"), string(channel.Type())),
	)
}

// CreateAndStartChannel 创建并启动 Channel
func (m *ChannelManager) CreateAndStartChannel(channel core.Channel, onMessage func(context.Context, *entity.IncomingMessage), ctx context.Context) error {
	if channel == nil {
		return fmt.Errorf("channel cannot be nil")
	}

	// 添加到管理器
	m.AddChannel(channel)

	// 设置消息回调
	channel.SetOnMessage(onMessage)

	// 启动 Channel
	go func() {
		if err := channel.Start(ctx); err != nil {
			m.logger.Error(i18n.T("adapter.start_channel_failed"),
				logging.String(i18n.T("adapter.name"), channel.Name()),
				logging.Err(err),
			)
		}
	}()

	m.logger.Info(i18n.T("adapter.channel_started"), logging.String(i18n.T("adapter.name"), channel.Name()))
	return nil
}

// CreateChannelsFromConfig 根据配置创建并启动所有启用的 Channel
// 实现配置驱动的 Channel 初始化，避免硬编码创建逻辑
func (m *ChannelManager) CreateChannelsFromConfig(
	channelsCfg *config.ChannelsConfig,
	onMessage func(context.Context, *entity.IncomingMessage),
	ctx context.Context,
) error {
	if channelsCfg == nil || channelsCfg.Channels == nil {
		m.logger.Info(i18n.T("adapter.no_channel_config"))
		return nil
	}

	m.logger.Info(i18n.T("adapter.start_create_channels"),
		logging.Int(i18n.T("adapter.total"), len(channelsCfg.Channels)),
	)

	var wg sync.WaitGroup
	errChan := make(chan error, len(channelsCfg.Channels))
	createdCount := 0

	for name, channelCfg := range channelsCfg.Channels {
		if !channelCfg.Enabled {
			m.logger.Debug(i18n.T("adapter.channel_disabled_skip"), logging.String(i18n.T("adapter.name"), name))
			continue
		}

		wg.Add(1)
		go func(name string, channelCfg config.Channel) {
			defer wg.Done()

			// 从全局注册表获取工厂函数
			factory, ok := GetFactory(name)
			if !ok {
				m.logger.Warn(i18n.T("adapter.channel_factory_not_registered"),
					logging.String(i18n.T("adapter.name"), name),
				)
				return
			}

			// 使用工厂函数创建 Channel
			channel, err := factory(channelCfg.Config)
			if err != nil {
				errChan <- fmt.Errorf("创建 %s Channel 失败: %w", name, err)
				return
			}

			// 创建并启动 Channel
			if err := m.CreateAndStartChannel(channel, onMessage, ctx); err != nil {
				errChan <- fmt.Errorf("启动 %s Channel 失败: %w", name, err)
				return
			}

			createdCount++
		}(name, channelCfg)
	}

	wg.Wait()
	close(errChan)

	// 收集所有错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// 如果有错误，返回组合错误
	if len(errors) > 0 {
		m.logger.Error(i18n.T("adapter.create_channels_partial_failed"),
			logging.Int(i18n.T("adapter.success"), createdCount),
			logging.Int("failed", len(errors)),
		)
		return fmt.Errorf("创建 Channels 失败: %d 个成功, %d 个失败: %v",
			createdCount, len(errors), errors)
	}

	m.logger.Info(i18n.T("adapter.channels_created"),
		logging.Int("created", createdCount),
	)

	return nil
}
