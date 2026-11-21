package messenger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessengerChannel(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
		AppSecret:       "test_secret",
		VerifyToken:     "verify_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)
	assert.NotNil(t, ch)
	assert.Equal(t, "messenger", ch.Name())
	assert.Equal(t, channel.ChannelTypeMessenger, ch.Type())
}

func TestNewMessengerChannel_EmptyToken(t *testing.T) {
	config := Config{
		PageID: "123456789",
	}

	_, err := NewMessengerChannel("messenger", config)
	assert.Equal(t, ErrPageAccessTokenEmpty, err)
}

func TestMessengerChannel_StartStop(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	err = ch.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ch.IsRunning())
	assert.Equal(t, channel.StatusRunning, ch.GetStatus())

	err = ch.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, ch.IsRunning())
	assert.Equal(t, channel.StatusStopped, ch.GetStatus())
}

func TestMessengerChannel_VerifyWebhook(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
		VerifyToken:     "my_verify_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	challenge, err := ch.VerifyWebhook("subscribe", "my_verify_token", "challenge123")
	require.NoError(t, err)
	assert.Equal(t, "challenge123", challenge)

	_, err = ch.VerifyWebhook("subscribe", "wrong_token", "challenge123")
	assert.Error(t, err)
}

func TestMessengerChannel_ParseWebhookData(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	webhookData := map[string]interface{}{
		"object": "page",
		"entry": []map[string]interface{}{
			{
				"id":   "123456789",
				"time": 1234567890,
				"messaging": []map[string]interface{}{
					{
						"sender": map[string]string{
							"id": "user_001",
						},
						"recipient": map[string]string{
							"id": "page_001",
						},
						"timestamp": 1234567890,
						"message": map[string]interface{}{
							"mid":  "msg_001",
							"text": "Hello, World!",
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(webhookData)
	require.NoError(t, err)

	msg, err := ch.HandleWebhook("/webhook", data)
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "msg_001", msg.ID)
	assert.Equal(t, "Hello, World!", msg.Content)
	assert.Equal(t, channel.MessageTypeText, msg.Type)
	assert.Equal(t, channel.DirectionInbound, msg.Direction)
	assert.Equal(t, "user_001", msg.From.ID)
}

func TestMessengerChannel_ParseWebhookData_Image(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	webhookData := map[string]interface{}{
		"object": "page",
		"entry": []map[string]interface{}{
			{
				"id":   "123456789",
				"time": 1234567890,
				"messaging": []map[string]interface{}{
					{
						"sender": map[string]string{
							"id": "user_001",
						},
						"recipient": map[string]string{
							"id": "page_001",
						},
						"timestamp": 1234567890,
						"message": map[string]interface{}{
							"mid": "msg_002",
							"attachments": []map[string]interface{}{
								{
									"type": "image",
									"payload": map[string]string{
										"url": "https://example.com/image.jpg",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(webhookData)
	require.NoError(t, err)

	msg, err := ch.HandleWebhook("/webhook", data)
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "msg_002", msg.ID)
	assert.Equal(t, "https://example.com/image.jpg", msg.Content)
	assert.Equal(t, channel.MessageTypeImage, msg.Type)
}

func TestMessengerChannel_ParseWebhookData_InvalidObject(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	webhookData := map[string]interface{}{
		"object": "invalid",
	}

	data, err := json.Marshal(webhookData)
	require.NoError(t, err)

	msg, err := ch.HandleWebhook("/webhook", data)
	require.NoError(t, err)
	assert.Nil(t, msg)
}

func TestMessengerChannel_VerifySignature(t *testing.T) {
	config := Config{
		PageID:          "123456789",
		PageAccessToken: "test_token",
		AppSecret:       "my_app_secret",
	}

	ch, err := NewMessengerChannel("messenger", config)
	require.NoError(t, err)

	data := []byte(`{"object":"page"}`)
	validSig := "sha256=" + computeHMACSHA256(data, "my_app_secret")

	assert.True(t, ch.verifySignature(data, validSig))
	assert.False(t, ch.verifySignature(data, "sha256=invalid"))
	assert.False(t, ch.verifySignature(data, "invalid_format"))
}

func computeHMACSHA256(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
