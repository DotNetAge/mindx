package config

type WeChatConfig struct {
	Token          string `mapstructure:"token" json:"token" yaml:"token"`
	AppID          string `mapstructure:"app_id" json:"app_id" yaml:"app_id"`
	AppSecret      string `mapstructure:"app_secret" json:"app_secret" yaml:"app_secret"`
	EncodingAESKey string `mapstructure:"encoding_aes_key" json:"encoding_aes_key" yaml:"encoding_aes_key"`
	Port           int    `mapstructure:"port" json:"port" yaml:"port"`
	Path           string `mapstructure:"path" json:"path" yaml:"path"`
	Type           string `mapstructure:"type" json:"type" yaml:"type"`
}

func (c *WeChatConfig) GetPort() int  { return c.Port }
func (c *WeChatConfig) GetPath() string { return c.Path }
