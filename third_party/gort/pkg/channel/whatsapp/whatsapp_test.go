package whatsapp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWhatsAppChannel(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
		AppSecret:     "test_secret",
		VerifyToken:   "verify_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)
	assert.NotNil(t, ch)
	assert.Equal(t, "whatsapp", ch.Name())
	assert.Equal(t, channel.ChannelTypeWhatsApp, ch.Type())
}

func TestNewWhatsAppChannel_EmptyPhoneNumberID(t *testing.T) {
	config := Config{
		AccessToken: "test_token",
	}

	_, err := NewWhatsAppChannel("whatsapp", config)
	assert.Equal(t, ErrPhoneNumberIDEmpty, err)
}

func TestNewWhatsAppChannel_EmptyAccessToken(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
	}

	_, err := NewWhatsAppChannel("whatsapp", config)
	assert.Equal(t, ErrAccessTokenEmpty, err)
}

func TestWhatsAppChannel_StartStop(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
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

func TestWhatsAppChannel_VerifyWebhook(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
		VerifyToken:   "my_verify_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)

	challenge, err := ch.VerifyWebhook("subscribe", "my_verify_token", "challenge123")
	require.NoError(t, err)
	assert.Equal(t, "challenge123", challenge)

	_, err = ch.VerifyWebhook("subscribe", "wrong_token", "challenge123")
	assert.Error(t, err)
}

func TestWhatsAppChannel_ParseWebhookData_Text(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)

	webhookData := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []map[string]interface{}{
			{
				"id": "123456789",
				"changes": []map[string]interface{}{
					{
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]string{
								"display_phone_number": "+1234567890",
								"phone_number_id":      "123456789",
							},
							"contacts": []map[string]interface{}{
								{
									"profile": map[string]string{
										"name": "John Doe",
									},
									"wa_id": "987654321",
								},
							},
							"messages": []map[string]interface{}{
								{
									"from":      "987654321",
									"id":        "wamid.msg001",
									"timestamp": "1234567890",
									"type":      "text",
									"text": map[string]string{
										"body": "Hello, World!",
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

	assert.Equal(t, "wamid.msg001", msg.ID)
	assert.Equal(t, "Hello, World!", msg.Content)
	assert.Equal(t, channel.MessageTypeText, msg.Type)
	assert.Equal(t, channel.DirectionInbound, msg.Direction)
	assert.Equal(t, "987654321", msg.From.ID)
	assert.Equal(t, "John Doe", msg.From.Name)
	assert.Equal(t, "whatsapp", msg.From.Platform)
}

func TestWhatsAppChannel_ParseWebhookData_Image(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)

	webhookData := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []map[string]interface{}{
			{
				"id": "123456789",
				"changes": []map[string]interface{}{
					{
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]string{
								"display_phone_number": "+1234567890",
								"phone_number_id":      "123456789",
							},
							"contacts": []map[string]interface{}{
								{
									"profile": map[string]string{
										"name": "John Doe",
									},
									"wa_id": "987654321",
								},
							},
							"messages": []map[string]interface{}{
								{
									"from":      "987654321",
									"id":        "wamid.msg002",
									"timestamp": "1234567890",
									"type":      "image",
									"image": map[string]string{
										"id":       "media_001",
										"mime_type": "image/jpeg",
										"sha256":   "abc123",
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

	assert.Equal(t, "wamid.msg002", msg.ID)
	assert.Equal(t, "media_001", msg.Content)
	assert.Equal(t, channel.MessageTypeImage, msg.Type)
}

func TestWhatsAppChannel_ParseWebhookData_InvalidObject(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
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

func TestWhatsAppChannel_BuildMessagePayload_Text(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg_001",
		"whatsapp",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"Hello, World!",
		channel.MessageTypeText,
	)

	payload := ch.buildMessagePayload("987654321", msg)

	assert.Equal(t, "whatsapp", payload["messaging_product"])
	assert.Equal(t, "individual", payload["recipient_type"])
	assert.Equal(t, "987654321", payload["to"])
	assert.Equal(t, "text", payload["type"])

	textMap, ok := payload["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", textMap["body"])
}

func TestWhatsAppChannel_BuildMessagePayload_Image(t *testing.T) {
	config := Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test_token",
	}

	ch, err := NewWhatsAppChannel("whatsapp", config)
	require.NoError(t, err)

	msg := channel.NewMessage(
		"msg_001",
		"whatsapp",
		channel.DirectionOutbound,
		channel.UserInfo{},
		"https://example.com/image.jpg",
		channel.MessageTypeImage,
	)

	payload := ch.buildMessagePayload("987654321", msg)

	assert.Equal(t, "image", payload["type"])

	imageMap, ok := payload["image"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https://example.com/image.jpg", imageMap["link"])
}
