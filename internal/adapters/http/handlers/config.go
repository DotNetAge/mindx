package handlers

import (
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"net/http"
	"time"

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

func (h *ConfigHandler) OllamaSyncModels(c *gin.Context) {
	// 1. 获取Ollama上的所有模型
	ollamaModels, err := h.getOllamaModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get Ollama models: %v", err)})
		return
	}

	// 2. 加载现有模型配置
	modelsCfg, err := config.LoadModelsConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load models config: %v", err)})
		return
	}

	// 3. 同步模型配置
	updatedModels := h.syncModels(ollamaModels, modelsCfg)

	// 4. 保存更新后的配置
	if err := config.SaveModelsConfig(updatedModels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save models config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ollama models synced successfully"})
}

// getOllamaModels 从Ollama API获取所有模型
func (h *ConfigHandler) getOllamaModels() ([]map[string]interface{}, error) {
	// 从配置中读取Ollama地址，默认使用Ollama的标准默认地址
	ollamaURL := "http://localhost:11434"

	// 尝试从配置文件中获取Ollama地址
	if serverCfg, err := config.LoadServerConfig(); err == nil && serverCfg.OllamaURL != "" {
		ollamaURL = serverCfg.OllamaURL
	}

	// 创建HTTP客户端
	client := &http.Client{Timeout: 5 * time.Second}

	// 发送请求到Ollama API
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

// syncModels 同步Ollama模型到现有配置
func (h *ConfigHandler) syncModels(ollamaModels []map[string]interface{}, existingCfg *config.ModelsConfig) *config.ModelsConfig {
	// 从配置中读取Ollama地址，默认使用Ollama的标准默认地址
	ollamaURL := "http://localhost:11434"

	// 尝试从配置文件中获取Ollama地址
	if serverCfg, err := config.LoadServerConfig(); err == nil && serverCfg.OllamaURL != "" {
		ollamaURL = serverCfg.OllamaURL
	}

	// 创建模型名称到配置的映射
	modelMap := make(map[string]*config.ModelConfig)
	for i := range existingCfg.Models {
		modelMap[existingCfg.Models[i].Name] = &existingCfg.Models[i]
	}

	// 处理每个Ollama模型
	for _, ollamaModel := range ollamaModels {
		modelName, ok := ollamaModel["name"].(string)
		if !ok {
			continue
		}

		// 检查是否已存在配置
		if existingModel, exists := modelMap[modelName]; exists {
			// 更新现有配置
			existingModel.BaseURL = ollamaURL
		} else {
			// 添加新模型配置
			newModel := config.ModelConfig{
				Name:        modelName,
				BaseURL:     ollamaURL,
				MaxTokens:   4096, // 默认值
				Temperature: 0.7,  // 默认值
			}
			existingCfg.Models = append(existingCfg.Models, newModel)
		}
	}

	return existingCfg
}
