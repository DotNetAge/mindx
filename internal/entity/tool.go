package entity

// Tool 工具定义
type Tool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Parameters  map[string]string `json:"parameters"`
	Vector      []float64         `json:"vector,omitempty"`
}
