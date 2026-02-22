package core

import "mindx/internal/entity"

// Store 向量存储接口
type Store interface {
	Put(key string, vector []float64, metadata interface{}) error
	Get(key string) (*entity.VectorEntry, error)
	Delete(key string) error
	Search(queryVec []float64, topN int) ([]entity.VectorEntry, error)
	SearchWithThreshold(queryVec []float64, topN int, minScore float64) ([]entity.VectorEntry, error)
	BatchPut(entries []entity.VectorEntry) error
	Scan(prefix string) ([]entity.VectorEntry, error)
	Close() error
}
