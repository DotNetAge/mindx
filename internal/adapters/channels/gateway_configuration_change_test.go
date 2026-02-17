package channels

import (
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGateway_ConfigurationChange_AddChannel 测试添加Channel
func TestGateway_ConfigurationChange_AddChannel(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	gateway.Manager().AddChannel(feishuChannel)
	feishuChannel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	msg := createTestMessage("feishu", "session1", "Hello")
	gateway.HandleMessage(context.Background(), msg)

	feishuMessages := feishuChannel.GetSentMessages()
	assert.Equal(t, 1, len(feishuMessages), "飞书应该有1条消息")

	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(wechatChannel)
	wechatChannel.Start(context.Background())

	msg = createTestMessage("wechat", "session2", "Hello")
	gateway.HandleMessage(context.Background(), msg)

	wechatMessages := wechatChannel.GetSentMessages()
	assert.Equal(t, 1, len(wechatMessages), "微信应该有1条消息")
}

// TestGateway_ConfigurationChange_RemoveChannel 测试移除Channel
func TestGateway_ConfigurationChange_RemoveChannel(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	msg := createTestMessage("feishu", "session1", "Hello")
	gateway.HandleMessage(context.Background(), msg)

	msg = createTestMessage("wechat", "session2", "Hello")
	gateway.HandleMessage(context.Background(), msg)

	feishuMessages := feishuChannel.GetSentMessages()
	wechatMessages := wechatChannel.GetSentMessages()

	assert.Equal(t, 1, len(feishuMessages), "飞书应该有1条消息")
	assert.Equal(t, 1, len(wechatMessages), "微信应该有1条消息")

	feishuChannel.Stop()

	msg = createTestMessage("feishu", "session1", "After removal")
	gateway.HandleMessage(context.Background(), msg)

	feishuMessages = feishuChannel.GetSentMessages()
	assert.Equal(t, 1, len(feishuMessages), "飞书消息数不应该增加")
}

// TestGateway_ConfigurationChange_SwitchDefaultChannel 测试切换默认Channel
func TestGateway_ConfigurationChange_SwitchDefaultChannel(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	ctxMgr := gateway.ChannelContextManager()
	assert.Equal(t, "realtime", ctxMgr.CurrentChannel("unknown_session"), "默认Channel应该是realtime")

	ctxMgr.SetDefaultChannel("feishu")
	assert.Equal(t, "feishu", ctxMgr.CurrentChannel("unknown_session"), "默认Channel应该是feishu")
}

// TestGateway_ConfigurationChange_UpdateOnMessageHandler 测试更新消息处理回调
func TestGateway_ConfigurationChange_UpdateOnMessageHandler(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	handler1Called := 0
	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		handler1Called++
		return "Handler1", "", nil
	})

	msg := createTestMessage("test", "session1", "Message 1")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 1, len(sentMessages), "应该有1条消息")
	assert.Equal(t, 1, handler1Called, "Handler1应该被调用1次")

	handler2Called := 0
	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		handler2Called++
		return "Handler2", "", nil
	})

	msg = createTestMessage("test", "session1", "Message 2")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages = channel.GetSentMessages()
	assert.Equal(t, 2, len(sentMessages), "应该有2条消息")
	assert.Equal(t, 1, handler1Called, "Handler1应该被调用1次")
	assert.Equal(t, 1, handler2Called, "Handler2应该被调用1次")
}

