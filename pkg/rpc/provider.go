package rpc

import "encoding/json"

// ProviderCreateParams are the params for provider.create.
type ProviderCreateParams struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	AuthToken string `json:"auth_token,omitempty"`
	IsLocal   bool   `json:"is_local,omitempty"`
}

// ProviderUpdateParams are the params for provider.update.
type ProviderUpdateParams struct {
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	AuthToken string `json:"auth_token,omitempty"`
	IsLocal   *bool  `json:"is_local,omitempty"`
}

// ProviderDeleteParams are the params for provider.delete.
type ProviderDeleteParams struct {
	Name string `json:"name"`
}

// FetchOllamaModelsParams are the params for provider.fetch_ollama_models.
type FetchOllamaModelsParams struct {
	BaseURL string `json:"base_url"`
}

// OllamaModelInfo represents a single model from Ollama's /api/tags.
type OllamaModelInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
	CreatedAt string `json:"created_at"`
}

// FetchOllamaModelDetailParams are the params for provider.fetch_ollama_model_detail.
type FetchOllamaModelDetailParams struct {
	BaseURL   string `json:"base_url"`
	ModelName string `json:"model_name"`
}

// OllamaModelDetail represents model detail from Ollama's /api/show.
type OllamaModelDetail struct {
	Name           string `json:"name"`
	ContextLength  int64  `json:"context_length"`
	ParameterSize  string `json:"parameter_size"`
	Quantization   string `json:"quantization"`
	ModelFamily    string `json:"model_family"`
	ParameterCount int64  `json:"parameter_count"`
}

// FetchSiliconFlowModelsParams are the params for provider.fetch_siliconflow_models.
type FetchSiliconFlowModelsParams struct {
	Provider string `json:"provider"`
}

// FetchOpenRouterModelsParams are the params for provider.fetch_openrouter_models.
type FetchOpenRouterModelsParams struct {
	Provider string `json:"provider"`
}

// FetchDashScopeModelsParams are the params for provider.fetch_dashscope_models.
type FetchDashScopeModelsParams struct {
	Provider string `json:"provider"`
}

// FetchBigModelModelsParams are the params for provider.fetch_bigmodel_models.
type FetchBigModelModelsParams struct {
	Provider string `json:"provider"`
}

// FetchTencentModelsParams are the params for provider.fetch_tencent_models.
type FetchTencentModelsParams struct {
	Provider string `json:"provider"`
}

// OnlineModelInfo 是在线模型库的规范化模型信息，
// 供硅基流动 / OpenRouter / 阿里百炼 / 智谱 / 腾讯云 TokenHub 五个浏览对话框共用同一套渲染结构。
type OnlineModelInfo struct {
	ID            string  `json:"id"`
	Title         string  `json:"title,omitempty"`           // OpenRouter 展示名
	Description   string  `json:"description,omitempty"`     // OpenRouter 模型简介
	ContextLength int64   `json:"context_length,omitempty"`  // 上下文窗口（token）
	Free          bool    `json:"free"`                      // 是否免费（硅基流动 API 无价格信息，恒为 false）
	CostPer1MIn   float64 `json:"cost_per_1m_in,omitempty"`  // 输入成本 ¥/M tokens（付费模型已按实时汇率换算）
	CostPer1MOut  float64 `json:"cost_per_1m_out,omitempty"` // 输出成本 ¥/M tokens
	FuncCalling   bool    `json:"func_calling,omitempty"`    // 支持工具调用
	Visioning     bool    `json:"visioning,omitempty"`       // 支持视觉输入
	OwnedBy       string  `json:"owned_by,omitempty"`        // 模型归属方（如 deepseek-ai）
}

func (c *Client) ProviderList() (json.RawMessage, error) {
	return c.CallWithTimeout("provider.list", nil)
}

func (c *Client) ProviderCreate(params ProviderCreateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("provider.create", params)
}

func (c *Client) ProviderUpdate(params ProviderUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("provider.update", params)
}

func (c *Client) ProviderDelete(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("provider.delete", ProviderDeleteParams{Name: name})
}
