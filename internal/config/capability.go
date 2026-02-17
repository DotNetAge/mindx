package config

// CapabilityConfig 能力配置
type CapabilityConfig struct {
	Capabilities      []Capability `mapstructure:"capabilities" json:"capabilities" yaml:"capabilities"`
	DefaultCapability string       `mapstructure:"default_capability" json:"default_capability" yaml:"default_capability"`
	FallbackToLocal   bool         `mapstructure:"fallback_to_local" json:"fallback_to_local" yaml:"fallback_to_local"`
	Description       string       `mapstructure:"description" json:"description" yaml:"description"`
}

// Capability 单个能力配置
type Capability struct {
	Name         string                 `mapstructure:"name" json:"name" yaml:"name"`
	Title        string                 `mapstructure:"title" json:"title" yaml:"title"`
	Icon         string                 `mapstructure:"icon" json:"icon" yaml:"icon"`
	Description  string                 `mapstructure:"description" json:"description" yaml:"description"`
	Model        string                 `mapstructure:"model" json:"model" yaml:"model"`
	BaseURL      string                 `mapstructure:"base_url,omitempty" json:"base_url,omitempty" yaml:"base_url,omitempty"`
	APIKey       string                 `mapstructure:"api_key,omitempty" json:"api_key,omitempty" yaml:"api_key,omitempty"`
	SystemPrompt string                 `mapstructure:"system_prompt" json:"system_prompt" yaml:"system_prompt"`
	Tools        []string               `mapstructure:"tools" json:"tools" yaml:"tools"`
	Temperature  float64                `mapstructure:"temperature,omitempty" json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens    int                    `mapstructure:"max_tokens,omitempty" json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	Modality     []string               `mapstructure:"modality,omitempty" json:"modality,omitempty" yaml:"modality,omitempty"`
	Enabled      bool                   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Vector       []float64              `mapstructure:"vector" json:"vector" yaml:"vector"`
	Extra        map[string]interface{} `mapstructure:"-" json:"-" yaml:"-"` // 额外字段
}
