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

// TestPipeline_E2E_CompleteFlow 端到端测试：完整流程
func TestPipeline_E2E_CompleteFlow(t *testing.T) {
	// 创建模拟组件
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				// IntentProcessor 调用
				return &core.ThinkingResult{
					Intent:   "weather_query",
					Keywords: []string{"天气", "北京"},
				}, nil
			}
			// ResponseProcessor 调用
			return &core.ThinkingResult{
				Answer: "北京今天天气晴朗，温度25度",
			}, nil
		},
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{
				ToolCallID: "call-123",
				Function: &core.ToolCallFunction{
					Name: "weather_tool",
					Arguments: map[string]interface{}{
						"location": "北京",
					},
				},
			}, nil
		},
	}

	memory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{
				{
					ID:        1,
					Content:   "北京昨天天气晴朗",
					Keywords:  []string{"北京", "天气"},
					CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	skillManager := &processors.MockSkillManager{
		SearchSkillsFunc: func(keywords ...string) ([]*core.Skill, error) {
			return []*core.Skill{
				{
					GetName: func() string { return "weather_skill" },
				},
			}, nil
		},
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "晴朗，温度25度", nil
		},
	}

	// 创建完整管线
	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(skillManager, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	// 执行
	thinkCtx := entity.NewThinkContext("北京天气怎么样", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 验证
	assert.NoError(t, err)

	// 验证意图识别
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "weather_query", thinkCtx.Intent.Type)
	assert.Equal(t, []string{"天气", "北京"}, thinkCtx.Intent.Keywords)

	// 验证记忆检索
	assert.NotNil(t, thinkCtx.Memories)
	assert.Equal(t, 1, len(thinkCtx.Memories))

	// 验证技能匹配
	assert.NotNil(t, thinkCtx.MatchedSkills)
	assert.Equal(t, "weather_skill", thinkCtx.MatchedSkills[0].Name)

	// MVP: SkillMatchProcessor 暂不填充 Tools，所以 ToolExecutionProcessor 会跳过
	// 验证工具执行被跳过（因为没有 Tools）
	assert.Nil(t, thinkCtx.ToolResults)

	// 验证响应生成
	assert.NotEmpty(t, thinkCtx.Response)
	assert.Equal(t, "北京今天天气晴朗，温度25度", thinkCtx.Response)

	// 验证执行时长
	assert.True(t, thinkCtx.Duration() > 0)
}

// TestPipeline_E2E_NoToolsNeeded 端到端测试：不需要工具的场景
func TestPipeline_E2E_NoToolsNeeded(t *testing.T) {
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				return &core.ThinkingResult{
					Intent:   "general_chat",
					Keywords: []string{"你好"},
				}, nil
			}
			return &core.ThinkingResult{
				Answer: "你好！有什么可以帮你的吗？",
			}, nil
		},
	}

	memory := &processors.MockMemory{}
	skillManager := &processors.MockSkillManager{
		SearchSkillsFunc: func(keywords ...string) ([]*core.Skill, error) {
			return []*core.Skill{}, nil // 无匹配技能
		},
	}

	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(skillManager, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	thinkCtx := entity.NewThinkContext("你好", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Equal(t, "general_chat", thinkCtx.Intent.Type)
	assert.Nil(t, thinkCtx.MatchedSkills) // 无匹配技能
	assert.Nil(t, thinkCtx.ToolResults)   // 无工具执行
	assert.Equal(t, "你好！有什么可以帮你的吗？", thinkCtx.Response)
}

// TestPipeline_E2E_ToolExecutionFailed 端到端测试：工具执行失败
// MVP: 由于 SkillMatchProcessor 不填充 Tools，此测试暂时跳过工具执行
func TestPipeline_E2E_ToolExecutionFailed(t *testing.T) {
	t.Skip("MVP: SkillMatchProcessor 暂不填充 Tools，Phase 2 实现")
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				return &core.ThinkingResult{
					Intent:   "weather_query",
					Keywords: []string{"天气"},
				}, nil
			}
			// ResponseProcessor 应该能处理工具失败的情况
			return &core.ThinkingResult{
				Answer: "抱歉，天气查询服务暂时不可用",
			}, nil
		},
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{
				ToolCallID: "call-123",
				Function: &core.ToolCallFunction{
					Name:      "weather_tool",
					Arguments: map[string]interface{}{},
				},
			}, nil
		},
	}

	memory := &processors.MockMemory{}
	skillManager := &processors.MockSkillManager{
		SearchSkillsFunc: func(keywords ...string) ([]*core.Skill, error) {
			return []*core.Skill{
				{GetName: func() string { return "weather_skill" }},
			}, nil
		},
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "", errors.New("API timeout")
		},
	}

	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(skillManager, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	thinkCtx := entity.NewThinkContext("天气怎么样", "session-123")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 工具失败不应该中断流程
	assert.NoError(t, err)
	assert.NotEmpty(t, thinkCtx.Response)
	assert.Equal(t, 1, len(thinkCtx.ToolResults))
	assert.NotEmpty(t, thinkCtx.ToolResults[0].Error)
}

