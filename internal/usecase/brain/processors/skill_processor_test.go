package processors

import (
	"context"
	"errors"
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHybridSearcher 模拟混合检索器
type MockHybridSearcher struct {
	SearchFunc func(query string, topK int) ([]*entity.SkillMatch, error)
}

func (m *MockHybridSearcher) Search(query string, topK int) ([]*entity.SkillMatch, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(query, topK)
	}
	return nil, nil
}

// MockToolAssembler 模拟工具组装器
type MockToolAssembler struct {
	AssembleToolsFunc func(skill *entity.Skill) ([]entity.ToolSchema, error)
}

func (m *MockToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
	if m.AssembleToolsFunc != nil {
		return m.AssembleToolsFunc(skill)
	}
	return []entity.ToolSchema{}, nil
}

// TestSkillMatchProcessor_Process_Success 测试成功匹配技能
func TestSkillMatchProcessor_Process_Success(t *testing.T) {
	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:          "weather_query",
						Description:   "天气查询",
						Goal:          "查询天气信息",
						SOP:           "1. 提取地点\n2. 调用API\n3. 生成响应",
						RequiredTools: []string{"web_search"},
					},
					Score: 0.95,
				},
			}, nil
		},
	}

	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			return []entity.ToolSchema{
				{
					Type: "function",
					Function: entity.ToolFunctionSchema{
						Name:        "web_search",
						Description: "网页搜索",
						Parameters:  map[string]interface{}{},
					},
				},
			}, nil
		},
	}

	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "weather_query",
		Keywords: []string{"天气", "北京"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	require.NoError(t, err)
	assert.NotNil(t, thinkCtx.MatchedSkills)
	assert.Len(t, thinkCtx.MatchedSkills, 1)
	assert.Equal(t, "weather_query", thinkCtx.MatchedSkills[0].Name)
	assert.NotEmpty(t, thinkCtx.MatchedSkills[0].SOPContent)
	assert.Len(t, thinkCtx.Tools, 1)
	assert.Equal(t, "web_search", thinkCtx.Tools[0].Function.Name)
}

// TestSkillMatchProcessor_Process_NoIntent 测试无意图时跳过
func TestSkillMatchProcessor_Process_NoIntent(t *testing.T) {
	searcher := &MockHybridSearcher{}
	toolAssembler := &MockToolAssembler{}
	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	// 不设置 Intent

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.MatchedSkills)
}

// TestSkillMatchProcessor_Process_EmptyQuery 测试空查询时跳过
func TestSkillMatchProcessor_Process_EmptyQuery(t *testing.T) {
	searcher := &MockHybridSearcher{}
	toolAssembler := &MockToolAssembler{}
	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "",
		Keywords: []string{},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.MatchedSkills)
}

// TestSkillMatchProcessor_Process_SearchFailed 测试搜索失败不影响流程
func TestSkillMatchProcessor_Process_SearchFailed(t *testing.T) {
	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return nil, errors.New("search error")
		},
	}

	toolAssembler := &MockToolAssembler{}
	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "test",
		Keywords: []string{"测试"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// 搜索失败不应该返回错误
	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.MatchedSkills)
}

// TestSkillMatchProcessor_Process_NoSkillsMatched 测试无匹配技能
func TestSkillMatchProcessor_Process_NoSkillsMatched(t *testing.T) {
	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{}, nil // 空结果
		},
	}

	toolAssembler := &MockToolAssembler{}
	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type:     "test",
		Keywords: []string{"测试"},
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	assert.NoError(t, err)
	assert.Nil(t, thinkCtx.MatchedSkills)
}

// TestSkillMatchProcessor_Process_ToolAssemblyFailed 测试工具组装失败
func TestSkillMatchProcessor_Process_ToolAssemblyFailed(t *testing.T) {
	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:          "test_skill",
						RequiredTools: []string{"missing_tool"},
					},
					Score: 0.9,
				},
			}, nil
		},
	}

	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			return nil, errors.New("required tools not found: [missing_tool]")
		},
	}

	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("测试", "session-123")
	thinkCtx.Intent = &entity.IntentContext{
		Type: "test",
	}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	// 工具组装失败应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required tools not found")
}

