package channels

import (
	"context"
	"fmt"
	"math/rand"
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGateway_NetworkResilience 测试网络异常恢复
func TestGateway_NetworkResilience(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		delay := time.Duration(rand.Intn(500)) * time.Millisecond
		time.Sleep(delay)
		return "OK", "", nil
	})

	start := time.Now()
	for i := 0; i < 100; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}
	duration := time.Since(start)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 100, len(sentMessages), "所有消息都应该被处理")

	t.Logf("处理100条消息耗时: %v（包含随机延迟）", duration)
}

// TestGateway_NetworkResilience_Timeout 测试超时处理
func TestGateway_NetworkResilience_Timeout(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	timeoutCount := 0
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if successCount%10 == 9 {
			timeoutCount++
			time.Sleep(2 * time.Second)
			return "OK", "", nil
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(3 * time.Second)

	sentMessages := channel.GetSentMessages()
	assert.Greater(t, len(sentMessages), 15, "至少应该有15条消息被处理")
}

// TestGateway_NetworkResilience_ConnectionLoss 测试连接丢失
func TestGateway_NetworkResilience_ConnectionLoss(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	phase := 0
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if phase == 1 {
			return "", "", fmt.Errorf("连接丢失")
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 20, successCount, "应该有20条成功消息")

	phase = 1

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Error Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 20, successCount, "成功消息数不应该增加")

	phase = 0

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Recovery Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 30, successCount, "应该有30条成功消息")
}

// TestGateway_NetworkResilience_MultipleChannels 测试多通道网络异常
func TestGateway_NetworkResilience_MultipleChannels(t *testing.T) {
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

	channelErrors := make(map[string]int)
	channelSuccesses := make(map[string]int)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		delay := time.Duration(rand.Intn(300)) * time.Millisecond
		time.Sleep(delay)

		if rand.Intn(10) == 0 {
			channelErrors[msg.ChannelID]++
			return "", "", fmt.Errorf("网络错误")
		}
		channelSuccesses[msg.ChannelID]++
		return "OK", "", nil
	})

	for i := 0; i < 60; i++ {
		channelNames := []string{"feishu", "wechat", "qq"}
		for _, channelName := range channelNames {
			msg := createTestMessage(channelName, "session1", fmt.Sprintf("Message %d", i))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	for _, channelName := range []string{"feishu", "wechat", "qq"} {
		total := channelErrors[channelName] + channelSuccesses[channelName]
		t.Logf("通道 %s: 成功=%d, 错误=%d, 总计=%d", channelName, channelSuccesses[channelName], channelErrors[channelName], total)
		assert.Greater(t, total, 50, "通道 %s 应该处理至少50条消息", channelName)
	}
}

// TestGateway_NetworkResilience_Retry 测试重试机制
func TestGateway_NetworkResilience_Retry(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	attempts := make(map[string]int)
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		attempts[msg.MessageID]++

		if attempts[msg.MessageID] < 3 {
			return "", "", fmt.Errorf("临时错误，需要重试")
		}

		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))

		for j := 0; j < 3; j++ {
			err := func() error {
				_, _, err := gateway.onMessage(context.Background(), msg, nil)
				return err
			}()

			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	assert.Equal(t, 10, successCount, "应该有10条成功消息")

	for msgID, attemptCount := range attempts {
		assert.Equal(t, 3, attemptCount, "消息 %s 应该尝试3次", msgID)
	}
}

// TestGateway_NetworkResilience_Backpressure 测试背压处理
func TestGateway_NetworkResilience_Backpressure(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(100 * time.Millisecond)
		return "OK", "", nil
	})

	start := time.Now()

	for i := 0; i < 50; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	duration := time.Since(start)
	t.Logf("处理50条消息耗时: %v", duration)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 50, len(sentMessages), "所有消息都应该被处理")

	assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
}

// TestGateway_NetworkResilience_PartialFailure 测试部分失败
func TestGateway_NetworkResilience_PartialFailure(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	wechatErrors := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if msg.ChannelID == "wechat" {
			if wechatErrors < 5 {
				wechatErrors++
				return "", "", fmt.Errorf("微信通道错误")
			}
		}
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("feishu", "session1", fmt.Sprintf("Feishu Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	for i := 0; i < 20; i++ {
		msg := createTestMessage("wechat", "session1", fmt.Sprintf("WeChat Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.Equal(t, 20, len(feishuMessages), "飞书应该有20条消息")
	assert.Equal(t, 15, len(wechatMessages), "微信应该有15条消息（5条失败）")
}

// TestGateway_NetworkResilience_QueueOverflow 测试队列溢出
func TestGateway_NetworkResilience_QueueOverflow(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(50 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(3 * time.Second)

	sentMessages := channel.GetSentMessages()
	t.Logf("处理了 %d 条消息", len(sentMessages))

	assert.Greater(t, len(sentMessages), 10, "应该处理至少10条消息")
}
