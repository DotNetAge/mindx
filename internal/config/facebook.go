package config

type FacebookConfig struct {
	PageID         string `mapstructure:"page_id" json:"page_id" yaml:"page_id"`
	PageAccessToken string `mapstructure:"page_access_token" json:"page_access_token" yaml:"page_access_token"`
	AppSecret      string `mapstructure:"app_secret" json:"app_secret" yaml:"app_secret"`
	VerifyToken    string `mapstructure:"verify_token" json:"verify_token" yaml:"verify_token"`
	Port           int    `mapstructure:"port" json:"port" yaml:"port"`
	Path           string `mapstructure:"path" json:"path" yaml:"path"`
}

func (c *FacebookConfig) GetPort() int  { return c.Port }
func (c *FacebookConfig) GetPath() string { return c.Path }
