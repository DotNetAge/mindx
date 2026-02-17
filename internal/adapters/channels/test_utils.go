package channels

import (
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// mockEmbeddingService 创建模拟的向量化服务
func mockEmbeddingService() *embedding.EmbeddingService {
	provider := &mockEmbeddingProvider{
		vectors: make(map[string][]float64),
	}
	return embedding.NewEmbeddingService(provider)
}

// mockEmbeddingProvider 模拟的向量化服务提供者
type mockEmbeddingProvider struct {
	vectors map[string][]float64
	mu      sync.RWMutex
}

func (m *mockEmbeddingProvider) GenerateEmbedding(text string) ([]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vec, exists := m.vectors[text]; exists {
		return vec, nil
	}

	vec := make([]float64, 5)
	for i := range vec {
		vec[i] = rand.Float64()
	}
	m.vectors[text] = vec
	return vec, nil
}

func (m *mockEmbeddingProvider) GenerateBatchEmbeddings(texts []string) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := m.GenerateEmbedding(text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}
	return vectors, nil
}

// createTestMessage 创建测试消息
func createTestMessage(channelID, sessionID, content string) *entity.IncomingMessage {
	return &entity.IncomingMessage{
		ChannelID:   channelID,
		ChannelName: channelID,
		SessionID:   sessionID,
		MessageID:   fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Sender: &entity.MessageSender{
			ID:   "user1",
			Name: "Test User",
			Type: "user",
		},
		Content:     content,
		ContentType: "text",
		Timestamp:   time.Now(),
	}
}

// waitForMessage 等待消息被处理
func waitForMessage(channel *MockChannel, expectedCount int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(channel.GetSentMessages()) >= expectedCount {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
