// Package config provides configuration management for the gort system.
// It supports loading configuration from YAML files, environment variables,
// and provides validation for all configuration options.
//
// Configuration Sources (in order of precedence):
//
//   1. Environment variables (GORT_*)
//   2. Configuration file (config.yaml)
//   3. Default values
//
// Configuration Structure:
//
//	config.yaml
//	├── server          # Server settings
//	│   ├── http_port   # HTTP server port
//	│   ├── ws_port     # WebSocket server port
//	│   └── timeouts    # Read/write timeouts
//	├── channels        # Channel configurations
//	│   ├── wechat      # WeChat settings
//	│   ├── dingtalk    # DingTalk settings
//	│   └── feishu      # Feishu settings
//	└── log             # Logging settings
//
// Basic Usage:
//
//	// Load configuration
//	loader := config.NewLoader()
//	if err := loader.Load("/path/to/config"); err != nil {
//	    log.Fatal(err)
//	}
//
//	cfg := loader.GetConfig()
//	fmt.Printf("HTTP Port: %d\n", cfg.Server.HTTPPort)
//
// Environment Variables:
//
// All configuration options can be set via environment variables
// with the GORT_ prefix and underscores replacing dots:
//
//   - GORT_SERVER_HTTP_PORT=8080
//   - GORT_CHANNELS_DINGTALK_ENABLED=true
//   - GORT_CHANNELS_DINGTALK_APP_KEY=your-key
//
// Validation:
//
// The package validates all configuration on load and returns
// detailed errors for invalid settings:
//
//   - Port numbers must be between 1-65535
//   - Webhook paths must start with /
//   - Log levels must be: debug, info, warn, error
//   - Enabled channels must have required credentials
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Error definitions for configuration validation.
var (
	// ErrInvalidPort is returned when a port number is outside the valid range (1-65535).
	ErrInvalidPort = errors.New("invalid port number, must be between 1 and 65535")

	// ErrInvalidWebhookPath is returned when a webhook path doesn't start with /.
	ErrInvalidWebhookPath = errors.New("webhook path must start with /")

	// ErrInvalidLogLevel is returned when an unsupported log level is specified.
	ErrInvalidLogLevel = errors.New("invalid log level, must be one of: debug, info, warn, error")

	// ErrMissingCredentials is returned when an enabled channel lacks required credentials.
	ErrMissingCredentials = errors.New("enabled channel is missing required credentials")

	// ErrConfigNotFound is returned when the configuration file cannot be found.
	ErrConfigNotFound = errors.New("configuration file not found")
)

// Config holds the complete application configuration.
// It is populated from configuration files and environment variables.
type Config struct {
	// Server contains HTTP and WebSocket server configuration.
	Server ServerConfig `mapstructure:"server"`

	// Channels contains configuration for all messaging channels.
	Channels ChannelsConfig `mapstructure:"channels"`

	// Log contains logging configuration.
	Log LogConfig `mapstructure:"log"`
}

// ServerConfig contains HTTP and WebSocket server settings.
type ServerConfig struct {
	// HTTPPort is the port for the HTTP server (default: 8080).
	HTTPPort int `mapstructure:"http_port"`

	// WSPort is the port for the WebSocket server (default: 8081).
	WSPort int `mapstructure:"ws_port"`

	// WebhookPath is the base path for webhook endpoints (default: "/webhook").
	WebhookPath string `mapstructure:"webhook_path"`

	// ReadTimeout is the HTTP read timeout in seconds (default: 30).
	ReadTimeout int `mapstructure:"read_timeout"`

	// WriteTimeout is the HTTP write timeout in seconds (default: 30).
	WriteTimeout int `mapstructure:"write_timeout"`
}

