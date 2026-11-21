package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 8080, cfg.Server.HTTPPort)
	assert.Equal(t, 8081, cfg.Server.WSPort)
	assert.Equal(t, "/webhook", cfg.Server.WebhookPath)
	assert.Equal(t, 30, cfg.Server.ReadTimeout)
	assert.Equal(t, 30, cfg.Server.WriteTimeout)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, "stdout", cfg.Log.Output)
	assert.False(t, cfg.Channels.WeChat.Enabled)
	assert.False(t, cfg.Channels.DingTalk.Enabled)
	assert.False(t, cfg.Channels.Feishu.Enabled)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:   "valid default config",
			config: DefaultConfig(),
		},
		{
			name: "invalid HTTP port",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    0,
					WSPort:      8081,
					WebhookPath: "/webhook",
				},
				Log: LogConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid WS port",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    8080,
					WSPort:      70000,
					WebhookPath: "/webhook",
				},
				Log: LogConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid webhook path",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    8080,
					WSPort:      8081,
					WebhookPath: "webhook",
				},
				Log: LogConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    8080,
					WSPort:      8081,
					WebhookPath: "/webhook",
				},
				Log: LogConfig{Level: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "enabled wechat without credentials",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    8080,
					WSPort:      8081,
					WebhookPath: "/webhook",
				},
				Log: LogConfig{Level: "info"},
				Channels: ChannelsConfig{
					WeChat: WeChatConfig{Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "enabled wechat with credentials",
			config: &Config{
				Server: ServerConfig{
					HTTPPort:    8080,
					WSPort:      8081,
					WebhookPath: "/webhook",
				},
				Log: LogConfig{Level: "info"},
				Channels: ChannelsConfig{
					WeChat: WeChatConfig{
						Enabled: true,
						Token:   "test-token",
						Secret:  "test-secret",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr error
	}{
		{
			name: "valid config",
			config: ServerConfig{
				HTTPPort:    8080,
				WSPort:      8081,
				WebhookPath: "/webhook",
			},
		},
		{
			name: "HTTP port too low",
			config: ServerConfig{
				HTTPPort:    0,
				WSPort:      8081,
				WebhookPath: "/webhook",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "HTTP port too high",
			config: ServerConfig{
				HTTPPort:    70000,
				WSPort:      8081,
				WebhookPath: "/webhook",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "WS port too low",
			config: ServerConfig{
				HTTPPort:    8080,
				WSPort:      0,
				WebhookPath: "/webhook",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "invalid webhook path",
			config: ServerConfig{
				HTTPPort:    8080,
				WSPort:      8081,
				WebhookPath: "webhook",
			},
			wantErr: ErrInvalidWebhookPath,
		},
		{
			name: "empty webhook path is valid",
			config: ServerConfig{
				HTTPPort:    8080,
				WSPort:      8081,
				WebhookPath: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLogConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  LogConfig
		wantErr error
	}{
		{
			name:   "debug level",
			config: LogConfig{Level: "debug"},
		},
		{
			name:   "info level",
			config: LogConfig{Level: "info"},
		},
		{
			name:   "warn level",
			config: LogConfig{Level: "warn"},
		},
		{
			name:   "error level",
			config: LogConfig{Level: "error"},
		},
		{
			name:    "invalid level",
			config:  LogConfig{Level: "invalid"},
			wantErr: ErrInvalidLogLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChannelsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ChannelsConfig
		wantErr bool
	}{
		{
			name:   "all disabled",
			config: ChannelsConfig{},
		},
		{
			name: "wechat enabled with credentials",
			config: ChannelsConfig{
				WeChat: WeChatConfig{
					Enabled: true,
					Token:   "token",
					Secret:  "secret",
				},
			},
		},
		{
			name: "wechat enabled without token",
			config: ChannelsConfig{
				WeChat: WeChatConfig{
					Enabled: true,
					Secret:  "secret",
				},
			},
			wantErr: true,
		},
		{
			name: "wechat enabled without secret",
			config: ChannelsConfig{
				WeChat: WeChatConfig{
					Enabled: true,
					Token:   "token",
				},
			},
			wantErr: true,
		},
		{
			name: "dingtalk enabled with credentials",
			config: ChannelsConfig{
				DingTalk: DingTalkConfig{
					Enabled:   true,
					AppKey:    "key",
					AppSecret: "secret",
				},
			},
		},
		{
			name: "dingtalk enabled without app key",
			config: ChannelsConfig{
				DingTalk: DingTalkConfig{
					Enabled:   true,
					AppSecret: "secret",
				},
			},
			wantErr: true,
		},
		{
			name: "feishu enabled with credentials",
			config: ChannelsConfig{
				Feishu: FeishuConfig{
					Enabled:   true,
					AppID:     "app_id",
					AppSecret: "secret",
				},
			},
		},
		{
			name: "feishu enabled without app id",
			config: ChannelsConfig{
				Feishu: FeishuConfig{
					Enabled:   true,
					AppSecret: "secret",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServerConfig_HTTPAddr(t *testing.T) {
	cfg := ServerConfig{HTTPPort: 8080}
	assert.Equal(t, ":8080", cfg.HTTPAddr())
}

func TestServerConfig_WSAddr(t *testing.T) {
	cfg := ServerConfig{WSPort: 8081}
	assert.Equal(t, ":8081", cfg.WSAddr())
}

func TestServerConfig_TimeoutDurations(t *testing.T) {
	cfg := ServerConfig{ReadTimeout: 30, WriteTimeout: 60}
	assert.Equal(t, 30*time.Second, cfg.ReadTimeoutDuration())
	assert.Equal(t, 60*time.Second, cfg.WriteTimeoutDuration())
}

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	assert.NotNil(t, loader)
	assert.NotNil(t, loader.config)
}

func TestLoader_GetConfig(t *testing.T) {
	loader := NewLoader()
	cfg := loader.GetConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Server.HTTPPort)
}

func TestConfig_Integration(t *testing.T) {
	t.Setenv("GORT_SERVER_HTTP_PORT", "9999")
	t.Setenv("GORT_LOG_LEVEL", "error")

	loader := NewLoader()
	err := loader.Load("")
	require.NoError(t, err)
	cfg := loader.GetConfig()
	assert.Equal(t, 9999, cfg.Server.HTTPPort)
	assert.Equal(t, "error", cfg.Log.Level)
}

func TestConfig_EnvOverride(t *testing.T) {
	t.Setenv("GORT_SERVER_HTTP_PORT", "7777")
	t.Setenv("GORT_SERVER_WS_PORT", "7778")
	t.Setenv("GORT_SERVER_WEBHOOK_PATH", "/env-webhook")
	t.Setenv("GORT_LOG_LEVEL", "debug")

	loader := NewLoader()
	err := loader.Load("")
	require.NoError(t, err)
	cfg := loader.GetConfig()

	assert.Equal(t, 7777, cfg.Server.HTTPPort)
	assert.Equal(t, 7778, cfg.Server.WSPort)
	assert.Equal(t, "/env-webhook", cfg.Server.WebhookPath)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestConfig_InvalidEnvPort(t *testing.T) {
	originalPort := os.Getenv("GORT_SERVER_HTTP_PORT")
	defer os.Setenv("GORT_SERVER_HTTP_PORT", originalPort)

	os.Setenv("GORT_SERVER_HTTP_PORT", "invalid")

	loader := NewLoader()
	err := loader.Load("")
	assert.Error(t, err)
}
