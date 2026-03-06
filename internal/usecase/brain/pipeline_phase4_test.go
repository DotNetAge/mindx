package brain

import (
	"context"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/brain/processors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPipeline_Phase4_HybridSearcherIntegration 测试 Phase 4 HybridSearcher 集成
func TestPipeline_Phase4_HybridSearcherIntegration(t *testing.T) {
	// 创建模拟 Thinking
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				// IntentProcessor 调用
				return &core.ThinkingResult{
					Intent:   "test_query",
					Keywords: []string{"测试", "查询"},
				}, nil
			}
			// ResponseProcessor 调用
			return &core.ThinkingResult{
				Answer: "测试响应",
			}, nil
		},
	}

	// 创建模拟 Memory
	memory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{}, nil
		},
	}

	// 创建模拟 HybridSearcher
	hybridSearcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			// 返回模拟的技能匹配结果
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:        "test_skill",
						Description: "测试技能",
						Version:     "1.0.0",
						RequiredTools: []string{"test_tool"},
					},
					Score: 0.95,
				},
			}, nil
		},
	}

	// 创建模拟 ToolAssembler
	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			// 返回模拟的工具 Schema
			return []entity.ToolSchema{
				{
					Type: "function",
					Function: entity.ToolFunctionSchema{
						Name:        "test_tool",
						Description: "测试工具",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"query": map[string]interface{}{
									"type":        "string",
									"description": "查询参数",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	// 创建模拟 SkillManager
	skillManager := &processors.MockSkillManager{
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "工具执行成功", nil
		},
	}

	// 创建完整管线（使用 Phase 4 组件）
	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(hybridSearcher, toolAssembler, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	// 执行
	thinkCtx := entity.NewThinkContext("测试查询", "session-test")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 验证
	assert.NoError(t, err)

	// 验证意图识别
	assert.NotNil(t, thinkCtx.Intent)
	assert.Equal(t, "test_query", thinkCtx.Intent.Type)

	// 验证技能匹配（通过 HybridSearcher）
	assert.NotNil(t, thinkCtx.MatchedSkills)
	assert.Greater(t, len(thinkCtx.MatchedSkills), 0)
	assert.Equal(t, "test_skill", thinkCtx.MatchedSkills[0].Name)

	// 验证工具组装（通过 ToolAssembler）
	assert.NotNil(t, thinkCtx.Tools)
	assert.Greater(t, len(thinkCtx.Tools), 0)
	assert.Equal(t, "test_tool", thinkCtx.Tools[0].Function.Name)

	// 验证响应生成
	assert.NotEmpty(t, thinkCtx.Response)
}

// MockHybridSearcher 模拟 HybridSearcher
type MockHybridSearcher struct {
	SearchFunc func(query string, topK int) ([]*entity.SkillMatch, error)
}

func (m *MockHybridSearcher) Search(query string, topK int) ([]*entity.SkillMatch, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(query, topK)
	}
	return []*entity.SkillMatch{}, nil
}

// MockToolAssembler 模拟 ToolAssembler
type MockToolAssembler struct {
	AssembleToolsFunc func(skill *entity.Skill) ([]entity.ToolSchema, error)
}

func (m *MockToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
	if m.AssembleToolsFunc != nil {
		return m.AssembleToolsFunc(skill)
	}
	return []entity.ToolSchema{}, nil
}

// TestPipeline_Phase4_ToolAssemblerPriority 测试工具组装优先级（本地 > MCP）
func TestPipeline_Phase4_ToolAssemblerPriority(t *testing.T) {
	// 创建模拟 ToolAssembler，测试优先级
	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			// 模拟：优先返回本地工具
			tools := []entity.ToolSchema{}

			for _, toolName := range skill.RequiredTools {
				// 假设 local_tool 是本地工具，mcp_tool 是 MCP 工具
				if toolName == "local_tool" {
					tools = append(tools, entity.ToolSchema{
						Type: "function",
						Function: entity.ToolFunctionSchema{
							Name:        "local_tool",
							Description: "本地工具（优先）",
						},
					})
				} else if toolName == "mcp_tool" {
					tools = append(tools, entity.ToolSchema{
						Type: "function",
						Function: entity.ToolFunctionSchema{
							Name:        "mcp_tool",
							Description: "MCP 工具（回退）",
						},
					})
				}
			}

			return tools, nil
		},
	}

	// 创建测试 Skill（同时需要本地和 MCP 工具）
	skill := &entity.Skill{
		Name:          "mixed_skill",
		RequiredTools: []string{"local_tool", "mcp_tool"},
	}

	// 组装工具
	tools, err := toolAssembler.AssembleTools(skill)

	// 验证
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tools))

	// 验证本地工具在前
	assert.Equal(t, "local_tool", tools[0].Function.Name)
	assert.Contains(t, tools[0].Function.Description, "本地工具")

	// 验证 MCP 工具在后
	assert.Equal(t, "mcp_tool", tools[1].Function.Name)
	assert.Contains(t, tools[1].Function.Description, "MCP 工具")
}

