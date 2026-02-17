package entity

// Capability 能力配置
type Capability struct {
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Icon         string    `json:"icon"`
	Description  string    `json:"description"`
	Model        string    `json:"model"`
	BaseURL      string    `json:"base_url"`
	APIKey       string    `json:"api_key"`
	SystemPrompt string    `json:"system_prompt"`
	Tools        []string  `json:"tools"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"max_tokens"`
	Modality     []string  `json:"modality"`
	Enabled      bool      `json:"enabled"`
	Vector       []float64 `json:"vector,omitempty"`
}
