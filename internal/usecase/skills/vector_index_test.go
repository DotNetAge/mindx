package skills

import (
	"mindx/internal/entity"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingProvider 模拟 Embedding 提供者
type MockEmbeddingProvider struct {
	dimension int
}

func NewMockEmbeddingProvider(dimension int) *MockEmbeddingProvider {
	return &MockEmbeddingProvider{dimension: dimension}
}

func (m *MockEmbeddingProvider) GenerateEmbedding(text string) ([]float64, error) {
	// 生成简单的模拟向量（基于文本长度）
	embedding := make([]float64, m.dimension)
	for i := 0; i < m.dimension; i++ {
		embedding[i] = float64(len(text)+i) / 100.0
	}
	return embedding, nil
}

func (m *MockEmbeddingProvider) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))
	for i, text := range texts {
		embedding, err := m.GenerateEmbedding(text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

// setupTestDB 创建测试用的 BadgerDB
func setupTestDB(t *testing.T) *badger.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // 禁用日志

	db, err := badger.Open(opts)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	return db
}

func TestVectorIndex_Index(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	skill := &entity.Skill{
		Name:        "weather_query",
		Description: "天气查询",
		Goal:        "查询天气信息",
		Triggers:    []string{"用户询问天气", "用户提到天气"},
	}

	// 索引 Skill
	err := idx.Index(skill)
	require.NoError(t, err)

	// 验证向量已生成
	assert.NotEmpty(t, skill.Embedding)
	assert.Len(t, skill.Embedding, 128)

	// 验证可以检索
	retrieved, err := idx.GetSkill("weather_query")
	require.NoError(t, err)
	assert.Equal(t, "weather_query", retrieved.Name)
	assert.NotEmpty(t, retrieved.Embedding)
}

func TestVectorIndex_IndexBatch(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	skills := []*entity.Skill{
		{
			Name: "weather_query",
			Goal: "查询天气信息",
		},
		{
			Name: "calculator",
			Goal: "执行数学计算",
		},
		{
			Name: "notes",
			Goal: "管理笔记",
		},
	}

	// 批量索引
	err := idx.IndexBatch(skills)
	require.NoError(t, err)

	// 验证所有 Skill 都有向量
	for _, skill := range skills {
		assert.NotEmpty(t, skill.Embedding)
		assert.Len(t, skill.Embedding, 128)
	}

	// 验证可以检索所有 Skills
	allSkills, err := idx.GetAllSkills()
	require.NoError(t, err)
	assert.Len(t, allSkills, 3)
}

func TestVectorIndex_Search(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	// 索引多个 Skills
	skills := []*entity.Skill{
		{
			Name: "weather_query",
			Goal: "查询天气信息",
			Triggers: []string{"用户询问天气"},
		},
		{
			Name: "calculator",
			Goal: "执行数学计算",
			Triggers: []string{"用户要求计算"},
		},
		{
			Name: "notes",
			Goal: "管理笔记",
			Triggers: []string{"用户创建笔记"},
		},
	}

	err := idx.IndexBatch(skills)
	require.NoError(t, err)

	// 搜索
	matches, err := idx.Search("查询天气", 2)
	require.NoError(t, err)

	// 验证结果
	assert.NotEmpty(t, matches)
	assert.LessOrEqual(t, len(matches), 2)

	// 验证分数在 [0, 1] 范围内
	for _, match := range matches {
		assert.GreaterOrEqual(t, match.Score, 0.0)
		assert.LessOrEqual(t, match.Score, 1.0)
		assert.NotNil(t, match.Skill)
	}
}

func TestVectorIndex_SearchByEmbedding(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	// 索引 Skill
	skill := &entity.Skill{
		Name: "weather_query",
		Goal: "查询天气信息",
	}

	err := idx.Index(skill)
	require.NoError(t, err)

	// 使用向量搜索
	queryEmbedding := make([]float32, 128)
	for i := 0; i < 128; i++ {
		queryEmbedding[i] = float32(i) / 100.0
	}

	matches, err := idx.SearchByEmbedding(queryEmbedding, 1)
	require.NoError(t, err)

	assert.Len(t, matches, 1)
	assert.Equal(t, "weather_query", matches[0].Skill.Name)
}

func TestVectorIndex_GetSkill(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	skill := &entity.Skill{
		Name:        "test_skill",
		Description: "测试技能",
		Goal:        "测试目标",
	}

	// 索引
	err := idx.Index(skill)
	require.NoError(t, err)

	// 获取
	retrieved, err := idx.GetSkill("test_skill")
	require.NoError(t, err)
	assert.Equal(t, "test_skill", retrieved.Name)
	assert.Equal(t, "测试技能", retrieved.Description)
	assert.Equal(t, "测试目标", retrieved.Goal)

	// 获取不存在的 Skill
	_, err = idx.GetSkill("non_existent")
	assert.Error(t, err)
}

func TestVectorIndex_GetAllSkills(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	// 索引多个 Skills
	skills := []*entity.Skill{
		{Name: "skill1", Goal: "目标1"},
		{Name: "skill2", Goal: "目标2"},
		{Name: "skill3", Goal: "目标3"},
	}

	err := idx.IndexBatch(skills)
	require.NoError(t, err)

	// 获取所有
	allSkills, err := idx.GetAllSkills()
	require.NoError(t, err)
	assert.Len(t, allSkills, 3)

	// 验证名称
	names := make(map[string]bool)
	for _, skill := range allSkills {
		names[skill.Name] = true
	}
	assert.True(t, names["skill1"])
	assert.True(t, names["skill2"])
	assert.True(t, names["skill3"])
}

func TestVectorIndex_Delete(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试目标",
	}

	// 索引
	err := idx.Index(skill)
	require.NoError(t, err)

	// 验证存在
	_, err = idx.GetSkill("test_skill")
	require.NoError(t, err)

	// 删除
	err = idx.Delete("test_skill")
	require.NoError(t, err)

	// 验证已删除
	_, err = idx.GetSkill("test_skill")
	assert.Error(t, err)
}

