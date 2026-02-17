package entity

import "encoding/json"

// VectorEntry 向量条目
type VectorEntry struct {
	Key      string          `json:"key"`
	Vector   []float64       `json:"vector"`
	Metadata json.RawMessage `json:"metadata"`
}
