package channels

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGateway_ErrorRecovery 测试错误恢复能力
func TestGateway_ErrorRecovery(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	errorCount := 0
	successCount := 0
	messageCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageCount++
		if messageCount%10 == 0 {
			errorCount++
			return "", "", fmt.Errorf("模拟错误")
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 100; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 10, errorCount, "应该有10次错误")
	assert.Equal(t, 90, successCount, "应该有90次成功")

	msg := createTestMessage("test", "session1", "Final message")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages := channel.GetSentMessages()
	assert.GreaterOrEqual(t, len(sentMessages), 90, "至少应该有90条成功消息")
}

// TestGateway_ErrorRecovery_Continuous 测试连续错误后的恢复
func TestGateway_ErrorRecovery_Continuous(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	errorMode := false
	errorCount := 0
	successCount := 0
	messageCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageCount++
		if errorMode {
			errorCount++
			return "", "", fmt.Errorf("连续错误")
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 0, errorCount, "初始阶段不应该有错误")
	assert.Equal(t, 20, successCount, "应该有20次成功")

	errorMode = true

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Error Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 10, errorCount, "应该有10次连续错误")

	errorMode = false

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Recovery Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 10, errorCount, "错误数不应该增加")
	assert.Equal(t, 30, successCount, "应该有30次成功（20初始+10恢复）")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 30, len(sentMessages), "应该有30条成功消息")
}

// TestGateway_ErrorRecovery_MultipleChannels 测试多通道错误恢复
func TestGateway_ErrorRecovery_MultipleChannels(t *testing.T) {
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
	channelMessageCounts := make(map[string]int)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		channelMessageCounts[msg.ChannelID]++
		if channelMessageCounts[msg.ChannelID]%15 == 0 {
			channelErrors[msg.ChannelID]++
			return "", "", fmt.Errorf("通道 %s 错误", msg.ChannelID)
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

// TestGateway_ErrorRecovery_WithForwarding 测试错误恢复后的消息转发
func TestGateway_ErrorRecovery_WithForwarding(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	messageCount := 0
	errorCount := 0
	forwardCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		messageCount++
		if messageCount <= 5 {
			errorCount++
			return "", "", fmt.Errorf("模拟错误")
		}
		if messageCount <= 8 {
			forwardCount++
			return "OK", "wechat", nil
		}
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("feishu", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 5, errorCount, "应该有5次错误")
	assert.Equal(t, 3, forwardCount, "应该有3次转发")

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.GreaterOrEqual(t, len(feishuMessages), 2, "飞书应该至少有2条成功消息")
	assert.GreaterOrEqual(t, len(wechatMessages), 3, "微信应该至少有3条转发消息")
}

// TestGateway_ErrorRecovery_PanicRecovery 测试panic后的恢复
func TestGateway_ErrorRecovery_PanicRecovery(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	panicCount := 0
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		defer func() {
			if r := recover(); r != nil {
				panicCount++
			}
		}()

		if successCount%10 == 9 {
			panic("模拟panic")
		}
		successCount++
		return "OK", "", nil
	})

	for i := 0; i < 30; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	assert.Equal(t, 3, panicCount, "应该有3次panic")
	assert.Equal(t, 27, successCount, "应该有27次成功")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 27, len(sentMessages), "应该有27条成功消息")
}
