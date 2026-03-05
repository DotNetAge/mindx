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

// TestResponseProcessor_Process_Success 测试成功生成响应
func TestResponseProcessor_Process_Success(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Answer: "北京今天天气晴朗，温度25度",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"天气", "北京"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, "北京今天天气晴朗，温度25度", thinkCtx.Response)
}

// TestResponseProcessor_Process_WithMemories 测试包含记忆的响应生成
func TestResponseProcessor_Process_WithMemories(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 验证 references 包含记忆信息
			assert.Contains(t, references, "相关记忆")
			assert.Contains(t, references, "北京昨天天气晴朗")

			return &core.ThinkingResult{
				Answer: "根据记忆，北京昨天天气晴朗，今天预计也不错",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"天气", "北京"},
	}
	thinkCtx.Memories = []*entity.MemoryPoint{
		{
			ID:        "1",
			Content:   "北京昨天天气晴朗",
			Keywords:  []string{"北京", "天气"},
			Timestamp: time.Now(),
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, thinkCtx.Response)
}

// TestResponseProcessor_Process_WithToolResults 测试包含工具结果的响应生成
func TestResponseProcessor_Process_WithToolResults(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 验证 references 包含工具执行结果
			assert.Contains(t, references, "工具执行结果")
			assert.Contains(t, references, "weather_tool")
			assert.Contains(t, references, "晴朗")

			return &core.ThinkingResult{
				Answer: "根据天气工具查询，北京今天天气晴朗",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"天气", "北京"},
	}
	thinkCtx.ToolResults = []entity.ToolExecResult{
		{
			ToolCallID:   "call-123",
			FunctionName: "weather_tool",
			Result:       "晴朗，温度25度",
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, thinkCtx.Response)
}

// TestResponseProcessor_Process_WithFailedTool 测试包含失败工具的响应生成
func TestResponseProcessor_Process_WithFailedTool(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 验证 references 包含工具失败信息
			assert.Contains(t, references, "执行失败")

			return &core.ThinkingResult{
				Answer: "抱歉，天气查询工具暂时不可用",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.ToolResults = []entity.ToolExecResult{
		{
			ToolCallID:   "call-123",
			FunctionName: "weather_tool",
			Error:        "API timeout",
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, thinkCtx.Response)
}

// TestResponseProcessor_Process_ThinkingFailed 测试 LLM 生成失败
func TestResponseProcessor_Process_ThinkingFailed(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return nil, errors.New("LLM error")
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("测试", "session-123")

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// 响应生成失败应该返回错误
	assert.Error(t, err)
	assert.Empty(t, thinkCtx.Response)
}

// TestResponseProcessor_Process_WithSendTo 测试包含 SendTo 的响应
func TestResponseProcessor_Process_WithSendTo(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{
				Answer: "已发送消息给张三",
				SendTo: "wechat:zhangsan",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("发消息给张三", "session-123")

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, "已发送消息给张三", thinkCtx.Response)
	assert.Equal(t, "wechat:zhangsan", thinkCtx.SendTo)
}

// TestResponseProcessor_Process_CompleteContext 测试完整上下文的响应生成
func TestResponseProcessor_Process_CompleteContext(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 验证 references 包含所有信息
			assert.Contains(t, references, "用户意图")
			assert.Contains(t, references, "关键词")
			assert.Contains(t, references, "相关记忆")
			assert.Contains(t, references, "工具执行结果")
			assert.Contains(t, references, "匹配的技能")

			return &core.ThinkingResult{
				Answer: "综合所有信息生成的完整响应",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"天气", "北京"},
	}
	thinkCtx.Memories = []*entity.MemoryPoint{
		{
			ID:      "1",
			Content: "记忆内容",
		},
	}
	thinkCtx.MatchedSkills = []*entity.SkillSOP{
		{
			Name: "weather_skill",
		},
	}
	thinkCtx.ToolResults = []entity.ToolExecResult{
		{
			FunctionName: "weather_tool",
			Result:       "晴朗",
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, "综合所有信息生成的完整响应", thinkCtx.Response)
}

// TestResponseProcessor_Name 测试处理器名称
func TestResponseProcessor_Name(t *testing.T) {
	processor := NewResponseProcessor(&MockThinking{})
	assert.Equal(t, "ResponseProcessor", processor.Name())
}

// TestResponseProcessor_BuildReferences_MaxMemories 测试最多显示3条记忆
func TestResponseProcessor_BuildReferences_MaxMemories(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			// 验证只包含前3条记忆
			return &core.ThinkingResult{
				Answer: "测试响应",
			}, nil
		},
	}

	processor := NewResponseProcessor(thinking)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	// 添加5条记忆
	thinkCtx.Memories = []*entity.MemoryPoint{
		{ID: "1", Content: "记忆1"},
		{ID: "2", Content: "记忆2"},
		{ID: "3", Content: "记忆3"},
		{ID: "4", Content: "记忆4"},
		{ID: "5", Content: "记忆5"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
}