// TestPipeline_E2E_MultipleScenarios 端到端测试：多种场景
func TestPipeline_E2E_MultipleScenarios(t *testing.T) {
	scenarios := []struct {
		name             string
		input            string
		expectedIntent   string
		expectedResponse string
		hasTools         bool
	}{
		{
			name:             "天气查询",
			input:            "明天北京天气",
			expectedIntent:   "weather_query",
			expectedResponse: "明天北京天气晴朗",
			hasTools:         true,
		},
		{
			name:             "日程创建",
			input:            "提醒我明天开会",
			expectedIntent:   "schedule_create",
			expectedResponse: "已设置明天的开会提醒",
			hasTools:         true,
		},
		{
			name:             "闲聊",
			input:            "你好",
			expectedIntent:   "general_chat",
			expectedResponse: "你好！",
			hasTools:         false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			thinking := &processors.MockThinking{
				ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
					if jsonResult {
						return &core.ThinkingResult{
							Intent:   scenario.expectedIntent,
							Keywords: []string{"test"},
						}, nil
					}
					return &core.ThinkingResult{
						Answer: scenario.expectedResponse,
					}, nil
				},
				ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
					if scenario.hasTools {
						return &core.ToolCallResult{
							ToolCallID: "call-123",
							Function: &core.ToolCallFunction{
								Name:      "test_tool",
								Arguments: map[string]interface{}{},
							},
						}, nil
					}
					return &core.ToolCallResult{NoCall: true}, nil
				},
			}

			memory := &processors.MockMemory{}
			skillManager := &processors.MockSkillManager{
				SearchSkillsFunc: func(keywords ...string) ([]*core.Skill, error) {
					if scenario.hasTools {
						return []*core.Skill{
							{GetName: func() string { return "test_skill" }},
						}, nil
					}
					return []*core.Skill{}, nil
				},
				ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
					return "tool result", nil
				},
			}

			pipeline := NewPipeline(
				processors.NewIntentProcessor(thinking, thinking),
				processors.NewMemoryRetrievalProcessor(memory, 5),
				processors.NewSkillMatchProcessor(skillManager, 3),
				processors.NewToolExecutionProcessor(thinking, skillManager),
				processors.NewResponseProcessor(thinking),
			)

			thinkCtx := entity.NewThinkContext(scenario.input, "session-123")
			ctx := context.Background()

			err := pipeline.Execute(ctx, thinkCtx)

			assert.NoError(t, err)
			assert.Equal(t, scenario.expectedIntent, thinkCtx.Intent.Type)
			assert.Equal(t, scenario.expectedResponse, thinkCtx.Response)
		})
	}
}

// BenchmarkPipeline_E2E 端到端性能基准测试
func BenchmarkPipeline_E2E(b *testing.B) {
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				return &core.ThinkingResult{
					Intent:   "test",
					Keywords: []string{"test"},
				}, nil
			}
			return &core.ThinkingResult{
				Answer: "test response",
			}, nil
		},
		ThinkWithToolsFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
			return &core.ToolCallResult{NoCall: true}, nil
		},
	}

	memory := &processors.MockMemory{}
	skillManager := &processors.MockSkillManager{}

	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(skillManager, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinkCtx := entity.NewThinkContext("test input", "session-123")
		_ = pipeline.Execute(ctx, thinkCtx)
	}
}
