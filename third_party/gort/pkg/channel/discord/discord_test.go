// Package discord provides a Channel adapter for Discord.
//
// Official API Documentation: https://discord.com/developers/docs
// API Version: Discord API v10
package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
)

// setupTestServer creates a mock Discord API server for testing.
func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "401: Unauthorized"})
			return
		}

		switch r.URL.Path {
		case "/api/v10/gateway":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"url": "wss://gateway.discord.gg",
			})
		case "/api/v10/channels/channel-1/messages":
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"id": "msg-id"})
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/v10/channels/channel-1/messages/msg-1":
			switch r.Method {
			case http.MethodPatch:
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"id": "edited-msg"})
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/v10/channels/channel-1/messages/msg-1/reactions/👍/@me":
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "Unknown Channel"})
		}
	}))
}

// TestNewChannel tests the NewChannel function.
func TestNewChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid config",
			config: Config{
				BotToken:         "test-token",
				DefaultChannelID: "123456789",
				HTTPTimeout:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name:        "missing token",
			config:      Config{},
			wantErr:     true,
			expectedErr: ErrTokenRequired,
		},
		{
			name: "with all fields",
			config: Config{
				BotToken:         "test-token",
				ApplicationID:    "app-id",
				PublicKey:        "public-key",
				DefaultChannelID: "123456789",
				HTTPTimeout:      60 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := NewChannel("test-discord", tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewChannel() expected error but got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("NewChannel() error = %v, expected %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("NewChannel() unexpected error = %v", err)
				return
			}

			if ch == nil {
				t.Error("NewChannel() returned nil channel")
				return
			}

			if ch.Name() != "test-discord" {
				t.Errorf("channel name = %v, want test-discord", ch.Name())
			}

			if ch.Type() != channel.ChannelTypeDiscord {
				t.Errorf("channel type = %v, want %v", ch.Type(), channel.ChannelTypeDiscord)
			}

			// Verify HTTP timeout
			timeout := ch.config.HTTPTimeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			if ch.httpClient.Timeout != timeout {
				t.Errorf("HTTP timeout = %v, want %v", ch.httpClient.Timeout, timeout)
			}
		})
	}
}

// TestConfig_Validate tests the Config.Validate method.
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid config",
			config: Config{
				BotToken: "test-token",
			},
			wantErr: false,
		},
		{
			name:        "empty token",
			config:      Config{},
			wantErr:     true,
			expectedErr: ErrTokenRequired,
		},
		{
			name: "valid with all fields",
			config: Config{
				BotToken:         "bot-token",
				ApplicationID:    "app-id",
				PublicKey:        "public-key",
				DefaultChannelID: "channel-id",
				HTTPTimeout:      30 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("Config.Validate() expected error but got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("Config.Validate() error = %v, expected %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Config.Validate() unexpected error = %v", err)
			}
		})
	}
}

// TestChannel_StartStop tests the Start and Stop methods.
func TestChannel_StartStop(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL + "/api/v10"
	defer func() { BaseURL = originalBaseURL }()

	ch, err := NewChannelWithHTTPClient("test", Config{
		BotToken:         "test-token",
		DefaultChannelID: "123456789",
	}, server.Client())
	if err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	ctx := context.Background()

	// Test Start
	err = ch.Start(ctx, func(ctx context.Context, msg *channel.Message) error {
		return nil
	})
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if ch.GetStatus() != channel.StatusRunning {
		t.Errorf("status = %v, want %v", ch.GetStatus(), channel.StatusRunning)
	}

	// Test Stop
	err = ch.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if ch.GetStatus() != channel.StatusStopped {
		t.Errorf("status = %v, want %v", ch.GetStatus(), channel.StatusStopped)
	}
}

