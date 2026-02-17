package config

// WhatsAppConfig WhatsApp 配置
type WhatsAppConfig struct {
	PhoneNumberID string `mapstructure:"phone_number_id" json:"phone_number_id"`
	BusinessID    string `mapstructure:"business_id" json:"business_id"`
	AccessToken   string `mapstructure:"access_token" json:"access_token"`
	VerifyToken   string `mapstructure:"verify_token" json:"verify_token"`
	Port          int    `mapstructure:"port" json:"port"`
	Path          string `mapstructure:"path" json:"path"`
}
