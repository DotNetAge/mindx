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

// TestGateway_Stability 测试网关长时间运行的稳定性
func TestGateway_Stability(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间运行的稳定性测试")
	}

	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	duration := 5 * time.Minute
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	messageCount := 0
	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			msg := createTestMessage("test", "session1", "Hi")
			gateway.HandleMessage(ctx, msg)
			messageCount++

			if messageCount%12 == 0 {
				t.Logf("已运行 %v，处理 %d 条消息", time.Since(startTime), messageCount)

				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.Logf("内存使用: Alloc=%v MB, TotalAlloc=%v MB, Sys=%v MB",
					m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)

				t.Logf("活跃消息数: %d", gateway.GetActiveMessageCount())
			}

		case <-ctx.Done():
			t.Logf("稳定性测试完成，共处理 %d 条消息", messageCount)
			return
		}
	}
}

// TestGateway_Stability_Short 短时间稳定性测试（用于快速验证）
func TestGateway_Stability_Short(t *testing.T) {
	embeddingSvc := mockEmbeddingService()
	gateway := NewGateway("realtime", embeddingSvc)

	channel := NewMockChannel("test", entity.ChannelTypeRealTime, "Test")
	gateway.Manager().AddChannel(channel)
	channel.Start(context.Background())

	gateway.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		return "OK", "", nil
	})

	duration := 30 * time.Second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	messageCount := 0
	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			msg := createTestMessage("test", "session1", fmt.Sprintf("Message %d", messageCount))
			gateway.HandleMessage(ctx, msg)
			messageCount++

			if messageCount%5 == 0 {
				t.Logf("已运行 %v，处理 %d 条消息", time.Since(startTime), messageCount)

				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.Logf("内存使用: Alloc=%v MB, TotalAlloc=%v MB, Sys=%v MB",
					m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)
			}

		case <-ctx.Done():
			t.Logf("短时间稳定性测试完成，共处理 %d 条消息", messageCount)

			sentMessages := channel.GetSentMessages()
			assert.Equal(t, messageCount, len(sentMessages), "所有消息都应该被处理")

			assert.Equal(t, 0, gateway.GetActiveMessageCount(), "所有消息应该处理完成")
			return
		}
	}
}

// TestGateway_Stability_MultipleChannels 测试多通道长时间运行的稳定性
func TestGateway_Stability_MultipleChannels(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间运行的稳定性测试")
	}

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

	duration := 3 * time.Minute
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	messageCount := 0
	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			channelNames := []string{"feishu", "wechat", "qq", "realtime"}
			for _, channelName := range channelNames {
				msg := createTestMessage(channelName, fmt.Sprintf("session%d", messageCount%10), "Hi")
				gateway.HandleMessage(ctx, msg)
				messageCount++
			}

			if messageCount%20 == 0 {
				t.Logf("已运行 %v，处理 %d 条消息", time.Since(startTime), messageCount)

				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.Logf("内存使用: Alloc=%v MB, TotalAlloc=%v MB, Sys=%v MB",
					m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)

				for _, ch := range channels {
					if mockCh, ok := ch.(*MockChannel); ok {
						t.Logf("Channel %s: 已发送 %d 条消息", ch.Name(), len(mockCh.GetSentMessages()))
					}
				}
			}

		case <-ctx.Done():
			t.Logf("多通道稳定性测试完成，共处理 %d 条消息", messageCount)
			return
		}
	}
}

// TestGateway_Stability_WithMemoryLeakCheck 测试内存泄漏
func TestGateway_Stability_WithMemoryLeakCheck(t *testing.T) {
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

	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		msg := createTestMessage("test", fmt.Sprintf("session%d", i%10), fmt.Sprintf("Message %d", i))
		gateway.HandleMessage(context.Background(), msg)
	}

	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memIncrease := finalMem.Alloc - initialMem.Alloc
	t.Logf("内存增长: %v bytes (%.2f MB)", memIncrease, float64(memIncrease)/1024/1024)

	sentMessages := channel.GetSentMessages()
	assert.Equal(t, messageCount, len(sentMessages), "所有消息都应该被处理")

	assert.Less(t, memIncrease, int64(50*1024*1024), "内存增长应该小于 50MB")
}
