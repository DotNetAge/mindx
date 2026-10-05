package rpc

import "encoding/json"

// TeamGetParams are the params for team.get.
type TeamGetParams struct {
	Name string `json:"name"`
}

// TeamDeleteParams are the params for team.delete.
type TeamDeleteParams struct {
	Name string `json:"name"`
}

// TeamHireParams are the params for team.hire / team.fire.
type TeamHireParams struct {
	Name string `json:"name"`
}

// TeamExportParams are the params for team.export（整队招聘包）。
type TeamExportParams struct {
	Name    string `json:"name"`
	OutPath string `json:"out_path"`
}

// TeamImportParams are the params for team.import（本地安装团队包）。
type TeamImportParams struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// TeamUpdateParams are the params for team.update（创建与编辑共用，name 为 key）。
// 指针语义：nil 保持不变，非 nil 覆盖（含空串/空切片清空），与 AgentUpdateParams 同模式。
// Members/Leader 为纯引用列表（Agent 名）；Profile 为 TEAM.md 正文（团队人设）。
type TeamUpdateParams struct {
	Name string `json:"name"`
	// NickName 昵称（2-3 字外号，展示主名），与 Agent 命名三件套同原理。
	NickName    *string  `json:"nick_name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Leader      *string  `json:"leader,omitempty"`
	Members     []string `json:"members,omitempty"`
	Icon        *string  `json:"icon,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Profile     *string  `json:"profile,omitempty"`
	Hired       *bool    `json:"hired,omitempty"`
}

func (c *Client) TeamList() (json.RawMessage, error) {
	return c.CallWithTimeout("team.list", nil)
}

func (c *Client) TeamGet(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("team.get", TeamGetParams{Name: name})
}

func (c *Client) TeamUpdate(params TeamUpdateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("team.update", params)
}

func (c *Client) TeamDelete(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("team.delete", TeamDeleteParams{Name: name})
}

// TeamHire 雇佣团队（团队会话可选入口）。
func (c *Client) TeamHire(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("team.hire", TeamHireParams{Name: name})
}

// TeamFire 解雇团队（雇佣视图不再呈现，不影响已存在的会话）。
func (c *Client) TeamFire(name string) (json.RawMessage, error) {
	return c.CallWithTimeout("team.fire", TeamHireParams{Name: name})
}

// TeamExport 导出团队整包（team/TEAM.md + 成员 Agent 目录）。
func (c *Client) TeamExport(name, outPath string) (json.RawMessage, error) {
	return c.CallWithTimeout("team.export", TeamExportParams{Name: name, OutPath: outPath})
}

// TeamImport 从本地团队分发包安装（整队落位）。
func (c *Client) TeamImport(path string, overwrite bool) (json.RawMessage, error) {
	return c.CallWithTimeout("team.import", TeamImportParams{Path: path, Overwrite: overwrite})
}
