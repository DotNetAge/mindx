package llama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"mindx/pkg/llama"
)

// OllamaService Ollama 服务实现
type OllamaService struct {
	model   string
	baseURL string
	client  *http.Client
}

// NewOllamaService 创建 Ollama 服务
// model: 模型名称，如 "llama2", "mistral", "qwen2" 等
// 默认 base URL 为 http://localhost:11434
func NewOllamaService(model string) *OllamaService {
	return &OllamaService{
		model:   model,
		baseURL: "http://localhost:11434",
		client:  &http.Client{},
	}
}

// WithBaseUrl 设置自定义 base URL
func (o *OllamaService) WithBaseUrl(url string) *OllamaService {
	o.baseURL = url
	return o
}

// Chat 单轮对话
func (o *OllamaService) Chat(question string) (string, error) {
	return o.MultipleChat([]llama.LlamaMessage{
		{Role: "user", Content: question},
	})
}

// MultipleChat 多轮对话
func (o *OllamaService) MultipleChat(messages []llama.LlamaMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("消息不能为空")
	}

	reqBody := map[string]any{
		"model":  o.model,
		"stream": false,
		"options": map[string]any{
			"temperature":     0.7,
			"enable_thinking": true,
		},
		"messages": messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	resp, err := o.client.Post(
		fmt.Sprintf("%s/api/chat", o.baseURL),
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误: 状态码 %d, 响应: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	message, ok := result["message"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("响应格式错误: 缺少 message")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("响应格式错误: 缺少 content")
	}

	return content, nil
}

// ChatWithAgent 带 system prompt 的对话
func (o *OllamaService) ChatWithAgent(agent string, question string) (string, error) {
	return o.MultipleChat([]llama.LlamaMessage{
		{Role: "system", Content: agent},
		{Role: "user", Content: question},
	})
}
