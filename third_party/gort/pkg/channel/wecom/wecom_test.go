package wecom

import (
	"context"
	"testing"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errType     error
	}{
		{
			name: "valid config with webhook URL",
			config: Config{
				WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test123",
			},
			expectError: false,
		},
		{
			name: "valid config with key only",
			config: Config{
				Key: "test123",
			},
			expectError: false,
		},
		{
			name:        "empty config",
			config:      Config{},
			expectError: true,
			errType:     ErrWebhookURLRequired,
		},
		{
			name: "invalid webhook URL",
			config: Config{
				WebhookURL: "://invalid-url",
			},
			expectError: true,
			errType:     ErrInvalidWebhookURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := NewChannel("wecom", tt.config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
				assert.Nil(t, ch)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ch)
				assert.Equal(t, "wecom", ch.Name())
				assert.Equal(t, channel.ChannelTypeWeCom, ch.Type())
			}
		})
	}
}

func TestChannel_StartStop(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	ctx := context.Background()
	err = ch.Start(ctx, handler)
	require.NoError(t, err)
	assert.True(t, ch.IsRunning())
	assert.Equal(t, channel.StatusRunning, ch.GetStatus())

	err = ch.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, ch.IsRunning())
	assert.Equal(t, channel.StatusStopped, ch.GetStatus())
}

func TestChannel_GetCapabilities(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	caps := ch.GetCapabilities()
	assert.True(t, caps.TextMessages)
	assert.True(t, caps.MarkdownMessages)
	assert.True(t, caps.ImageMessages)
	assert.True(t, caps.NewsMessages)
	assert.True(t, caps.FileMessages)
	assert.True(t, caps.VoiceMessages)
	assert.True(t, caps.TemplateCard)
	assert.False(t, caps.LocationMessages)
	assert.False(t, caps.ReadReceipts)
}

func TestBuildTextPayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Hello, World!",
		channel.MessageTypeText,
	)

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "text", payload["msgtype"])
	textData, ok := payload["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", textData["content"])
}

func TestBuildTextPayload_WithMentions(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Hello @all",
		channel.MessageTypeText,
	)
	msg.SetMetadata("mentioned_list", []string{"@all", "user123"})
	msg.SetMetadata("mentioned_mobile_list", []string{"13800138000"})

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	textData, ok := payload["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, textData, "mentioned_list")
	assert.Contains(t, textData, "mentioned_mobile_list")
}

func TestBuildMarkdownPayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"**Bold** text",
		channel.MessageTypeMarkdown,
	)

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "markdown", payload["msgtype"])
	markdownData, ok := payload["markdown"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "**Bold** text", markdownData["content"])
}

func TestBuildMarkdownPayload_V2(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"# Heading",
		channel.MessageTypeMarkdown,
	)
	msg.SetMetadata("markdown_version", "v2")

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "markdown_v2", payload["msgtype"])
}

func TestBuildNewsPayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Article Title",
		channel.MessageTypeNews,
	)
	msg.SetMetadata("description", "Article description")
	msg.SetMetadata("url", "https://example.com/article")
	msg.SetMetadata("picurl", "https://example.com/image.jpg")

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "news", payload["msgtype"])
	newsData, ok := payload["news"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, newsData, "articles")
}

func TestBuildFilePayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"File message",
		channel.MessageTypeFile,
	)
	msg.SetMetadata("media_id", "media123")

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "file", payload["msgtype"])
	fileData, ok := payload["file"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "media123", fileData["media_id"])
}

func TestBuildFilePayload_NoMediaID(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"File message",
		channel.MessageTypeFile,
	)

	_, err = ch.buildPayload(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "media_id")
}

func TestBuildVoicePayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Voice message",
		channel.MessageTypeVoice,
	)
	msg.SetMetadata("media_id", "voice123")

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "voice", payload["msgtype"])
	voiceData, ok := payload["voice"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "voice123", voiceData["media_id"])
}

func TestBuildTemplateCardPayload(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Template card",
		channel.MessageTypeTemplateCard,
	)
	msg.SetMetadata("template_card", map[string]interface{}{
		"card_type": "text_notice",
		"source": map[string]interface{}{
			"desc": "Notification",
		},
	})

	payload, err := ch.buildPayload(msg)
	require.NoError(t, err)

	assert.Equal(t, "template_card", payload["msgtype"])
	assert.Contains(t, payload, "template_card")
}

func TestBuildTemplateCardPayload_NoData(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Template card",
		channel.MessageTypeTemplateCard,
	)

	_, err = ch.buildPayload(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template_card")
}

func TestChannel_SendMessage_NotRunning(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"wecom",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Test",
		channel.MessageTypeText,
	)

	err = ch.SendMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestChannel_SendMessage_NilMessage(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	require.NoError(t, err)
	defer ch.Stop(ctx)

	err = ch.SendMessage(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestChannel_HandleWebhook(t *testing.T) {
	config := Config{
		Key: "test123",
	}

	ch, err := NewChannel("wecom", config)
	require.NoError(t, err)

	// WeCom webhook robots are outbound only
	_, err = ch.HandleWebhook("/webhook", []byte("{}"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "do not support inbound messages")
}
