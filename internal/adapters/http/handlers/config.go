package handlers

import (
	"mindx/internal/config"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct{}

func NewConfigHandler() *ConfigHandler {
	return &ConfigHandler{}
}

func (h *ConfigHandler) GetServerConfig(c *gin.Context) {
	cfg, err := config.LoadServerConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"server": cfg})
}

func (h *ConfigHandler) SaveServerConfig(c *gin.Context) {
	var req struct {
		Server *config.GlobalConfig `json:"server"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.SaveServerConfig(req.Server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Server config saved successfully"})
}

func (h *ConfigHandler) GetModelsConfig(c *gin.Context) {
	cfg, err := config.LoadModelsConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": cfg})
}

func (h *ConfigHandler) SaveModelsConfig(c *gin.Context) {
	var req struct {
		Models *config.ModelsConfig `json:"models"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.SaveModelsConfig(req.Models); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Models config saved successfully"})
}

func (h *ConfigHandler) GetCapabilitiesConfig(c *gin.Context) {
	cfg, err := config.LoadCapabilitiesConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	modelsCfg, err := config.LoadModelsConfig()
	if err != nil {
		modelsCfg = &config.ModelsConfig{Models: []config.ModelConfig{}}
	}

	c.JSON(http.StatusOK, gin.H{
		"capabilities": cfg,
		"models":       modelsCfg,
	})
}

func (h *ConfigHandler) SaveCapabilitiesConfig(c *gin.Context) {
	var req struct {
		Capabilities *config.CapabilityConfig `json:"capabilities"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.SaveCapabilitiesConfig(req.Capabilities); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Capabilities config saved successfully"})
}

func (h *ConfigHandler) GetGeneralConfig(c *gin.Context) {
	serverCfg, err := config.LoadServerConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	workspacePath, err := config.GetWorkspacePath()
	if err != nil {
		workspacePath = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"workplace": workspacePath,
		"server": gin.H{
			"address": serverCfg.Host,
			"port":    serverCfg.Port,
		},
	})
}

func (h *ConfigHandler) SaveGeneralConfig(c *gin.Context) {
	var req struct {
		Workplace string `json:"workplace"`
		Server    struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
		} `json:"server"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	serverCfg, err := config.LoadServerConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	serverCfg.Host = req.Server.Address
	serverCfg.Port = req.Server.Port

	if err := config.SaveServerConfig(serverCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "General config saved successfully"})
}
