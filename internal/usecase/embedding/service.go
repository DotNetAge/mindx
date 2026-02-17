package embedding

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/utils"
	"fmt"
	"sync"

	"github.com/hashicorp/golang-lru/v2"
)

// EmbeddingService 向量化服务
// 系统所有向量化操作都通过该服务进行
type EmbeddingService struct {
	provider core.EmbeddingProvider
	cache    *lru.Cache[string, []float64]
	mutex    sync.RWMutex
}

// EmbeddingService 创建Embedding服务
func NewEmbeddingService(provider core.EmbeddingProvider) *EmbeddingService {
	// 初始化LRU缓存，大小设为500，适配个人使用场景
	cache, _ := lru.New[string, []float64](500)
	return &EmbeddingService{
		provider: provider,
		cache:    cache,
	}
}

// GenerateEmbedding 生成向量（带缓存）
func (s *EmbeddingService) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	s.mutex.RLock()
	if vec, exists := s.cache.Get(text); exists {
		s.mutex.RUnlock()
		return vec, nil
	}
	s.mutex.RUnlock()

	// 检查 provider 是否可用
	if s.provider == nil {
		return nil, fmt.Errorf("embedding provider is not configured")
	}

	vec, err := s.provider.GenerateEmbedding(text)
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	s.cache.Add(text, vec)
	s.mutex.Unlock()

	return vec, nil
}

// GenerateBatchEmbeddings 批量生成向量
func (s *EmbeddingService) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	embeddings := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := s.GenerateEmbedding(text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		embeddings[i] = vec
	}

	return embeddings, nil
}

// FindMostSimilar 找到最相似的向量
func (s *EmbeddingService) FindMostSimilar(queryVec []float64, candidates []entity.SimilarityResult, topN int) []entity.SimilarityResult {
	return utils.FindMostSimilar(queryVec, candidates, topN)
}

// GetCacheSize 获取缓存大小
func (s *EmbeddingService) GetCacheSize() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cache.Len()
}

// ClearCache 清空缓存
func (s *EmbeddingService) ClearCache() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.cache.Purge()
}
