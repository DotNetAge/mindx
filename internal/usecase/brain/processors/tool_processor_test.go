package processors

import (
	"context"
	"errors"
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolExecutionProcessor_Process_Success 测试成功执行工具
func TestToolExecutionProcessor_Process_Success(t *testing.T) {
	thinking := &MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			return &core.ThinkingResult{}, nil
		},
	}

	// 重写 ThinkWithTools 方法
	thinking.ThinkWithToolsFunc = func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
		return &core.ToolCallResult{
			ToolCallID: "call-123",
			Function: &core.ToolCallFunction{
				Name: "weather_tool",
				Arguments: map[string]interface{}{
					"location": "北京",
				},
			},
		}, nil
	}

	skillManager := &MockSkillManager{
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "北京今天天气晴朗", nil
		},
	}

	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Tools = []entity.ToolSchema{
		{
			Type: "function",
			Function: entity.ToolFunctionSchema{
				Name:        "weather_tool",
				Description: "查询天气",
				Parameters:  map[string]interface{}{},
			},
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.ToolResults)
	assert.Equal(t, 1, len(thinkCtx.ToolResults))
	assert.Equal(t, "weather_tool", thinkCtx.ToolResults[0].FunctionName)
	assert.Equal(t, "北京今天天气晴朗", thinkCtx.ToolResults[0].Result)
}

// TestToolExecutionProcessor_Process_NoTools 测试无工具时跳过
func TestToolExecutionProcessor_Process_NoTools(t *testing.T) {
	thinking := &MockThinking{}
	skillManager := &MockSkillManager{}

	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	// 不设置 Tools

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.ToolResults)
}

// TestToolExecutionProcessor_Process_LLMDecisionFailed 测试 LLM 决策失败
func TestToolExecutionProcessor_Process_LLMDecisionFailed(t *testing.T) {
	thinking := &MockThinking{
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return nil, errors.New("LLM decision failed")
		},
	}

	skillManager := &MockSkillManager{}
	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Tools = []entity.ToolSchema{
		{
			Function: entity.ToolFunctionSchema{
				Name: "test_tool",
			},
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// LLM 决策失败不应该返回错误
	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.ToolResults)
}

// TestToolExecutionProcessor_Process_NoCall 测试 LLM 决定不调用工具
func TestToolExecutionProcessor_Process_NoCall(t *testing.T) {
	thinking := &MockThinking{
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{
				NoCall: true, // 决定不调用
			}, nil
		},
	}

	skillManager := &MockSkillManager{}
	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Tools = []entity.ToolSchema{
		{
			Function: entity.ToolFunctionSchema{
				Name: "test_tool",
			},
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.ToolResults)
}

// TestToolExecutionProcessor_Process_ToolExecutionFailed 测试工具执行失败
func TestToolExecutionProcessor_Process_ToolExecutionFailed(t *testing.T) {
	thinking := &MockThinking{
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{
				ToolCallID: "call-123",
				Function: &core.ToolCallFunction{
					Name:      "failing_tool",
					Arguments: map[string]interface{}{},
				},
			}, nil
		},
	}

	skillManager := &MockSkillManager{
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "", errors.New("tool execution error")
		},
	}

	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Tools = []entity.ToolSchema{
		{
			Function: entity.ToolFunctionSchema{
				Name: "failing_tool",
			},
		},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// 工具执行失败不应该中断流程
	assert.NoError(t, err)
	assert.NotNil(t, thinkCtx.ToolResults)
	assert.Equal(t, 1, len(thinkCtx.ToolResults))
	assert.Equal(t, "failing_tool", thinkCtx.ToolResults[0].FunctionName)
	assert.NotEmpty(t, thinkCtx.ToolResults[0].Error)
}

// TestToolExecutionProcessor_Process_BatchToolCalls 测试批量工具调用
func TestToolExecutionProcessor_Process_BatchToolCalls(t *testing.T) {
	thinking := &MockThinking{
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{
				ToolCalls: []core.ToolCallItem{
					{
						ToolCallID: "call-1",
						Function: &core.ToolCallFunction{
							Name:      "tool1",
							Arguments: map[string]interface{}{},
						},
					},
					{
						ToolCallID: "call-2",
						Function: &core.ToolCallFunction{
							Name:      "tool2",
							Arguments: map[string]interface{}{},
						},
					},
				},
			}, nil
		},
	}

	skillManager := &MockSkillManager{
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "result for " + function.Name, nil
		},
	}

	processor := NewToolExecutionProcessor(thinking, skillManager)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Tools = []entity.ToolSchema{
		{Function: entity.ToolFunctionSchema{Name: "tool1"}},
		{Function: entity.ToolFunctionSchema{Name: "tool2"}},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(thinkCtx.ToolResults))
	assert.Equal(t, "tool1", thinkCtx.ToolResults[0].FunctionName)
	assert.Equal(t, "tool2", thinkCtx.ToolResults[1].FunctionName)
}

// TestToolExecutionProcessor_Name 测试处理器名称
func TestToolExecutionProcessor_Name(t *testing.T) {
	processor := NewToolExecutionProcessor(&MockThinking{}, &MockSkillManager{})
	assert.Equal(t, "ToolExecutionProcessor", processor.Name())
}