func TestVectorIndex_Clear(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	// 索引多个 Skills
	skills := []*entity.Skill{
		{Name: "skill1", Goal: "目标1"},
		{Name: "skill2", Goal: "目标2"},
		{Name: "skill3", Goal: "目标3"},
	}

	err := idx.IndexBatch(skills)
	require.NoError(t, err)

	// 验证存在
	allSkills, err := idx.GetAllSkills()
	require.NoError(t, err)
	assert.Len(t, allSkills, 3)

	// 清空
	err = idx.Clear()
	require.NoError(t, err)

	// 验证已清空
	allSkills, err = idx.GetAllSkills()
	require.NoError(t, err)
	assert.Len(t, allSkills, 0)
}

func TestVectorIndex_CosineSimilarity(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "similar vectors",
			a:        []float32{1, 1, 0},
			b:        []float32{1, 0.5, 0},
			expected: 0.9486833, // cos(angle) ≈ 0.9487
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := idx.cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, similarity, 0.01)
		})
	}
}

func TestVectorIndex_SortByScore(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	matches := []*entity.SkillMatch{
		{Skill: &entity.Skill{Name: "skill1"}, Score: 0.5},
		{Skill: &entity.Skill{Name: "skill2"}, Score: 0.9},
		{Skill: &entity.Skill{Name: "skill3"}, Score: 0.3},
		{Skill: &entity.Skill{Name: "skill4"}, Score: 0.7},
	}

	idx.sortByScore(matches)

	// 验证降序排列
	assert.Equal(t, "skill2", matches[0].Skill.Name) // 0.9
	assert.Equal(t, "skill4", matches[1].Skill.Name) // 0.7
	assert.Equal(t, "skill1", matches[2].Skill.Name) // 0.5
	assert.Equal(t, "skill3", matches[3].Skill.Name) // 0.3
}

func TestVectorIndex_SerializeDeserialize(t *testing.T) {
	db := setupTestDB(t)
	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	original := []float32{1.5, 2.3, 3.7, 4.2, 5.9}

	// 序列化
	serialized := idx.serializeVector(original)
	assert.NotEmpty(t, serialized)

	// 反序列化
	deserialized := idx.deserializeVector(serialized)
	assert.Equal(t, len(original), len(deserialized))

	// 验证值相等
	for i := 0; i < len(original); i++ {
		assert.InDelta(t, original[i], deserialized[i], 0.0001)
	}
}

func BenchmarkVectorIndex_Index(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	require.NoError(b, err)
	defer db.Close()

	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	skill := &entity.Skill{
		Name: "test_skill",
		Goal: "测试目标",
		Triggers: []string{"触发条件1", "触发条件2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		skill.Name = "test_skill_" + string(rune(i))
		idx.Index(skill)
	}
}

func BenchmarkVectorIndex_Search(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	require.NoError(b, err)
	defer db.Close()

	embeddingService := NewMockEmbeddingProvider(128)
	idx := NewVectorIndex(db, embeddingService)

	// 索引 100 个 Skills
	skills := make([]*entity.Skill, 100)
	for i := 0; i < 100; i++ {
		skills[i] = &entity.Skill{
			Name: "skill_" + string(rune(i)),
			Goal: "目标 " + string(rune(i)),
		}
	}
	idx.IndexBatch(skills)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search("查询测试", 10)
	}
}
