package rpc

import "encoding/json"

// ModelGetParams are the params for model.get.
type ModelGetParams struct {
	Name string `json:"name"`
}

// ModelSwitchParams are the params for model.switch.
type ModelSwitchParams struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

// ModelCreateParams are the params for model.create.
type ModelCreateParams struct {
	Name              string  `json:"name"`
	Title             string  `json:"title"`
	Description       string  `json:"description,omitempty"`
	Provider          string  `json:"provider"`
	BaseURL           string  `json:"base_url,omitempty"`
	APIKey            string  `json:"api_key,omitempty"`
	AuthToken         string  `json:"auth_token,omitempty"`
	MaxTokens         int64   `json:"max_tokens,omitempty"`
	ContextLength     int64   `json:"context_length,omitempty"`
	IsLocal           bool    `json:"is_local,omitempty"`
	FuncCalling       bool    `json:"func_calling,omitempty"`
	Structuring       bool    `json:"structuring,omitempty"`
	WebSearching      bool    `json:"web_searching,omitempty"`
	PrefixCon         bool    `json:"prefix_con,omitempty"`
	ContextCache      bool    `json:"context_cache,omitempty"`
	TopP              float64 `json:"top_p,omitempty"`
	TopK              float64 `json:"top_k,omitempty"`
	Temperature       float64 `json:"temperature,omitempty"`
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
	FrequencyPenalty  float64 `json:"frequency_penalty,omitempty"`
	Enabled           bool    `json:"enabled,omitempty"`
	MaxTurns          int     `json:"max_turns,omitempty"`
	CostPer1MIn       float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut      float64 `json:"cost_per_1m_out,omitempty"`
}

// ModelUpdateParams are the params for model.update.
type ModelUpdateParams struct {
	Name              string   `json:"name"`
	Title             string   `json:"title,omitempty"`
	Description       string   `json:"description,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	BaseURL           string   `json:"base_url,omitempty"`
	APIKey            string   `json:"api_key,omitempty"`
	AuthToken         string   `json:"auth_token,omitempty"`
	MaxTokens         *int64   `json:"max_tokens,omitempty"`
	ContextLength     *int64   `json:"context_length,omitempty"`
	IsLocal           *bool    `json:"is_local,omitempty"`
	FuncCalling       *bool    `json:"func_calling,omitempty"`
	Structuring       *bool    `json:"structuring,omitempty"`
	WebSearching      *bool    `json:"web_searching,omitempty"`
	PrefixCon         *bool    `json:"prefix_con,omitempty"`
	ContextCache      *bool    `json:"context_cache,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	TopK              *float64 `json:"top_k,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
	FrequencyPenalty  *float64 `json:"frequency_penalty,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty"`
	MaxTurns          *int     `json:"max_turns,omitempty"`
	CostPer1MIn       *float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut      *float64 `json:"cost_per_1m_out,omitempty"`
}

// ModelDeleteParams are the params for model.delete.
type ModelDeleteParams struct {
	Name string `json:"name"`
}

func (c *Client) ModelList() (json.RawMessage, error) {
	return c.CallWithTimeout("model.list", nil)
}

func (c *Client) ModelGet(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("model.get", ModelGetParams{Name: name})
}

func (c *Client) ModelSwitch(name, provider string) (json.RawMessage, error) {
	return c.CallWithTimeout("model.switch", ModelSwitchParams{Name: name, Provider: provider})
}

func (c *Client) ModelCreate(params ModelCreateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("model.create", params)
}

func (c *Client) ModelUpdate(params ModelUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("model.update", params)
}

func (c *Client) ModelDelete(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("model.delete", ModelDeleteParams{Name: name})
}
