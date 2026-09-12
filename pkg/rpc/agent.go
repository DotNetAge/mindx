package rpc

import "encoding/json"

// AgentCreateParams are the params for agent.create.
type AgentCreateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Description  string         `json:"description"`
	Introduction string         `json:"introduction,omitempty"`
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

// AgentHireParams are the params for agent.hire / agent.fire.
type AgentHireParams struct {
	Name string `json:"name"`
}

// AgentUpdateParams are the params for agent.update.
type AgentUpdateParams struct {
	Name         string `json:"name"`
	Role         string `json:"role,omitempty"`
	Description  string `json:"description,omitempty"`
	Introduction string `json:"introduction,omitempty"`
	// Soul / IdentityBody 用指针区分「未传」与「清空」：nil 保持不变，非 nil 覆盖（含空串清空）。
	// IdentityBody 对应 IDENTITY.md 身份正文（写入后与 frontmatter introduction 保持一致），
	// Soul 对应 SOUL.md 正文。
	Soul         *string  `json:"soul,omitempty"`
	IdentityBody *string  `json:"identity_body,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	ExcludeTools []string `json:"exclude_tools,omitempty"`
	// AllowsTools 云技能（MCP 服务）清单，条目 "mcp:<server>"；该属性放的全是 MCP 工具。
	// 指针区分「未传」与「清空」：nil 保持不变，非 nil 全量覆盖（含空切片清空）。
	AllowsTools *[]string      `json:"allows_tools,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
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

// AgentHire 雇佣 Agent（写入 meta.hired=true，雇佣后对会话可用）。
func (c *Client) AgentHire(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.hire", AgentHireParams{Name: name})
}

// AgentFire 解雇 Agent（写入 meta.hired=false，解雇后对会话不可用）。
func (c *Client) AgentFire(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.fire", AgentHireParams{Name: name})
}

func (c *Client) AgentUpdate(params AgentUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.update", params)
}

func (c *Client) AgentReload() (json.RawMessage, error) {
	return c.CallWithTimeout("agent.reload", nil)
}
