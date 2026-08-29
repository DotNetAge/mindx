package rpc

import "encoding/json"

// SkillListParams are the params for skill.list.
type SkillListParams struct {
	AgentName string `json:"agent_name,omitempty"`
}

// SkillGetParams are the params for skill.get.
type SkillGetParams struct {
	Name      string `json:"name"`
	AgentName string `json:"agent_name,omitempty"`
}

// SkillDeleteParams are the params for skill.delete.
type SkillDeleteParams struct {
	Name      string `json:"name"`
	AgentName string `json:"agent_name,omitempty"`
}

func (c *Client) SkillList(agentName string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.list", SkillListParams{AgentName: agentName})
}

func (c *Client) SkillGet(name, agentName string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.get", SkillGetParams{Name: name, AgentName: agentName})
}

func (c *Client) SkillDelete(name, agentName string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.delete", SkillDeleteParams{Name: name, AgentName: agentName})
}

func (c *Client) SkillReload() (json.RawMessage, error) {
	return c.CallWithTimeout("skill.reload", nil)
}
