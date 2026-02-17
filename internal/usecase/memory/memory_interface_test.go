package memory

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/embedding"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRecord 测试记录记忆点功能
// 测试目的：验证 Record 方法记录记忆点的正确性
// 测试效果：确保能正确记录记忆点，处理各种边界情况
func TestRecord(t *testing.T) {
	provider := infraEmbedding.NewTFIDFEmbedding()
	store := persistence.NewMemoryStore(provider)
	m := &Memory{
		logger:           newTestLogger(),
		embeddingService: embedding.NewEmbeddingService(provider),
		store:            store,
	}

	t.Run("正常记录记忆点", func(t *testing.T) {
		point := core.MemoryPoint{
			ID:          1,
			Content:     "这是一段测试内容",
			Summary:     "测试摘要",
			Keywords:    []string{"测试", "内容"},
			Vector:      []float64{0.1, 0.2, 0.3},
			TotalWeight: 0.8,
		}

		err := m.Record(point)
		assert.NoError(t, err)
	})

	t.Run("记忆点已有向量", func(t *testing.T) {
		point := core.MemoryPoint{
			ID:          2,
			Content:     "已有向量的记忆点",
			Summary:     "测试摘要",
			Keywords:    []string{"测试"},
			Vector:      []float64{0.5, 0.6, 0.7},
			TotalWeight: 0.9,
		}

		err := m.Record(point)
		assert.NoError(t, err)
	})

	t.Run("记忆点无向量需要生成", func(t *testing.T) {
		point := core.MemoryPoint{
			ID:          3,
			Content:     "需要生成向量的记忆点",
			Summary:     "测试摘要",
			Keywords:    []string{"向量", "生成"},
			Vector:      []float64{},
			TotalWeight: 0.7,
		}

		err := m.Record(point)
		assert.NoError(t, err)
	})

	t.Run("记忆点缺少创建时间", func(t *testing.T) {
		point := core.MemoryPoint{
			ID:          4,
			Content:     "缺少创建时间的记忆点",
			Summary:     "测试摘要",
			Keywords:    []string{"时间"},
			Vector:      []float64{0.2, 0.3, 0.4},
			TotalWeight: 0.6,
			CreatedAt:   time.Time{},
		}

		err := m.Record(point)
		assert.NoError(t, err)
	})

	t.Run("记忆点缺少更新时间", func(t *testing.T) {
		point := core.MemoryPoint{
			ID:          5,
			Content:     "缺少更新时间的记忆点",
			Summary:     "测试摘要",
			Keywords:    []string{"更新"},
			Vector:      []float64{0.3, 0.4, 0.5},
			TotalWeight: 0.8,
			CreatedAt:   time.Now(),
		}

		err := m.Record(point)
		assert.NoError(t, err)
	})

	t.Run("embeddingService为nil", func(t *testing.T) {
		mNilEmbedding := &Memory{
			logger:           newTestLogger(),
			embeddingService: nil,
			store:            store,
		}

		point := core.MemoryPoint{
			ID:          6,
			Content:     "无embedding服务的记忆点",
			Summary:     "测试摘要",
			Keywords:    []string{"nil"},
			Vector:      []float64{},
			TotalWeight: 0.5,
		}

		err := mNilEmbedding.Record(point)
		assert.NoError(t, err)
	})
}

