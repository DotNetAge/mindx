package rpc

import "encoding/json"

// RuleGetParams are the params for rule.get.
type RuleGetParams struct {
	ID string `json:"id"`
}

// RuleCreateParams are the params for rule.create.
type RuleCreateParams struct {
	ID       string `json:"id"`
	Intro    string `json:"intro"`
	Scope    string `json:"scope,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
}

// RuleUpdateParams are the params for rule.update.
type RuleUpdateParams struct {
	ID       string  `json:"id"`
	Intro    *string `json:"intro,omitempty"`
	Scope    *string `json:"scope,omitempty"`
	Priority *int    `json:"priority,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
}

// RuleDeleteParams are the params for rule.delete.
type RuleDeleteParams struct {
	ID string `json:"id"`
}

func (c *Client) RuleList() (json.RawMessage, error) {
	return c.CallWithTimeout("rule.list", nil)
}

func (c *Client) RuleGet(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("rule.get", RuleGetParams{ID: id})
}

func (c *Client) RuleCreate(params RuleCreateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("rule.create", params)
}

func (c *Client) RuleUpdate(params RuleUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("rule.update", params)
}

func (c *Client) RuleDelete(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("rule.delete", RuleDeleteParams{ID: id})
}
