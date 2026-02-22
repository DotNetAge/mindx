package config

type TelegramConfig struct {
	BotToken    string `mapstructure:"bot_token" json:"bot_token" yaml:"bot_token"`
	WebhookURL  string `mapstructure:"webhook_url" json:"webhook_url" yaml:"webhook_url"`
	SecretToken string `mapstructure:"secret_token" json:"secret_token" yaml:"secret_token"`
	Port        int    `mapstructure:"port" json:"port" yaml:"port"`
	Path        string `mapstructure:"path" json:"path" yaml:"path"`
	UseWebhook  bool   `mapstructure:"use_webhook" json:"use_webhook" yaml:"use_webhook"`
}

func (c *TelegramConfig) GetPort() int  { return c.Port }
func (c *TelegramConfig) GetPath() string { return c.Path }
