package config

// CapabilityConfig 能力配置
type CapabilityConfig struct {
	Capabilities      []Capability `mapstructure:"capabilities" json:"capabilities"`
	DefaultCapability string       `mapstructure:"default_capability" json:"default_capability"`
	FallbackToLocal   bool         `mapstructure:"fallback_to_local" json:"fallback_to_local"`
	Description       string       `mapstructure:"description" json:"description"`
}

// Capability 单个能力配置
type Capability struct {
	Name         string                 `mapstructure:"name" json:"name"`
	Title        string                 `mapstructure:"title" json:"title"`
	Icon         string                 `mapstructure:"icon" json:"icon"`
	Description  string                 `mapstructure:"description" json:"description"`
	Model        string                 `mapstructure:"model" json:"model"`
	BaseURL      string                 `mapstructure:"base_url" json:"base_url"`
	APIKey       string                 `mapstructure:"api_key" json:"api_key"`
	SystemPrompt string                 `mapstructure:"system_prompt" json:"system_prompt"`
	Tools        []string               `mapstructure:"tools" json:"tools"`
	Temperature  float64                `mapstructure:"temperature" json:"temperature"`
	MaxTokens    int                    `mapstructure:"max_tokens" json:"max_tokens"`
	Modality     []string               `mapstructure:"modality" json:"modality"`
	Enabled      bool                   `mapstructure:"enabled" json:"enabled"`
	Vector       []float64              `mapstructure:"vector" json:"vector"`
	Extra        map[string]interface{} `mapstructure:"-" json:"-"` // 额外字段
}