// TestChannel_SendMessage tests the SendMessage method.
func TestChannel_SendMessage(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL + "/api/v10"
	defer func() { BaseURL = originalBaseURL }()

	ch, err := NewChannelWithHTTPClient("test", Config{
		BotToken:         "test-token",
		DefaultChannelID: "default-channel",
	}, server.Client())
	if err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// Set status to running
	ch.SetStatus(channel.StatusRunning)

	ctx := context.Background()

	tests := []struct {
		name    string
		msg     *channel.Message
		wantErr bool
	}{
		{
			name: "text message",
			msg: &channel.Message{
				ID:      "msg-1",
				Content: "Hello Discord",
				Type:    channel.MessageTypeText,
				To:      channel.UserInfo{ID: "channel-1"},
			},
			wantErr: false,
		},
		{
			name: "markdown message",
			msg: &channel.Message{
				ID:      "msg-2",
				Content: "**Bold** text",
				Type:    channel.MessageTypeMarkdown,
				To:      channel.UserInfo{ID: "channel-1"},
			},
			wantErr: false,
		},
		{
			name: "image message with URL",
			msg: func() *channel.Message {
				m := &channel.Message{
					ID:      "msg-3",
					Content: "Image message",
					Type:    channel.MessageTypeImage,
					To:      channel.UserInfo{ID: "channel-1"},
				}
				m.SetMetadata("image_url", "https://example.com/image.png")
				return m
			}(),
			wantErr: false,
		},
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "channel not running",
			msg: &channel.Message{
				ID:      "msg-4",
				Content: "Test",
				Type:    channel.MessageTypeText,
				To:      channel.UserInfo{ID: "channel-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "channel not running" {
				ch.SetStatus(channel.StatusStopped)
			} else {
				ch.SetStatus(channel.StatusRunning)
			}

			err := ch.SendMessage(ctx, tt.msg)

			if tt.wantErr {
				if err == nil {
					t.Error("SendMessage() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("SendMessage() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestChannel_SendMessage_NoChannelID tests sending without channel ID.
func TestChannel_SendMessage_NoChannelID(t *testing.T) {
	ch, err := NewChannel("test", Config{
		BotToken: "test-token",
		// No default channel ID
	})
	if err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	ch.SetStatus(channel.StatusRunning)

	msg := &channel.Message{
		ID:      "msg-1",
		Content: "Test message",
		Type:    channel.MessageTypeText,
		// No To.ID
	}

	ctx := context.Background()
	err = ch.SendMessage(ctx, msg)
	if err == nil {
		t.Error("SendMessage() expected error for missing channel ID but got nil")
	}
}

// TestBuildMessagePayload tests the buildMessagePayload method.
func TestBuildMessagePayload(t *testing.T) {
	ch, _ := NewChannel("test", Config{
		BotToken:         "test-token",
		DefaultChannelID: "channel-id",
	})

	tests := []struct {
		name         string
		msg          *channel.Message
		wantContent  string
		wantEmbeds   bool
		wantReplyRef bool
	}{
		{
			name: "text message",
			msg: &channel.Message{
				Content: "Hello World",
				Type:    channel.MessageTypeText,
			},
			wantContent: "Hello World",
		},
		{
			name: "markdown message",
			msg: &channel.Message{
				Content: "**Bold**",
				Type:    channel.MessageTypeMarkdown,
			},
			wantContent: "**Bold**",
		},
		{
			name: "image message with URL",
			msg: func() *channel.Message {
				m := &channel.Message{
					Content: "Image",
					Type:    channel.MessageTypeImage,
				}
				m.SetMetadata("image_url", "https://example.com/img.png")
				return m
			}(),
			wantEmbeds: true,
		},
		{
			name: "image message without URL",
			msg: &channel.Message{
				Content: "Image content",
				Type:    channel.MessageTypeImage,
			},
			wantContent: "Image content",
		},
		{
			name: "message with reply",
			msg: func() *channel.Message {
				m := &channel.Message{
					Content: "Reply",
					Type:    channel.MessageTypeText,
				}
				m.SetMetadata("reply_to_message_id", "original-msg-id")
				return m
			}(),
			wantContent:  "Reply",
			wantReplyRef: true,
		},
		{
			name: "message with embeds",
			msg: func() *channel.Message {
				m := &channel.Message{
					Content: "With embeds",
					Type:    channel.MessageTypeText,
				}
				m.SetMetadata("embeds", []map[string]interface{}{
					{"title": "Embed Title"},
				})
				return m
			}(),
			wantContent: "With embeds",
			wantEmbeds:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := ch.buildMessagePayload(tt.msg)

			if tt.wantContent != "" {
				if content, ok := payload["content"].(string); !ok || content != tt.wantContent {
					t.Errorf("content = %v, want %v", content, tt.wantContent)
				}
			}

			if tt.wantEmbeds {
				if _, ok := payload["embeds"]; !ok {
					t.Error("expected embeds in payload")
				}
			}

			if tt.wantReplyRef {
				if _, ok := payload["message_reference"]; !ok {
					t.Error("expected message_reference in payload")
				}
			}
		})
	}
}

// TestBuildEmbed tests the BuildEmbed function.
func TestBuildEmbed(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		url         string
		color       int
		wantURL     bool
	}{
		{
			name:        "basic embed",
			title:       "Test Title",
			description: "Test Description",
			url:         "",
			color:       0x00ff00,
			wantURL:     false,
		},
		{
			name:        "embed with URL",
			title:       "Test Title",
			description: "Test Description",
			url:         "https://example.com",
			color:       0xff0000,
			wantURL:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embed := BuildEmbed(tt.title, tt.description, tt.url, tt.color)

			if embed["title"] != tt.title {
				t.Errorf("title = %v, want %v", embed["title"], tt.title)
			}

			if embed["description"] != tt.description {
				t.Errorf("description = %v, want %v", embed["description"], tt.description)
			}

			if embed["color"] != tt.color {
				t.Errorf("color = %v, want %v", embed["color"], tt.color)
			}

			_, hasURL := embed["url"]
			if hasURL != tt.wantURL {
				t.Errorf("has URL = %v, want %v", hasURL, tt.wantURL)
			}

			if tt.wantURL && embed["url"] != tt.url {
				t.Errorf("url = %v, want %v", embed["url"], tt.url)
			}
		})
	}
}

// TestBuildEmbedField tests the BuildEmbedField function.
func TestBuildEmbedField(t *testing.T) {
	field := BuildEmbedField("Field Name", "Field Value", true)

	if field["name"] != "Field Name" {
		t.Errorf("name = %v, want Field Name", field["name"])
	}

	if field["value"] != "Field Value" {
		t.Errorf("value = %v, want Field Value", field["value"])
	}

	if field["inline"] != true {
		t.Errorf("inline = %v, want true", field["inline"])
	}

	// Test non-inline
	field2 := BuildEmbedField("Name2", "Value2", false)
	if field2["inline"] != false {
		t.Errorf("inline = %v, want false", field2["inline"])
	}
}

// TestBuildEmbedFooter tests the BuildEmbedFooter function.
func TestBuildEmbedFooter(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		iconURL  string
		wantIcon bool
	}{
		{
			name:     "footer without icon",
			text:     "Footer Text",
			iconURL:  "",
			wantIcon: false,
		},
		{
			name:     "footer with icon",
			text:     "Footer Text",
			iconURL:  "https://example.com/icon.png",
			wantIcon: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			footer := BuildEmbedFooter(tt.text, tt.iconURL)

			if footer["text"] != tt.text {
				t.Errorf("text = %v, want %v", footer["text"], tt.text)
			}

			_, hasIcon := footer["icon_url"]
			if hasIcon != tt.wantIcon {
				t.Errorf("has icon = %v, want %v", hasIcon, tt.wantIcon)
			}
		})
	}
}

// TestChannel_GetCapabilities tests the GetCapabilities method.
func TestChannel_GetCapabilities(t *testing.T) {
	ch, _ := NewChannel("test", Config{
		BotToken: "test-token",
	})

	caps := ch.GetCapabilities()

	// Verify all expected capabilities
	expectedTrue := map[string]bool{
		"TextMessages":     caps.TextMessages,
		"MarkdownMessages": caps.MarkdownMessages,
		"ImageMessages":    caps.ImageMessages,
		"FileMessages":     caps.FileMessages,
		"AudioMessages":    caps.AudioMessages,
		"VideoMessages":    caps.VideoMessages,
		"ReactionMessages": caps.ReactionMessages,
		"MessageEditing":   caps.MessageEditing,
		"MessageDeletion":  caps.MessageDeletion,
		"Threads":          caps.Threads,
	}

	for name, value := range expectedTrue {
		if !value {
			t.Errorf("GetCapabilities().%s = false, want true", name)
		}
	}

	// Verify capabilities that should be false
	if caps.LocationMessages {
		t.Error("GetCapabilities().LocationMessages = true, want false")
	}

	if caps.ReadReceipts {
		t.Error("GetCapabilities().ReadReceipts = true, want false")
	}

	if caps.TypingIndicators {
		t.Error("GetCapabilities().TypingIndicators = true, want false")
	}
}

// TestChannel_HandleWebhook tests the HandleWebhook method.
func TestChannel_HandleWebhook(t *testing.T) {
	ch, _ := NewChannel("test", Config{
		BotToken:  "test-token",
		PublicKey: "test-public-key",
	})

	tests := []struct {
		name         string
		path         string
		data         []byte
		wantErr      bool
		wantNil      bool
		expectedFrom string
		expectedTo   string
	}{
		{
			name: "valid message event",
			path: "/webhook/discord",
			data: []byte(`{
				"type": 0,
				"d": {
					"id": "msg-123",
					"channel_id": "channel-456",
					"guild_id": "guild-789",
					"author": {
						"id": "user-111",
						"username": "TestUser",
						"bot": false
					},
					"content": "Hello World",
					"timestamp": "2024-01-01T00:00:00.000Z"
				}
			}`),
			wantErr:      false,
			wantNil:      false,
			expectedFrom: "user-111",
			expectedTo:   "channel-456",
		},
		{
			name: "bot message - should be ignored",
			path: "/webhook/discord",
			data: []byte(`{
				"type": 0,
				"d": {
					"id": "msg-123",
					"channel_id": "channel-456",
					"author": {
						"id": "bot-111",
						"username": "BotUser",
						"bot": true
					},
					"content": "Bot message"
				}
			}`),
			wantErr: false,
			wantNil: true,
		},
		{
			name: "non-dispatch event",
			path: "/webhook/discord",
			data: []byte(`{
				"type": 1,
				"d": {}
			}`),
			wantErr: false,
			wantNil: true,
		},
		{
			name:    "invalid JSON",
			path:    "/webhook/discord",
			data:    []byte(`invalid json`),
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ch.HandleWebhook(tt.path, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("HandleWebhook() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("HandleWebhook() unexpected error = %v", err)
				return
			}

			if tt.wantNil {
				if msg != nil {
					t.Error("HandleWebhook() expected nil message but got non-nil")
				}
				return
			}

			if msg == nil {
				t.Error("HandleWebhook() returned nil message unexpectedly")
				return
			}

			if msg.From.ID != tt.expectedFrom {
				t.Errorf("From.ID = %v, want %v", msg.From.ID, tt.expectedFrom)
			}

			if msg.To.ID != tt.expectedTo {
				t.Errorf("To.ID = %v, want %v", msg.To.ID, tt.expectedTo)
			}
		})
	}
}

// TestChannel_EditMessage tests the EditMessage method.
func TestChannel_EditMessage(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL + "/api/v10"
	defer func() { BaseURL = originalBaseURL }()

	ch, _ := NewChannelWithHTTPClient("test", Config{
		BotToken: "test-token",
	}, server.Client())

	ctx := context.Background()
	msg := &channel.Message{
		Content: "Edited content",
		Type:    channel.MessageTypeText,
	}

	err := ch.EditMessage(ctx, "channel-1", "msg-1", msg)
	if err != nil {
		t.Errorf("EditMessage() error = %v", err)
	}
}

// TestChannel_DeleteMessage tests the DeleteMessage method.
func TestChannel_DeleteMessage(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL + "/api/v10"
	defer func() { BaseURL = originalBaseURL }()

	ch, _ := NewChannelWithHTTPClient("test", Config{
		BotToken: "test-token",
	}, server.Client())

	ctx := context.Background()
	err := ch.DeleteMessage(ctx, "channel-1", "msg-1")
	if err != nil {
		t.Errorf("DeleteMessage() error = %v", err)
	}
}

// TestChannel_AddReaction tests the AddReaction method.
func TestChannel_AddReaction(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL + "/api/v10"
	defer func() { BaseURL = originalBaseURL }()

	ch, _ := NewChannelWithHTTPClient("test", Config{
		BotToken: "test-token",
	}, server.Client())

	ctx := context.Background()
	err := ch.AddReaction(ctx, "channel-1", "msg-1", "👍")
	if err != nil {
		t.Errorf("AddReaction() error = %v", err)
	}
}

// TestChannel_UploadFile tests the UploadFile method.
func TestChannel_UploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("expected multipart/form-data Content-Type, got %s", contentType)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "file-msg"})
	}))
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	ch, _ := NewChannelWithHTTPClient("test", Config{
		BotToken: "test-token",
	}, server.Client())

	ctx := context.Background()
	fileData := []byte("test file content")

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{
			name:     "upload with content",
			filename: "test.txt",
			content:  "File description",
			wantErr:  false,
		},
		{
			name:     "upload without content",
			filename: "empty.txt",
			content:  "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ch.UploadFile(ctx, "channel-1", fileData, tt.filename, tt.content)
			if tt.wantErr {
				if err == nil {
					t.Error("UploadFile() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("UploadFile() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestParseResponse tests the parseResponse method.
func TestParseResponse(t *testing.T) {
	ch, _ := NewChannel("test", Config{
		BotToken: "test-token",
	})

	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		wantErr      bool
		expectedErr  error
	}{
		{
			name:         "success 200",
			statusCode:   http.StatusOK,
			responseBody: `{"id": "msg-1"}`,
			wantErr:      false,
		},
		{
			name:         "created 201",
			statusCode:   http.StatusCreated,
			responseBody: `{"id": "msg-1"}`,
			wantErr:      false,
		},
		{
			name:         "no content 204",
			statusCode:   http.StatusNoContent,
			responseBody: "",
			wantErr:      false,
		},
		{
			name:         "not found 404",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message": "Unknown Channel"}`,
			wantErr:      true,
			expectedErr:  ErrChannelNotFound,
		},
		{
			name:         "forbidden 403",
			statusCode:   http.StatusForbidden,
			responseBody: `{"message": "Missing Access"}`,
			wantErr:      true,
			expectedErr:  ErrForbidden,
		},
		{
			name:         "rate limited 429",
			statusCode:   http.StatusTooManyRequests,
			responseBody: `{"message": "You are being rate limited"}`,
			wantErr:      true,
			expectedErr:  ErrRateLimited,
		},
		{
			name:         "bad request 400",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"message": "Invalid Form Body"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
			}

			err := ch.parseResponse(resp, nil)

			if tt.wantErr {
				if err == nil {
					t.Error("parseResponse() expected error but got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("parseResponse() error = %v, expected %v", err, tt.expectedErr)
				}
			} else {
				if err != nil {
					t.Errorf("parseResponse() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestUrlEncodeEmoji tests the urlEncodeEmoji function.
func TestUrlEncodeEmoji(t *testing.T) {
	tests := []struct {
		name     string
		emoji    string
		expected string
	}{
		{
			name:     "unicode emoji",
			emoji:    "👍",
			expected: "%F0%9F%91%8D",
		},
		{
			name:     "custom emoji",
			emoji:    "custom:123456789",
			expected: "custom:123456789",
		},
		{
			name:     "heart emoji",
			emoji:    "❤️",
			expected: "%E2%9D%A4%EF%B8%8F",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := urlEncodeEmoji(tt.emoji)
			if result != tt.expected {
				t.Errorf("urlEncodeEmoji() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestChannel_Start_InvalidToken tests starting with invalid token.
func TestChannel_Start_InvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 401 for invalid token
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "401: Unauthorized",
		})
	}))
	defer server.Close()

	// Override base URL for testing
	originalBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	ch, _ := NewChannelWithHTTPClient("test", Config{
		BotToken:         "invalid-token",
		DefaultChannelID: "123456789",
	}, server.Client())

	ctx := context.Background()
	err := ch.Start(ctx, func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	if err == nil {
		t.Error("Start() expected error for invalid token but got nil")
	}
}

// TestConstants tests the exported constants.
func TestConstants(t *testing.T) {
	// Verify base URL
	if BaseURL != "https://discord.com/api/v10" {
		t.Errorf("BaseURL = %v, want https://discord.com/api/v10", BaseURL)
	}

	// Verify endpoint constants
	endpoints := map[string]string{
		"EndpointCreateMessage":  EndpointCreateMessage,
		"EndpointEditMessage":    EndpointEditMessage,
		"EndpointDeleteMessage":  EndpointDeleteMessage,
		"EndpointCreateReaction": EndpointCreateReaction,
		"EndpointGetChannel":     EndpointGetChannel,
		"EndpointGetGuild":       EndpointGetGuild,
		"EndpointGateway":        EndpointGateway,
		"EndpointGatewayBot":     EndpointGatewayBot,
	}

	for name, value := range endpoints {
		if value == "" {
			t.Errorf("%s is empty", name)
		}
	}
}

// TestErrorDefinitions tests the error definitions.
func TestErrorDefinitions(t *testing.T) {
	errors := map[string]error{
		"ErrTokenRequired":   ErrTokenRequired,
		"ErrInvalidToken":    ErrInvalidToken,
		"ErrChannelNotFound": ErrChannelNotFound,
		"ErrMessageNotFound": ErrMessageNotFound,
		"ErrForbidden":       ErrForbidden,
		"ErrRateLimited":     ErrRateLimited,
	}

	for name, err := range errors {
		if err == nil {
			t.Errorf("%s is nil", name)
		}
	}
}
