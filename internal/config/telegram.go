package config

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	BotToken    string `mapstructure:"bot_token" json:"bot_token"`
	WebhookURL  string `mapstructure:"webhook_url" json:"webhook_url"`
	SecretToken string `mapstructure:"secret_token" json:"secret_token"`
	Port        int    `mapstructure:"port" json:"port"`
	Path        string `mapstructure:"path" json:"path"`
	UseWebhook  bool   `mapstructure:"use_webhook" json:"use_webhook"`
}
