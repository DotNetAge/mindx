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