// TestSkillMatchProcessor_Process_MultipleTools 测试多个工具组装
func TestSkillMatchProcessor_Process_MultipleTools(t *testing.T) {
	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:          "weather_query",
						RequiredTools: []string{"web_search", "http_request"},
						OptionalTools: []string{"location_service"},
					},
					Score: 0.95,
				},
			}, nil
		},
	}

	toolAssembler := &MockToolAssembler{
		AssembleToolsFunc: func(skill *entity.Skill) ([]entity.ToolSchema, error) {
			return []entity.ToolSchema{
				{Type: "function", Function: entity.ToolFunctionSchema{Name: "web_search"}},
				{Type: "function", Function: entity.ToolFunctionSchema{Name: "http_request"}},
				{Type: "function", Function: entity.ToolFunctionSchema{Name: "location_service"}},
			}, nil
		},
	}

	processor := NewSkillMatchProcessor(searcher, toolAssembler, 3)

	thinkCtx := entity.NewThinkContext("北京天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{Type: "weather_query"}

	ctx := context.Background()
	err := processor.Process(ctx, thinkCtx)

	require.NoError(t, err)
	assert.Len(t, thinkCtx.Tools, 3)
}

// TestSkillMatchProcessor_Name 测试处理器名称
func TestSkillMatchProcessor_Name(t *testing.T) {
	processor := NewSkillMatchProcessor(&MockHybridSearcher{}, &MockToolAssembler{}, 3)
	assert.Equal(t, "SkillMatchProcessor", processor.Name())
}

// TestSkillMatchProcessor_DefaultTopK 测试默认 TopK 值
func TestSkillMatchProcessor_DefaultTopK(t *testing.T) {
	processor := NewSkillMatchProcessor(&MockHybridSearcher{}, &MockToolAssembler{}, 0)
	assert.Equal(t, 3, processor.topK)

	processor2 := NewSkillMatchProcessor(&MockHybridSearcher{}, &MockToolAssembler{}, -1)
	assert.Equal(t, 3, processor2.topK)
}

// TestSkillMatchProcessor_BuildSearchQuery 测试搜索查询构建
func TestSkillMatchProcessor_BuildSearchQuery(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		intentType    string
		expectedQuery string
	}{
		{
			name:          "使用用户输入",
			input:         "北京天气怎么样",
			intentType:    "weather_query",
			expectedQuery: "北京天气怎么样",
		},
		{
			name:          "回退到意图类型",
			input:         "",
			intentType:    "weather_query",
			expectedQuery: "weather_query",
		},
		{
			name:          "都为空",
			input:         "",
			intentType:    "",
			expectedQuery: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedQuery string
			searcher := &MockHybridSearcher{
				SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
					capturedQuery = query
					return []*entity.SkillMatch{}, nil
				},
			}

			processor := NewSkillMatchProcessor(searcher, &MockToolAssembler{}, 3)

			thinkCtx := entity.NewThinkContext(tt.input, "session-123")
			thinkCtx.Intent = &entity.IntentContext{Type: tt.intentType}

			processor.Process(context.Background(), thinkCtx)

			if tt.expectedQuery != "" {
				assert.Equal(t, tt.expectedQuery, capturedQuery)
			}
		})
	}
}

// TestSkillMatchProcessor_SOPContent 测试 SOP 内容加载
func TestSkillMatchProcessor_SOPContent(t *testing.T) {
	sopContent := `1. 提取地点信息
2. 调用天气 API
3. 生成响应`

	searcher := &MockHybridSearcher{
		SearchFunc: func(query string, topK int) ([]*entity.SkillMatch, error) {
			return []*entity.SkillMatch{
				{
					Skill: &entity.Skill{
						Name:          "weather_query",
						SOP:           sopContent,
						RequiredTools: []string{},
					},
					Score: 0.95,
				},
			}, nil
		},
	}

	processor := NewSkillMatchProcessor(searcher, &MockToolAssembler{}, 3)

	thinkCtx := entity.NewThinkContext("天气", "session-123")
	thinkCtx.Intent = &entity.IntentContext{Type: "weather"}

	err := processor.Process(context.Background(), thinkCtx)

	require.NoError(t, err)
	assert.NotNil(t, thinkCtx.MatchedSkills)
	assert.Equal(t, sopContent, thinkCtx.MatchedSkills[0].SOPContent)
}