// TestGateway_ConfigurationChange_MultipleChannelsDynamic 测试动态多Channel配置
func TestGateway_ConfigurationChange_MultipleChannelsDynamic(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		channelName := fmt.Sprintf("channel%d", i)
		channel := NewMockChannel(channelName, entity.ChannelTypeRealTime, fmt.Sprintf("Channel %d", i))
		gateway.Manager().AddChannel(channel)
		channel.Start(context.Background())

		for j := 0; j < 3; j++ {
			msg := createTestMessage(channelName, "session1", fmt.Sprintf("Message %d-%d", i, j))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	for i := 0; i < 5; i++ {
		channelName := fmt.Sprintf("channel%d", i)
		channel, err := gateway.Manager().Get(channelName)
		assert.NoError(t, err, "获取Channel %s 不应该出错", channelName)
		assert.NotNil(t, channel, "Channel %s 应该存在", channelName)

		mockChannel := channel.(*MockChannel)
		sentMessages := mockChannel.GetSentMessages()
		assert.Equal(t, 3, len(sentMessages), "Channel %s 应该有3条消息", channelName)
	}
}

// TestGateway_ConfigurationChange_ChannelRestart 测试Channel重启
func TestGateway_ConfigurationChange_ChannelRestart(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 5, len(sentMessages), "应该有5条消息")

	channel.Stop()
	assert.False(t, channel.IsRunning(), "Channel应该已停止")

	channel.Start(context.Background())
	assert.True(t, channel.IsRunning(), "Channel应该已启动")

	for i := 0; i < 5; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Restart Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	sentMessages = channel.GetSentMessages()
	assert.GreaterOrEqual(t, len(sentMessages), 5, "应该至少有5条消息")
}

// TestGateway_ConfigurationChange_ContextManagerConfig 测试上下文管理器配置
func TestGateway_ConfigurationChange_ContextManagerConfig(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	ctxMgr := gateway.ChannelContextManager()

	ctxMgr.Set("session1", "feishu")
	assert.Equal(t, "feishu", ctxMgr.CurrentChannel("session1"), "当前Channel应该是feishu")

	ctxMgr.Set("session1", "wechat")
	assert.Equal(t, "wechat", ctxMgr.CurrentChannel("session1"), "当前Channel应该是wechat")

	ctxMgr.Set("session2", "qq")
	assert.Equal(t, "qq", ctxMgr.CurrentChannel("session2"), "当前Channel应该是qq")

	assert.Equal(t, 2, ctxMgr.Count(), "应该有2个会话")

	ctxMgr.Delete("session1")
	assert.Equal(t, 1, ctxMgr.Count(), "应该有1个会话")
	assert.Equal(t, "realtime", ctxMgr.CurrentChannel("session1"), "session1应该使用默认Channel")
}

// TestGateway_ConfigurationChange_EmbeddingServiceConfig 测试向量化服务配置
func TestGateway_ConfigurationChange_EmbeddingServiceConfig(t *testing.T) {
	provider1 := &mockEmbeddingProvider{
		vectors: make(map[string][]float64),
	}
	embeddingSvc1 := embedding.NewEmbeddingService(provider1)

	gateway := NewGateway("realtime", embeddingSvc1)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	msg := createTestMessage("test", "session1", "Hello")
	gateway.HandleMessage(context.Background(), msg)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, 1, len(sentMessages), "应该有1条消息")

	provider2 := &mockEmbeddingProvider{
		vectors: make(map[string][]float64),
	}
	embeddingSvc2 := embedding.NewEmbeddingService(provider2)

	newGateway := NewGateway("realtime", embeddingSvc2)
	newChannel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	newGateway.Manager().AddChannel(newChannel)
	newChannel.Start(context.Background())

	newGateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	msg = createTestMessage("test", "session1", "Hello with new service")
	newGateway.HandleMessage(context.Background(), msg)

	newSentMessages := newChannel.GetSentMessages()
	assert.Equal(t, 1, len(newSentMessages), "新网关应该有1条消息")
}

// TestGateway_ConfigurationChange_ChannelStatusUpdate 测试Channel状态更新
func TestGateway_ConfigurationChange_ChannelStatusUpdate(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)

	status := channel.GetStatus()
	assert.False(t, status.Running, "Channel不应该运行")

	channel.Start(context.Background())

	status = channel.GetStatus()
	assert.True(t, status.Running, "Channel应该运行")

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	for i := 0; i < 10; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	status = channel.GetStatus()
	assert.Equal(t, int64(10), status.TotalMessages, "应该有10条消息")
}
