package skills

import (
	"mindx/internal/entity"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKeywordIndex_IndexSkill 测试索引单个 Skill
func TestKeywordIndex_IndexSkill(t *testing.T) {
	index := NewKeywordIndex()

	def := &entity.SkillDef{
		Name:        "weather_query",
		Description: "查询天气信息",
		Tags:        []string{"天气", "weather", "气温"},
	}

	index.IndexSkill(def)

	// 验证索引成功
	result, ok := index.GetSkill("weather_query")
	assert.True(t, ok)
	assert.Equal(t, "weather_query", result.Name)
}

// TestKeywordIndex_Search_ExactMatch 测试精确匹配
func TestKeywordIndex_Search_ExactMatch(t *testing.T) {
	index := NewKeywordIndex()

	// 索引多个 Skills
	index.IndexSkill(&entity.SkillDef{
		Name:        "weather_query",
		Description: "查询天气",
		Tags:        []string{"天气", "weather"},
	})

	index.IndexSkill(&entity.SkillDef{
		Name:        "calendar_reminder",
		Description: "日历提醒",
		Tags:        []string{"日历", "提醒"},
	})

	// 搜索
	matches := index.Search([]string{"天气"}, 3)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "weather_query", matches[0].Name)
	assert.Greater(t, matches[0].Score, 0.0)
}

// TestKeywordIndex_Search_MultipleKeywords 测试多关键词匹配
func TestKeywordIndex_Search_MultipleKeywords(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{
		Name:        "weather_query",
		Description: "查询天气信息",
		Tags:        []string{"天气", "查询"},
	})

	index.IndexSkill(&entity.SkillDef{
		Name:        "time_query",
		Description: "查询时间",
		Tags:        []string{"时间", "查询"},
	})

	// 搜索多个关键词
	matches := index.Search([]string{"天气", "查询"}, 3)

	// weather_query 应该得分更高（匹配两个关键词）
	assert.Greater(t, len(matches), 0)
	assert.Equal(t, "weather_query", matches[0].Name)
	assert.Greater(t, matches[0].Score, 1.0)
}

// TestKeywordIndex_Search_FuzzyMatch 测试模糊匹配
func TestKeywordIndex_Search_FuzzyMatch(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{
		Name:        "calculator",
		Description: "数学计算器",
		Tags:        []string{"计算", "数学"},
	})

	// 搜索部分匹配的关键词
	matches := index.Search([]string{"计"}, 3)

	// 应该能模糊匹配到 "计算"
	assert.Greater(t, len(matches), 0)
	assert.Equal(t, "calculator", matches[0].Name)
}

// TestKeywordIndex_Search_TopK 测试 TopK 限制
func TestKeywordIndex_Search_TopK(t *testing.T) {
	index := NewKeywordIndex()

	// 索引 5 个 Skills
	for i := 1; i <= 5; i++ {
		index.IndexSkill(&entity.SkillDef{
			Name:        "skill_" + string(rune('0'+i)),
			Description: "测试技能",
			Tags:        []string{"测试"},
		})
	}

	// 限制返回 3 个
	matches := index.Search([]string{"测试"}, 3)

	assert.Equal(t, 3, len(matches))
}

// TestKeywordIndex_Search_NoMatch 测试无匹配结果
func TestKeywordIndex_Search_NoMatch(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{
		Name: "weather_query",
		Tags: []string{"天气"},
	})

	// 搜索不存在的关键词
	matches := index.Search([]string{"不存在的关键词"}, 3)

	assert.Equal(t, 0, len(matches))
}

// TestKeywordIndex_Search_EmptyKeywords 测试空关键词
func TestKeywordIndex_Search_EmptyKeywords(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{
		Name: "test_skill",
		Tags: []string{"test"},
	})

	// 搜索空关键词
	matches := index.Search([]string{}, 3)

	assert.Equal(t, 0, len(matches))
}

// TestKeywordIndex_Search_Scoring 测试评分机制
func TestKeywordIndex_Search_Scoring(t *testing.T) {
	index := NewKeywordIndex()

	// Skill A: 匹配 2 个关键词
	index.IndexSkill(&entity.SkillDef{
		Name: "skill_a",
		Tags: []string{"天气", "查询"},
	})

	// Skill B: 匹配 1 个关键词
	index.IndexSkill(&entity.SkillDef{
		Name: "skill_b",
		Tags: []string{"天气"},
	})

	matches := index.Search([]string{"天气", "查询"}, 3)

	// skill_a 应该排在前面（分数更高）
	assert.Equal(t, 2, len(matches))
	assert.Equal(t, "skill_a", matches[0].Name)
	assert.Greater(t, matches[0].Score, matches[1].Score)
}

// TestKeywordIndex_GetAllSkills 测试获取所有 Skills
func TestKeywordIndex_GetAllSkills(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{Name: "skill_1"})
	index.IndexSkill(&entity.SkillDef{Name: "skill_2"})
	index.IndexSkill(&entity.SkillDef{Name: "skill_3"})

	skills := index.GetAllSkills()

	assert.Equal(t, 3, len(skills))
}

// TestKeywordIndex_Clear 测试清空索引
func TestKeywordIndex_Clear(t *testing.T) {
	index := NewKeywordIndex()

	index.IndexSkill(&entity.SkillDef{Name: "test_skill"})

	// 清空前
	assert.Equal(t, 1, len(index.GetAllSkills()))

	// 清空
	index.Clear()

	// 清空后
	assert.Equal(t, 0, len(index.GetAllSkills()))
}

// TestTokenize 测试分词函数
func TestTokenize(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "英文分词",
			input:    "weather_query",
			expected: []string{"weather", "query"},
		},
		{
			name:     "中文分词",
			input:    "天气查询",
			expected: []string{"天气查询"},
		},
		{
			name:     "混合分词",
			input:    "weather-query/tool",
			expected: []string{"weather", "query", "tool"},
		},
		{
			name:     "去重",
			input:    "test test test",
			expected: []string{"test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tokenize(tc.input)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

// TestKeywordIndex_ConcurrentAccess 测试并发访问
func TestKeywordIndex_ConcurrentAccess(t *testing.T) {
	index := NewKeywordIndex()

	// 并发索引
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			index.IndexSkill(&entity.SkillDef{
				Name: "skill_" + string(rune('0'+id)),
				Tags: []string{"test"},
			})
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证
	skills := index.GetAllSkills()
	assert.Equal(t, 10, len(skills))
}

// BenchmarkKeywordIndex_Search 性能基准测试
func BenchmarkKeywordIndex_Search(b *testing.B) {
	index := NewKeywordIndex()

	// 索引 100 个 Skills
	for i := 0; i < 100; i++ {
		index.IndexSkill(&entity.SkillDef{
			Name:        "skill_" + string(rune('0'+i%10)),
			Description: "测试技能",
			Tags:        []string{"测试", "skill", "test"},
		})
	}

	keywords := []string{"测试", "skill"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Search(keywords, 5)
	}
}
