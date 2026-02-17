package channels

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGateway_GracefulShutdown 测试优雅关闭
func TestGateway_GracefulShutdown(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(50 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 20; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "关闭不应该出错")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 20, len(sentMessages), "所有消息都应该被处理")

	assert.False(t, channel.IsRunning(), "Channel 应该已停止")
}

// TestGateway_GracefulShutdown_WithActiveMessages 测试有活跃消息时的优雅关闭
func TestGateway_GracefulShutdown_WithActiveMessages(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	processedMessages := 0
	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		processedMessages++
		time.Sleep(100 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "关闭不应该出错")

	assert.Equal(t, 10, processedMessages, "所有消息都应该被处理")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "所有消息都应该被发送")

	assert.False(t, channel.IsRunning(), "Channel 应该已停止")
}

// TestGateway_GracefulShutdown_MultipleChannels 测试多通道优雅关闭
func TestGateway_GracefulShutdown_MultipleChannels(t *testing.T) {
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
		time.Sleep(30 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 15; i++ {
		channelNames := []string{"feishu", "wechat", "qq"}
		for _, channelName := range channelNames {
			msg := createTestMessage(channelName, "session1", fmt.Sprintf("Message %d", i))
			go gateway.HandleMessage(context.Background(), msg)
		}
	}

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "关闭不应该出错")

	for _, ch := range channels {
		mockCh := ch.(*MockChannel)
		sentMessages := mockCh.GetSentMessages()
		assert.Equal(t, 15, len(sentMessages), "通道 %s 应该有15条消息", ch.Name())
		assert.False(t, mockCh.IsRunning(), "通道 %s 应该已停止", ch.Name())
	}
}

// TestGateway_GracefulShutdown_Timeout 测试超时情况
func TestGateway_GracefulShutdown_Timeout(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(200 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.Error(t, err, "应该超时")

	assert.True(t, channel.IsRunning(), "Channel 可能还在运行")
}

// TestGateway_GracefulShutdown_Empty 测试空网关关闭
func TestGateway_GracefulShutdown_Empty(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "空网关关闭不应该出错")
}

// TestGateway_GracefulShutdown_WithErrorHandling 测试错误处理时的优雅关闭
func TestGateway_GracefulShutdown_WithErrorHandling(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	errorCount := 0
	successCount := 0

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if successCount%5 == 4 {
			errorCount++
			return "", "", fmt.Errorf("模拟错误")
		}
		successCount++
		time.Sleep(30 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "关闭不应该出错")

	assert.Equal(t, 2, errorCount, "应该有2次错误")
	assert.Equal(t, 8, successCount, "应该有8次成功")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 8, len(sentMessages), "应该有8条成功消息")
}

// TestGateway_GracefulShutdown_ContextCancellation 测试上下文取消
func TestGateway_GracefulShutdown_ContextCancellation(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(100 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.Error(t, err, "应该超时")
}

// TestGateway_GracefulShutdown_ChannelStopError 测试Channel停止错误
func TestGateway_GracefulShutdown_ChannelStopError(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	channel.SetStopError(fmt.Errorf("channel stop error"))
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	msg := createTestMessage("test", "session1", "Message")
	gateway.HandleMessage(context.Background(), msg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "即使Channel停止出错，网关关闭也应该成功")
}
