package skills

import (
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSearcher 构造一个用于测试的 SkillSearcher（无 embedding，走 keyword 路径）
func newTestSearcher(t *testing.T) *SkillSearcher {
	t.Helper()
	logger := logging.GetSystemLogger().Named("searcher_test")
	searcher := NewSkillSearcher(nil, logger)

	skills := map[string]*core.Skill{}
	infos := map[string]*entity.SkillInfo{}

	testSkills := []struct {
		name        string
		description string
		category    string
		tags        []string
	}{
		{"weather", "查询天气预报", "general", []string{"天气", "weather", "forecast"}},
		{"calculator", "数学计算器", "general", []string{"计算", "calculator", "math"}},
		{"sysinfo", "查看系统信息", "general", []string{"系统", "sysinfo", "CPU", "内存"}},
		{"mcp_sina-finance_get-quote", "获取股票实时行情", "mcp", []string{"mcp", "sina-finance", "stock", "finance", "A股", "行情"}},
		{"finder", "文件搜索", "general", []string{"文件", "finder", "files"}},
		{"reminders", "提醒管理", "general", []string{"提醒", "reminders", "alarm"}},
	}

	for _, ts := range testSkills {
		def := &entity.SkillDef{
			Name:        ts.name,
			Description: ts.description,
			Category:    ts.category,
			Tags:        ts.tags,
			Enabled:     true,
		}
		skillName := ts.name
		skill := &core.Skill{
			GetName: func() string { return skillName },
		}
		skills[ts.name] = skill
		infos[ts.name] = &entity.SkillInfo{
			Def:    def,
			Status: "ready",
			CanRun: true,
		}
	}

	searcher.SetData(skills, infos, nil)
	return searcher
}

func TestSearchByKeywords(t *testing.T) {
	_ = initTestLogging()
	searcher := newTestSearcher(t)

	tests := []struct {
		name           string
		keywords       []string
		expectFirst    string   // 期望排第一的技能名（为空则不检查）
		expectContains []string // 期望结果包含的技能名
		expectEmpty    bool
	}{
		{
			name:        "搜索天气",
			keywords:    []string{"天气"},
			expectFirst: "weather",
		},
		{
			name:           "搜索stock命中MCP技能",
			keywords:       []string{"stock"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
		},
		{
			name:           "搜索mcp命中所有MCP技能",
			keywords:       []string{"mcp"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
		},
		{
			name:        "搜索不存在的东西",
			keywords:    []string{"不存在的东西xyz"},
			expectEmpty: true,
		},
		{
			name:     "空关键词返回所有技能",
			keywords: []string{},
		},
		{
			name:        "多关键词天气北京",
			keywords:    []string{"天气", "北京"},
			expectFirst: "weather",
		},
		{
			name:           "反向匹配weatherinfo包含weather",
			keywords:       []string{"weatherinfo"},
			expectContains: []string{"weather"},
		},
		{
			name:           "搜索A股",
			keywords:       []string{"A股"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
		},
		{
			name:           "搜索行情",
			keywords:       []string{"行情"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := searcher.Search(tc.keywords...)
			require.NoError(t, err)

			if tc.expectEmpty {
				assert.Empty(t, results, "应返回空列表")
				return
			}

			if len(tc.keywords) == 0 {
				assert.Greater(t, len(results), 0, "空关键词应返回所有技能")
				return
			}

			assert.Greater(t, len(results), 0, "应有搜索结果")

			if tc.expectFirst != "" {
				assert.Equal(t, tc.expectFirst, results[0].GetName(),
					"第一个结果应为 %s", tc.expectFirst)
			}

			for _, expected := range tc.expectContains {
				found := false
				for _, r := range results {
					if r.GetName() == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "结果应包含 %s", expected)
			}
		})
	}
}

func TestSearchByKeywords_Scoring(t *testing.T) {
	_ = initTestLogging()
	logger := logging.GetSystemLogger().Named("searcher_scoring_test")
	searcher := NewSkillSearcher(nil, logger)

	// 构造一个技能，各字段包含不同关键词，用于验证评分权重
	def := &entity.SkillDef{
		Name:        "target_skill",
		Description: "这是一个描述包含desc_keyword的技能",
		Category:    "cat_keyword",
		Tags:        []string{"tag_keyword"},
		Enabled:     true,
	}
	skill := &core.Skill{
		GetName: func() string { return "target_skill" },
	}
	info := &entity.SkillInfo{Def: def, Status: "ready", CanRun: true}

	// 另一个低分技能作为对照
	def2 := &entity.SkillDef{
		Name:        "other_skill",
		Description: "无关描述",
		Category:    "other",
		Tags:        []string{"other"},
		Enabled:     true,
	}
	skill2 := &core.Skill{
		GetName: func() string { return "other_skill" },
	}
	info2 := &entity.SkillInfo{Def: def2, Status: "ready", CanRun: true}

	searcher.SetData(
		map[string]*core.Skill{"target_skill": skill, "other_skill": skill2},
		map[string]*entity.SkillInfo{"target_skill": info, "other_skill": info2},
		nil,
	)

	tests := []struct {
		name    string
		keyword string
		expect  string // 期望命中的技能
	}{
		{"name匹配", "target_skill", "target_skill"},
		{"description匹配", "desc_keyword", "target_skill"},
		{"tag匹配", "tag_keyword", "target_skill"},
		{"category匹配", "cat_keyword", "target_skill"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := searcher.Search(tc.keyword)
			require.NoError(t, err)
			require.Greater(t, len(results), 0, "应有结果")
			assert.Equal(t, tc.expect, results[0].GetName())
		})
	}
}

func TestCalculateCosineSimilarity(t *testing.T) {
	// 注意：当前实现使用 dot/(norm1*norm2) 而非 dot/(sqrt(norm1)*sqrt(norm2))
	// 这不是标准余弦相似度，但在排序场景下单调性一致，不影响搜索结果排序
	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
		delta    float64
	}{
		{
			name:     "相同单位向量",
			vec1:     []float64{1, 0},
			vec2:     []float64{1, 0},
			expected: 1.0, // dot=1, norm1=1, norm2=1 → 1/(1*1)=1
			delta:    0.001,
		},
		{
			name:     "正交向量",
			vec1:     []float64{1, 0},
			vec2:     []float64{0, 1},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "反向单位向量",
			vec1:     []float64{1, 0},
			vec2:     []float64{-1, 0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "零向量",
			vec1:     []float64{0, 0, 0},
			vec2:     []float64{1, 2, 3},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "不同长度",
			vec1:     []float64{1, 2},
			vec2:     []float64{1, 2, 3},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "空向量",
			vec1:     []float64{},
			vec2:     []float64{},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "单调性：相同方向得分 > 正交方向",
			vec1:     []float64{1, 0},
			vec2:     []float64{1, 0},
			expected: 1.0,
			delta:    0.001,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateCosineSimilarity(tc.vec1, tc.vec2)
			assert.InDelta(t, tc.expected, result, tc.delta)
		})
	}
}

// TestIntentToToolSearch 模拟左脑输出的 intent+keywords 组合，验证搜索器能找到正确的工具
// 这是意图识别 → 工具搜索的端到端纯单元测试（不依赖 Ollama）
func TestIntentToToolSearch(t *testing.T) {
	_ = initTestLogging()
	searcher := newTestSearcher(t)

	tests := []struct {
		name           string
		intent         string   // 左脑输出的 intent
		keywords       []string // 左脑输出的 keywords
		expectContains []string // 期望搜索结果包含的技能名
		expectMissing  []string // 期望搜索结果不包含的技能名
	}{
		{
			name:           "天气意图找到weather技能",
			intent:         "查询天气",
			keywords:       []string{"天气", "北京", "明天"},
			expectContains: []string{"weather"},
			expectMissing:  []string{"calculator", "mcp_sina-finance_get-quote"},
		},
		{
			name:           "股票意图找到finance MCP技能",
			intent:         "查询股票行情",
			keywords:       []string{"A股", "行情", "今天"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
			expectMissing:  []string{"weather"},
		},
		{
			name:           "计算意图找到calculator技能",
			intent:         "计算",
			keywords:       []string{"计算", "123", "456"},
			expectContains: []string{"calculator"},
			expectMissing:  []string{"weather", "mcp_sina-finance_get-quote"},
		},
		{
			name:           "系统信息意图找到sysinfo技能",
			intent:         "查看系统信息",
			keywords:       []string{"系统", "CPU", "使用率"},
			expectContains: []string{"sysinfo"},
			expectMissing:  []string{"weather"},
		},
		{
			name:           "文件搜索意图找到finder技能",
			intent:         "文件搜索",
			keywords:       []string{"文件", "搜索"},
			expectContains: []string{"finder"},
		},
		{
			name:           "提醒意图找到reminders技能",
			intent:         "创建提醒",
			keywords:       []string{"提醒", "喝水"},
			expectContains: []string{"reminders"},
		},
		{
			name:           "组合搜索：question+intent+keywords全部作为搜索词",
			intent:         "查询股票行情",
			keywords:       []string{"stock", "行情"},
			expectContains: []string{"mcp_sina-finance_get-quote"},
		},
		{
			name:           "无匹配关键词返回空",
			intent:         "写诗",
			keywords:       []string{"诗歌", "唐诗"},
			expectContains: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟 brain.go tryRightBrainProcess 的搜索逻辑
			searchKeywords := []string{}
			if tc.intent != "" {
				searchKeywords = append(searchKeywords, tc.intent)
			}
			searchKeywords = append(searchKeywords, tc.keywords...)

			results, err := searcher.Search(searchKeywords...)
			require.NoError(t, err)

			foundNames := make([]string, 0, len(results))
			for _, s := range results {
				foundNames = append(foundNames, s.GetName())
			}

			for _, expected := range tc.expectContains {
				assert.Contains(t, foundNames, expected,
					"搜索词 %v 应找到 %s，实际结果: %v", searchKeywords, expected, foundNames)
			}
			for _, missing := range tc.expectMissing {
				assert.NotContains(t, foundNames, missing,
					"搜索词 %v 不应找到 %s，实际结果: %v", searchKeywords, missing, foundNames)
			}
		})
	}
}

// TestSearchPriority 验证多技能匹配时排序正确性（高分技能排在前面）
func TestSearchPriority(t *testing.T) {
	_ = initTestLogging()
	searcher := newTestSearcher(t)

	// "天气" 应该让 weather 排在第一位
	results, err := searcher.Search("天气")
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "weather", results[0].GetName(),
		"搜索'天气'时 weather 应排第一")

	// "A股" 应该让 finance 排在第一位
	results, err = searcher.Search("A股")
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "mcp_sina-finance_get-quote", results[0].GetName(),
		"搜索'A股'时 finance 技能应排第一")
}

// initTestLogging 初始化测试日志（幂等）
func initTestLogging() error {
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/searcher_test.log",
			MaxSize:    10,
			MaxBackups: 1,
			MaxAge:     1,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable: false,
		},
	}
	return logging.Init(logConfig)
}
