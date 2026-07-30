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
