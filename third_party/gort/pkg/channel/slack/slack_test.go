package slack

import (
	"context"
	"strings"
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
			name: "valid config",
			config: Config{
				BotToken: "xoxb-test-token",
			},
			expectError: false,
		},
		{
			name:        "empty token",
			config:      Config{},
			expectError: true,
			errType:     ErrTokenRequired,
		},
		{
			name: "invalid token format",
			config: Config{
				BotToken: "invalid-token",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := NewChannel("slack", tt.config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
				assert.Nil(t, ch)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ch)
				assert.Equal(t, "slack", ch.Name())
				assert.Equal(t, channel.ChannelTypeSlack, ch.Type())
			}
		})
	}
}

func TestChannel_GetCapabilities(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	caps := ch.GetCapabilities()
	assert.True(t, caps.TextMessages)
	assert.True(t, caps.MarkdownMessages)
	assert.True(t, caps.ImageMessages)
	assert.True(t, caps.FileMessages)
	assert.True(t, caps.BlockKit)
	assert.True(t, caps.ReactionMessages)
	assert.True(t, caps.Threads)
	assert.True(t, caps.Interactive)
	assert.True(t, caps.MessageEditing)
	assert.True(t, caps.MessageDeletion)
	assert.False(t, caps.LocationMessages)
	assert.False(t, caps.ReadReceipts)
}

func TestBuildMessagePayload_Text(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "C123456"},
		"Hello, World!",
		channel.MessageTypeText,
	)

	payload := ch.buildMessagePayload(msg, "C123456")

	assert.Equal(t, "C123456", payload["channel"])
	assert.Equal(t, "Hello, World!", payload["text"])
}

func TestBuildMessagePayload_Markdown(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "C123456"},
		"*Bold* text",
		channel.MessageTypeMarkdown,
	)

	payload := ch.buildMessagePayload(msg, "C123456")

	assert.Equal(t, "C123456", payload["channel"])
	assert.Equal(t, "*Bold* text", payload["text"])
	assert.Equal(t, true, payload["mrkdwn"])
}

func TestBuildMessagePayload_WithThread(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "C123456"},
		"Reply in thread",
		channel.MessageTypeText,
	)
	msg.SetMetadata("thread_ts", "1234567890.123456")

	payload := ch.buildMessagePayload(msg, "C123456")

	assert.Equal(t, "1234567890.123456", payload["thread_ts"])
}

func TestBuildMessagePayload_WithBlocks(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "C123456"},
		"Message with blocks",
		channel.MessageTypeText,
	)

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Bold* text",
			},
		},
	}
	msg.SetMetadata("blocks", blocks)

	payload := ch.buildMessagePayload(msg, "C123456")

	assert.Contains(t, payload, "blocks")
}

func TestBuildButtonBlock(t *testing.T) {
	button := BuildButtonBlock("Click me", "action-1", "value-1", "primary")

	assert.Equal(t, "button", button["type"])
	assert.Equal(t, "action-1", button["action_id"])
	assert.Equal(t, "value-1", button["value"])
	assert.Equal(t, "primary", button["style"])

	text, ok := button["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "plain_text", text["type"])
	assert.Equal(t, "Click me", text["text"])
}

func TestBuildSelectMenuBlock(t *testing.T) {
	options := []map[string]interface{}{
		BuildOption("Option 1", "opt1"),
		BuildOption("Option 2", "opt2"),
	}

	menu := BuildSelectMenuBlock("Select an option", "select-1", options)

	assert.Equal(t, "static_select", menu["type"])
	assert.Equal(t, "select-1", menu["action_id"])
	assert.Contains(t, menu, "options")

	placeholder, ok := menu["placeholder"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Select an option", placeholder["text"])
}

func TestBuildOption(t *testing.T) {
	option := BuildOption("Option 1", "opt1")

	assert.Contains(t, option, "text")
	assert.Equal(t, "opt1", option["value"])

	text, ok := option["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Option 1", text["text"])
}

func TestComputeSignature(t *testing.T) {
	config := Config{
		BotToken:      "xoxb-test-token",
		SigningSecret: "8f742231b10e8888abcd1234567890abcdef",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	timestamp := "1531420618"
	body := []byte(`{"token":"test","challenge":"challenge123"}`)

	sig := ch.computeSignature(timestamp, body)
	assert.NotEmpty(t, sig)
	assert.True(t, strings.HasPrefix(sig, "v0="))
}

func TestVerifyWebhookSignature(t *testing.T) {
	config := Config{
		BotToken:      "xoxb-test-token",
		SigningSecret: "8f742231b10e8888abcd1234567890abcdef",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	timestamp := "1531420618"
	body := []byte(`{"token":"test","challenge":"challenge123"}`)

	// Compute expected signature
	expectedSig := ch.computeSignature(timestamp, body)

	// Verify correct signature
	valid := ch.VerifyWebhookSignature(body, timestamp, expectedSig)
	assert.True(t, valid)

	// Verify incorrect signature
	invalid := ch.VerifyWebhookSignature(body, timestamp, "v0=invalid")
	assert.False(t, invalid)
}

func TestVerifyWebhookSignature_NoSecret(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
		// No signing secret
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	// Should return true when no secret is configured
	valid := ch.VerifyWebhookSignature([]byte("test"), "123", "v0=anything")
	assert.True(t, valid)
}

func TestHandleWebhook_URLVerification(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	data := []byte(`{"type":"url_verification","challenge":"challenge123"}`)
	msg, err := ch.HandleWebhook("/webhook", data)

	assert.NoError(t, err)
	assert.Nil(t, msg) // URL verification returns nil
}

func TestHandleWebhook_Message(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	data := []byte(`{
		"type": "event_callback",
		"event": {
			"type": "message",
			"channel": "C123456",
			"user": "U123456",
			"text": "Hello from Slack",
			"ts": "1234567890.123456"
		}
	}`)

	msg, err := ch.HandleWebhook("/webhook", data)

	assert.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "Hello from Slack", msg.Content)
	assert.Equal(t, channel.MessageTypeText, msg.Type)
	assert.Equal(t, "U123456", msg.From.ID)
	assert.Equal(t, "C123456", msg.To.ID)
}

func TestHandleWebhook_BotMessage(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	data := []byte(`{
		"type": "event_callback",
		"event": {
			"type": "message",
			"channel": "C123456",
			"user": "",
			"bot_id": "B123456",
			"text": "Bot message"
		}
	}`)

	msg, err := ch.HandleWebhook("/webhook", data)

	assert.NoError(t, err)
	assert.Nil(t, msg) // Bot messages are ignored
}

func TestChannel_SendMessage_NotRunning(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "C123456"},
		"Test",
		channel.MessageTypeText,
	)

	err = ch.SendMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestChannel_SendMessage_NoChannel(t *testing.T) {
	config := Config{
		BotToken: "xoxb-test-token",
	}

	ch, err := NewChannel("slack", config)
	require.NoError(t, err)

	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	// Start without default channel
	err = ch.Start(ctx, handler)
	if err != nil && err != ErrTokenRequired {
		t.Skipf("Cannot start channel (expected for test token): %v", err)
	}
	defer ch.Stop(ctx)

	msg := channel.NewMessage(
		"msg-001",
		"slack",
		channel.DirectionOutbound,
		channel.UserInfo{}, // No channel
		"Test",
		channel.MessageTypeText,
	)

	err = ch.SendMessage(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel ID")
}
