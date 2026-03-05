package skills

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"mindx/internal/core"
	"mindx/internal/entity"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// VectorIndex 向量索引
// 使用 BadgerDB 存储 Skill 的向量表示，支持相似度搜索
type VectorIndex struct {
	db               *badger.DB
	embeddingService core.EmbeddingProvider
	dimension        int
	mu               sync.RWMutex
}

// NewVectorIndex 创建向量索引
func NewVectorIndex(db *badger.DB, embeddingService core.EmbeddingProvider) *VectorIndex {
	return &VectorIndex{
		db:               db,
		embeddingService: embeddingService,
		dimension:        0, // 将在第一次索引时确定
	}
}

// Index 索引单个 Skill
func (idx *VectorIndex) Index(skill *entity.Skill) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 1. 生成向量
	text := skill.GetEmbeddingText()
	embedding, err := idx.embeddingService.GenerateEmbedding(text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// 2. 确定维度
	if idx.dimension == 0 {
		idx.dimension = len(embedding)
	} else if len(embedding) != idx.dimension {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", idx.dimension, len(embedding))
	}

	// 3. 转换为 float32（节省空间）
	embedding32 := make([]float32, len(embedding))
	for i, v := range embedding {
		embedding32[i] = float32(v)
	}
	skill.Embedding = embedding32

	// 4. 存储到 BadgerDB
	return idx.db.Update(func(txn *badger.Txn) error {
		// 存储 Skill 数据
		skillKey := []byte(fmt.Sprintf("skill:%s", skill.Name))
		skillData, err := json.Marshal(skill)
		if err != nil {
			return fmt.Errorf("failed to marshal skill: %w", err)
		}

		if err := txn.Set(skillKey, skillData); err != nil {
			return fmt.Errorf("failed to store skill: %w", err)
		}

		// 存储向量数据（用于快速检索）
		vectorKey := []byte(fmt.Sprintf("vector:%s", skill.Name))
		vectorData := idx.serializeVector(embedding32)

		if err := txn.Set(vectorKey, vectorData); err != nil {
			return fmt.Errorf("failed to store vector: %w", err)
		}

		return nil
	})
}

// IndexBatch 批量索引 Skills
func (idx *VectorIndex) IndexBatch(skills []*entity.Skill) error {
	// 1. 批量生成向量
	texts := make([]string, len(skills))
	for i, skill := range skills {
		texts[i] = skill.GetEmbeddingText()
	}

	embeddings, err := idx.embeddingService.GenerateBatchEmbeddings(texts)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	// 2. 批量存储
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(txn *badger.Txn) error {
		for i, skill := range skills {
			embedding := embeddings[i]

			// 确定维度
			if idx.dimension == 0 {
				idx.dimension = len(embedding)
			}

			// 转换为 float32
			embedding32 := make([]float32, len(embedding))
			for j, v := range embedding {
				embedding32[j] = float32(v)
			}
			skill.Embedding = embedding32

			// 存储 Skill 数据
			skillKey := []byte(fmt.Sprintf("skill:%s", skill.Name))
			skillData, err := json.Marshal(skill)
			if err != nil {
				return fmt.Errorf("failed to marshal skill %s: %w", skill.Name, err)
			}

			if err := txn.Set(skillKey, skillData); err != nil {
				return fmt.Errorf("failed to store skill %s: %w", skill.Name, err)
			}

			// 存储向量数据
			vectorKey := []byte(fmt.Sprintf("vector:%s", skill.Name))
			vectorData := idx.serializeVector(embedding32)

			if err := txn.Set(vectorKey, vectorData); err != nil {
				return fmt.Errorf("failed to store vector %s: %w", skill.Name, err)
			}
		}

		return nil
	})
}

// Search 向量相似度搜索
func (idx *VectorIndex) Search(query string, topK int) ([]*entity.SkillMatch, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 1. 生成查询向量
	queryEmbedding, err := idx.embeddingService.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// 转换为 float32
	queryEmbedding32 := make([]float32, len(queryEmbedding))
	for i, v := range queryEmbedding {
		queryEmbedding32[i] = float32(v)
	}

	return idx.SearchByEmbedding(queryEmbedding32, topK)
}

