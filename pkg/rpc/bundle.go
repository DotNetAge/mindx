package rpc

import "encoding/json"

// AgentExportParams are the params for agent.export（导出 Agent 分发包到指定路径）。
type AgentExportParams struct {
	Name    string `json:"name"`
	OutPath string `json:"out_path"`
}

// AgentImportParams are the params for agent.import（本地安装 Agent 分发包）。
type AgentImportParams struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// SkillExportParams are the params for skill.export（导出 Skill 分发包到指定路径）。
type SkillExportParams struct {
	Name    string `json:"name"`
	OutPath string `json:"out_path"`
}

// SkillImportParams are the params for skill.import（本地安装 Skill 分发包）。
type SkillImportParams struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// MarketListParams are the params for market.list。
type MarketListParams struct{}

// MarketInstallParams are the params for market.install（下载市场分发包并安装）。
type MarketInstallParams struct {
	Kind string `json:"kind"` // "agent" | "skill"
	Name string `json:"name"`
	// Overwrite 目标已存在时覆盖安装（前端二次确认后才携带）。
	Overwrite bool `json:"overwrite,omitempty"`
}

func (c *Client) AgentExport(name, outPath string) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.export", AgentExportParams{Name: name, OutPath: outPath})
}

func (c *Client) AgentImport(path string, overwrite bool) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.import", AgentImportParams{Path: path, Overwrite: overwrite})
}

func (c *Client) SkillExport(name, outPath string) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.export", SkillExportParams{Name: name, OutPath: outPath})
}

func (c *Client) SkillImport(path string, overwrite bool) (json.RawMessage, error) {
	return c.CallWithTimeout("skill.import", SkillImportParams{Path: path, Overwrite: overwrite})
}

func (c *Client) MarketList() (json.RawMessage, error) {
	return c.CallWithTimeout("market.list", MarketListParams{})
}

func (c *Client) MarketInstall(kind, name string) (json.RawMessage, error) {
	return c.CallWithTimeout("market.install", MarketInstallParams{Kind: kind, Name: name})
}
