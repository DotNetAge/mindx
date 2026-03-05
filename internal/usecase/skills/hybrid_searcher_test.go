package skills

import (
	"encoding/json"
	"mindx/internal/entity"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridSearcher_Search(t *testing.T) {
	// 设置测试环境
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	// 索引测试数据
	skills := []*entity.Skill{
		{
			Name: "weather_query",
			Goal: "查询天气信息",
			Triggers: []string{"用户询问天气", "用户提到天气"},
			Tags: []string{"weather", "query", "天气"},
		},
		{
			Name: "calculator",
			Goal: "执行数学计算",
			Triggers: []string{"用户要求计算", "用户提到计算"},
			Tags: []string{"calculator", "math", "计算"},
		},
		{
			Name: "notes",
			Goal: "管理笔记",
			Triggers: []string{"用户创建笔记", "用户提到笔记"},
			Tags: []string{"notes", "productivity", "笔记"},
		},
	}

	// 索引到 vectorIndex
	err := vectorIndex.IndexBatch(skills)
	require.NoError(t, err)

	// 索引到 keywordIndex（使用旧的 SkillDef 格式）
	for _, skill := range skills {
		def := &entity.SkillDef{
			Name:        skill.Name,
			Description: skill.Goal,
			Tags:        skill.Tags,
		}
		keywordIndex.IndexSkill(def)
	}

	// 创建混合检索器
	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 测试搜索
	matches, err := searcher.Search("查询天气", 3)
	require.NoError(t, err)

	assert.NotEmpty(t, matches)
	assert.LessOrEqual(t, len(matches), 3)

	// 验证分数
	for _, match := range matches {
		assert.GreaterOrEqual(t, match.Score, 0.0)
		assert.LessOrEqual(t, match.Score, 1.0)
		assert.NotNil(t, match.Skill)
	}
}

func TestHybridSearcher_SearchWithWeights(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
		Tags: []string{"test"},
	}

	vectorIndex.Index(skill)
	keywordIndex.IndexSkill(&entity.SkillDef{
		Name: "test_skill",
		Tags: []string{"test"},
	})

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 测试不同权重
	tests := []struct {
		name          string
		vectorWeight  float64
		keywordWeight float64
	}{
		{"vector only", 1.0, 0.0},
		{"keyword only", 0.0, 1.0},
		{"balanced", 0.5, 0.5},
		{"vector heavy", 0.8, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := searcher.SearchWithWeights("test", 1, tt.vectorWeight, tt.keywordWeight)
			require.NoError(t, err)
			assert.NotEmpty(t, matches)
		})
	}
}

func TestHybridSearcher_Cache(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
	}

	vectorIndex.Index(skill)

	config := &HybridSearchConfig{
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
		CacheSize:     10,
		CacheTTL:      1 * time.Second,
	}

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, config)

	// 第一次搜索（缓存未命中）
	matches1, err := searcher.Search("test", 1)
	require.NoError(t, err)

	stats1 := searcher.GetCacheStats()
	assert.Equal(t, int64(0), stats1.Hits)
	assert.Equal(t, int64(1), stats1.Misses)

	// 第二次搜索（缓存命中）
	matches2, err := searcher.Search("test", 1)
	require.NoError(t, err)

	stats2 := searcher.GetCacheStats()
	assert.Equal(t, int64(1), stats2.Hits)
	assert.Equal(t, int64(1), stats2.Misses)

	// 验证结果相同
	assert.Equal(t, len(matches1), len(matches2))

	// 等待缓存过期
	time.Sleep(1100 * time.Millisecond)

	// 第三次搜索（缓存过期，未命中）
	_, err = searcher.Search("test", 1)
	require.NoError(t, err)

	stats3 := searcher.GetCacheStats()
	assert.Equal(t, int64(1), stats3.Hits)
	assert.Equal(t, int64(2), stats3.Misses)
}

func TestHybridSearcher_CacheEviction(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
	}

	vectorIndex.Index(skill)

	config := &HybridSearchConfig{
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
		CacheSize:     2, // 只缓存 2 个条目
		CacheTTL:      1 * time.Minute,
	}

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, config)

	// 搜索 3 次不同的查询
	searcher.Search("query1", 1)
	searcher.Search("query2", 1)
	searcher.Search("query3", 1) // 这次会触发驱逐

	stats := searcher.GetCacheStats()
	assert.Equal(t, int64(1), stats.Evicts)
}

