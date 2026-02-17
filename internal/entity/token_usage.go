package entity

import "time"

// TokenUsage 记录 Token 消耗
type TokenUsage struct {
	ID               int       `json:"id"`
	Model            string    `json:"model"`
	Duration         int64     `json:"duration"`         // 模型执行总时长(毫秒)
	CompletionTokens int       `json:"completion_tokens"` // 补全 Token 数
	TotalTokens      int       `json:"total_tokens"`     // 总 Token 数
	PromptTokens     int       `json:"prompt_tokens"`    // 提示 Token 数
	CreatedAt        time.Time `json:"created_at"`
}

// TokenUsageSummary Token 使用汇总
type TokenUsageSummary struct {
	TotalRequests         int64   `json:"total_requests"`
	TotalDuration         int64   `json:"total_duration"`         // 总时长(毫秒)
	AvgDurationPerRequest float64 `json:"avg_duration_per_request"` // 平均时长(毫秒)
	TotalTokens           int64   `json:"total_tokens"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64   `json:"total_completion_tokens"`
	AvgTokensPerRequest   float64 `json:"avg_tokens_per_request"`
}

// TokenUsageByModelSummary 按模型分组的 Token 使用统计
type TokenUsageByModelSummary struct {
	Model                 string  `json:"model"`
	TotalRequests         int64   `json:"total_requests"`
	TotalDuration         int64   `json:"total_duration"`         // 总时长(毫秒)
	AvgDurationPerRequest float64 `json:"avg_duration_per_request"` // 平均时长(毫秒)
	TotalTokens           int64   `json:"total_tokens"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64   `json:"total_completion_tokens"`
	AvgTokensPerRequest   float64 `json:"avg_tokens_per_request"`
}