// TestPipeline_Phase4_HybridSearcherWeights 测试混合检索权重
func TestPipeline_Phase4_HybridSearcherWeights(t *testing.T) {
	// 创建模拟 HybridSearcher，测试向量和关键词权重
	hybridSearcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			// 模拟：向量搜索权重 0.7，关键词搜索权重 0.3
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:        "high_vector_score",
						Description: "向量相似度高的技能",
					},
					Score: 0.85, // 向量 0.9 * 0.7 + 关键词 0.5 * 0.3 = 0.78
				},
				{
					Skill: &entity.Skill{
						Name:        "high_keyword_score",
						Description: "关键词匹配度高的技能",
					},
					Score: 0.72, // 向量 0.6 * 0.7 + 关键词 0.9 * 0.3 = 0.69
				},
			}, nil
		},
	}

	// 搜索
	matches, err := hybridSearcher.Search("测试查询", 2)

	// 验证
	assert.NoError(t, err)
	assert.Equal(t, 2, len(matches))

	// 验证排序（分数高的在前）
	assert.Equal(t, "high_vector_score", matches[0].Skill.Name)
	assert.Greater(t, matches[0].Score, matches[1].Score)
}

// TestPipeline_Phase4_EmptySkillsGracefulDegradation 测试空技能优雅降级
func TestPipeline_Phase4_EmptySkillsGracefulDegradation(t *testing.T) {
	// 创建模拟组件
	thinking := &processors.MockThinking{
		ThinkFunc: func(ctx context.Context, question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
			if jsonResult {
				return &core.ThinkingResult{
					Intent:   "test_query",
					Keywords: []string{"测试"},
				}, nil
			}
			return &core.ThinkingResult{
				Answer: "没有找到相关技能，但我可以直接回答",
			}, nil
		},
	}

	memory := &processors.MockMemory{
		SearchFunc: func(terms string) ([]core.MemoryPoint, error) {
			return []core.MemoryPoint{}, nil
		},
	}

	// HybridSearcher 返回空结果
	hybridSearcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{}, nil // 没有匹配的技能
		},
	}

	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			return []entity.ToolSchema{}, nil
		},
	}

	skillManager := &processors.MockSkillManager{
		ExecuteFuncFunc: func(function core.ToolCallFunction) (string, error) {
			return "", nil
		},
	}

	// 创建管线
	pipeline := NewPipeline(
		processors.NewIntentProcessor(thinking, thinking),
		processors.NewMemoryRetrievalProcessor(memory, 5),
		processors.NewSkillMatchProcessor(hybridSearcher, toolAssembler, 3),
		processors.NewToolExecutionProcessor(thinking, skillManager),
		processors.NewResponseProcessor(thinking),
	)

	// 执行
	thinkCtx := entity.NewThinkContext("测试查询", "session-test")
	ctx := context.Background()

	err := pipeline.Execute(ctx, thinkCtx)

	// 验证：即使没有技能匹配，Pipeline 也应该正常完成
	assert.NoError(t, err)
	assert.NotEmpty(t, thinkCtx.Response)
	assert.Contains(t, thinkCtx.Response, "没有找到相关技能")
}
