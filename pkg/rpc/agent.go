package rpc

import "encoding/json"

// AgentCreateParams are the params for agent.create.
type AgentCreateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Description  string         `json:"description"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model"`
	Skills       []string       `json:"skills,omitempty"`
	Body         string         `json:"body,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

// AgentGetParams are the params for agent.get.
type AgentGetParams struct {
	Name string `json:"name"`
}

// AgentScoreParams are the params for agent.score.
type AgentScoreParams struct {
	AgentName string `json:"agent_name"`
	Task      string `json:"task"`
	Score     int    `json:"score"`
	Notes     string `json:"notes,omitempty"`
}

// AgentUpdateParams are the params for agent.update.
type AgentUpdateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role,omitempty"`
	Description  string         `json:"description,omitempty"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model,omitempty"`
	Skills       []string       `json:"skills,omitempty"`
	ExcludeTools []string       `json:"exclude_tools,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (c *Client) AgentList() (json.RawMessage, error) {
	return c.CallWithTimeout("agent.list", nil)
}

func (c *Client) AgentGet(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.get", AgentGetParams{Name: name})
}

func (c *Client) AgentCreate(params AgentCreateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.create", params)
}

func (c *Client) AgentScore(params AgentScoreParams) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.score", params)
}

func (c *Client) AgentUpdate(params AgentUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.update", params)
}

func (c *Client) AgentReload() (json.RawMessage, error) {
	return c.CallWithTimeout("agent.reload", nil)
}
