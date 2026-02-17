package entity

// Capability 能力配置
type Capability struct {
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Icon         string    `json:"icon"`
	Description  string    `json:"description"`
	Model        string    `json:"model"`
	SystemPrompt string    `json:"system_prompt"`
	Tools        []string  `json:"tools"`
	Modality     []string  `json:"modality"`
	Enabled      bool      `json:"enabled"`
	Vector       []float64 `json:"vector,omitempty"`
}
