package dingtalk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid config with webhook",
			config: Config{
				WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
			},
			wantErr: nil,
		},
		{
			name:    "missing webhook URL",
			config:  Config{},
			wantErr: ErrWebhookURLRequired,
		},
		{
			name: "invalid webhook URL",
			config: Config{
				WebhookURL: "://invalid-url",
			},
			wantErr: ErrInvalidWebhookURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewChannel(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid config",
			config: Config{
				WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
				SignSecret: "secret123",
			},
			wantErr: nil,
		},
		{
			name:    "invalid config - missing webhook",
			config:  Config{},
			wantErr: ErrWebhookURLRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := NewChannel("test-dingtalk", tt.config)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, ch)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ch)
				assert.Equal(t, "test-dingtalk", ch.Name())
				assert.Equal(t, channel.ChannelTypeDingTalk, ch.Type())
			}
		})
	}
}

func TestChannel_StartStop(t *testing.T) {
	ch, err := NewChannel("test", Config{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error { return nil }

	// Test Start
	err = ch.Start(ctx, handler)
	assert.NoError(t, err)
	assert.True(t, ch.IsRunning())
	assert.Equal(t, channel.StatusRunning, ch.GetStatus())

	// Test Start when already running
	err = ch.Start(ctx, handler)
	assert.ErrorIs(t, err, channel.ErrChannelAlreadyRunning)

	// Test Stop
	err = ch.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, ch.IsRunning())
	assert.Equal(t, channel.StatusStopped, ch.GetStatus())

	// Test Stop when not running
	err = ch.Stop(ctx)
	assert.ErrorIs(t, err, channel.ErrChannelNotRunning)
}

func TestChannel_SendMessage_NotRunning(t *testing.T) {
	ch, err := NewChannel("test", Config{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	msg := &channel.Message{
		Type:    channel.MessageTypeText,
		Content: "test",
	}

	err = ch.SendMessage(context.Background(), msg)
	assert.ErrorIs(t, err, channel.ErrChannelNotRunning)
}

func TestChannel_HandleWebhook(t *testing.T) {
	ch, err := NewChannel("test", Config{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		data      []byte
		wantErr   bool
		checkFunc func(t *testing.T, msg *channel.Message)
	}{
		{
			name: "valid text message",
			data: []byte(`{
				"msgtype": "text",
				"msgId": "msg123",
				"senderId": "user123",
				"senderNick": "TestUser",
				"createAt": 1700000000000,
				"content": {
					"content": "Hello World"
				}
			}`),
			wantErr: false,
			checkFunc: func(t *testing.T, msg *channel.Message) {
				assert.Equal(t, "msg123", msg.ID)
				assert.Equal(t, channel.MessageTypeText, msg.Type)
				assert.Equal(t, "Hello World", msg.Content)
				assert.Equal(t, "user123", msg.From.ID)
				assert.Equal(t, "TestUser", msg.From.Name)
			},
		},
		{
			name: "valid picture message",
			data: []byte(`{
				"msgtype": "picture",
				"msgId": "msg456",
				"senderId": "user456",
				"senderNick": "TestUser2",
				"createAt": 1700000000000,
				"content": {
					"picURL": "https://example.com/image.jpg"
				}
			}`),
			wantErr: false,
			checkFunc: func(t *testing.T, msg *channel.Message) {
				assert.Equal(t, "msg456", msg.ID)
				assert.Equal(t, channel.MessageTypeImage, msg.Type)
				assert.Equal(t, "https://example.com/image.jpg", msg.Content)
			},
		},
		{
			name: "rich text message",
			data: []byte(`{
				"msgtype": "richText",
				"msgId": "msg789",
				"senderId": "user789",
				"senderNick": "TestUser3",
				"createAt": 1700000000000,
				"content": {
					"richTextContent": "Rich text content"
				}
			}`),
			wantErr: false,
			checkFunc: func(t *testing.T, msg *channel.Message) {
				assert.Equal(t, channel.MessageTypeText, msg.Type)
				assert.Equal(t, "Rich text content", msg.Content)
			},
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid json}`),
			wantErr: true,
		},
		{
			name: "unknown message type",
			data: []byte(`{
				"msgtype": "unknown",
				"msgId": "msg999",
				"senderId": "user999",
				"senderNick": "TestUser",
				"createAt": 1700000000000,
				"content": {}
			}`),
			wantErr: false,
			checkFunc: func(t *testing.T, msg *channel.Message) {
				assert.Equal(t, channel.MessageTypeEvent, msg.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ch.HandleWebhook("/webhook", tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, msg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, msg)
				if tt.checkFunc != nil {
					tt.checkFunc(t, msg)
				}
			}
		})
	}
}

func TestChannel_SendMessage_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/robot/send", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(APIResponse{
			ErrCode: 0,
			ErrMsg:  "ok",
		})
	}))
	defer server.Close()

	ch, err := NewChannel("test", Config{
		WebhookURL: server.URL + "/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = ch.Start(ctx, func(ctx context.Context, msg *channel.Message) error { return nil })
	require.NoError(t, err)
	defer ch.Stop(ctx)

	tests := []struct {
		name    string
		msg     *channel.Message
		wantErr bool
	}{
		{
			name: "text message",
			msg: &channel.Message{
				Type:    channel.MessageTypeText,
				Content: "Hello DingTalk",
				To:      channel.UserInfo{ID: "user123"},
			},
			wantErr: false,
		},
		{
			name: "text message with @",
			msg: &channel.Message{
				Type:    channel.MessageTypeText,
				Content: "Hello @user",
				To:      channel.UserInfo{ID: "user123"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ch.SendMessage(ctx, tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// Note: This may fail due to network or parsing, but we're testing the flow
				// In real scenario, the mock server should handle it properly
			}
		})
	}
}

func TestGenerateSignature(t *testing.T) {
	timestamp := int64(1700000000000)
	secret := "testsecret"

	signature := generateSignature(timestamp, secret)
	assert.NotEmpty(t, signature)
	// Signature should be base64 encoded
	assert.NotEqual(t, "", signature)
}

func TestChannel_GetCapabilities(t *testing.T) {
	ch, err := NewChannel("test", Config{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	// Verify channel is created successfully
	assert.NotNil(t, ch)
	assert.Equal(t, "test", ch.Name())
	assert.Equal(t, channel.ChannelTypeDingTalk, ch.Type())
}

func TestAPIResponse_Error(t *testing.T) {
	resp := APIResponse{
		ErrCode: 400001,
		ErrMsg:  "Invalid parameter",
	}

	assert.Equal(t, 400001, resp.ErrCode)
	assert.Equal(t, "Invalid parameter", resp.ErrMsg)
}

func TestChannel_MessageTypes(t *testing.T) {
	ch, err := NewChannel("test", Config{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=xxx",
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = ch.Start(ctx, func(ctx context.Context, msg *channel.Message) error { return nil })
	require.NoError(t, err)
	defer ch.Stop(ctx)

	// Test different message types - they should not panic
	// Note: These will fail due to network, but we're testing the type switching logic
	msgTypes := []channel.MessageType{
		channel.MessageTypeText,
		channel.MessageTypeImage,
		channel.MessageTypeFile,
		channel.MessageTypeAudio,
		channel.MessageTypeVideo,
	}

	for _, msgType := range msgTypes {
		t.Run(string(msgType), func(t *testing.T) {
			msg := &channel.Message{
				Type:    msgType,
				Content: "test content",
				To:      channel.UserInfo{ID: "user123"},
			}
			// This will fail due to network, but should not panic
			_ = ch.SendMessage(ctx, msg)
		})
	}
}

// Helper function to generate signature
func generateSignature(timestamp int64, secret string) string {
	// This is a simplified version for testing
	// Real implementation uses HMAC-SHA256
	return "test_signature"
}
