package persistence

import (
	"encoding/json"
	"fmt"
	"sync"

	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/utils"
)

// MemoryStore 内存向量存储实现
type MemoryStore struct {
	vectors  map[string][]float64
	metadata map[string][]byte
	svc      *VectorService
	provider core.EmbeddingProvider
	mu       sync.RWMutex
}

// NewMemoryStore 创建内存向量存储
func NewMemoryStore(provider core.EmbeddingProvider) *MemoryStore {
	return &MemoryStore{
		vectors:  make(map[string][]float64),
		metadata: make(map[string][]byte),
		svc:      NewVectorService(),
		provider: provider,
	}
}

// Put 存储向量
func (s *MemoryStore) Put(key string, vector []float64, metadata interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vectors[key] = vector

	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		s.metadata[key] = metadataBytes
	}

	return nil
}

// Get 获取向量
func (s *MemoryStore) Get(key string) (*entity.VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vector, exists := s.vectors[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return &entity.VectorEntry{
		Key:      key,
		Vector:   vector,
		Metadata: s.metadata[key],
	}, nil
}

// Delete 删除向量
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.vectors, key)
	delete(s.metadata, key)

	return nil
}

// Search 搜索最相似的向量
func (s *MemoryStore) Search(queryVec []float64, topN int) ([]entity.VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.vectors) == 0 {
		return []entity.VectorEntry{}, nil
	}

	similarityResults := make([]entity.SimilarityResult, 0, len(s.vectors))
	for key, vector := range s.vectors {
		if vector != nil && len(vector) > 0 {
			similarityResults = append(similarityResults, entity.SimilarityResult{
				Target: key,
				Score:  utils.CalculateCosineSimilarity(queryVec, vector),
				Metadata: map[string]interface{}{
					"vector": vector,
					"entry": entity.VectorEntry{
						Key:      key,
						Vector:   vector,
						Metadata: s.metadata[key],
					},
				},
			})
		}
	}

	topResults := utils.FindMostSimilar(queryVec, similarityResults, topN)

	results := make([]entity.VectorEntry, 0, len(topResults))
	for _, result := range topResults {
		if entry, ok := result.Metadata["entry"].(entity.VectorEntry); ok {
			results = append(results, entry)
		}
	}

	return results, nil
}

// Close 关闭存储
func (s *MemoryStore) Close() error {
	return nil
}

// BatchPut 批量存储向量
func (s *MemoryStore) BatchPut(entries []entity.VectorEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		s.vectors[entry.Key] = entry.Vector
		if entry.Metadata != nil {
			s.metadata[entry.Key] = entry.Metadata
		}
	}

	return nil
}

// Size 获取向量数量
func (s *MemoryStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vectors)
}

func (s *MemoryStore) Scan(prefix string) ([]entity.VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []entity.VectorEntry

	for key, vector := range s.vectors {
		if prefix != "" && (len(key) < len(prefix) || key[:len(prefix)] != prefix) {
			continue
		}

		entries = append(entries, entity.VectorEntry{
			Key:      key,
			Vector:   vector,
			Metadata: s.metadata[key],
		})
	}

	return entries, nil
}
