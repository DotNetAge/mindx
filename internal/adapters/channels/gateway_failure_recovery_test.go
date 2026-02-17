package channels

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGateway_FailureRecovery_ChannelFailure 测试Channel故障恢复
func TestGateway_FailureRecovery_ChannelFailure(t *testing.T) {
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
	assert.Equal(t, 10, len(sentMessages), "应该有10条消息")

	channel.Stop()
	assert.False(t, channel.IsRunning(), "Channel应该已停止")

	msg := createTestMessage("test", "session1", "Message during failure")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages = channel.GetSentMessages()
	assert.Equal(t, 10, len(sentMessages), "消息数不应该增加")

	channel.Start(context.Background())
	assert.True(t, channel.IsRunning(), "Channel应该已恢复")

	msg = createTestMessage("test", "session1", "Message after recovery")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages = channel.GetSentMessages()
	assert.GreaterOrEqual(t, len(sentMessages), 11, "应该有至少11条消息")
}

// TestGateway_FailureRecovery_MultipleChannelFailure 测试多Channel故障恢复
func TestGateway_FailureRecovery_MultipleChannelFailure(t *testing.T) {
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

	channels[1].Stop()
	assert.False(t, channels[1].IsRunning(), "微信Channel应该已停止")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("wechat", "session1", fmt.Sprintf("WeChat Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	channels[1].Start(context.Background())
	assert.True(t, channels[1].IsRunning(), "微信Channel应该已恢复")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("wechat", "session1", fmt.Sprintf("Recovery Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	wechatMessages := channels[1].(*MockChannel).GetSentMessages()
	assert.GreaterOrEqual(t, len(wechatMessages), 5, "微信应该至少有5条恢复后的消息")
}

// TestGateway_FailureRecovery_GatewayRestart 测试网关重启
func TestGateway_FailureRecovery_GatewayRestart(t *testing.T) {
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
	assert.Equal(t, 10, len(sentMessages), "应该有10条消息")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gateway.Shutdown(ctx)
	assert.NoError(t, err, "关闭不应该出错")

	assert.False(t, channel.IsRunning(), "Channel应该已停止")

	newGateway := NewGateway("realtime", embeddingSvc)
	newChannel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	newGateway.Manager().AddChannel(newChannel)
	newChannel.Start(context.Background())

	newGateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("New Message %d", i))
		newGateway.HandleMessage(context.Background(), msg)
	}

	newSentMessages := newChannel.GetSentMessages()
	assert.Equal(t, 5, len(newSentMessages), "新网关应该有5条消息")
}

// TestGateway_FailureRecovery_ContextManagerFailure 测试上下文管理器故障
func TestGateway_FailureRecovery_ContextManagerFailure(t *testing.T) {
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
	sessionCount := ctxMgr.Count()
	assert.Greater(t, sessionCount, 0, "应该有会话")

	ctxMgr.Clear()
	assert.Equal(t, 0, ctxMgr.Count(), "会话应该被清空")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Recovery Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sessionCount = ctxMgr.Count()
	assert.Greater(t, sessionCount, 0, "应该有新的会话")

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 15, len(sentMessages), "应该有15条消息")
}

// TestGateway_FailureRecovery_EmbeddingServiceFailure 测试向量化服务故障
func TestGateway_FailureRecovery_EmbeddingServiceFailure(t *testing.T) {
	provider := &failingEmbeddingProvider{
		failCount: 3,
	}
	embeddingSvc := embedding.NewEmbeddingService(provider)
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
	assert.GreaterOrEqual(t, len(sentMessages), 7, "应该至少有7条成功消息")
}

// failingEmbeddingProvider 模拟失败的向量化服务提供者
type failingEmbeddingProvider struct {
	failCount int
	attempts  int
}

func (f *failingEmbeddingProvider) GenerateEmbedding(text string) ([]float64, error) {
	f.attempts++
	if f.attempts <= f.failCount {
		return nil, fmt.Errorf("embedding service error")
	}
	vec := make([]float64, 5)
	for i := range vec {
		vec[i] = float64(i)
	}
	return vec, nil
}

func (f *failingEmbeddingProvider) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := f.GenerateEmbedding(text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}
	return vectors, nil
}

// TestGateway_FailureRecovery_MessageQueueOverflow 测试消息队列溢出
func TestGateway_FailureRecovery_MessageQueueOverflow(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		time.Sleep(100 * time.Millisecond)
		return "OK", "", nil
	})

	for i := 0; i < 50; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		go gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(6 * time.Second)

	sentMessages := channel.GetSentMessages()
	t.Logf("处理了 %d 条消息", len(sentMessages))

	assert.Greater(t, len(sentMessages), 40, "应该处理至少40条消息")
}

// TestGateway_FailureRecovery_PartialChannelFailure 测试部分Channel故障
func TestGateway_FailureRecovery_PartialChannelFailure(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	qqChannel := NewMockChannel("qq", entity.ChannelTypeQQ, "QQ")

	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	gateway.Manager().AddChannel(qqChannel)

	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())
	qqChannel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("feishu", "session1", fmt.Sprintf("Feishu Message %d", i))
		gateway.HandleMessage(context.Background(), msg)

		msg = createTestMessage("wechat", "session1", fmt.Sprintf("WeChat Message %d", i))
		gateway.HandleMessage(context.Background(), msg)

		msg = createTestMessage("qq", "session1", fmt.Sprintf("QQ Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	wechatChannel.Stop()
	assert.False(t, wechatChannel.IsRunning(), "微信Channel应该已停止")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("feishu", "session1", fmt.Sprintf("Feishu Message %d", i+10))
		gateway.HandleMessage(context.Background(), msg)

		msg = createTestMessage("wechat", "session1", fmt.Sprintf("WeChat Message %d", i+10))
		gateway.HandleMessage(context.Background(), msg)

		msg = createTestMessage("qq", "session1", fmt.Sprintf("QQ Message %d", i+10))
		gateway.HandleMessage(context.Background(), msg)
	}

	wechatChannel.Start(context.Background())
	assert.True(t, wechatChannel.IsRunning(), "微信Channel应该已恢复")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("wechat", "session1", fmt.Sprintf("Recovery Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()
	qqMessages := qqChannel.GetSentMessages()

	assert.Equal(t, 15, len(feishuMessages), "飞书应该有15条消息")
	assert.GreaterOrEqual(t, len(wechatMessages), 5, "微信应该至少有5条消息")
	assert.Equal(t, 15, len(qqMessages), "QQ应该有15条消息")
}
