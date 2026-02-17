package persistence

import (
	"fmt"
	"os"
	"path/filepath"

	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/utils"
)

// createDirectoryIfNotExists 创建目录（如果不存在）
func createDirectoryIfNotExists(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("数据库路径不能为空")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return nil
}

// NewStore 创建向量存储
func NewStore(storeType string, dbPath string, provider core.EmbeddingProvider) (Store, error) {
	switch storeType {
	case "badger":
		if dbPath == "" {
			dbPath = filepath.Join("data", "vectors")
		}

		if err := os.MkdirAll(dbPath, 0755); err != nil {
			return nil, fmt.Errorf("创建数据库目录失败: %w", err)
		}

		return NewBadgerStore(dbPath, provider)
	case "memory":
		fallthrough
	default:
		return NewMemoryStore(provider), nil
	}
}

// VectorService 向量相似度计算服务
type VectorService struct{}

// NewVectorService 创建向量服务
func NewVectorService() *VectorService {
	return &VectorService{}
}

// FindMostSimilar 找到最相似的向量
func (s *VectorService) FindMostSimilar(queryVec []float64, candidates []entity.SimilarityResult, topN int) []entity.SimilarityResult {
	return utils.FindMostSimilar(queryVec, candidates, topN)
}