// ChannelsConfig contains configuration for all messaging platform channels.
type ChannelsConfig struct {
	// WeChat contains WeChat Work configuration.
	WeChat WeChatConfig `mapstructure:"wechat"`

	// DingTalk contains DingTalk configuration.
	DingTalk DingTalkConfig `mapstructure:"dingtalk"`

	// Feishu contains Feishu/Lark configuration.
	Feishu FeishuConfig `mapstructure:"feishu"`
}

// WeChatConfig contains WeChat Work channel configuration.
type WeChatConfig struct {
	// Enabled enables or disables the WeChat channel.
	Enabled bool `mapstructure:"enabled"`

	// Token is the WeChat verification token.
	Token string `mapstructure:"token"`

	// AppID is the WeChat app ID.
	AppID string `mapstructure:"app_id"`

	// Secret is the WeChat app secret.
	Secret string `mapstructure:"secret"`
}

// DingTalkConfig contains DingTalk channel configuration.
type DingTalkConfig struct {
	// Enabled enables or disables the DingTalk channel.
	Enabled bool `mapstructure:"enabled"`

	// AppKey is the DingTalk app key.
	AppKey string `mapstructure:"app_key"`

	// AppSecret is the DingTalk app secret.
	AppSecret string `mapstructure:"app_secret"`
}

// FeishuConfig contains Feishu/Lark channel configuration.
type FeishuConfig struct {
	// Enabled enables or disables the Feishu channel.
	Enabled bool `mapstructure:"enabled"`

	// AppID is the Feishu app ID.
	AppID string `mapstructure:"app_id"`

	// AppSecret is the Feishu app secret.
	AppSecret string `mapstructure:"app_secret"`
}

// LogConfig contains logging configuration.
type LogConfig struct {
	// Level is the log level: debug, info, warn, error (default: "info").
	Level string `mapstructure:"level"`

	// Format is the log format: text, json (default: "text").
	Format string `mapstructure:"format"`

	// Output is the log output destination: stdout, stderr, or file path (default: "stdout").
	Output string `mapstructure:"output"`
}

// DefaultConfig returns a Config with default values.
//
// Returns a new Config initialized with sensible defaults.
//
// Example:
//
//	cfg := config.DefaultConfig()
//	fmt.Printf("Default HTTP port: %d\n", cfg.Server.HTTPPort)
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort:     8080,
			WSPort:       8081,
			WebhookPath:  "/webhook",
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Channels: ChannelsConfig{
			WeChat:   WeChatConfig{Enabled: false},
			DingTalk: DingTalkConfig{Enabled: false},
			Feishu:   FeishuConfig{Enabled: false},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

// Validate validates the complete configuration.
// It checks server, log, and channel configurations.
//
// Returns an error if any configuration is invalid.
//
// Example:
//
//	if err := cfg.Validate(); err != nil {
//	    log.Fatalf("Invalid configuration: %v", err)
//	}
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log config: %w", err)
	}
	if err := c.Channels.Validate(); err != nil {
		return fmt.Errorf("channels config: %w", err)
	}
	return nil
}

// Validate validates the server configuration.
//
// Returns an error if port numbers or webhook path are invalid.
func (s *ServerConfig) Validate() error {
	if s.HTTPPort < 1 || s.HTTPPort > 65535 {
		return ErrInvalidPort
	}
	if s.WSPort < 1 || s.WSPort > 65535 {
		return ErrInvalidPort
	}
	if s.WebhookPath != "" && !strings.HasPrefix(s.WebhookPath, "/") {
		return ErrInvalidWebhookPath
	}
	return nil
}

// Validate validates the log configuration.
//
// Returns an error if the log level is invalid.
func (l *LogConfig) Validate() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[l.Level] {
		return ErrInvalidLogLevel
	}
	return nil
}

