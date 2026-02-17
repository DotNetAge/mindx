package embedding

import "strings"

// TFIDFEmbedding TF-IDF向量提供者（简化版，不依赖外部服务）
type TFIDFEmbedding struct {
	vocabulary    map[string]int // 词到索引的映射
	documentCount int            // 文档数量
	initialized   bool
}

// NewTFIDFEmbedding 创建TF-IDF embedding提供者
func NewTFIDFEmbedding() *TFIDFEmbedding {
	return &TFIDFEmbedding{
		vocabulary: make(map[string]int),
	}
}

// Train 训练TF-IDF模型（从文档集中构建词表）
func (t *TFIDFEmbedding) Train(documents []string) {
	if len(documents) == 0 {
		return
	}

	// 简单分词（按空格和标点分割）
	wordsSeen := make(map[string]bool)
	for _, doc := range documents {
		words := t.tokenize(doc)
		for _, word := range words {
			wordsSeen[word] = true
		}
	}

	// 构建词表
	idx := 0
	for word := range wordsSeen {
		t.vocabulary[word] = idx
		idx++
	}

	t.documentCount = len(documents)
	t.initialized = true
}

// GenerateEmbedding 生成TF-IDF向量
func (t *TFIDFEmbedding) GenerateEmbedding(text string) ([]float64, error) {
	if !t.initialized {
		// 未训练时，使用简单的字符频率向量
		return t.generateCharFrequencyVector(text), nil
	}

	words := t.tokenize(text)
	termFreq := make(map[string]int)

	// 计算词频
	for _, word := range words {
		termFreq[word]++
	}

	// 生成向量
	vector := make([]float64, len(t.vocabulary))
	for word, freq := range termFreq {
		if idx, exists := t.vocabulary[word]; exists {
			vector[idx] = float64(freq)
		}
	}

	// 归一化
	return t.normalize(vector), nil
}

// GenerateBatchEmbeddings 批量生成向量
func (t *TFIDFEmbedding) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := t.GenerateEmbedding(text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = vec
	}
	return embeddings, nil
}

// tokenize 简单分词
func (t *TFIDFEmbedding) tokenize(text string) []string {
	// 简化分词：按空格和常见标点分割
	var words []string
	word := ""
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '.' || r == '!' || r == '?' {
			if word != "" {
				words = append(words, strings.ToLower(word))
				word = ""
			}
		} else {
			word += string(r)
		}
	}
	if word != "" {
		words = append(words, strings.ToLower(word))
	}
	return words
}

// generateCharFrequencyVector 生成字符频率向量（fallback方案）
func (t *TFIDFEmbedding) generateCharFrequencyVector(text string) []float64 {
	vector := make([]float64, 256) // ASCII字符频率
	for _, r := range text {
		if int(r) < 256 {
			vector[int(r)]++
		}
	}
	return t.normalize(vector)
}

// normalize 向量归一化
func (t *TFIDFEmbedding) normalize(vector []float64) []float64 {
	var sum float64
	for _, v := range vector {
		sum += v * v
	}
	if sum == 0 {
		return vector
	}
	norm := sqrt(sum)
	for i, v := range vector {
		vector[i] = v / norm
	}
	return vector
}

// sqrt 计算平方根
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := 1.0
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}
