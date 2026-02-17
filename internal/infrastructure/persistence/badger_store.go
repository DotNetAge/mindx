package persistence

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v4"

	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/utils"
)

// BadgerStore Badger向量存储实现
type BadgerStore struct {
	db       *badger.DB
	svc      *VectorService
	provider core.EmbeddingProvider
}

// NewBadgerStore 创建Badger向量存储
func NewBadgerStore(dbPath string, provider core.EmbeddingProvider) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("打开 Badger 数据库失败: %w", err)
	}

	return &BadgerStore{
		db:      db,
		svc:     NewVectorService(),
		provider: provider,
	}, nil
}

// Put 存储向量
func (s *BadgerStore) Put(key string, vector []float64, metadata interface{}) error {
	if vector == nil {
		return fmt.Errorf("vector cannot be nil")
	}

	entry := entity.VectorEntry{
		Key:    key,
		Vector: vector,
	}

	if metadata != nil {
		switch m := metadata.(type) {
		case []byte:
			entry.Metadata = m
		case json.RawMessage:
			entry.Metadata = m
		default:
			metadataBytes, err := json.Marshal(metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}
			entry.Metadata = metadataBytes
		}
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), entryBytes)
	})
}

// Get 获取向量
func (s *BadgerStore) Get(key string) (*entity.VectorEntry, error) {
	var entry entity.VectorEntry

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})

	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// Delete 删除向量
func (s *BadgerStore) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Search 搜索最相似的向量
func (s *BadgerStore) Search(queryVec []float64, topN int) ([]entity.VectorEntry, error) {
	var candidates []entity.VectorEntry

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			var entry entity.VectorEntry
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			})
			if err != nil {
				continue
			}

			if entry.Vector != nil && len(entry.Vector) > 0 {
				candidates = append(candidates, entry)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return []entity.VectorEntry{}, nil
	}

	similarityResults := make([]entity.SimilarityResult, 0, len(candidates))
	for _, candidate := range candidates {
		similarityResults = append(similarityResults, entity.SimilarityResult{
			Target: candidate.Key,
			Score:  utils.CalculateCosineSimilarity(queryVec, candidate.Vector),
			Metadata: map[string]interface{}{
				"vector": candidate.Vector,
				"entry":  candidate,
			},
		})
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

// Close 关闭数据库
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// BatchPut 批量存储向量
func (s *BadgerStore) BatchPut(entries []entity.VectorEntry) error {
	if len(entries) == 0 {
		return nil
	}

	return s.db.Update(func(txn *badger.Txn) error {
		for _, entry := range entries {
			entryBytes, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if err := txn.Set([]byte(entry.Key), entryBytes); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BadgerStore) Scan(prefix string) ([]entity.VectorEntry, error) {
	var entries []entity.VectorEntry

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			if prefix != "" && len(key) < len(prefix) || (prefix != "" && key[:len(prefix)] != prefix) {
				continue
			}

			var entry entity.VectorEntry
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			})
			if err != nil {
				continue
			}

			entries = append(entries, entry)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}
