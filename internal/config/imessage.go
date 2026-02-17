package config

// IMessageConfig iMessage 配置
type IMessageConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
	IMsgPath   string `mapstructure:"imsg_path" json:"imsg_path"`
	Region     string `mapstructure:"region" json:"region"`
	Debounce   string `mapstructure:"debounce" json:"debounce"`
	WatchSince int64  `mapstructure:"watch_since" json:"watch_since"`
}
