package processors

import (
	"context"
	"errors"
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMemoryRetrievalProcessor_Process_Success 测试成功检索记忆
func TestMemoryRetrievalProcessor_Process_Success(t *testing.T) {
	memory := &MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{
				{
					ID:       1,
					Content:  "北京今天天气晴朗",
					Keywords: []string{"北京", "天气"},
					CreatedAt: time.Now(),
				},
				{
					ID:       2,
					Content:  "北京明天有雨",
					Keywords: []string{"北京", "天气", "明天"},
					CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	processor := NewMemoryRetrievalProcessor(memory, 5)

	thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"北京", "天气"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Memories)
	assert.Equal(t, 2, len(thinkCtx.Memories))
	assert.Equal(t, "北京今天天气晴朗", thinkCtx.Memories[0].Content)
	assert.Equal(t, []string{"北京", "天气"}, thinkCtx.Memories[0].Keywords)
}

// TestMemoryRetrievalProcessor_Process_NoIntent 测试无意图时跳过
func TestMemoryRetrievalProcessor_Process_NoIntent(t *testing.T) {
	memory := &MockMemory{}
	processor := NewMemoryRetrievalProcessor(memory, 5)

	thinkCtx := entity.NewThinkContext("测试输入", "session-123")
	// 不设置 Intent

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.Memories)
}

// TestMemoryRetrievalProcessor_Process_NoKeywords 测试无关键词时跳过
func TestMemoryRetrievalProcessor_Process_NoKeywords(t *testing.T) {
	memory := &MockMemory{}
	processor := NewMemoryRetrievalProcessor(memory, 5)

	thinkCtx := entity.NewThinkContext("测试输入", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "general_chat",
		Keywords: []string{}, // 空关键词
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.Memories)
}

// TestMemoryRetrievalProcessor_Process_SearchFailed 测试搜索失败不影响流程
func TestMemoryRetrievalProcessor_Process_SearchFailed(t *testing.T) {
	memory := &MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return nil, errors.New("search error")
		},
	}

	processor := NewMemoryRetrievalProcessor(memory, 5)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"北京", "天气"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// 搜索失败不应该返回错误
	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.Memories)
}

// TestMemoryRetrievalProcessor_Process_TopKLimit 测试 TopK 限制
func TestMemoryRetrievalProcessor_Process_TopKLimit(t *testing.T) {
	memory := &MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			// 返回 10 个记忆点
			memories := make([]core.MemoryPoint, 10)
			for i := 0; i < 10; i++ {
				memories[i] = core.MemoryPoint{
					ID:       i + 1,
					Content:  "记忆内容",
					Keywords: []string{"关键词"},
					CreatedAt: time.Now(),
				}
			}
			return memories, nil
		},
	}

	processor := NewMemoryRetrievalProcessor(memory, 3) // 限制返回 3 个

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "test",
		Keywords: []string{"测试"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(thinkCtx.Memories)) // 应该只返回 3 个
}

// TestMemoryRetrievalProcessor_Process_EmptyResult 测试空结果
func TestMemoryRetrievalProcessor_Process_EmptyResult(t *testing.T) {
	memory := &MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{}, nil // 空结果
		},
	}

	processor := NewMemoryRetrievalProcessor(memory, 5)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "test",
		Keywords: []string{"测试"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Memories)
	assert.Equal(t, 0, len(thinkCtx.Memories))
}

// TestMemoryRetrievalProcessor_Name 测试处理器名称
func TestMemoryRetrievalProcessor_Name(t *testing.T) {
	processor := NewMemoryRetrievalProcessor(&MockMemory{}, 5)
	assert.Equal(t, "MemoryRetrievalProcessor", processor.Name())
}

// TestMemoryRetrievalProcessor_BuildSearchTerms 测试搜索词构建
func TestMemoryRetrievalProcessor_BuildSearchTerms(t *testing.T) {
	processor := NewMemoryRetrievalProcessor(&MockMemory{}, 5)

	testCases := []struct {
		name     string
		keywords []string
		expected string
	}{
		{
			name:     "单个关键词",
			keywords: []string{"天气"},
			expected: "天气",
		},
		{
			name:     "多个关键词",
			keywords: []string{"北京", "天气", "明天"},
			expected: "北京 天气 明天",
		},
		{
			name:     "空关键词",
			keywords: []string{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processor.buildSearchTerms(tc.keywords)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMemoryRetrievalProcessor_DefaultTopK 测试默认 TopK 值
func TestMemoryRetrievalProcessor_DefaultTopK(t *testing.T) {
	processor := NewMemoryRetrievalProcessor(&MockMemory{}, 0) // 传入 0
	assert.Equal(t, 5, processor.topK) // 应该使用默认值 5

	processor2 := NewMemoryRetrievalProcessor(&MockMemory{}, -1) // 传入负数
	assert.Equal(t, 5, processor2.topK) // 应该使用默认值 5
}
