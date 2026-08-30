package rpc

import "encoding/json"

// ModelGetParams are the params for model.get.
type ModelGetParams struct {
	Name string `json:"name"`
	// Provider 可选：跨供应商存在同名模型时用于按组合键（Provider/Name）精确寻址。
	Provider string `json:"provider,omitempty"`
}

// ModelSwitchParams are the params for model.switch.
type ModelSwitchParams struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

// ModelCreateParams are the params for model.create.
type ModelCreateParams struct {
	Name          string  `json:"name"`
	Title         string  `json:"title"`
	Description   string  `json:"description,omitempty"`
	Provider      string  `json:"provider"`
	BaseURL       string  `json:"base_url,omitempty"`
	APIKey        string  `json:"api_key,omitempty"`
	AuthToken     string  `json:"auth_token,omitempty"`
	ContextLength int64   `json:"context_length,omitempty"`
	IsLocal       bool    `json:"is_local,omitempty"`
	FuncCalling   bool    `json:"func_calling,omitempty"`
	Structuring   bool    `json:"structuring,omitempty"`
	WebSearching  bool    `json:"web_searching,omitempty"`
	Visioning     bool    `json:"visioning,omitempty"`
	PrefixCon     bool    `json:"prefix_con,omitempty"`
	ContextCache  bool    `json:"context_cache,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	Enabled       bool    `json:"enabled,omitempty"`
	MaxTurns      int     `json:"max_turns,omitempty"`
	CostPer1MIn   float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut  float64 `json:"cost_per_1m_out,omitempty"`
	// RequestTimeout 单次 LLM 调用最大等待时长（秒），0 表示未配置。
	RequestTimeout int64 `json:"request_timeout,omitempty"`
}

// ModelUpdateParams are the params for model.update.
type ModelUpdateParams struct {
	Name          string   `json:"name"`
	Title         string   `json:"title,omitempty"`
	Description   string   `json:"description,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	BaseURL       string   `json:"base_url,omitempty"`
	APIKey        string   `json:"api_key,omitempty"`
	AuthToken     string   `json:"auth_token,omitempty"`
	ContextLength *int64   `json:"context_length,omitempty"`
	IsLocal       *bool    `json:"is_local,omitempty"`
	FuncCalling   *bool    `json:"func_calling,omitempty"`
	Structuring   *bool    `json:"structuring,omitempty"`
	WebSearching  *bool    `json:"web_searching,omitempty"`
	Visioning     *bool    `json:"visioning,omitempty"`
	PrefixCon     *bool    `json:"prefix_con,omitempty"`
	ContextCache  *bool    `json:"context_cache,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
	MaxTurns      *int     `json:"max_turns,omitempty"`
	CostPer1MIn   *float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut  *float64 `json:"cost_per_1m_out,omitempty"`
	// RequestTimeout 单次 LLM 调用最大等待时长（秒），nil 表示不修改。
	RequestTimeout *int64 `json:"request_timeout,omitempty"`
}

// ModelDeleteParams are the params for model.delete.
type ModelDeleteParams struct {
	Name string `json:"name"`
	// Provider 可选：跨供应商存在同名模型时用于按组合键（Provider/Name）精确寻址。
	Provider string `json:"provider,omitempty"`
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
