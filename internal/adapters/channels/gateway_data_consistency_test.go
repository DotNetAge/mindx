package channels

import (
	"mindx/internal/entity"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGateway_DataConsistency 测试数据一致性
func TestGateway_DataConsistency(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	sessions := []string{"session1", "session2", "session3"}

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		ctxMgr := gateway.ChannelContextManager()
		sessionCtx := ctxMgr.Get(msg.SessionID)
		assert.NotNil(t, sessionCtx, "会话上下文不应该为空")
		assert.Equal(t, msg.SessionID, sessionCtx.SessionID, "会话ID应该匹配")

		return fmt.Sprintf("Reply to %s", msg.Content), "", nil
	})

	for _, sessionID := range sessions {
		for i := 0; i < 5; i++ {
			msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	ctxMgr := gateway.ChannelContextManager()
	for _, sessionID := range sessions {
		sessionCtx := ctxMgr.Get(sessionID)
		assert.NotNil(t, sessionCtx, "会话上下文不应该为空")
		assert.Equal(t, "test", sessionCtx.CurrentChannel, "当前Channel应该是test")
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 15, len(sentMessages), "应该有15条消息（3个会话×5条）")
}

// TestGateway_DataConsistency_SessionIsolation 测试会话隔离
func TestGateway_DataConsistency_SessionIsolation(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	sessionMessages := make(map[string]int)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		sessionMessages[msg.SessionID]++
		return fmt.Sprintf("Reply to %s from %s", msg.Content, msg.SessionID), "", nil
	})

	sessions := []string{"user1", "user2", "user3"}
	for _, sessionID := range sessions {
		for i := 0; i < 3; i++ {
			msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	for _, sessionID := range sessions {
		assert.Equal(t, 3, sessionMessages[sessionID], "会话 %s 应该有3条消息", sessionID)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 9, len(sentMessages), "应该有9条消息")

	for _, msg := range sentMessages {
		assert.Contains(t, sessions, msg.SessionID, "会话ID应该在预期列表中")
	}
}

// TestGateway_DataConsistency_ChannelSwitching 测试Channel切换时的数据一致性
func TestGateway_DataConsistency_ChannelSwitching(t *testing.T) {
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
		ctxMgr := gateway.ChannelContextManager()
		sessionCtx := ctxMgr.Get(sessionID)
		assert.NotNil(t, sessionCtx, "会话上下文不应该为空")

		currentChannel := sessionCtx.CurrentChannel
		assert.NotEmpty(t, currentChannel, "当前Channel不应该为空")

		if msg.ChannelID == "feishu" {
			return "我想切换到微信", "wechat", nil
		}
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("feishu", sessionID, fmt.Sprintf("Feishu Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get(sessionID)
	t.Logf("当前Channel: %s", sessionCtx.CurrentChannel)

	for i := 0; i < 3; i++ {
		msg := createTestMessage("wechat", sessionID, fmt.Sprintf("WeChat Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sessionCtx = ctxMgr.Get(sessionID)
	t.Logf("最终当前Channel: %s", sessionCtx.CurrentChannel)

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.Equal(t, 5, len(feishuMessages), "飞书应该有5条消息")
	assert.GreaterOrEqual(t, len(wechatMessages), 3, "微信应该至少有3条消息")
}

// TestGateway_DataConsistency_MultipleSessions 测试多会话数据一致性
func TestGateway_DataConsistency_MultipleSessions(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	sessionData := make(map[string][]string)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if _, exists := sessionData[msg.SessionID]; !exists {
			sessionData[msg.SessionID] = []string{}
		}
		sessionData[msg.SessionID] = append(sessionData[msg.SessionID], msg.Content)
		return fmt.Sprintf("Reply: %s", msg.Content), "", nil
	})

	numSessions := 10
	messagesPerSession := 5

	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session%d", i)
		for j := 0; j < messagesPerSession; j++ {
			msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d from %s", j, sessionID))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session%d", i)
		assert.Equal(t, messagesPerSession, len(sessionData[sessionID]), "会话 %s 应该有 %d 条消息", sessionID, messagesPerSession)

		for j := 0; j < messagesPerSession; j++ {
			expectedContent := fmt.Sprintf("Message %d from %s", j, sessionID)
			assert.Contains(t, sessionData[sessionID], expectedContent, "应该包含消息内容")
		}
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, numSessions*messagesPerSession, len(sentMessages), "应该有 %d 条消息", numSessions*messagesPerSession)
}

// TestGateway_DataConsistency_MessageOrdering 测试消息顺序一致性
func TestGateway_DataConsistency_MessageOrdering(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	var messageOrder []string

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageOrder = append(messageOrder, msg.Content)
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 20, len(messageOrder), "应该有20条消息")

	for i := 0; i < 20; i++ {
		expectedContent := fmt.Sprintf("Message %d", i)
		assert.Equal(t, expectedContent, messageOrder[i], "消息顺序应该正确")
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 20, len(sentMessages), "应该有20条发送的消息")
}

// TestGateway_DataConsistency_ContextPreservation 测试上下文保持
func TestGateway_DataConsistency_ContextPreservation(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	sessionID := "session1"
	messageCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageCount++
		return fmt.Sprintf("Message count: %d", messageCount), "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get(sessionID)
	assert.NotNil(t, sessionCtx, "会话上下文不应该为空")
	assert.Equal(t, "test", sessionCtx.CurrentChannel, "当前Channel应该是test")

	assert.Equal(t, 10, messageCount, "消息计数应该是10")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "应该有10条消息")
}
