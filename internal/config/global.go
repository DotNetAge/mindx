package config

type GlobalConfig struct {
	Version     string            `mapstructure:"version" yaml:"version"`
	Host        string            `mapstructure:"host" yaml:"host"`
	Port        int               `mapstructure:"port" yaml:"port"`
	WsPort      int               `mapstructure:"ws_port" yaml:"ws_port"`
	OllamaURL   string            `mapstructure:"ollama_url" yaml:"ollama_url"`
	Brain       BrainConfig       `mapstructure:"brain" yaml:"brain"`
	IndexModel  string            `mapstructure:"index_model" yaml:"index_model"`
	Embedding   string            `mapstructure:"embedding" yaml:"embedding"`
	Memory      MemoryConfig      `mapstructure:"memory" yaml:"memory"`
	VectorStore VectorStoreConfig `mapstructure:"vector_store" yaml:"vector_store"`
}

type BrainConfig struct {
	LeftbrainModel  ModelConfig       `mapstructure:"leftbrain" yaml:"leftbrain"`
	RightbrainModel ModelConfig       `mapstructure:"rightbrain" yaml:"rightbrain"`
	TokenBudget     TokenBudgetConfig `mapstructure:"token_budget" yaml:"token_budget"`
}

type MemoryConfig struct {
	Enabled      bool   `mapstructure:"enabled" yaml:"enabled"`
	SummaryModel string `mapstructure:"summary_model" yaml:"summary_model"`
	KeywordModel string `mapstructure:"keyword_model" yaml:"keyword_model"`
	Schedule     string `mapstructure:"schedule" yaml:"schedule"`
}

type VectorStoreConfig struct {
	Type     string `mapstructure:"type" yaml:"type"`
	DataPath string `mapstructure:"data_path" yaml:"data_path"`
}
