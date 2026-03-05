package brain

import (
	"context"
	"errors"
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockProcessor 模拟处理器
type MockProcessor struct {
	name      string
	processFunc func(ctx context.Context, thinkCtx *entity.ThinkContext) error
}

func (m *MockProcessor) Name() string {
	return m.name
}

func (m *MockProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	if m.processFunc != nil {
		return m.processFunc(ctx, thinkCtx)
	}
	return nil
}

// TestPipeline_Execute_Success 测试管线成功执行
func TestPipeline_Execute_Success(t *testing.T) {
	// 创建模拟处理器
	p1 := &MockProcessor{
		name: "processor1",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			thinkCtx.Metadata["p1"] = "executed"
			return nil
		},
	}

	p2 := &MockProcessor{
		name: "processor2",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			thinkCtx.Metadata["p2"] = "executed"
			return nil
		},
	}

	// 创建管线
	pipeline := NewPipeline(p1, p2)

	// 创建上下文
	thinkCtx := entity.NewThinkContext("test input", "session-123")
	ctx := context.Background()

	// 执行管线
	err := pipeline.Execute(ctx, thinkCtx)

	// 验证
	assert.NoError(t, err)
	assert.Equal(t, "executed", thinkCtx.Metadata["p1"])
	assert.Equal(t, "executed", thinkCtx.Metadata["p2"])
	assert.False(t, thinkCtx.HasErrors())
}

// TestPipeline_Execute_ProcessorError 测试处理器失败
func TestPipeline_Execute_ProcessorError(t *testing.T) {
	expectedErr := errors.New("processor error")

	// 创建会失败的处理器
	p1 := &MockProcessor{
		name: "processor1",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			return nil
		},
	}

	p2 := &MockProcessor{
		name: "processor2",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			return expectedErr
		},
	}

	p3 := &MockProcessor{
		name: "processor3",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			thinkCtx.Metadata["p3"] = "should not execute"
			return nil
		},
	}

	// 创建管线
	pipeline := NewPipeline(p1, p2, p3)

	// 创建上下文
	thinkCtx := entity.NewThinkContext("test input", "session-123")
	ctx := context.Background()

	// 执行管线
	err := pipeline.Execute(ctx, thinkCtx)

	// 验证
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "processor2 failed")
	assert.True(t, thinkCtx.HasErrors())
	assert.Equal(t, 1, len(thinkCtx.Errors))
	assert.Equal(t, "processor2", thinkCtx.Errors[0].ProcessorName)
	assert.Nil(t, thinkCtx.Metadata["p3"]) // p3 不应该执行
}

// TestPipeline_Execute_EmptyPipeline 测试空管线
func TestPipeline_Execute_EmptyPipeline(t *testing.T) {
	pipeline := NewPipeline()

	thinkCtx := entity.NewThinkContext("test input", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.False(t, thinkCtx.HasErrors())
}

// TestPipeline_Execute_ContextModification 测试上下文修改
func TestPipeline_Execute_ContextModification(t *testing.T) {
	// 模拟 IntentProcessor
	intentProcessor := &MockProcessor{
		name: "intent",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			thinkCtx.Intent = &entity.IntentContext{
				Type:       "weather_query",
				Keywords:   []string{"天气", "北京"},
				Confidence: 0.9,
			}
			return nil
		},
	}

	// 模拟 ResponseProcessor
	responseProcessor := &MockProcessor{
		name: "response",
		processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
			thinkCtx.Response = "今天北京天气晴朗"
			return nil
		},
	}

	pipeline := NewPipeline(intentProcessor, responseProcessor)

	thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "weather_query", thinkCtx.Intent.Type)
	assert.Equal(t, []string{"天气", "北京"}, thinkCtx.Intent.Keywords)
	assert.Equal(t, "今天北京天气晴朗", thinkCtx.Response)
}

// TestPipeline_GetProcessors 测试获取处理器列表
func TestPipeline_GetProcessors(t *testing.T) {
	p1 := &MockProcessor{name: "p1"}
	p2 := &MockProcessor{name: "p2"}

	pipeline := NewPipeline(p1, p2)

	processors := pipeline.GetProcessors()

	assert.Equal(t, 2, len(processors))
	assert.Equal(t, "p1", processors[0].Name())
	assert.Equal(t, "p2", processors[1].Name())
}

// BenchmarkPipeline_Execute 性能基准测试
func BenchmarkPipeline_Execute(b *testing.B) {
	// 创建 5 个简单处理器
	processors := make([]interface{}, 5)
	for i := 0; i < 5; i++ {
		processors[i] = &MockProcessor{
			name: "processor",
			processFunc: func(ctx context.Context, thinkCtx *entity.ThinkContext) error {
				// 模拟一些简单操作
				thinkCtx.Metadata["key"] = "value"
				return nil
			},
		}
	}

	// 类型转换
	coreProcessors := make([]interface{}, len(processors))
	for i, p := range processors {
		coreProcessors[i] = p
	}

	pipeline := NewPipeline(processors[0].(*MockProcessor), processors[1].(*MockProcessor),
		processors[2].(*MockProcessor), processors[3].(*MockProcessor), processors[4].(*MockProcessor))

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinkCtx := entity.NewThinkContext("test input", "session-123")
		_ = pipeline.Execute(ctx, thinkCtx)
	}
}
