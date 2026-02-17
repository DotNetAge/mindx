package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaEmbeddingRequest Ollama embedding请求
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse Ollama embedding响应
type OllamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// OllamaEmbedding Ollama embedding提供者
type OllamaEmbedding struct {
	client  *http.Client
	baseURL string
	model   string
	timeout time.Duration
}

// NewOllamaEmbedding 创建Ollama embedding提供者
func NewOllamaEmbedding(baseURL, model string) (*OllamaEmbedding, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text" // Ollama默认embedding模型
	}

	// 移除 /v1 后缀，因为 embedding API 不需要它
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL[:len(baseURL)-3]
	}

	return &OllamaEmbedding{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		model:   model,
		timeout: 30 * time.Second,
	}, nil
}

// GenerateEmbedding 生成单个文本的embedding
func (o *OllamaEmbedding) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 请求体
	reqBody := OllamaEmbeddingRequest{
		Model:  o.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	url := fmt.Sprintf("%s/api/embeddings", o.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应 - Ollama 的 /api/embed 返回直接的数组
	// 尝试两种格式：{"embedding": []} 和直接的 []

	// 先尝试标准格式
	var respBody OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err == nil && len(respBody.Embedding) > 0 {
		return respBody.Embedding, nil
	}

	// 如果标准格式失败，尝试直接解析为数组
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewBuffer(jsonBody))
	var directArray []float64
	if err := json.NewDecoder(resp.Body).Decode(&directArray); err == nil && len(directArray) > 0 {
		return directArray, nil
	}

	return nil, fmt.Errorf("empty embedding returned")
}

// GenerateBatchEmbeddings 批量生成embedding
func (o *OllamaEmbedding) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	embeddings := make([][]float64, 0, len(texts))

	for _, text := range texts {
		embedding, err := o.GenerateEmbedding(text)
		if err != nil {
			// 单个失败不影响其他
			continue
		}
		embeddings = append(embeddings, embedding)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("failed to generate any embeddings")
	}

	return embeddings, nil
}
