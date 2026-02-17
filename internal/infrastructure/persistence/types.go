package persistence

import "mindx/internal/entity"

type Store interface {
	Put(key string, vector []float64, metadata interface{}) error
	Get(key string) (*entity.VectorEntry, error)
	Delete(key string) error
	Search(queryVec []float64, topN int) ([]entity.VectorEntry, error)
	BatchPut(entries []entity.VectorEntry) error
	Scan(prefix string) ([]entity.VectorEntry, error)
	Close() error
}
