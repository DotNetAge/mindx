package memory

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCalculateTimeWeight 测试时间权重计算功能
// 测试目的：验证 calculateTimeWeight 方法根据记忆点创建时间计算权重值的正确性
// 测试效果：确保权重值随时间推移而递减，当前时间权重为1.0，前3天快速衰减，3天后缓慢衰减
func TestCalculateTimeWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name     string
		t        time.Time
		expected float64
	}{
		{
			name:     "当前时间",
			t:        time.Now(),
			expected: 1.0,
		},
		{
			name:     "1天前",
			t:        time.Now().Add(-24 * time.Hour),
			expected: 1.0 / (1.0 + 0.8*1.0),
		},
		{
			name:     "3天前",
			t:        time.Now().Add(-3 * 24 * time.Hour),
			expected: 0.29411764705882354,
		},
		{
			name:     "7天前",
			t:        time.Now().Add(-7 * 24 * time.Hour),
			expected: 1.0 / (1.0 + 0.3*7.0),
		},
		{
			name:     "30天前",
			t:        time.Now().Add(-30 * 24 * time.Hour),
			expected: 1.0 / (1.0 + 0.3*30.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateTimeWeight(tt.t)
			assert.InDelta(t, tt.expected, result, 0.3)
		})
	}
}

// TestCalculateRepeatWeight 测试重复权重计算功能
// 测试目的：验证 calculateRepeatWeight 方法对文本重复权重的计算
// 测试效果：确保重复权重计算逻辑能正确处理不同文本输入
func TestCalculateRepeatWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name string
		text string
	}{
		{
			name: "普通文本",
			text: "这是一段普通的文本",
		},
		{
			name: "空文本",
			text: "",
		},
		{
			name: "长文本",
			text: "这是一段很长的文本内容，包含了多个句子和段落，用于测试重复权重的计算逻辑",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateRepeatWeight(tt.text)
			assert.True(t, result >= 1.0)
			assert.True(t, result <= 2.0)
		})
	}
}

