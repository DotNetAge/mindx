package config

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	AppKey        string `mapstructure:"app_key" json:"app_key"`
	AppSecret     string `mapstructure:"app_secret" json:"app_secret"`
	AgentID       string `mapstructure:"agent_id" json:"agent_id"`
	EncryptKey    string `mapstructure:"encrypt_key" json:"encrypt_key"`
	WebhookSecret string `mapstructure:"webhook_secret" json:"webhook_secret"`
	Port          int    `mapstructure:"port" json:"port"`
	Path          string `mapstructure:"path" json:"path"`
}
