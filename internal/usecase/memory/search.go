package memory

import (
	"encoding/json"
	"math"
	"mindx/internal/core"
	"mindx/internal/entity"
	apperrors "mindx/internal/errors"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"sort"
	"strings"
)

func (m *Memory) Search(terms string) ([]core.MemoryPoint, error) {
	m.logger.Debug(i18n.T("memory.start_search"), logging.String(i18n.T("memory.terms"), terms))

	if m.embeddingService == nil {
		return []core.MemoryPoint{}, nil
	}

	termVector, err := m.embeddingService.GenerateEmbedding(terms)
	if err != nil {
		m.logger.Error(i18n.T("memory.gen_search_vector_failed"), logging.Err(err))
		if m.store == nil {
			return []core.MemoryPoint{}, nil
		}
		entries, err := m.store.Search(nil, 10)
		if err != nil {
			m.logger.Error(i18n.T("memory.vector_search_failed"), logging.Err(err))
			return nil, apperrors.Wrap(err, apperrors.ErrTypeMemory, "向量搜索失败")
		}
		filtered := m.filterByKeywords(entries, terms)
		sorted := m.sortByWeight(filtered, 3)
		return sorted, nil
	}

	allMemories, err := m.getAllMemories()
	if err != nil {
		m.logger.Error(i18n.T("memory.get_memories_failed"), logging.Err(err))
		return nil, apperrors.Wrap(err, apperrors.ErrTypeMemory, "获取记忆点失败")
	}
	var candidatePoints []core.MemoryPoint
	for _, memoryPoint := range allMemories {
		if len(memoryPoint.Vector) == 0 || len(termVector) == 0 {
			continue
		}

		similarity := m.calculateCosineSimilarity(memoryPoint.Vector, termVector)
		if similarity >= 0.5 {
			candidatePoints = append(candidatePoints, memoryPoint)
		}
	}

	var candidateEntries []entity.VectorEntry
	for _, point := range candidatePoints {
		metadataBytes, err := json.Marshal(point)
		if err != nil {
			continue
		}
		candidateEntries = append(candidateEntries, entity.VectorEntry{
			Metadata: metadataBytes,
		})
	}

	filteredPoints := m.filterByKeywords(candidateEntries, terms)
	m.logger.Debug(i18n.T("memory.after_filter"), logging.Int(i18n.T("memory.count"), len(filteredPoints)))

	sorted := m.sortByWeight(filteredPoints, 3)

	m.logger.Info(i18n.T("memory.search_complete"), logging.Int(i18n.T("memory.found"), len(sorted)))

	return sorted, nil
}

func (m *Memory) filterByKeywords(entries []entity.VectorEntry, terms string) []core.MemoryPoint {
	var filtered []core.MemoryPoint
	termsLower := strings.ToLower(terms)

	for _, entry := range entries {
		var point core.MemoryPoint
		if err := json.Unmarshal(entry.Metadata, &point); err != nil {
			continue
		}

		similarity := m.calculateKeywordSimilarity(point.Keywords, termsLower)
		if similarity > 0.6 {
			filtered = append(filtered, point)
		}
	}

	return filtered
}

func (m *Memory) calculateKeywordSimilarity(keywords []string, terms string) float64 {
	if len(keywords) == 0 {
		return 0
	}

	matchCount := 0
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if strings.Contains(terms, kwLower) || strings.Contains(kwLower, terms) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(keywords))
}

func (m *Memory) sortByWeight(points []core.MemoryPoint, topN int) []core.MemoryPoint {
	if len(points) == 0 {
		return []core.MemoryPoint{}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].TotalWeight > points[j].TotalWeight
	})

	if topN > len(points) {
		topN = len(points)
	}

	return points[:topN]
}

func (m *Memory) calculateCosineSimilarity(vec1, vec2 []float64) float64 {
	var dotProduct, norm1, norm2 float64
	for i := range vec1 {
		if i >= len(vec2) {
			break
		}
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func (m *Memory) simpleTokenize(text string) []string {
	tokens := strings.Fields(text)
	var keywords []string

	for _, token := range tokens {
		token = strings.Trim(token, "，。！？、；：\"\"''（）")
		if len(token) >= 2 {
			keywords = append(keywords, token)
		}
	}

	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	return keywords
}
