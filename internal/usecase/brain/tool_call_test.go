package brain_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestOllamaToolCall(t *testing.T) {
	baseURL := "http://localhost:11434/v1"
	apiKey := "ollama"
	model := "qwen3:0.6b"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "weather",
				Description: "天气查询技能，查询全球城市天气信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type":        "string",
							"description": "城市名称",
						},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	t.Log("=== Step 1: 发送工具调用请求 ===")
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个工具调用助手。根据用户的请求，从可用的工具中选择合适的工具并调用。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "明天广州的天气如何",
			},
		},
		Tools:      tools,
		ToolChoice: "auto",
	}

	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	t.Logf("请求内容:\n%s", string(reqJSON))

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("响应内容:\n%s", string(respJSON))

	if len(resp.Choices) == 0 {
		t.Fatal("没有返回 choices")
	}

	choice := resp.Choices[0]
	t.Logf("Content: %s", choice.Message.Content)
	t.Logf("ToolCalls count: %d", len(choice.Message.ToolCalls))
	t.Logf("FunctionCall: %v", choice.Message.FunctionCall)
	t.Logf("ReasoningContent: %s", choice.Message.ReasoningContent)

	if len(choice.Message.ToolCalls) == 0 {
		if choice.Message.FunctionCall != nil {
			t.Logf("使用旧格式 FunctionCall: %s", choice.Message.FunctionCall.Name)
		} else {
			t.Fatal("模型没有调用工具！")
		}
	} else {
		toolCall := choice.Message.ToolCalls[0]
		t.Logf("工具调用成功! Function: %s, Arguments: %s", toolCall.Function.Name, toolCall.Function.Arguments)

		t.Log("\n=== Step 2: 回传工具结果 ===")
		messages := append(req.Messages, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    "广州明天天气：晴转多云，气温 18-25°C，空气质量良好",
		})

		req2 := openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		}

		resp2, err := client.CreateChatCompletion(context.Background(), req2)
		if err != nil {
			t.Fatalf("回传结果失败: %v", err)
		}

		resp2JSON, _ := json.MarshalIndent(resp2, "", "  ")
		t.Logf("最终响应:\n%s", string(resp2JSON))

		if len(resp2.Choices) > 0 {
			t.Logf("最终答案: %s", resp2.Choices[0].Message.Content)
		}
	}
}

func TestOllamaDirectAPI(t *testing.T) {
	ollamaURL := "http://localhost:11434/api/chat"

	reqBody := map[string]interface{}{
		"model": "qwen3:0.6b",
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个工具调用助手。根据用户的请求，从可用的工具中选择合适的工具并调用。"},
			{"role": "user", "content": "明天广州的天气如何"},
		},
		"stream": false,
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "weather",
					"description": "天气查询技能",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"city": map[string]interface{}{
								"type":        "string",
								"description": "城市名称",
							},
						},
						"required": []string{"city"},
					},
				},
			},
		},
	}

	reqJSON, _ := json.Marshal(reqBody)
	t.Logf("直接请求 Ollama:\n%s", string(reqJSON))

	resp, err := http.Post(ollamaURL, "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Ollama 响应:\n%s", string(resultJSON))
}

func TestOllamaViaOpenAIClient(t *testing.T) {
	baseURL := "http://localhost:11434/v1"

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("解析URL失败: %v", err)
	}
	t.Logf("BaseURL: %s, Host: %s", parsedURL.String(), parsedURL.Host)

	config := openai.DefaultConfig("ollama")
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	req := openai.ChatCompletionRequest{
		Model: "qwen3:0.6b",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello",
			},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("简单请求失败: %v", err)
	}

	t.Logf("简单请求成功: %s", resp.Choices[0].Message.Content)
}
