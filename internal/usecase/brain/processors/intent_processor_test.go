package processors

import (
	"context"
	"errors"
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIntentProcessor_Process_Success 测试成功识别意图
func TestIntentProcessor_Process_Success(t *testing.T) {
	localThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "weather_query",
				Keywords: []string{"天气", "北京"},
			}, nil
		},
	}

	cloudThinking := &MockThinking{}

	processor := NewIntentProcessor(localThinking, cloudThinking)

	thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
	ctx := context.Background()

	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "weather_query", thinkCtx.Intent.Type)
	assert.Equal(t, []string{"天气", "北京"}, thinkCtx.Intent.Keywords)
	assert.Equal(t, 1.0, thinkCtx.Intent.Confidence)
}

// TestIntentProcessor_Process_LocalFallbackToCloud 测试本地模型失败降级到云端
func TestIntentProcessor_Process_LocalFallbackToCloud(t *testing.T) {
	localThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, errors.New("local model error")
		},
	}

	cloudThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "schedule_create",
				Keywords: []string{"提醒", "明天", "开会"},
			}, nil
		},
	}

	processor := NewIntentProcessor(localThinking, cloudThinking)

	thinkCtx := entity.NewThinkContext("明天提醒我开会", "session-123")
	ctx := context.Background()

	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "schedule_create", thinkCtx.Intent.Type)
	assert.Equal(t, []string{"提醒", "明天", "开会"}, thinkCtx.Intent.Keywords)
}

// TestIntentProcessor_Process_BothFailed 测试本地和云端都失败
func TestIntentProcessor_Process_BothFailed(t *testing.T) {
	localThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, errors.New("local model error")
		},
	}

	cloudThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, errors.New("cloud model error")
		},
	}

	processor := NewIntentProcessor(localThinking, cloudThinking)

	thinkCtx := entity.NewThinkContext("测试输入", "session-123")
	ctx := context.Background()

	err := processor.Process(ctx, thinkCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intent recognition failed")
	assert.Nil(t, thinkCtx.Intent)
}

// TestIntentProcessor_Process_EmptyIntent 测试空意图
func TestIntentProcessor_Process_EmptyIntent(t *testing.T) {
	localThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "", // 空意图
				Keywords: []string{"test"},
			}, nil
		},
	}

	cloudThinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Intent:   "general_chat",
				Keywords: []string{"闲聊"},
			}, nil
		},
	}

	processor := NewIntentProcessor(localThinking, cloudThinking)

	thinkCtx := entity.NewThinkContext("你好", "session-123")
	ctx := context.Background()

	err := processor.Process(ctx, thinkCtx)

	// 本地返回空意图，应该降级到云端
	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "general_chat", thinkCtx.Intent.Type)
}

// TestIntentProcessor_Name 测试处理器名称
func TestIntentProcessor_Name(t *testing.T) {
	processor := NewIntentProcessor(&MockThinking{}, &MockThinking{})
	assert.Equal(t, "IntentProcessor", processor.Name())
}

// TestIntentProcessor_Process_ComplexIntent 测试复杂意图识别
func TestIntentProcessor_Process_ComplexIntent(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedIntent string
		expectedKeywords []string
	}{
		{
			name:           "天气查询",
			input:          "明天北京天气怎么样",
			expectedIntent: "weather_query",
			expectedKeywords: []string{"天气", "北京", "明天"},
		},
		{
			name:           "日程创建",
			input:          "提醒我明天下午3点开会",
			expectedIntent: "schedule_create",
			expectedKeywords: []string{"提醒", "明天", "下午", "3点", "开会"},
		},
		{
			name:           "计算",
			input:          "帮我算一下 123 * 456",
			expectedIntent: "calculation",
			expectedKeywords: []string{"计算", "123", "456"},
		},
		{
			name:           "闲聊",
			input:          "你好",
			expectedIntent: "general_chat",
			expectedKeywords: []string{"你好"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localThinking := &MockThinking{
				ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
					return &core.ThinkingResult{
						Intent:   tc.expectedIntent,
						Keywords: tc.expectedKeywords,
					}, nil
				},
			}

			processor := NewIntentProcessor(localThinking, &MockThinking{})

			thinkCtx := entity.NewThinkContext(tc.input, "session-123")
			ctx := context.Background()

			err := processor.Process(ctx, thinkCtx)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedIntent, thinkCtx.Intent.Type)
			assert.Equal(t, tc.expectedKeywords, thinkCtx.Intent.Keywords)
		})
	}
}
