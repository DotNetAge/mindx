package config

// QQConfig QQ 配置
type QQConfig struct {
	AppID       string `mapstructure:"app_id" json:"app_id"`
	AppSecret   string `mapstructure:"app_secret" json:"app_secret"`
	Token       string `mapstructure:"token" json:"token"`
	Port        int    `mapstructure:"port" json:"port"`
	Path        string `mapstructure:"path" json:"path"`
	Sandbox     bool   `mapstructure:"sandbox" json:"sandbox"`
	Description string `mapstructure:"description" json:"description"`
	// OneBot 协议配置
	WebSocketURL string `mapstructure:"websocket_url" json:"websocket_url"` // OneBot WebSocket 地址 (ws://localhost:8080)
	AccessToken  string `mapstructure:"access_token" json:"access_token"`   // OneBot access_token
}
