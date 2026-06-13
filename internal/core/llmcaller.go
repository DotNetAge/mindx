package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gochat "github.com/DotNetAge/gochat"
	chatcore "github.com/DotNetAge/gochat/core"
	harness "github.com/DotNetAge/goharness/config"
)

// LLMResult 封装一次 LLM 调用的返回结果，包含响应文本和 Token 消耗。
type LLMResult struct {
	// Tokens 记录了本次调用的 Token 使用统计（输入/输出/总计）。
	Tokens chatcore.Usage

	// Result 是 LLM 返回的响应文本内容。
	Result string
}

// Json 尝试将 Result 作为 JSON 反序列化并返回解析后的值。
// 如果 Result 不是合法 JSON，则原样返回字符串。
func (l *LLMResult) Json() any {
	var data any
	if err := json.Unmarshal([]byte(l.Result), &data); err != nil {
		return l.Result
	}
	return data
}

// LLMCaller 定义了可执行的 LLM 行为接口。
// 实现该接口的类型可以接收一组消息并调用 LLM 返回结果。
type LLMCaller interface {
	// Call 执行 LLM 调用，messages 为连续的用户消息序列。
	// 返回 LLMResult 或在出错时返回 error。
	Call(messages ...string) (LLMResult, error)
}

// llmCaller 是 Executable 的默认实现，封装了一次 LLM 调用的完整配置。
type llmCaller struct {
	// sysPrompt 是系统提示词，作为 SystemMessage 发送给 LLM。
	sysPrompt string

	// config 是模型配置，包含 API 连接参数和生成参数。
	config *harness.ModelConfig
}

// NewCaller 创建并返回一个 Executable 实例。
//
// cfg 为模型配置（API Key、BaseURL、模型名称、生成参数等），
// systemPrompt 为系统提示词，在每次调用时作为首条 SystemMessage 发送。
func NewCaller(cfg *harness.ModelConfig, systemPrompt string) LLMCaller {
	return &llmCaller{
		sysPrompt: systemPrompt,
		config:    cfg,
	}
}

// Call 执行一次 LLM 非流式调用。
//
// 构造的消息序列为：SystemMessage(sysPrompt) + UserMessage(messages[0]) + ...
// 使用 4 分钟超时的 context，调用 gochat.Client().GetResponse() 获取响应。
//
// 返回的 LLMResult 包含响应文本和 Token 使用统计（如果 LLM 返回了 usage 数据）。
func (b *llmCaller) Call(messages ...string) (LLMResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	chatMsgs := []chatcore.Message{
		chatcore.NewSystemMessage(b.sysPrompt),
	}
	for _, msg := range messages {
		chatMsgs = append(chatMsgs, chatcore.NewUserMessage(msg))
	}

	builder := gochat.Client().
		Config(
			gochat.WithAPIKey(b.config.APIKey),
			gochat.WithBaseURL(b.config.BaseURL),
			gochat.WithTimeout(4*time.Minute),
		).
		WithContext(ctx).
		Messages(chatMsgs...).
		Model(b.config.Name).
		MaxTokens(int(b.config.MaxTokens))

	if b.config.Temperature > 0 {
		builder = builder.Temperature(b.config.Temperature)
	}
	if b.config.TopP > 0 {
		builder = builder.TopP(b.config.TopP)
	}
	if b.config.TopK > 0 {
		builder = builder.TopK(int(b.config.TopK))
	}
	if b.config.RepetitionPenalty != 0 {
		builder = builder.PresencePenalty(b.config.RepetitionPenalty)
	}
	if b.config.FrequencyPenalty != 0 {
		builder = builder.FrequencyPenalty(b.config.FrequencyPenalty)
	}

	resp, err := builder.GetResponse()
	if err != nil {
		return LLMResult{}, fmt.Errorf("behavior: %w", err)
	}

	result := LLMResult{
		Result: resp.Content,
	}
	if resp.Usage != nil {
		result.Tokens = *resp.Usage
	}

	return result, nil
}
