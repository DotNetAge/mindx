package entity

// SimilarityResult 相似度结果
type SimilarityResult struct {
	Target   string                 `json:"target"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
