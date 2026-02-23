package memory

import (
	"context"
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func (m *Memory) generateSummary(text string) (string, error) {
	if m.llmClient == nil {
		if len(text) > 200 {
			return text[:200] + "...", nil
		}
		return text, nil
	}

	resp, err := m.llmClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: m.summaryModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "请将以下对话内容精炼成一段简洁的摘要，保留关键信息：",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return text, nil
}
func (m *Memory) generateKeywords(text string) ([]string, error) {
	if m.llmClient == nil {
		return m.simpleTokenize(text), nil
	}

	resp, err := m.llmClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: m.keywordModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "请从以下对话内容中提取 3-5 个最重要的关键词：",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
					Name: "keywords",
					Schema: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"keywords": {
								Type: jsonschema.Array,
								Items: &jsonschema.Definition{
									Type: jsonschema.String,
								},
							},
						},
						Required: []string{"keywords"},
					},
					Strict: true,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	if len(resp.Choices) > 0 {
		var result struct {
			Keywords []string `json:"keywords"`
		}
		if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err == nil {
			return result.Keywords, nil
		}
	}

	return m.simpleTokenize(text), nil
}
