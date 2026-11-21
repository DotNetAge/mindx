package channel

import (
	"context"
	"sync"
	"testing"

)

// BenchmarkBaseChannel_Creation benchmarks BaseChannel creation.
func BenchmarkBaseChannel_Creation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewBaseChannel("test-channel", ChannelTypeWeChat)
	}
}

// BenchmarkBaseChannel_SetStatus benchmarks status updates.
func BenchmarkBaseChannel_SetStatus(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ch.SetStatus(StatusRunning)
		ch.SetStatus(StatusStopped)
	}
}

// BenchmarkBaseChannel_GetStatus benchmarks status retrieval.
func BenchmarkBaseChannel_GetStatus(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	ch.SetStatus(StatusRunning)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ch.GetStatus()
	}
}

// BenchmarkBaseChannel_IsRunning benchmarks IsRunning check.
func BenchmarkBaseChannel_IsRunning(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	ch.SetStatus(StatusRunning)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ch.IsRunning()
	}
}

// BenchmarkBaseChannel_SetHandler benchmarks handler setting.
func BenchmarkBaseChannel_SetHandler(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ch.SetHandler(handler)
	}
}

// BenchmarkBaseChannel_GetHandler benchmarks handler retrieval.
func BenchmarkBaseChannel_GetHandler(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	ch.SetHandler(handler)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ch.GetHandler()
	}
}

// BenchmarkBaseChannel_ParallelStatus benchmarks parallel status access.
func BenchmarkBaseChannel_ParallelStatus(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)
	ch.SetStatus(StatusRunning)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = ch.GetStatus()
		}
	})
}

// BenchmarkBaseChannel_ParallelSetStatus benchmarks parallel status updates.
func BenchmarkBaseChannel_ParallelSetStatus(b *testing.B) {
	ch := NewBaseChannel("test", ChannelTypeWeChat)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				ch.SetStatus(StatusRunning)
			} else {
				ch.SetStatus(StatusStopped)
			}
			i++
		}
	})
}

// BenchmarkChannelCapabilities benchmarks capabilities struct.
func BenchmarkChannelCapabilities(b *testing.B) {
	b.Run("creation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = ChannelCapabilities{
				TextMessages:     true,
				MarkdownMessages: true,
				ImageMessages:    true,
				FileMessages:     true,
				AudioMessages:    true,
				VideoMessages:    true,
				ReactionMessages: true,
				Threads:          true,
			}
		}
	})

	b.Run("access", func(b *testing.B) {
		caps := ChannelCapabilities{
			TextMessages:     true,
			MarkdownMessages: true,
			ImageMessages:    true,
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = caps.TextMessages
			_ = caps.MarkdownMessages
			_ = caps.ImageMessages
		}
	})
}

// BenchmarkMessageHandler benchmarks message handler invocation.
func BenchmarkMessageHandler(b *testing.B) {
	ctx := context.Background()
	msg := NewMessage(
		"msg-1",
		"test",
		DirectionInbound,
		UserInfo{ID: "user-1"},
		"Hello World",
		MessageTypeText,
	)

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = handler(ctx, msg)
	}
}

// BenchmarkChannelTypes benchmarks channel type operations.
func BenchmarkChannelTypes(b *testing.B) {
	types := []ChannelType{
		ChannelTypeWeChat,
		ChannelTypeDingTalk,
		ChannelTypeFeishu,
		ChannelTypeTelegram,
		ChannelTypeMessenger,
		ChannelTypeWhatsApp,
		ChannelTypeIMessage,
		ChannelTypeWeCom,
		ChannelTypeSlack,
		ChannelTypeDiscord,
	}

	b.Run("comparison", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, t := range types {
				if t == ChannelTypeWeChat {
					_ = "wechat"
				}
			}
		}
	})

	b.Run("string-conversion", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, t := range types {
				_ = string(t)
			}
		}
	})
}

// BenchmarkMutexContention benchmarks mutex contention scenarios.
func BenchmarkMutexContention(b *testing.B) {
	b.Run("low-contention", func(b *testing.B) {
		var mu sync.Mutex
		var counter int
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	})

	b.Run("high-contention", func(b *testing.B) {
		var mu sync.Mutex
		var counter int
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})

	b.Run("rwmutex-read", func(b *testing.B) {
		var mu sync.RWMutex
		var counter int
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.RLock()
				_ = counter
				mu.RUnlock()
			}
		})
	})

	b.Run("rwmutex-write", func(b *testing.B) {
		var mu sync.RWMutex
		var counter int
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})
}

// BenchmarkContextOperations benchmarks context operations.
func BenchmarkContextOperations(b *testing.B) {
	b.Run("background", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = context.Background()
		}
	})

	b.Run("with-cancel", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_ = ctx
		}
	})

	b.Run("with-timeout", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			cancel()
			_ = ctx
		}
	})
}