func TestHybridSearcher_ClearCache(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
	}

	vectorIndex.Index(skill)

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 搜索并缓存
	searcher.Search("test", 1)
	searcher.Search("test", 1) // 缓存命中

	stats1 := searcher.GetCacheStats()
	assert.Equal(t, int64(1), stats1.Hits)

	// 清空缓存
	searcher.ClearCache()

	// 再次搜索（缓存未命中）
	searcher.Search("test", 1)

	stats2 := searcher.GetCacheStats()
	assert.Equal(t, int64(1), stats2.Hits) // 统计不会重置
	assert.Equal(t, int64(2), stats2.Misses)
}

func TestHybridSearcher_GetCacheHitRate(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
	}

	vectorIndex.Index(skill)

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 初始命中率为 0
	assert.Equal(t, 0.0, searcher.GetCacheHitRate())

	// 搜索 5 次，其中 3 次命中
	searcher.Search("test", 1) // miss
	searcher.Search("test", 1) // hit
	searcher.Search("test", 1) // hit
	searcher.Search("other", 1) // miss
	searcher.Search("test", 1) // hit

	// 命中率 = 3/5 = 0.6
	hitRate := searcher.GetCacheHitRate()
	assert.InDelta(t, 0.6, hitRate, 0.01)
}

func TestHybridSearcher_SetWeights(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 默认权重
	vw, kw := searcher.GetWeights()
	assert.InDelta(t, 0.7, vw, 0.01)
	assert.InDelta(t, 0.3, kw, 0.01)

	// 设置新权重
	searcher.SetWeights(0.5, 0.5)

	vw, kw = searcher.GetWeights()
	assert.InDelta(t, 0.5, vw, 0.01)
	assert.InDelta(t, 0.5, kw, 0.01)

	// 设置不平衡权重（会自动归一化）
	searcher.SetWeights(3.0, 1.0)

	vw, kw = searcher.GetWeights()
	assert.InDelta(t, 0.75, vw, 0.01)
	assert.InDelta(t, 0.25, kw, 0.01)
}

func TestHybridSearcher_FallbackToKeyword(t *testing.T) {
	db := setupTestDB(t)

	// 创建一个会失败的 embedding service
	failingEmbedding := &FailingEmbeddingProvider{}

	vectorIndex := NewVectorIndex(db, failingEmbedding)
	keywordIndex := NewKeywordIndex()

	// 创建一个 Skill 并索引到 vectorIndex（不使用 embedding）
	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试技能",
		Tags: []string{"test"},
	}

	// 手动存储到 vectorIndex（绕过 embedding 生成）
	err := db.Update(func(txn *badger.Txn) error {
		skillKey := []byte("skill:test_skill")
		skillData, _ := json.Marshal(skill)
		return txn.Set(skillKey, skillData)
	})
	require.NoError(t, err)

	// 索引到 keywordIndex
	keywordIndex.IndexSkill(&entity.SkillDef{
		Name: "test_skill",
		Tags: []string{"test"},
	})

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	// 向量搜索会失败，应该回退到关键词搜索
	matches, err := searcher.Search("test", 1)
	require.NoError(t, err)

	// 应该能从关键词搜索获得结果
	assert.NotEmpty(t, matches)
}

// FailingEmbeddingProvider 模拟失败的 embedding 提供者
type FailingEmbeddingProvider struct{}

func (f *FailingEmbeddingProvider) GenerateEmbedding(text string) ([]float64, error) {
	return nil, assert.AnError
}

func (f *FailingEmbeddingProvider) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	return nil, assert.AnError
}

func BenchmarkHybridSearcher_Search(b *testing.B) {
	db := setupTestDB(&testing.T{})
	embeddingService := NewMockEmbeddingProvider(128)

	vectorIndex := NewVectorIndex(db, embeddingService)
	keywordIndex := NewKeywordIndex()

	// 索引 100 个 Skills
	skills := make([]*entity.Skill, 100)
	for i := 0; i < 100; i++ {
		skill := &entity.Skill{
			Name: "skill_" + string(rune(i)),
			Goal: "目标 " + string(rune(i)),
			Tags: []string{"tag" + string(rune(i))},
		}
		skills[i] = skill

		keywordIndex.IndexSkill(&entity.SkillDef{
			Name: skill.Name,
			Tags: skill.Tags,
		})
	}
	vectorIndex.IndexBatch(skills)

	searcher := NewHybridSearcher(vectorIndex, keywordIndex, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search("查询测试", 10)
	}
}