// SearchByEmbedding 直接使用向量搜索
func (idx *VectorIndex) SearchByEmbedding(queryEmbedding []float32, topK int) ([]*entity.SkillMatch, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var matches []*entity.SkillMatch

	// 2. 遍历所有向量，计算相似度
	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("vector:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			vectorKey := item.Key()

			// 提取 skill name
			skillName := string(vectorKey[7:]) // 跳过 "vector:" 前缀

			// 读取向量数据
			var vectorData []byte
			err := item.Value(func(val []byte) error {
				vectorData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				return err
			}

			embedding := idx.deserializeVector(vectorData)

			// 计算余弦相似度
			similarity := idx.cosineSimilarity(queryEmbedding, embedding)

			// 读取 Skill 数据
			skillKey := []byte(fmt.Sprintf("skill:%s", skillName))
			skillItem, err := txn.Get(skillKey)
			if err != nil {
				continue // 跳过无法读取的 Skill
			}

			var skill entity.Skill
			err = skillItem.Value(func(val []byte) error {
				return json.Unmarshal(val, &skill)
			})
			if err != nil {
				continue
			}

			matches = append(matches, &entity.SkillMatch{
				Skill: &skill,
				Score: float64(similarity),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// 3. 排序并返回 TopK
	idx.sortByScore(matches)

	if len(matches) > topK {
		matches = matches[:topK]
	}

	return matches, nil
}

// GetSkill 获取单个 Skill
func (idx *VectorIndex) GetSkill(name string) (*entity.Skill, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var skill entity.Skill

	err := idx.db.View(func(txn *badger.Txn) error {
		skillKey := []byte(fmt.Sprintf("skill:%s", name))
		item, err := txn.Get(skillKey)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &skill)
		})
	})

	if err != nil {
		return nil, err
	}

	return &skill, nil
}

// GetAllSkills 获取所有 Skills
func (idx *VectorIndex) GetAllSkills() ([]*entity.Skill, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var skills []*entity.Skill

	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("skill:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			var skill entity.Skill
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &skill)
			})
			if err != nil {
				continue
			}

			skills = append(skills, &skill)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return skills, nil
}

// Delete 删除 Skill
func (idx *VectorIndex) Delete(name string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(txn *badger.Txn) error {
		skillKey := []byte(fmt.Sprintf("skill:%s", name))
		vectorKey := []byte(fmt.Sprintf("vector:%s", name))

		if err := txn.Delete(skillKey); err != nil {
			return err
		}

		if err := txn.Delete(vectorKey); err != nil {
			return err
		}

		return nil
	})
}

// Clear 清空索引
func (idx *VectorIndex) Clear() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(txn *badger.Txn) error {
		// 删除所有 skill: 和 vector: 前缀的键
		prefixes := []string{"skill:", "vector:"}

		for _, prefix := range prefixes {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = []byte(prefix)
			it := txn.NewIterator(opts)

			var keysToDelete [][]byte
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				keysToDelete = append(keysToDelete, key)
			}
			it.Close()

			for _, key := range keysToDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// serializeVector 序列化向量（float32 数组 -> bytes）
func (idx *VectorIndex) serializeVector(vector []float32) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, vector)
	return buf.Bytes()
}

// deserializeVector 反序列化向量（bytes -> float32 数组）
func (idx *VectorIndex) deserializeVector(data []byte) []float32 {
	vector := make([]float32, len(data)/4)
	buf := bytes.NewReader(data)
	binary.Read(buf, binary.LittleEndian, &vector)
	return vector
}

// cosineSimilarity 计算余弦相似度
func (idx *VectorIndex) cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// sortByScore 按分数排序（降序）
func (idx *VectorIndex) sortByScore(matches []*entity.SkillMatch) {
	// 简单的冒泡排序（对于小数据集足够）
	n := len(matches)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if matches[j].Score < matches[j+1].Score {
				matches[j], matches[j+1] = matches[j+1], matches[j]
			}
		}
	}
}
