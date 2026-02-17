package config

// WeChatConfig 微信配置
type WeChatConfig struct {
	Token          string `mapstructure:"token" json:"token"`
	AppID          string `mapstructure:"app_id" json:"app_id"`
	AppSecret      string `mapstructure:"app_secret" json:"app_secret"`
	EncodingAESKey string `mapstructure:"encoding_aes_key" json:"encoding_aes_key"`
	Port           int    `mapstructure:"port" json:"port"`
	Path           string `mapstructure:"path" json:"path"`
	Type           string `mapstructure:"type" json:"type"`
}
