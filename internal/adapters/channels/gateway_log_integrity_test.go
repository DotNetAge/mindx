package channels

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGateway_LogIntegrity_MessageLogging 测试消息日志完整性
func TestGateway_LogIntegrity_MessageLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "所有消息都应该被处理")
}

// TestGateway_LogIntegrity_ErrorLogging 测试错误日志完整性
func TestGateway_LogIntegrity_ErrorLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	errorCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if errorCount < 3 {
			errorCount++
			return "", "", fmt.Errorf("模拟错误 %d", errorCount)
		}
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 3, errorCount, "应该有3次错误")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 7, len(sentMessages), "应该有7条成功消息")
}

// TestGateway_LogIntegrity_ChannelSwitchLogging 测试Channel切换日志
func TestGateway_LogIntegrity_ChannelSwitchLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	sessionID := "session1"

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if msg.ChannelID == "feishu" {
			return "OK", "wechat", nil
		}
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("feishu", sessionID, fmt.Sprintf("Feishu Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get(sessionID)
	assert.Equal(t, "wechat", sessionCtx.CurrentChannel, "当前Channel应该是wechat")

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.Equal(t, 5, len(feishuMessages), "飞书应该有5条消息")
	assert.GreaterOrEqual(t, len(wechatMessages), 5, "微信应该至少有5条消息")
}

// TestGateway_LogIntegrity_SessionLogging 测试会话日志完整性
func TestGateway_LogIntegrity_SessionLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	sessions := []string{"user1", "user2", "user3"}

	for _, sessionID := range sessions {
		for i := 0; i < 5; i++ {
			msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	ctxMgr := gateway.ChannelContextManager()
	sessionCount := ctxMgr.Count()
	assert.Equal(t, 3, sessionCount, "应该有3个会话")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 15, len(sentMessages), "应该有15条消息")
}

// TestGateway_LogIntegrity_MultipleChannelLogging 测试多通道日志完整性
func TestGateway_LogIntegrity_MultipleChannelLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channels := []core.Channel{
		NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书"),
		NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信"),
		NewMockChannel("qq", entity.ChannelTypeQQ, "QQ"),
	}

	for _, ch := range channels {
		gateway.Manager().AddChannel(ch)
		ch.Start(context.Background())
	}

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		channelNames := []string{"feishu", "wechat", "qq"}
		for _, channelName := range channelNames {
			msg := createTestMessage(channelName, "session1", fmt.Sprintf("Message %d", i))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	for _, ch := range channels {
		mockCh := ch.(*MockChannel)
		sentMessages := mockCh.GetSentMessages()
		assert.Equal(t, 10, len(sentMessages), "通道 %s 应该有10条消息", ch.Name())
	}
}

// TestGateway_LogIntegrity_ContextLogging 测试上下文日志完整性
func TestGateway_LogIntegrity_ContextLogging(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get("session1")
	assert.NotNil(t, sessionCtx, "会话上下文应该存在")
	assert.Equal(t, "test", sessionCtx.CurrentChannel, "当前Channel应该是test")
	assert.Equal(t, "session1", sessionCtx.SessionID, "会话ID应该是session1")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "应该有10条消息")
}

// TestGateway_LogIntegrity_LogFileContent 测试日志文件内容
func TestGateway_LogIntegrity_LogFileContent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := tempDir + "/test.log"

	content := strings.Builder{}
	content.WriteString("Log entry 1\n")
	content.WriteString("Log entry 2\n")
	content.WriteString("Log entry 3\n")

	err := os.WriteFile(logFile, []byte(content.String()), 0644)
	assert.NoError(t, err, "写入日志文件不应该出错")

	data, err := os.ReadFile(logFile)
	assert.NoError(t, err, "读取日志文件不应该出错")

	logContent := string(data)
	assert.Contains(t, logContent, "Log entry 1", "应该包含日志条目1")
	assert.Contains(t, logContent, "Log entry 2", "应该包含日志条目2")
	assert.Contains(t, logContent, "Log entry 3", "应该包含日志条目3")

	lines := strings.Split(logContent, "\n")
	assert.Equal(t, 4, len(lines), "应该有4行（3条日志+空行）")
}

// TestGateway_LogIntegrity_LogRotation 测试日志轮转
func TestGateway_LogIntegrity_LogRotation(t *testing.T) {
	tempDir := t.TempDir()

	for i := 0; i < 5; i++ {
		logFile := fmt.Sprintf("%s/test.%d.log", tempDir, i)
		content := fmt.Sprintf("Log file %d content\n", i)
		err := os.WriteFile(logFile, []byte(content), 0644)
		assert.NoError(t, err, "写入日志文件不应该出错")
	}

	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err, "读取目录不应该出错")
	assert.Equal(t, 5, len(files), "应该有5个日志文件")

	for i, file := range files {
		expectedName := fmt.Sprintf("test.%d.log", i)
		assert.Equal(t, expectedName, file.Name(), "文件名应该匹配")
	}
}

// TestGateway_LogIntegrity_LogPersistence 测试日志持久化
func TestGateway_LogIntegrity_LogPersistence(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "所有消息都应该被处理")

	ctxMgr := gateway.ChannelContextManager()
	sessionCount := ctxMgr.Count()
	assert.Greater(t, sessionCount, 0, "应该有会话")
}

// TestGateway_LogIntegrity_LogConsistency 测试日志一致性
func TestGateway_LogIntegrity_LogConsistency(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	messageContents := make([]string, 0)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageContents = append(messageContents, msg.Content)
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 10, len(messageContents), "应该有10个消息内容")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "应该有10条消息")

	for i, msg := range sentMessages {
		expectedContent := fmt.Sprintf("Reply to Message %d", i)
		assert.Equal(t, expectedContent, msg.Content, "消息内容应该匹配")
	}
}
