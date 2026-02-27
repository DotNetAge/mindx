package config

type GlobalConfig struct {
	Version        string            `mapstructure:"version" json:"version" yaml:"version"`
	Host           string            `mapstructure:"host" json:"host" yaml:"host"`
	Port           int               `mapstructure:"port" json:"port" yaml:"port"`
	WsPort         int               `mapstructure:"ws_port" json:"ws_port" yaml:"ws_port"`
	OllamaURL      string            `mapstructure:"ollama_url,omitempty" json:"ollama_url,omitempty" yaml:"ollama_url,omitempty"`
	TokenBudget    TokenBudgetConfig `mapstructure:"token_budget" json:"token_budget" yaml:"token_budget"`
	Subconscious   BrainHalfConfig   `mapstructure:"subconscious" json:"subconscious" yaml:"subconscious"`
	Consciousness  BrainHalfConfig   `mapstructure:"consciousness" json:"consciousness" yaml:"consciousness"`
	EmbeddingModel string            `mapstructure:"embedding_model" json:"embedding_model" yaml:"embedding_model"`
	DefaultModel   string            `mapstructure:"default_model" json:"default_model" yaml:"default_model"`
	Memory         MemoryConfig      `mapstructure:"memory,omitempty" json:"memory,omitempty" yaml:"memory,omitempty"`
	VectorStore    VectorStoreConfig `mapstructure:"vector_store" json:"vector_store" yaml:"vector_store"`
	WebSocket      WebSocketConfig   `mapstructure:"websocket,omitempty" json:"websocket,omitempty" yaml:"websocket,omitempty"`
}

type WebSocketConfig struct {
	MaxConnections int      `mapstructure:"max_connections" json:"max_connections" yaml:"max_connections"`
	PingInterval   int      `mapstructure:"ping_interval" json:"ping_interval" yaml:"ping_interval"`
	AllowedOrigins []string `mapstructure:"allowed_origins" json:"allowed_origins" yaml:"allowed_origins"`
	Token          string   `mapstructure:"token" json:"token" yaml:"token"`
}

type BrainHalfConfig struct {
	Left  string `mapstructure:"left" json:"left" yaml:"left"`
	Right string `mapstructure:"right" json:"right" yaml:"right"`
}

type MemoryConfig struct {
	Enabled      bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	SummaryModel string `mapstructure:"summary_model,omitempty" json:"summary_model,omitempty" yaml:"summary_model,omitempty"`
	KeywordModel string `mapstructure:"keyword_model,omitempty" json:"keyword_model,omitempty" yaml:"keyword_model,omitempty"`
	Schedule     string `mapstructure:"schedule" json:"schedule" yaml:"schedule"`
}

type VectorStoreConfig struct {
	Type     string `mapstructure:"type" json:"type" yaml:"type"`
	DataPath string `mapstructure:"data_path" json:"data_path" yaml:"data_path"`
}
