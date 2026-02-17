package handlers

import (
	"mindx/internal/config"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	configPath string
}

type Settings struct {
	ModelConfig config.ModelConfig `json:"model_config"`
	AppConfig   AppConfig         `json:"app_config"`
}

type AppConfig struct {
	Theme               string `json:"theme"`
	Language            string `json:"language"`
	EnableNotifications bool   `json:"enable_notifications"`
	AutoSaveHistory     bool   `json:"auto_save_history"`
}

func NewSettingsHandler() *SettingsHandler {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".bot", "settings.json")

	return &SettingsHandler{
		configPath: configPath,
	}
}

func (h *SettingsHandler) getSettings(c *gin.Context) {
	settings, err := h.loadSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) saveSettings(c *gin.Context) {
	var settings Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.saveSettingsToFile(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings saved successfully"})
}

func (h *SettingsHandler) loadSettings() (*Settings, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return h.getDefaultSettings(), nil
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (h *SettingsHandler) saveSettingsToFile(settings *Settings) error {
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.configPath, data, 0644)
}

func (h *SettingsHandler) getDefaultSettings() *Settings {
	return &Settings{
		ModelConfig: config.ModelConfig{
			Name:        "llama3.2",
			Provider:    "ollama",
			Model:       "llama3.2",
			APIKey:      "",
			BaseURL:     "http://localhost:11434",
			Temperature: 0.7,
			MaxTokens:   2048,
		},
		AppConfig: AppConfig{
			Theme:               "dark",
			Language:            "zh-CN",
			EnableNotifications: true,
			AutoSaveHistory:     true,
		},
	}
}
