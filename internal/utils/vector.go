package utils

import (
	"math"
	"sort"

	"mindx/internal/entity"
)

// CalculateCosineSimilarity 计算余弦相似度
// 返回值范围: [-1, 1]，1表示完全相同，0表示正交，-1表示完全相反
func CalculateCosineSimilarity(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var dotProduct, norm1, norm2 float64
	for i := 0; i < len(vec1); i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// FindMostSimilar 找到最相似的向量
// topN: 返回前N个最相似的结果
func FindMostSimilar(queryVec []float64, candidates []entity.SimilarityResult, topN int) []entity.SimilarityResult {
	if len(candidates) == 0 {
		return []entity.SimilarityResult{}
	}

	type scoredResult struct {
		result entity.SimilarityResult
		score  float64
	}

	scored := make([]scoredResult, 0, len(candidates))
	for _, candidate := range candidates {
		if candidateVec, ok := candidate.Metadata["vector"].([]float64); ok {
			score := CalculateCosineSimilarity(queryVec, candidateVec)
			scored = append(scored, scoredResult{
				result: candidate,
				score:  score,
			})
		}
	}

	// 按分数降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if topN > len(scored) {
		topN = len(scored)
	}

	result := make([]entity.SimilarityResult, topN)
	for i := 0; i < topN; i++ {
		result[i] = scored[i].result
		result[i].Score = scored[i].score
	}

	return result
}
