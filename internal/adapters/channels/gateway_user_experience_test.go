package channels

import (
	"mindx/internal/entity"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGateway_UserExperience_ResponseTime 测试响应时间
func TestGateway_UserExperience_ResponseTime(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	responseTimes := make([]time.Duration, 0)

	for i := 0; i < 50; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))

		start := time.Now()
		gateway.HandleMessage(context.Background(), msg)

		for len(channel.GetSentMessages()) <= i {
			time.Sleep(10 * time.Millisecond)
		}
		responseTime := time.Since(start)
		responseTimes = append(responseTimes, responseTime)
	}

	var total time.Duration
	for _, rt := range responseTimes {
		total += rt
	}
	avgResponseTime := total / time.Duration(len(responseTimes))

	t.Logf("平均响应时间: %v", avgResponseTime)

	fastResponses := 0
	for _, rt := range responseTimes {
		if rt < time.Second {
			fastResponses++
		}
	}
	fastResponseRate := float64(fastResponses) / float64(len(responseTimes))

	assert.Greater(t, fastResponseRate, 0.95, "95%的消息应该在1秒内响应")
}

// TestGateway_UserExperience_ConcurrentUsers 测试并发用户
func TestGateway_UserExperience_ConcurrentUsers(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(50 * time.Millisecond)
		return "OK", "", nil
	})

	numUsers := 10
	messagesPerUser := 10

	start := time.Now()

	var wg sync.WaitGroup
	for u := 0; u < numUsers; u++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("user%d", userID)
			for m := 0; m < messagesPerUser; m++ {
				msg := createTestMessage("test", sessionID, fmt.Sprintf("User %d Message %d", userID, m))
				gateway.HandleMessage(context.Background(), msg)
			}
		}(u)
	}
	wg.Wait()

	duration := time.Since(start)
	totalMessages := numUsers * messagesPerUser

	t.Logf("处理 %d 个用户的 %d 条消息耗时: %v", numUsers, totalMessages, duration)
	t.Logf("平均每条消息: %v", duration/time.Duration(totalMessages))

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, totalMessages, len(sentMessages), "所有消息都应该被处理")
}

// TestGateway_UserExperience_SessionSwitching 测试会话切换体验
func TestGateway_UserExperience_SessionSwitching(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return fmt.Sprintf("Reply to %s from %s", msg.Content, msg.SessionID), "", nil
	})

	sessions := []string{"user1", "user2", "user3"}
	messagesPerSession := 5

	for i := 0; i < messagesPerSession*len(sessions); i++ {
		sessionID := sessions[i%len(sessions)]
		msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, messagesPerSession*len(sessions), len(sentMessages), "所有消息都应该被处理")

	for _, msg := range sentMessages {
		assert.Contains(t, sessions, msg.SessionID, "会话ID应该在预期列表中")
	}
}

// TestGateway_UserExperience_ChannelSwitching 测试Channel切换体验
func TestGateway_UserExperience_ChannelSwitching(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	sessionID := "user1"

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if msg.ChannelID == "feishu" {
			return "OK", "wechat", nil
		}
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("feishu", sessionID, fmt.Sprintf("Feishu Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	for i := 0; i < 10; i++ {
		msg := createTestMessage("wechat", sessionID, fmt.Sprintf("WeChat Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.Equal(t, 10, len(feishuMessages), "飞书应该有10条消息")
	assert.GreaterOrEqual(t, len(wechatMessages), 10, "微信应该至少有10条消息")

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get(sessionID)
	assert.Equal(t, "wechat", sessionCtx.CurrentChannel, "当前Channel应该是wechat")
}

// TestGateway_UserExperience_ErrorFeedback 测试错误反馈
func TestGateway_UserExperience_ErrorFeedback(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	errorCount := 0
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if successCount%10 == 9 {
			errorCount++
			return "", "", fmt.Errorf("处理失败")
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 50; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 45, len(sentMessages), "应该有45条成功消息")
	assert.Equal(t, 5, errorCount, "应该有5次错误")

	t.Logf("成功率: %.2f%%", float64(successCount)/float64(successCount+errorCount)*100)
}

// TestGateway_UserExperience_LongRunningSession 测试长时间会话
func TestGateway_UserExperience_LongRunningSession(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	sessionID := "long_session"
	messageCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageCount++
		return fmt.Sprintf("Message #%d", messageCount), "", nil
	})

	start := time.Now()

	for i := 0; i < 100; i++ {
		msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(start)
	t.Logf("处理100条消息耗时: %v", duration)
	t.Logf("平均每条消息: %v", duration/100)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 100, len(sentMessages), "所有消息都应该被处理")

	ctxMgr := gateway.ChannelContextManager()
	sessionCtx := ctxMgr.Get(sessionID)
	assert.NotNil(t, sessionCtx, "会话上下文应该存在")
}

// TestGateway_UserExperience_MediaMixed 测试混合媒体类型
func TestGateway_UserExperience_MediaMixed(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	contentTypes := []string{"text", "image", "audio", "video", "file"}

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return fmt.Sprintf("Processed %s content", msg.ContentType), "", nil
	})

	for i := 0; i < 25; i++ {
		contentType := contentTypes[i%len(contentTypes)]
		msg := createTestMessage("test", "session1", fmt.Sprintf("Content %d", i))
		msg.ContentType = contentType
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 25, len(sentMessages), "所有消息都应该被处理")
}

// TestGateway_UserExperience_LatencyUnderLoad 测试负载下的延迟
func TestGateway_UserExperience_LatencyUnderLoad(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(20 * time.Millisecond)
		return "OK", "", nil
	})

	latencies := make([]time.Duration, 0)

	for i := 0; i < 100; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))

		start := time.Now()
		gateway.HandleMessage(context.Background(), msg)

		for len(channel.GetSentMessages()) <= i {
			time.Sleep(5 * time.Millisecond)
		}
		latency := time.Since(start)
		latencies = append(latencies, latency)
	}

	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	avgLatency := total / time.Duration(len(latencies))

	t.Logf("平均延迟: %v", avgLatency)

	maxLatency := time.Duration(0)
	for _, latency := range latencies {
		if latency > maxLatency {
			maxLatency = latency
		}
	}
	t.Logf("最大延迟: %v", maxLatency)

	assert.Less(t, avgLatency, 500*time.Millisecond, "平均延迟应该小于500ms")
}
