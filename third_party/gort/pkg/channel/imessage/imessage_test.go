package imessage

import (
	"context"
	"runtime"
	"testing"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/DotNetAge/gort/pkg/channel/imsg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	config := Config{
		DefaultService: "iMessage",
		Region:         "US",
	}

	ch, err := NewChannel("imessage", config)
	if err != nil {
		// imsg CLI might not be installed
		assert.True(t, err == ErrIMsgNotInstalled || err == ErrNotMacOS)
		return
	}

	require.NotNil(t, ch)
	assert.Equal(t, "imessage", ch.Name())
	assert.Equal(t, channel.ChannelTypeIMessage, ch.Type())
}

func TestNewChannel_NotMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Test requires non-macOS system")
	}

	config := Config{}
	_, err := NewChannel("imessage", config)
	assert.Equal(t, ErrNotMacOS, err)
}

func TestChannel_GetCapabilities(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	config := Config{
		DefaultService:         "iMessage",
		EnableTypingIndicators: true,
		EnableReactions:        true,
	}

	ch, err := NewChannel("imessage", config)
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	caps := ch.GetCapabilities()
	assert.True(t, caps.TextMessages)
	assert.True(t, caps.ImageMessages)
	assert.True(t, caps.AudioMessages)
	assert.True(t, caps.VideoMessages)
	assert.True(t, caps.FileMessages)
	assert.False(t, caps.LocationMessages)
	assert.False(t, caps.TemplateMessages)
	assert.False(t, caps.ReadReceipts)
}

func TestNormalizeHandle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"+1 (415) 555-1212", "+14155551212"},
		{"+44 20 7123 4567", "+442071234567"},
		{"tel:+1-555-123-4567", "+15551234567"},
		{"user@example.com", "user@example.com"},
		{"  +86 138 1234 5678  ", "+8613812345678"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ch.normalizeHandle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345", true},
		{"", false},
		{"123abc", false},
		{"+123", false},
		{"000", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMessage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	// Test text message conversion
	t.Run("text message", func(t *testing.T) {
		imsgMsg := &imsg.Message{
			ID:        12345,
			ChatID:    67890,
			GUID:      "test-guid-123",
			Sender:    "+14155551212",
			IsFromMe:  false,
			Text:      "Hello, World!",
			CreatedAt: "2024-01-15T10:30:00Z",
		}

		msg := ch.convertMessage(imsgMsg)
		require.NotNil(t, msg)
		assert.Equal(t, "12345", msg.ID)
		assert.Equal(t, "Hello, World!", msg.Content)
		assert.Equal(t, channel.MessageTypeText, msg.Type)
		assert.Equal(t, "+14155551212", msg.From.ID)
		assert.Equal(t, "67890", msg.To.ID)
	})

	// Test reaction message conversion
	t.Run("reaction message", func(t *testing.T) {
		imsgMsg := &imsg.Message{
			ID:            12346,
			ChatID:        67890,
			GUID:          "reaction-guid",
			Sender:        "+14155551212",
			IsFromMe:      false,
			IsReaction:    true,
			ReactionType:  "love",
			ReactionEmoji: "❤️",
			IsReactionAdd: true,
			ReactedToGUID: "original-message-guid",
		}

		msg := ch.convertMessage(imsgMsg)
		require.NotNil(t, msg)
		assert.Equal(t, channel.MessageTypeEvent, msg.Type)

		isReaction, _ := msg.GetMetadata("is_reaction")
		assert.Equal(t, true, isReaction)

		reactionType, _ := msg.GetMetadata("reaction_type")
		assert.Equal(t, "love", reactionType)
	})

	// Test message with attachment
	t.Run("message with attachment", func(t *testing.T) {
		imsgMsg := &imsg.Message{
			ID:     12347,
			ChatID: 67890,
			Text:   "Check out this photo",
			Attachments: []imsg.Attachment{
				{
					Filename:     "photo.jpg",
					MIMEType:     "image/jpeg",
					OriginalPath: "/path/to/photo.jpg",
				},
			},
		}

		msg := ch.convertMessage(imsgMsg)
		require.NotNil(t, msg)
		assert.Equal(t, channel.MessageTypeImage, msg.Type)

		attachPath, _ := msg.GetMetadata("attachment_path")
		assert.Equal(t, "/path/to/photo.jpg", attachPath)
	})

	// Test nil message
	t.Run("nil message", func(t *testing.T) {
		msg := ch.convertMessage(nil)
		assert.Nil(t, msg)
	})
}

func TestChannel_SendMessage_NotRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	msg := channel.NewMessage(
		"test-id",
		"imessage",
		channel.DirectionOutbound,
		channel.UserInfo{ID: "+14155551212"},
		"Test message",
		channel.MessageTypeText,
	)

	err = ch.SendMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestChannel_SendMessage_NilMessage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	err = ch.SendMessage(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestChannel_SendMessage_NoRecipient(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	// Start the channel with a mock handler
	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	if err != nil {
		t.Skipf("Cannot start channel: %v", err)
	}
	defer ch.Stop(ctx)

	msg := channel.NewMessage(
		"test-id",
		"imessage",
		channel.DirectionOutbound,
		channel.UserInfo{}, // No recipient
		"Test message",
		channel.MessageTypeText,
	)

	err = ch.SendMessage(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recipient")
}

func TestChannel_Stop(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	if err != nil {
		t.Skipf("Cannot start channel: %v", err)
	}

	assert.True(t, ch.IsRunning())

	err = ch.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, ch.IsRunning())
}

func TestChannel_GetChats_NotRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	_, err = ch.GetChats(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestChannel_GetHistory_NotRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	ch, err := NewChannel("imessage", Config{})
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	_, err = ch.GetHistory(context.Background(), 12345, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestChannel_SendReaction_NotEnabled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	config := Config{
		EnableReactions: false,
	}

	ch, err := NewChannel("imessage", config)
	if err != nil {
		t.Skipf("imsg not installed: %v", err)
	}

	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	if err != nil {
		t.Skipf("Cannot start channel: %v", err)
	}
	defer ch.Stop(ctx)

	err = ch.SendReaction(ctx, 12345, "message-guid", "love")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}
