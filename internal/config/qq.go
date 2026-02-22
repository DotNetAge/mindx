package config

type QQConfig struct {
	AppID        string `mapstructure:"app_id" json:"app_id" yaml:"app_id"`
	AppSecret    string `mapstructure:"app_secret" json:"app_secret" yaml:"app_secret"`
	Token        string `mapstructure:"token" json:"token" yaml:"token"`
	Port         int    `mapstructure:"port" json:"port" yaml:"port"`
	Path         string `mapstructure:"path" json:"path" yaml:"path"`
	Sandbox      bool   `mapstructure:"sandbox" json:"sandbox" yaml:"sandbox"`
	Description  string `mapstructure:"description" json:"description" yaml:"description"`
	WebSocketURL string `mapstructure:"websocket_url" json:"websocket_url" yaml:"websocket_url"`
	AccessToken  string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`
}

func (c *QQConfig) GetPort() int  { return c.Port }
func (c *QQConfig) GetPath() string { return c.Path }
