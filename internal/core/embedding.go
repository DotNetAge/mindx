package core

// EmbeddingProvider embedding提供者接口
type EmbeddingProvider interface {
	GenerateEmbedding(text string) ([]float64, error)
	GenerateBatchEmbeddings(texts []string) ([][]float64, error)
}