// TestCalculateEmphasisWeight 测试强调权重计算功能
// 测试目的：验证 calculateEmphasisWeight 方法检测强调性词汇并返回相应权重
// 测试效果：确保包含不同强调词返回不同的权重值，普通文本返回默认权重
func TestCalculateEmphasisWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name     string
		text     string
		expected float64
	}{
		{
			name:     "包含务必",
			text:     "务必按时完成",
			expected: 0.4,
		},
		{
			name:     "包含关键",
			text:     "这是关键步骤",
			expected: 0.35,
		},
		{
			name:     "包含重要",
			text:     "这是一个重要的决定",
			expected: 0.3,
		},
		{
			name:     "包含记住",
			text:     "请记住这个信息",
			expected: 0.25,
		},
		{
			name:     "包含一定要",
			text:     "这个一定要完成",
			expected: 0.25,
		},
		{
			name:     "包含千万别",
			text:     "千万别忘记",
			expected: 0.25,
		},
		{
			name:     "包含英文must",
			text:     "You must do this",
			expected: 0.4,
		},
		{
			name:     "包含英文important",
			text:     "This is important",
			expected: 0.3,
		},
		{
			name:     "包含感叹号",
			text:     "这很重要！",
			expected: 0.3 + 0.05,
		},
		{
			name:     "普通文本",
			text:     "这是一段普通的文本",
			expected: 0.2,
		},
		{
			name:     "空文本",
			text:     "",
			expected: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateEmphasisWeight(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateTotalWeight 测试总权重计算功能
// 测试目的：验证 calculateTotalWeight 方法按场景化动态比例计算总权重的正确性
// 测试效果：确保不同场景下的权重比例计算正确
func TestCalculateTotalWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name           string
		timeWeight     float64
		repeatWeight   float64
		emphasisWeight float64
		scene          string
		expected       float64
	}{
		{
			name:           "默认场景",
			timeWeight:     1.0,
			repeatWeight:   1.0,
			emphasisWeight: 0.35,
			scene:          "default",
			expected:       1.0*0.4 + 0.35*0.35 + 1.0*0.25,
		},
		{
			name:           "聊天场景",
			timeWeight:     1.0,
			repeatWeight:   1.0,
			emphasisWeight: 0.35,
			scene:          "chat",
			expected:       1.0*0.6 + 0.35*0.25 + 1.0*0.15,
		},
		{
			name:           "知识场景",
			timeWeight:     1.0,
			repeatWeight:   1.0,
			emphasisWeight: 0.35,
			scene:          "knowledge",
			expected:       1.0*0.2 + 0.35*0.4 + 1.0*0.4,
		},
		{
			name:           "低时间权重-聊天场景",
			timeWeight:     0.1,
			repeatWeight:   1.0,
			emphasisWeight: 0.35,
			scene:          "chat",
			expected:       0.1*0.6 + 0.35*0.25 + 1.0*0.15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateTotalWeight(tt.timeWeight, tt.repeatWeight, tt.emphasisWeight, tt.scene)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

// TestSimpleTokenize 测试简单分词功能
// 测试目的：验证 simpleTokenize 方法对文本进行分词的正确性
// 测试效果：确保能正确处理中英文、标点符号、空文本、单个字符及超过5个词的截断
func TestSimpleTokenize(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "简单分词",
			text:     "hello world test",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "中文分词",
			text:     "你好 世界 测试",
			expected: []string{"你好", "世界", "测试"},
		},
		{
			name:     "带标点符号",
			text:     "你好，世界！测试。",
			expected: []string{"你好，世界！测试"},
		},
		{
			name:     "空文本",
			text:     "",
			expected: nil,
		},
		{
			name:     "单个字符",
			text:     "a b c",
			expected: nil,
		},
		{
			name:     "超过5个词",
			text:     "one two three four five six seven",
			expected: []string{"one", "two", "three", "four", "five"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.simpleTokenize(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateKeywordSimilarity 测试关键词相似度计算功能
// 测试目的：验证 calculateKeywordSimilarity 方法计算关键词与搜索词相似度的正确性
// 测试效果：确保完全匹配、部分匹配、无匹配、大小写不敏感等场景都能正确计算
func TestCalculateKeywordSimilarity(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name     string
		keywords []string
		terms    string
		expected float64
	}{
		{
			name:     "完全匹配",
			keywords: []string{"hello", "world"},
			terms:    "hello world",
			expected: 1.0,
		},
		{
			name:     "部分匹配",
			keywords: []string{"hello", "world", "test"},
			terms:    "hello",
			expected: 1.0 / 3.0,
		},
		{
			name:     "无匹配",
			keywords: []string{"hello", "world"},
			terms:    "test",
			expected: 0.0,
		},
		{
			name:     "空关键词",
			keywords: []string{},
			terms:    "hello",
			expected: 0.0,
		},
		{
			name:     "大小写不敏感",
			keywords: []string{"Hello", "World"},
			terms:    "hello",
			expected: 0.5,
		},
		{
			name:     "包含关系",
			keywords: []string{"hello"},
			terms:    "hello world",
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateKeywordSimilarity(tt.keywords, tt.terms)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

// TestSortByWeight 测试按权重排序功能
// 测试目的：验证 sortByWeight 方法按权重降序排序并返回指定数量的正确性
// 测试效果：确保能正确降序排序、返回前N个、处理边界情况（空数组、topN大于数组长度）
func TestSortByWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	points := []core.MemoryPoint{
		{TotalWeight: 0.5},
		{TotalWeight: 0.9},
		{TotalWeight: 0.3},
		{TotalWeight: 0.7},
		{TotalWeight: 0.1},
	}

	t.Run("按权重降序排序", func(t *testing.T) {
		result := m.sortByWeight(points, 5)
		assert.Len(t, result, 5)
		assert.Equal(t, 0.9, result[0].TotalWeight)
		assert.Equal(t, 0.7, result[1].TotalWeight)
		assert.Equal(t, 0.5, result[2].TotalWeight)
		assert.Equal(t, 0.3, result[3].TotalWeight)
		assert.Equal(t, 0.1, result[4].TotalWeight)
	})

	t.Run("返回前3个", func(t *testing.T) {
		result := m.sortByWeight(points, 3)
		assert.Len(t, result, 3)
		assert.Equal(t, 0.9, result[0].TotalWeight)
		assert.Equal(t, 0.7, result[1].TotalWeight)
		assert.Equal(t, 0.5, result[2].TotalWeight)
	})

	t.Run("topN大于数组长度", func(t *testing.T) {
		result := m.sortByWeight(points, 10)
		assert.Len(t, result, 5)
	})

	t.Run("空数组", func(t *testing.T) {
		result := m.sortByWeight([]core.MemoryPoint{}, 3)
		assert.Len(t, result, 0)
	})
}

// TestGenerateSummary 测试摘要生成功能
// 测试目的：验证 generateSummary 方法在无LLM客户端时的降级处理
// 测试效果：确保短文本直接返回，长文本截取前200字符并添加省略号
func TestGenerateSummary(t *testing.T) {
	m := &Memory{
		llmClient: nil,
		logger:    newTestLogger(),
	}

	t.Run("无LLM客户端-短文本", func(t *testing.T) {
		text := "这是一段短文本"
		result, err := m.generateSummary(text)
		assert.NoError(t, err)
		assert.Equal(t, text, result)
	})

	t.Run("无LLM客户端-长文本", func(t *testing.T) {
		text := "这是一段很长的文本内容，包含了超过200个字符的内容，用于测试摘要生成功能，当文本长度超过200个字符时，应该截取前200个字符并添加省略号。这段文本的长度应该足够长，以确保能够触发截断逻辑。"
		result, err := m.generateSummary(text)
		assert.NoError(t, err)
		assert.Len(t, result, 203)
		assert.True(t, len(result) > 200)
		assert.Contains(t, result, "...")
	})
}

// TestGenerateKeywords 测试关键词生成功能
// 测试目的：验证 generateKeywords 方法在无LLM客户端时的降级处理
// 测试效果：确保能使用简单分词方法生成关键词
func TestGenerateKeywords(t *testing.T) {
	m := &Memory{
		llmClient: nil,
		logger:    newTestLogger(),
	}

	t.Run("无LLM客户端", func(t *testing.T) {
		text := "hello world test"
		result, err := m.generateKeywords(text)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// TestFilterByKeywords 测试关键词过滤功能
// 测试目的：验证 filterByKeywords 方法根据关键词相似度过滤记忆点的正确性
// 测试效果：确保能正确处理空entries等边界情况
func TestFilterByKeywords(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	t.Run("空entries", func(t *testing.T) {
		result := m.filterByKeywords([]entity.VectorEntry{}, "hello")
		assert.Len(t, result, 0)
	})
}

// TestCalculateCosineSimilarity 测试余弦相似度计算功能
// 测试目的：验证 calculateCosineSimilarity 方法计算向量之间余弦相似度的正确性
// 测试效果：确保相似度计算在0-1之间，相同向量相似度为1.0，完全不同向量相似度为0.0
func TestCalculateCosineSimilarity(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
	}{
		{
			name:     "相同向量",
			vec1:     []float64{1.0, 2.0, 3.0},
			vec2:     []float64{1.0, 2.0, 3.0},
			expected: 1.0,
		},
		{
			name:     "完全不同向量",
			vec1:     []float64{1.0, 0.0, 0.0},
			vec2:     []float64{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "部分相似向量",
			vec1:     []float64{1.0, 2.0, 3.0},
			vec2:     []float64{2.0, 4.0, 6.0},
			expected: 1.0,
		},
		{
			name:     "空向量",
			vec1:     []float64{},
			vec2:     []float64{1.0, 2.0, 3.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateCosineSimilarity(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.01)
			assert.True(t, result >= 0.0 && result <= 1.0)
		})
	}
}

// TestDetermineOptimalK 测试确定最优k值功能
// 测试目的：验证 determineOptimalK 方法根据数据量确定最优k值的正确性
// 测试效果：确保k值随数据量变化而调整，且在合理范围内
func TestDetermineOptimalK(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	tests := []struct {
		name        string
		pointCount  int
		expectedMin int
		expectedMax int
	}{
		{
			name:        "少量数据-4个点",
			pointCount:  4,
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "少量数据-8个点",
			pointCount:  8,
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "中等数据-12个点",
			pointCount:  12,
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name:        "中等数据-30个点",
			pointCount:  30,
			expectedMin: 4,
			expectedMax: 4,
		},
		{
			name:        "大量数据-100个点",
			pointCount:  100,
			expectedMin: 2,
			expectedMax: 10,
		},
		{
			name:        "大量数据-200个点",
			pointCount:  200,
			expectedMin: 2,
			expectedMax: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.determineOptimalK(tt.pointCount)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}
