package brain

import (
	"context"
	"errors"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/brain/processors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestPipeline_Integration_IntentAndMemory 测试意图识别和记忆检索的集成
func TestPipeline_Integration_IntentAndMemory(t *testing.T) {
	// 创建模拟的 Thinking
	localThinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "weather_query",
				Keywords: []string{"天气", "北京"},
			}, nil
		},
	}

	// 创建模拟的 Memory
	memory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{
				{
					ID:        1,
					Content:   "北京今天天气晴朗",
					Keywords:  []string{"北京", "天气"},
					CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	// 创建处理器
	intentProcessor := processors.NewIntentProcessor(localThinking, localThinking)
	memoryProcessor := processors.NewMemoryRetrievalProcessor(memory, 5)

	// 创建管线
	pipeline := NewPipeline(intentProcessor, memoryProcessor)

	// 创建上下文
	thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
	ctx := context.Background()

	// 执行管线
	err := pipeline.Execute(ctx, thinkCtx)

	// 验证
	assert.NoError(t, err)

	// 验证意图识别结果
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "weather_query", thinkCtx.Intent.Type)
	assert.Equal(t, []string{"天气", "北京"}, thinkCtx.Intent.Keywords)

	// 验证记忆检索结果
	assert.NotNil(t, thinkCtx.Memories)
	assert.Equal(t, 1, len(thinkCtx.Memories))
	assert.Equal(t, "北京今天天气晴朗", thinkCtx.Memories[0].Content)
}

// TestPipeline_Integration_IntentFailure 测试意图识别失败时的行为
func TestPipeline_Integration_IntentFailure(t *testing.T) {
	// 创建会失败的 Thinking
	failingThinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, errors.New("thinking failed")
		},
	}

	memory := &processors.MockMemory{}

	intentProcessor := processors.NewIntentProcessor(failingThinking, failingThinking)
	memoryProcessor := processors.NewMemoryRetrievalProcessor(memory, 5)

	pipeline := NewPipeline(intentProcessor, memoryProcessor)

	thinkCtx := entity.NewThinkContext("测试输入", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IntentProcessor failed")

	// 记忆检索不应该执行
	assert.Nil(t, thinkCtx.Memories)
}

// TestPipeline_Integration_MemoryFailure 测试记忆检索失败不影响流程
func TestPipeline_Integration_MemoryFailure(t *testing.T) {
	localThinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "test_intent",
				Keywords: []string{"test"},
			}, nil
		},
	}

	// 创建会失败的 Memory
	failingMemory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return nil, errors.New("memory search failed")
		},
	}

	intentProcessor := processors.NewIntentProcessor(localThinking, localThinking)
	memoryProcessor := processors.NewMemoryRetrievalProcessor(failingMemory, 5)

	pipeline := NewPipeline(intentProcessor, memoryProcessor)

	thinkCtx := entity.NewThinkContext("测试输入", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 记忆检索失败不应该影响整体流程
	assert.NoError(t, err)

	// 意图识别应该成功
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "test_intent", thinkCtx.Intent.Type)

	// 记忆应该为空
	assert.Nil(t, thinkCtx.Memories)
}

// TestPipeline_Integration_CompleteFlow 测试完整流程
func TestPipeline_Integration_CompleteFlow(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedIntent string
		expectedKeywords []string
		memoryCount    int
	}{
		{
			name:           "天气查询",
			input:          "明天北京天气怎么样",
			expectedIntent: "weather_query",
			expectedKeywords: []string{"天气", "北京", "明天"},
			memoryCount:    2,
		},
		{
			name:           "日程创建",
			input:          "提醒我明天下午3点开会",
			expectedIntent: "schedule_create",
			expectedKeywords: []string{"提醒", "明天", "开会"},
			memoryCount:    1,
		},
		{
			name:           "闲聊",
			input:          "你好",
			expectedIntent: "general_chat",
			expectedKeywords: []string{"你好"},
			memoryCount:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建模拟组件
			localThinking := &processors.MockThinking{
				ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
					return &core.ThinkingResult{
						Intent:   tc.expectedIntent,
						Keywords: tc.expectedKeywords,
					}, nil
				},
			}

			memory := &processors.MockMemory{
				SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
					memories := make([]core.MemoryPoint, tc.memoryCount)
					for i := 0; i < tc.memoryCount; i++ {
						memories[i] = core.MemoryPoint{
							ID:        i + 1,
							Content:   "记忆内容",
							Keywords:  tc.expectedKeywords,
							CreatedAt: time.Now(),
						}
					}
					return memories, nil
				},
			}

			// 创建管线
			intentProcessor := processors.NewIntentProcessor(localThinking, localThinking)
			memoryProcessor := processors.NewMemoryRetrievalProcessor(memory, 5)
			pipeline := NewPipeline(intentProcessor, memoryProcessor)

			// 执行
			thinkCtx := entity.NewThinkContext(tc.input, "session-123")
			ctx := context.Background()
			err := pipeline.Execute(ctx, thinkCtx)

			// 验证
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedIntent, thinkCtx.Intent.Type)
			assert.Equal(t, tc.expectedKeywords, thinkCtx.Intent.Keywords)
			assert.Equal(t, tc.memoryCount, len(thinkCtx.Memories))
		})
	}
}

// BenchmarkPipeline_Integration 性能基准测试
func BenchmarkPipeline_Integration(b *testing.B) {
	// 创建模拟组件
	localThinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "weather_query",
				Keywords: []string{"天气", "北京"},
			}, nil
		},
	}

	memory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{
				{
					ID:        1,
					Content:   "记忆内容",
					Keywords:  []string{"天气", "北京"},
					CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	intentProcessor := processors.NewIntentProcessor(localThinking, localThinking)
	memoryProcessor := processors.NewMemoryRetrievalProcessor(memory, 5)
	pipeline := NewPipeline(intentProcessor, memoryProcessor)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
		_ = pipeline.Execute(ctx, thinkCtx)
	}
}

// TestPipeline_Integration_ContextDuration 测试执行时长记录
func TestPipeline_Integration_ContextDuration(t *testing.T) {
	localThinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			time.Sleep(10 * time.Millisecond) // 模拟耗时
			return &core.ThinkingResult{
				Intent:   "test",
				Keywords: []string{"test"},
			}, nil
		},
	}

	memory := &processors.MockMemory{}

	intentProcessor := processors.NewIntentProcessor(localThinking, localThinking)
	memoryProcessor := processors.NewMemoryRetrievalProcessor(memory, 5)
	pipeline := NewPipeline(intentProcessor, memoryProcessor)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.True(t, thinkCtx.Duration() >= 10*time.Millisecond)
}
