package config

import (
	"encoding/json"
	"os"
)

// ChannelsConfig 渠道配置
type ChannelsConfig struct {
	EnabledChannels []string           `json:"enabled_channels"`
	Channels        map[string]Channel `json:"channels"`
}

// Channel 渠道配置
type Channel struct {
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name"`
	Icon    string                 `json:"icon"`
	Config  map[string]interface{} `json:"config"`
}

// Load 从文件加载配置
func (c *ChannelsConfig) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

// Save 保存配置到文件
func (c *ChannelsConfig) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// EnableChannel 启用通道
func (c *ChannelsConfig) EnableChannel(channelID string) error {
	ch, ok := c.Channels[channelID]
	if !ok {
		return os.ErrNotExist
	}
	ch.Enabled = true
	c.Channels[channelID] = ch

	// 添加到启用列表
	if !contains(c.EnabledChannels, channelID) {
		c.EnabledChannels = append(c.EnabledChannels, channelID)
	}
	return nil
}

// DisableChannel 禁用通道
func (c *ChannelsConfig) DisableChannel(channelID string) error {
	ch, ok := c.Channels[channelID]
	if !ok {
		return os.ErrNotExist
	}
	ch.Enabled = false
	c.Channels[channelID] = ch

	// 从启用列表移除
	c.EnabledChannels = remove(c.EnabledChannels, channelID)
	return nil
}

// UpdateChannelConfig 更新通道配置
func (c *ChannelsConfig) UpdateChannelConfig(channelID string, config map[string]interface{}) error {
	ch, ok := c.Channels[channelID]
	if !ok {
		return os.ErrNotExist
	}
	ch.Config = config
	c.Channels[channelID] = ch
	return nil
}

// GetAllChannels 获取所有通道
func (c *ChannelsConfig) GetAllChannels() map[string]*Channel {
	result := make(map[string]*Channel)
	for id, ch := range c.Channels {
		chCopy := ch
		result[id] = &chCopy
	}
	return result
}

// IsChannelEnabled 检查通道是否启用
func (c *ChannelsConfig) IsChannelEnabled(channelID string) bool {
	return c.Channels[channelID].Enabled
}

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// remove 从切片中移除字符串
func remove(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// NewChannelsConfig 创建新的通道配置
func NewChannelsConfig(configPath string) (*ChannelsConfig, error) {
	cfg := &ChannelsConfig{
		Channels: make(map[string]Channel),
	}
	if err := cfg.Load(configPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return cfg, nil
}
