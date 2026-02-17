package config

type ModelsConfig struct {
	Models         []ModelConfig  `mapstructure:"models" json:"models"`
	BrainModels    map[string]string `mapstructure:"brain_models,omitempty" json:"brain_models,omitempty"`
	EmbeddingModel string          `mapstructure:"embedding_model,omitempty" json:"embedding_model,omitempty"`
	DefaultModel   string          `mapstructure:"default_model,omitempty" json:"default_model,omitempty"`
}

type ModelConfig struct {
	Name        string  `mapstructure:"name" json:"name" yaml:"name"`
	Domain      string  `mapstructure:"domain" json:"domain,omitempty" yaml:"domain"`
	APIKey      string  `mapstructure:"api_key" json:"api_key" yaml:"api_key"`
	BaseURL     string  `mapstructure:"base_url" json:"base_url" yaml:"base_url"`
	Temperature float64 `mapstructure:"temperature" json:"temperature,omitempty" yaml:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens" json:"max_tokens,omitempty" yaml:"max_tokens"`
	Description string  `mapstructure:"description,omitempty" json:"description,omitempty" yaml:"description"`

	Provider string `mapstructure:"provider" json:"provider,omitempty" yaml:"provider,omitempty"`
	Model    string `mapstructure:"model" json:"model,omitempty" yaml:"model,omitempty"`
}

type TokenBudgetConfig struct {
	ReservedOutputTokens int `mapstructure:"reserved_output_tokens" json:"reserved_output_tokens" yaml:"reserved_output_tokens"`
	MinHistoryRounds     int `mapstructure:"min_history_rounds" json:"min_history_rounds" yaml:"min_history_rounds"`
	AvgTokensPerRound    int `mapstructure:"avg_tokens_per_round" json:"avg_tokens_per_round" yaml:"avg_tokens_per_round"`
}