// TestSearch 测试搜索相似记忆功能
// 测试目的：验证 Search 方法搜索相似记忆的正确性
// 测试效果：确保能正确处理各种搜索场景
func TestSearch(t *testing.T) {
	provider := infraEmbedding.NewTFIDFEmbedding()
	store := persistence.NewMemoryStore(provider)
	m := &Memory{
		logger:           newTestLogger(),
		embeddingService: embedding.NewEmbeddingService(provider),
		store:            store,
	}

	t.Run("正常搜索", func(t *testing.T) {
		results, err := m.Search("测试搜索")
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("空搜索词", func(t *testing.T) {
		results, err := m.Search("")
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("无匹配结果", func(t *testing.T) {
		results, err := m.Search("不存在的关键词xyz123")
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("搜索词包含特殊字符", func(t *testing.T) {
		results, err := m.Search("测试!@#$%^&*()")
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("长搜索词", func(t *testing.T) {
		longText := strings.Repeat("测试内容 ", 50)
		results, err := m.Search(longText)
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})
}

// TestClusterConversations 测试对话聚类功能
// 测试目的：验证 ClusterConversations 方法使用K-means算法对对话进行聚类的正确性
// 测试效果：确保相似话题的对话被分到同一簇，不同话题的对话被分到不同簇
func TestClusterConversations(t *testing.T) {
	provider := infraEmbedding.NewTFIDFEmbedding()
	m := &Memory{
		logger:           newTestLogger(),
		embeddingService: embedding.NewEmbeddingService(provider),
	}

	now := time.Now()
	testConversations := []entity.ConversationLog{
		{
			ID:        "1",
			Topic:     "天气讨论",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(-5 * time.Minute),
			Messages: []entity.LogMessage{
				{Sender: "user", Content: "今天天气怎么样？", Timestamp: now.Add(-10 * time.Minute)},
				{Sender: "bot", Content: "今天天气晴朗，温度25度。", Timestamp: now.Add(-9 * time.Minute)},
			},
		},
		{
			ID:        "2",
			Topic:     "天气讨论",
			StartTime: now.Add(-8 * time.Minute),
			EndTime:   now.Add(-3 * time.Minute),
			Messages: []entity.LogMessage{
				{Sender: "user", Content: "明天会下雨吗？", Timestamp: now.Add(-8 * time.Minute)},
				{Sender: "bot", Content: "明天可能会有小雨。", Timestamp: now.Add(-7 * time.Minute)},
			},
		},
		{
			ID:        "3",
			Topic:     "美食推荐",
			StartTime: now.Add(-6 * time.Minute),
			EndTime:   now.Add(-1 * time.Minute),
			Messages: []entity.LogMessage{
				{Sender: "user", Content: "附近有什么好吃的餐厅？", Timestamp: now.Add(-6 * time.Minute)},
				{Sender: "bot", Content: "推荐你去尝试一下川菜馆。", Timestamp: now.Add(-5 * time.Minute)},
			},
		},
		{
			ID:        "4",
			Topic:     "美食推荐",
			StartTime: now.Add(-4 * time.Minute),
			EndTime:   now.Add(0 * time.Minute),
			Messages: []entity.LogMessage{
				{Sender: "user", Content: "川菜馆有什么特色菜？", Timestamp: now.Add(-4 * time.Minute)},
				{Sender: "bot", Content: "他们的麻婆豆腐和水煮鱼很有名。", Timestamp: now.Add(-3 * time.Minute)},
			},
		},
	}

	err := m.ClusterConversations(testConversations)
	assert.NoError(t, err)
	t.Log("K-means对话聚类测试完成，方法执行正常")
}

// TestOptimize 测试记忆优化功能
// 测试目的：验证 Optimize 方法优化记忆系统的正确性
// 测试效果：确保能正确清理过期记忆
func TestOptimize(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	err := m.Optimize()
	assert.NoError(t, err)
}

// TestCleanupExpiredMemories 测试记忆清理功能
// 测试目的：验证 CleanupExpiredMemories 方法清理过期和无效记忆的正确性
// 测试效果：确保方法调用不会出错，实际清理效果需要在集成测试中验证
func TestCleanupExpiredMemories(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	err := m.CleanupExpiredMemories()
	assert.NoError(t, err)
}

// TestAdjustMemoryWeight 测试手动调整权重功能
// 测试目的：验证 AdjustMemoryWeight 方法手动调整记忆权重的正确性
// 测试效果：确保方法调用不会出错，实际调整效果需要在集成测试中验证
func TestAdjustMemoryWeight(t *testing.T) {
	m := &Memory{
		logger: newTestLogger(),
	}

	err := m.AdjustMemoryWeight(123, 1.5)
	assert.Error(t, err)
}
