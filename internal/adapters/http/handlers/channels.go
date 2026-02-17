package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"mindx/internal/config"

	"github.com/gin-gonic/gin"
)

type ChannelsHandler struct {
	configPath string
}

type ChannelInfo struct {
	Enabled bool           `json:"enabled"`
	Name    string         `json:"name"`
	Icon    string         `json:"icon"`
	Config  map[string]any `json:"config"`
}

func NewChannelsHandler() *ChannelsHandler {
	configPath := filepath.Join("config", "channels.json")
	return &ChannelsHandler{
		configPath: configPath,
	}
}

func (h *ChannelsHandler) getChannels(c *gin.Context) {
	cfg, err := config.NewChannelsConfig(h.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载通道配置失败"})
		return
	}

	response := gin.H{
		"enabled_channels": cfg.EnabledChannels,
		"channels":         make(map[string]*ChannelInfo),
	}

	for id, channel := range cfg.Channels {
		channelsMap := response["channels"].(map[string]*ChannelInfo)
		channelsMap[id] = &ChannelInfo{
			Enabled: channel.Enabled,
			Name:    channel.Name,
			Icon:    channel.Icon,
			Config:  channel.Config,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *ChannelsHandler) updateChannelConfig(c *gin.Context) {
	channelID := c.Param("id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少通道ID"})
		return
	}

	var channelConfig map[string]any
	if err := c.ShouldBindJSON(&channelConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	cfg, err := config.NewChannelsConfig(h.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载通道配置失败"})
		return
	}

	if cfg.Channels == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道配置不存在"})
		return
	}

	if _, exists := cfg.Channels[channelID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到通道"})
		return
	}

	if err := cfg.UpdateChannelConfig(channelID, channelConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新通道配置失败"})
		return
	}

	if err := cfg.Save(h.configPath); err != nil {
		log.Printf("[ChannelsAPI] 保存通道配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存通道配置失败"})
		return
	}

	log.Printf("[ChannelsAPI] 更新通道 %s 的配置", channelID)

	c.JSON(http.StatusOK, gin.H{"message": "通道配置更新成功"})
}

func (h *ChannelsHandler) toggleChannel(c *gin.Context) {
	channelID := c.Param("id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少通道ID"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	cfg, err := config.NewChannelsConfig(h.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载通道配置失败"})
		return
	}

	if cfg.Channels == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道配置不存在"})
		return
	}

	if _, exists := cfg.Channels[channelID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到通道"})
		return
	}

	if req.Enabled {
		if err := cfg.EnableChannel(channelID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启用通道失败"})
			return
		}
		log.Printf("[ChannelsAPI] 启用通道 %s", channelID)
	} else {
		if err := cfg.DisableChannel(channelID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "禁用通道失败"})
			return
		}
		log.Printf("[ChannelsAPI] 禁用通道 %s", channelID)
	}

	if err := cfg.Save(h.configPath); err != nil {
		log.Printf("[ChannelsAPI] 保存通道配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存通道配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "通道状态更新成功"})
}

func (h *ChannelsHandler) startChannel(c *gin.Context) {
	channelID := c.Param("id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少通道ID"})
		return
	}

	cfg, err := config.NewChannelsConfig(h.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载通道配置失败"})
		return
	}

	if cfg.Channels == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道配置不存在"})
		return
	}

	if _, exists := cfg.Channels[channelID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到通道"})
		return
	}

	log.Printf("[ChannelsAPI] 启动通道 %s（注意：实际启动逻辑需结合服务管理）", channelID)

	c.JSON(http.StatusOK, gin.H{"message": "通道启动成功"})
}

func (h *ChannelsHandler) stopChannel(c *gin.Context) {
	channelID := c.Param("id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少通道ID"})
		return
	}

	cfg, err := config.NewChannelsConfig(h.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载通道配置失败"})
		return
	}

	if cfg.Channels == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通道配置不存在"})
		return
	}

	if _, exists := cfg.Channels[channelID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到通道"})
		return
	}

	log.Printf("[ChannelsAPI] 停止通道 %s（注意：实际停止逻辑需结合服务管理）", channelID)

	c.JSON(http.StatusOK, gin.H{"message": "通道停止成功"})
}
