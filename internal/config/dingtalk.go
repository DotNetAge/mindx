package config

type DingTalkConfig struct {
	AppKey        string `mapstructure:"app_key" json:"app_key" yaml:"app_key"`
	AppSecret     string `mapstructure:"app_secret" json:"app_secret" yaml:"app_secret"`
	AgentID       string `mapstructure:"agent_id" json:"agent_id" yaml:"agent_id"`
	EncryptKey    string `mapstructure:"encrypt_key" json:"encrypt_key" yaml:"encrypt_key"`
	WebhookSecret string `mapstructure:"webhook_secret" json:"webhook_secret" yaml:"webhook_secret"`
	Port          int    `mapstructure:"port" json:"port" yaml:"port"`
	Path          string `mapstructure:"path" json:"path" yaml:"path"`
}

func (c *DingTalkConfig) GetPort() int  { return c.Port }
func (c *DingTalkConfig) GetPath() string { return c.Path }