// Validate validates the channels configuration.
//
// Returns an error if any enabled channel is missing required credentials.
func (c *ChannelsConfig) Validate() error {
	if c.WeChat.Enabled {
		if c.WeChat.Token == "" || c.WeChat.Secret == "" {
			return fmt.Errorf("%w: wechat", ErrMissingCredentials)
		}
	}
	if c.DingTalk.Enabled {
		if c.DingTalk.AppKey == "" || c.DingTalk.AppSecret == "" {
			return fmt.Errorf("%w: dingtalk", ErrMissingCredentials)
		}
	}
	if c.Feishu.Enabled {
		if c.Feishu.AppID == "" || c.Feishu.AppSecret == "" {
			return fmt.Errorf("%w: feishu", ErrMissingCredentials)
		}
	}
	return nil
}

// HTTPAddr returns the HTTP server address in ":port" format.
//
// Returns a string suitable for use with http.ListenAndServe.
//
// Example:
//
//	addr := cfg.Server.HTTPAddr()
//	http.ListenAndServe(addr, handler)
func (s *ServerConfig) HTTPAddr() string {
	return fmt.Sprintf(":%d", s.HTTPPort)
}

// WSAddr returns the WebSocket server address in ":port" format.
//
// Returns a string suitable for use with WebSocket server initialization.
//
// Example:
//
//	addr := cfg.Server.WSAddr()
//	wsServer.Listen(addr)
func (s *ServerConfig) WSAddr() string {
	return fmt.Sprintf(":%d", s.WSPort)
}

// ReadTimeoutDuration returns the read timeout as a time.Duration.
//
// Returns the timeout suitable for use with http.Server.
//
// Example:
//
//	server := &http.Server{
//	    ReadTimeout: cfg.Server.ReadTimeoutDuration(),
//	}
func (s *ServerConfig) ReadTimeoutDuration() time.Duration {
	return time.Duration(s.ReadTimeout) * time.Second
}

// WriteTimeoutDuration returns the write timeout as a time.Duration.
//
// Returns the timeout suitable for use with http.Server.
//
// Example:
//
//	server := &http.Server{
//	    WriteTimeout: cfg.Server.WriteTimeoutDuration(),
//	}
func (s *ServerConfig) WriteTimeoutDuration() time.Duration {
	return time.Duration(s.WriteTimeout) * time.Second
}

// Loader handles configuration loading from files and environment.
// It uses viper for configuration management.
type Loader struct {
	config *Config
}

// NewLoader creates a new configuration loader.
//
// Returns a Loader initialized with default configuration.
//
// Example:
//
//	loader := config.NewLoader()
//	if err := loader.Load(""); err != nil {
//	    log.Fatal(err)
//	}
func NewLoader() *Loader {
	return &Loader{
		config: DefaultConfig(),
	}
}

// Load loads configuration from files and environment variables.
// It searches for config.yaml in the current directory, ./config/,
// and the specified configPath.
//
// Parameters:
//   - configPath: Additional path to search for configuration files
//
// Returns an error if configuration loading or validation fails.
//
// Example:
//
//	loader := config.NewLoader()
//	
//	// Load from default locations
//	if err := loader.Load(""); err != nil {
//	    log.Fatal(err)
//	}
//	
//	// Or load from specific path
//	if err := loader.Load("/etc/gort"); err != nil {
//	    log.Fatal(err)
//	}
func (l *Loader) Load(configPath string) error {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	if configPath != "" {
		v.AddConfigPath(configPath)
	}
	v.SetEnvPrefix("GORT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFound) {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	l.config = &cfg
	return l.config.Validate()
}

// GetConfig returns the loaded configuration.
//
// Returns the Config loaded by Load(). If Load() was not called,
// returns the default configuration.
//
// Example:
//
//	cfg := loader.GetConfig()
//	fmt.Printf("Server port: %d\n", cfg.Server.HTTPPort)
func (l *Loader) GetConfig() *Config {
	return l.config
}

// setDefaults sets default configuration values in viper.
//
// Parameters:
//   - v: The viper instance to configure
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.ws_port", 8081)
	v.SetDefault("server.webhook_path", "/webhook")
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.output", "stdout")
}
