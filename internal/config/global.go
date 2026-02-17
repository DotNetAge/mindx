package config

type GlobalConfig struct {
	Version        string            `mapstructure:"version" yaml:"version"`
	Host           string            `mapstructure:"host" yaml:"host"`
	Port           int               `mapstructure:"port" yaml:"port"`
	WsPort         int               `mapstructure:"ws_port" yaml:"ws_port"`
	OllamaURL      string            `mapstructure:"ollama_url,omitempty" yaml:"ollama_url,omitempty"`
	TokenBudget    TokenBudgetConfig `mapstructure:"token_budget" yaml:"token_budget"`
	Subconscious   BrainHalfConfig   `mapstructure:"subconscious" yaml:"subconscious"`
	Consciousness  BrainHalfConfig   `mapstructure:"consciousness" yaml:"consciousness"`
	MemoryModel    string            `mapstructure:"memory_model" yaml:"memory_model"`
	IndexModel     string            `mapstructure:"index_model" yaml:"index_model"`
	EmbeddingModel string            `mapstructure:"embedding_model" yaml:"embedding_model"`
	DefaultModel   string            `mapstructure:"default_model" yaml:"default_model"`
	Memory         MemoryConfig      `mapstructure:"memory,omitempty" yaml:"memory,omitempty"`
	VectorStore    VectorStoreConfig `mapstructure:"vector_store" yaml:"vector_store"`
}

type BrainHalfConfig struct {
	Default string `mapstructure:"default" yaml:"default"`
	Left    string `mapstructure:"left" yaml:"left"`
	Right   string `mapstructure:"right" yaml:"right"`
}

type MemoryConfig struct {
	Enabled      bool   `mapstructure:"enabled" yaml:"enabled"`
	SummaryModel string `mapstructure:"summary_model,omitempty" yaml:"summary_model"`
	KeywordModel string `mapstructure:"keyword_model,omitempty" yaml:"keyword_model"`
	Schedule     string `mapstructure:"schedule" yaml:"schedule"`
}

type VectorStoreConfig struct {
	Type     string `mapstructure:"type" yaml:"type"`
	DataPath string `mapstructure:"data_path" yaml:"data_path"`
}
