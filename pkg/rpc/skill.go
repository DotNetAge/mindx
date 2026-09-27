package rpc

import "encoding/json"

// SkillListParams are the params for skill.list.
// 作用域（PR-PROMPTS 三级库）：ProjectDir 非空 → 项目级库发现（晋升界面展示）；
// AgentName 非空 → Agent 级私有库；均为空 → 全局库。
type SkillListParams struct {
	AgentName  string `json:"agent_name,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
}

// SkillPromoteParams are the params for skill.promote.
// 晋升路径：项目级 → Agent 级 / 全局级、Agent 级 → 全局级（用户裁决）。
type SkillPromoteParams struct {
	Name       string `json:"name"`
	From       string `json:"from"` // "agent" | "project"
	To         string `json:"to"`   // "global" | "agent"
	AgentName  string `json:"agent_name,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
	Overwrite  bool   `json:"overwrite,omitempty"`
}

func (c *Client) SkillList(agentName string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.list", SkillListParams{AgentName: agentName})
}

func (c *Client) SkillPromote(name, from, to string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.promote", SkillPromoteParams{Name: name, From: from, To: to})
}

// SkillGetParams are the params for skill.get.
type SkillGetParams struct {
	Name string `json:"name"`
}

// SkillDeleteParams are the params for skill.delete.
type SkillDeleteParams struct {
	Name string `json:"name"`
}

func (c *Client) SkillGet(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.get", SkillGetParams{Name: name})
}

func (c *Client) SkillDelete(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.delete", SkillDeleteParams{Name: name})
}

func (c *Client) SkillReload() (json.RawMessage, error) {
	return c.CallWithTimeout("skill.reload", nil)
}
