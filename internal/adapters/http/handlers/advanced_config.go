package handlers

import (
	"mindx/internal/config"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type AdvancedConfigHandler struct {
	configPath string
}

func NewAdvancedConfigHandler() *AdvancedConfigHandler {
	configPath := getConfigPath()
	return &AdvancedConfigHandler{
		configPath: configPath,
	}
}

func getConfigPath() string {
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		return configPath
	}
	return "./config"
}

type AdvancedConfigResponse struct {
	OllamaURL   string               `json:"ollama_url"`
	Brain       BrainConfigResponse  `json:"brain"`
	IndexModel  string               `json:"index_model"`
	Embedding   string               `json:"embedding"`
	Memory      MemoryConfigResponse `json:"memory"`
	VectorStore VectorStoreResponse  `json:"vector_store"`
}

type BrainConfigResponse struct {
	Leftbrain   ModelConfigResponse `json:"leftbrain"`
	Rightbrain  ModelConfigResponse `json:"rightbrain"`
	TokenBudget TokenBudgetResponse `json:"token_budget"`
}

type ModelConfigResponse struct {
	Name        string  `json:"name"`
	Domain      string  `json:"domain"`
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type TokenBudgetResponse struct {
	ReservedOutputTokens int `json:"reserved_output_tokens"`
	MinHistoryRounds     int `json:"min_history_rounds"`
	AvgTokensPerRound    int `json:"avg_tokens_per_round"`
}

type MemoryConfigResponse struct {
	Enabled      bool   `json:"enabled"`
	SummaryModel string `json:"summary_model"`
	KeywordModel string `json:"keyword_model"`
	Schedule     string `json:"schedule"`
}

type VectorStoreResponse struct {
	Type     string `json:"type"`
	DataPath string `json:"data_path"`
}

type ServerConfigFile struct {
	Server config.GlobalConfig `yaml:"server"`
}

func (h *AdvancedConfigHandler) GetAdvancedConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Advanced config API needs to be rewritten for new config system"})
}

func (h *AdvancedConfigHandler) SaveAdvancedConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Advanced config API needs to be rewritten for new config system"})
}
