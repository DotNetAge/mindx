package config

type ModelsConfig struct {
	Models []ModelConfig `mapstructure:"models" json:"models" yaml:"models"`
}

type BrainModelsConfig struct {
	SubconsciousLeftModel  string `mapstructure:"subconscious_left" json:"subconscious_left,omitempty" yaml:"subconscious_left"`
	SubconsciousRightModel string `mapstructure:"subconscious_right" json:"subconscious_right,omitempty" yaml:"subconscious_right"`
	ConsciousnessLeftModel string `mapstructure:"consciousness_left" json:"consciousness_left,omitempty" yaml:"consciousness_left"`
	ConsciousnessRightModel string `mapstructure:"consciousness_right" json:"consciousness_right,omitempty" yaml:"consciousness_right"`
}

type ModelConfig struct {
	Name        string  `mapstructure:"name" json:"name" yaml:"name"`
	Description string  `mapstructure:"description,omitempty" json:"description,omitempty" yaml:"description"`
	Domain      string  `mapstructure:"domain,omitempty" json:"domain,omitempty" yaml:"domain"`
	BaseURL     string  `mapstructure:"base_url" json:"base_url" yaml:"base_url"`
	APIKey      string  `mapstructure:"api_key" json:"api_key" yaml:"api_key"`
	Temperature float64 `mapstructure:"temperature" json:"temperature,omitempty" yaml:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens" json:"max_tokens,omitempty" yaml:"max_tokens"`
}

type TokenBudgetConfig struct {
	ReservedOutputTokens int `mapstructure:"reserved_output_tokens" json:"reserved_output_tokens" yaml:"reserved_output_tokens"`
	MinHistoryRounds     int `mapstructure:"min_history_rounds" json:"min_history_rounds" yaml:"min_history_rounds"`
	AvgTokensPerRound    int `mapstructure:"avg_tokens_per_round" json:"avg_tokens_per_round" yaml:"avg_tokens_per_round"`
}
