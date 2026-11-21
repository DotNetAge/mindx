package imessage

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChannel_Integration_SendMessage tests sending a real iMessage.
// This test requires:
// - macOS with Messages.app signed in
// - Automation permission granted
// - A valid recipient phone number
func TestChannel_Integration_SendMessage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage integration tests require macOS")
	}

	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	// Get recipient from environment or use default
	recipient := os.Getenv("IMSG_RECIPIENT")
	if recipient == "" {
		recipient = "+8613580538348"
	}

	t.Logf("Sending test message to: %s", recipient)

	// Create channel
	config := Config{
		DefaultService: "iMessage",
		Region:         "CN",
	}

	ch, err := NewChannel("imessage-test", config)
	require.NoError(t, err, "Failed to create channel")
	require.NotNil(t, ch)

	// Start channel
	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		t.Logf("Received message: %s", msg.Content)
		return nil
	}

	err = ch.Start(ctx, handler)
	require.NoError(t, err, "Failed to start channel")
	defer ch.Stop(ctx)

	// Wait for channel to be ready
	time.Sleep(2 * time.Second)

	// Create test message
	msg := channel.NewMessage(
		fmt.Sprintf("test-%d", time.Now().Unix()),
		"imessage-test",
		channel.DirectionOutbound,
		channel.UserInfo{
			ID:       "me",
			Platform: "imessage",
		},
		fmt.Sprintf("Test message from gort at %s", time.Now().Format("15:04:05")),
		channel.MessageTypeText,
	)
	msg.To = channel.UserInfo{
		ID:       recipient,
		Platform: "imessage",
	}

	// Send message
	t.Log("Sending message...")
	err = ch.SendMessage(ctx, msg)
	require.NoError(t, err, "Failed to send message")
	t.Log("Message sent successfully!")

	// Wait a bit to ensure message is delivered
	time.Sleep(3 * time.Second)
}

// TestChannel_Integration_SendMultipleMessages tests sending multiple messages in sequence.
func TestChannel_Integration_SendMultipleMessages(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage integration tests require macOS")
	}

	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	// Get recipient from environment or use default
	recipient := os.Getenv("IMSG_RECIPIENT")
	if recipient == "" {
		recipient = "+8613580538348"
	}

	// Create channel
	config := Config{
		DefaultService: "iMessage",
		Region:         "CN",
	}

	ch, err := NewChannel("imessage-test", config)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Start channel
	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	require.NoError(t, err)
	defer ch.Stop(ctx)

	// Wait for channel to be ready
	time.Sleep(2 * time.Second)

	// Send multiple messages
	messages := []string{
		"Test message 1/3",
		"Test message 2/3",
		"Test message 3/3",
	}

	for i, text := range messages {
		msg := channel.NewMessage(
			fmt.Sprintf("test-%d-%d", time.Now().Unix(), i),
			"imessage-test",
			channel.DirectionOutbound,
			channel.UserInfo{
				ID:       "me",
				Platform: "imessage",
			},
			text,
			channel.MessageTypeText,
		)
		msg.To = channel.UserInfo{
			ID:       recipient,
			Platform: "imessage",
		}

		t.Logf("Sending message %d/%d: %s", i+1, len(messages), text)
		err = ch.SendMessage(ctx, msg)
		require.NoError(t, err, "Failed to send message %d", i+1)

		// Small delay between messages
		time.Sleep(2 * time.Second)
	}

	t.Log("All messages sent successfully!")
}

// TestChannel_Integration_SendWithChinese tests sending Chinese characters.
func TestChannel_Integration_SendWithChinese(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage integration tests require macOS")
	}

	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	// Get recipient from environment or use default
	recipient := os.Getenv("IMSG_RECIPIENT")
	if recipient == "" {
		recipient = "+8613725260426"
	}

	// Create channel
	config := Config{
		DefaultService: "iMessage",
		Region:         "CN",
	}

	ch, err := NewChannel("imessage-test", config)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Start channel
	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	require.NoError(t, err)
	defer ch.Stop(ctx)

	// Wait for channel to be ready
	time.Sleep(2 * time.Second)

	// Send message with Chinese characters
	msg := channel.NewMessage(
		fmt.Sprintf("test-%d", time.Now().Unix()),
		"imessage-test",
		channel.DirectionOutbound,
		channel.UserInfo{
			ID:       "me",
			Platform: "imessage",
		},
		"这是一条中文测试消息 🎉",
		channel.MessageTypeText,
	)
	msg.To = channel.UserInfo{
		ID:       recipient,
		Platform: "imessage",
	}

	t.Log("Sending Chinese message...")
	err = ch.SendMessage(ctx, msg)
	require.NoError(t, err, "Failed to send Chinese message")
	t.Log("Chinese message sent successfully!")
}

// TestChannel_Integration_SendWithSpecialChars tests sending special characters.
func TestChannel_Integration_SendWithSpecialChars(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage integration tests require macOS")
	}

	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	// Get recipient from environment or use default
	recipient := os.Getenv("IMSG_RECIPIENT")
	if recipient == "" {
		recipient = "+8613580538348"
	}

	// Create channel
	config := Config{
		DefaultService: "iMessage",
		Region:         "CN",
	}

	ch, err := NewChannel("imessage-test", config)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Start channel
	ctx := context.Background()
	handler := func(ctx context.Context, msg *channel.Message) error {
		return nil
	}

	err = ch.Start(ctx, handler)
	require.NoError(t, err)
	defer ch.Stop(ctx)

	// Wait for channel to be ready
	time.Sleep(2 * time.Second)

	// Send message with special characters
	msg := channel.NewMessage(
		fmt.Sprintf("test-%d", time.Now().Unix()),
		"imessage-test",
		channel.DirectionOutbound,
		channel.UserInfo{
			ID:       "me",
			Platform: "imessage",
		},
		`Special chars: "quotes", 'apostrophes', backslash\, newline
and tab`,
		channel.MessageTypeText,
	)
	msg.To = channel.UserInfo{
		ID:       recipient,
		Platform: "imessage",
	}

	t.Log("Sending special characters message...")
	err = ch.SendMessage(ctx, msg)
	require.NoError(t, err, "Failed to send special characters message")
	t.Log("Special characters message sent successfully!")
}

// TestChannel_Capabilities tests that capabilities are correctly set for send-only mode.
func TestChannel_Capabilities(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iMessage tests require macOS")
	}

	config := Config{
		DefaultService: "iMessage",
		Region:         "CN",
	}

	ch, err := NewChannel("imessage-test", config)
	if err != nil {
		t.Skipf("Failed to create channel: %v", err)
	}

	caps := ch.GetCapabilities()

	// Send-only mode capabilities
	assert.True(t, caps.TextMessages, "Should support text messages")
	assert.True(t, caps.ImageMessages, "Should support image messages")
	assert.True(t, caps.FileMessages, "Should support file messages")
	assert.False(t, caps.ReadReceipts, "Should not support read receipts in send-only mode")
	assert.False(t, caps.TypingIndicators, "Should not support typing indicators in send-only mode")
	assert.False(t, caps.ReactionMessages, "Should not support reactions in send-only mode")
}
