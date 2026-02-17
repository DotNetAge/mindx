package config

// FacebookConfig Facebook 配置
type FacebookConfig struct {
	PageID       string `mapstructure:"page_id" json:"page_id"`
	PageAccessToken string `mapstructure:"page_access_token" json:"page_access_token"`
	AppSecret    string `mapstructure:"app_secret" json:"app_secret"`
	VerifyToken  string `mapstructure:"verify_token" json:"verify_token"`
	Port         int    `mapstructure:"port" json:"port"`
	Path         string `mapstructure:"path" json:"path"`
}
