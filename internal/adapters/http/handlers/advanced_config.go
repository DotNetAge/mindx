package handlers

import (
	"mindx/internal/config"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
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
	data, err := os.ReadFile(filepath.Join(h.configPath, "server.yml"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read server.yml"})
		return
	}

	var configFile ServerConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse server.yml"})
		return
	}

	srv := configFile.Server
	response := AdvancedConfigResponse{
		OllamaURL:  srv.OllamaURL,
		IndexModel: srv.IndexModel,
		Embedding:  srv.Embedding,
		Brain: BrainConfigResponse{
			Leftbrain: ModelConfigResponse{
				Name:        srv.Brain.LeftbrainModel.Name,
				Domain:      srv.Brain.LeftbrainModel.Domain,
				APIKey:      srv.Brain.LeftbrainModel.APIKey,
				BaseURL:     srv.Brain.LeftbrainModel.BaseURL,
				Temperature: srv.Brain.LeftbrainModel.Temperature,
				MaxTokens:   srv.Brain.LeftbrainModel.MaxTokens,
			},
			Rightbrain: ModelConfigResponse{
				Name:        srv.Brain.RightbrainModel.Name,
				Domain:      srv.Brain.RightbrainModel.Domain,
				APIKey:      srv.Brain.RightbrainModel.APIKey,
				BaseURL:     srv.Brain.RightbrainModel.BaseURL,
				Temperature: srv.Brain.RightbrainModel.Temperature,
				MaxTokens:   srv.Brain.RightbrainModel.MaxTokens,
			},
			TokenBudget: TokenBudgetResponse{
				ReservedOutputTokens: srv.Brain.TokenBudget.ReservedOutputTokens,
				MinHistoryRounds:     srv.Brain.TokenBudget.MinHistoryRounds,
				AvgTokensPerRound:    srv.Brain.TokenBudget.AvgTokensPerRound,
			},
		},
		Memory: MemoryConfigResponse{
			Enabled:      srv.Memory.Enabled,
			SummaryModel: srv.Memory.SummaryModel,
			KeywordModel: srv.Memory.KeywordModel,
			Schedule:     srv.Memory.Schedule,
		},
		VectorStore: VectorStoreResponse{
			Type:     srv.VectorStore.Type,
			DataPath: srv.VectorStore.DataPath,
		},
	}

	c.JSON(http.StatusOK, response)
}

func (h *AdvancedConfigHandler) SaveAdvancedConfig(c *gin.Context) {
	var req AdvancedConfigResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	data, err := os.ReadFile(filepath.Join(h.configPath, "server.yml"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read server.yml"})
		return
	}

	var configFile ServerConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse server.yml"})
		return
	}

	configFile.Server.OllamaURL = req.OllamaURL
	configFile.Server.IndexModel = req.IndexModel
	configFile.Server.Embedding = req.Embedding
	configFile.Server.Brain.LeftbrainModel = config.ModelConfig{
		Name:        req.Brain.Leftbrain.Name,
		Domain:      req.Brain.Leftbrain.Domain,
		APIKey:      req.Brain.Leftbrain.APIKey,
		BaseURL:     req.Brain.Leftbrain.BaseURL,
		Temperature: req.Brain.Leftbrain.Temperature,
		MaxTokens:   req.Brain.Leftbrain.MaxTokens,
	}
	configFile.Server.Brain.RightbrainModel = config.ModelConfig{
		Name:        req.Brain.Rightbrain.Name,
		Domain:      req.Brain.Rightbrain.Domain,
		APIKey:      req.Brain.Rightbrain.APIKey,
		BaseURL:     req.Brain.Rightbrain.BaseURL,
		Temperature: req.Brain.Rightbrain.Temperature,
		MaxTokens:   req.Brain.Rightbrain.MaxTokens,
	}
	configFile.Server.Brain.TokenBudget = config.TokenBudgetConfig{
		ReservedOutputTokens: req.Brain.TokenBudget.ReservedOutputTokens,
		MinHistoryRounds:     req.Brain.TokenBudget.MinHistoryRounds,
		AvgTokensPerRound:    req.Brain.TokenBudget.AvgTokensPerRound,
	}
	configFile.Server.Memory = config.MemoryConfig{
		Enabled:      req.Memory.Enabled,
		SummaryModel: req.Memory.SummaryModel,
		KeywordModel: req.Memory.KeywordModel,
		Schedule:     req.Memory.Schedule,
	}
	configFile.Server.VectorStore = config.VectorStoreConfig{
		Type:     req.VectorStore.Type,
		DataPath: req.VectorStore.DataPath,
	}

	outData, err := yaml.Marshal(&configFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal server.yml"})
		return
	}

	if err := os.WriteFile(filepath.Join(h.configPath, "server.yml"), outData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write server.yml"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved successfully"})
}
