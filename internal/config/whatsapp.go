package config

type WhatsAppConfig struct {
	PhoneNumberID string `mapstructure:"phone_number_id" json:"phone_number_id" yaml:"phone_number_id"`
	BusinessID    string `mapstructure:"business_id" json:"business_id" yaml:"business_id"`
	AccessToken   string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`
	VerifyToken   string `mapstructure:"verify_token" json:"verify_token" yaml:"verify_token"`
	Port          int    `mapstructure:"port" json:"port" yaml:"port"`
	Path          string `mapstructure:"path" json:"path" yaml:"path"`
}

func (c *WhatsAppConfig) GetPort() int  { return c.Port }
func (c *WhatsAppConfig) GetPath() string { return c.Path }
