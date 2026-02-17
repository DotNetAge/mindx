package channels

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGateway_ResourceUsage 测试资源占用
func TestGateway_ResourceUsage(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channels := []core.Channel{
		NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书"),
		NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信"),
		NewMockChannel("qq", entity.ChannelTypeQQ, "QQ"),
		NewMockChannel("realtime", entity.ChannelTypeRealTime, "实时通道"),
	}

	for _, ch := range channels {
		gateway.Manager().AddChannel(ch)
		ch.Start(context.Background())
	}

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		sessionID := fmt.Sprintf("session%d", i%10)
		msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memIncrease := finalMem.Alloc - initialMem.Alloc
	t.Logf("内存增长: %v bytes (%.2f MB)", memIncrease, float64(memIncrease)/1024/1024)

	assert.Less(t, memIncrease, int64(100*1024*1024), "内存增长应该小于 100MB")

	assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
}

// TestGateway_ResourceUsage_MultipleSessions 测试多会话资源占用
func TestGateway_ResourceUsage_MultipleSessions(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	numSessions := 100
	messagesPerSession := 10

	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session%d", i)
		for j := 0; j < messagesPerSession; j++ {
			msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", j))
			gateway.HandleMessage(context.Background(), msg)
		}
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memIncrease := finalMem.Alloc - initialMem.Alloc
	t.Logf("内存增长: %v bytes (%.2f MB)", memIncrease, float64(memIncrease)/1024/1024)

	ctxMgr := gateway.ChannelContextManager()
	sessionCount := ctxMgr.Count()
	t.Logf("会话数量: %d", sessionCount)

	assert.Equal(t, numSessions, sessionCount, "应该有100个会话")
	assert.Less(t, memIncrease, int64(50*1024*1024), "内存增长应该小于 50MB")

	assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
}

// TestGateway_ResourceUsage_ChannelSwitching 测试Channel切换的资源占用
func TestGateway_ResourceUsage_ChannelSwitching(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	feishuChannel := NewMockChannel("feishu", entity.ChannelTypeFeishu, "飞书")
	wechatChannel := NewMockChannel("wechat", entity.ChannelTypeWeChat, "微信")
	gateway.Manager().AddChannel(feishuChannel)
	gateway.Manager().AddChannel(wechatChannel)
	feishuChannel.Start(context.Background())
	wechatChannel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		if msg.ChannelID == "feishu" {
			return "OK", "wechat", nil
		}
		return "OK", "feishu", nil
	})

	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	for i := 0; i < 200; i++ {
		msg := createTestMessage("feishu", "session1", fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memIncrease := finalMem.Alloc - initialMem.Alloc
	t.Logf("内存增长: %v bytes (%.2f MB)", memIncrease, float64(memIncrease)/1024/1024)

	assert.Less(t, memIncrease, int64(30*1024*1024), "内存增长应该小于 30MB")

	assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
}

// TestGateway_ResourceUsage_EmbeddingCache 测试向量化缓存资源占用
func TestGateway_ResourceUsage_EmbeddingCache(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	uniqueMessages := 500
	for i := 0; i < uniqueMessages; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Unique message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	runtime.GC()
	var midMem runtime.MemStats
	runtime.ReadMemStats(&midMem)

	memAfterUnique := midMem.Alloc - initialMem.Alloc
	t.Logf("处理唯一消息后内存增长: %v bytes (%.2f MB)", memAfterUnique, float64(memAfterUnique)/1024/1024)

	for i := 0; i < 500; i++ {
		msg := createTestMessage("test", "session1", fmt.Sprintf("Unique message %d", i%100))
		gateway.HandleMessage(context.Background(), msg)
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memAfterRepeated := finalMem.Alloc - initialMem.Alloc
	t.Logf("处理重复消息后内存增长: %v bytes (%.2f MB)", memAfterRepeated, float64(memAfterRepeated)/1024/1024)

	cacheSize := embeddingSvc.GetCacheSize()
	t.Logf("向量化缓存大小: %d", cacheSize)

	assert.Greater(t, cacheSize, 0, "缓存应该有内容")
	assert.Less(t, memAfterRepeated, int64(50*1024*1024), "内存增长应该小于 50MB")
}

// TestGateway_ResourceUsage_GoroutineLeak 测试goroutine泄漏
func TestGateway_ResourceUsage_GoroutineLeak(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		msg := createTestMessage("test", fmt.Sprintf("session%d", i%10), fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - initialGoroutines

	t.Logf("Goroutine数量: 初始=%d, 最终=%d, 增长=%d", initialGoroutines, finalGoroutines, goroutineIncrease)

	assert.LessOrEqual(t, goroutineIncrease, 10, "Goroutine增长应该小于等于10")
}

// TestGateway_ResourceUsage_MemoryPressure 测试内存压力
func TestGateway_ResourceUsage_MemoryPressure(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	for i := 0; i < 5000; i++ {
		sessionID := fmt.Sprintf("session%d", i%100)
		msg := createTestMessage("test", sessionID, fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)

		if i%1000 == 999 {
			runtime.GC()
			var currentMem runtime.MemStats
			runtime.ReadMemStats(&currentMem)
			memIncrease := currentMem.Alloc - initialMem.Alloc
			t.Logf("处理 %d 条消息后内存增长: %v bytes (%.2f MB)", i+1, memIncrease, float64(memIncrease)/1024/1024)
		}
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memIncrease := finalMem.Alloc - initialMem.Alloc
	t.Logf("最终内存增长: %v bytes (%.2f MB)", memIncrease, float64(memIncrease)/1024/1024)

	assert.Less(t, memIncrease, int64(200*1024*1024), "内存增长应该小于 200MB")

	assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
}
