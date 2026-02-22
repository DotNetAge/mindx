package config

type FeishuConfig struct {
	AppID             string `mapstructure:"app_id" json:"app_id" yaml:"app_id"`
	AppSecret         string `mapstructure:"app_secret" json:"app_secret" yaml:"app_secret"`
	EncryptKey        string `mapstructure:"encrypt_key" json:"encrypt_key" yaml:"encrypt_key"`
	VerificationToken string `mapstructure:"verification_token" json:"verification_token" yaml:"verification_token"`
	Port              int    `mapstructure:"port" json:"port" yaml:"port"`
	Path              string `mapstructure:"path" json:"path" yaml:"path"`
}

func (c *FeishuConfig) GetPort() int  { return c.Port }
func (c *FeishuConfig) GetPath() string { return c.Path }
