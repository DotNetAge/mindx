package config

// FeishuConfig 飞书 Webhook 配置
type FeishuConfig struct {
	AppID             string `mapstructure:"app_id" json:"app_id"`
	AppSecret         string `mapstructure:"app_secret" json:"app_secret"`
	EncryptKey        string `mapstructure:"encrypt_key" json:"encrypt_key"`
	VerificationToken string `mapstructure:"verification_token" json:"verification_token"`
	Port              int    `mapstructure:"port" json:"port"`
	Path              string `mapstructure:"path" json:"path"`
}
