package config

type IMessageConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	IMsgPath   string `mapstructure:"imsg_path" json:"imsg_path" yaml:"imsg_path"`
	Region     string `mapstructure:"region" json:"region" yaml:"region"`
	Debounce   string `mapstructure:"debounce" json:"debounce" yaml:"debounce"`
	WatchSince int64  `mapstructure:"watch_since" json:"watch_since" yaml:"watch_since"`
}
