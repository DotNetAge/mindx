package rpc

import "encoding/json"

// UIOpenParams are the params for ui.open.
// Path 为 CLI 侧过闸后的绝对路径（工作区路径闸在 CLI 执行，daemon 信任并广播）。
type UIOpenParams struct {
	Path string `json:"path"`
}

// UIOpenLinkParams are the params for ui.open_link.
// URL 仅 http/https（CLI 侧校验）。
type UIOpenLinkParams struct {
	URL string `json:"url"`
}

// UIRunParams are the params for ui.run.
// Cwd 为 CLI 进程工作目录（Agent 工作区），客户端据此定位终端会话目录。
type UIRunParams struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd,omitempty"`
}

// UIOpen 受理「客户端打开文件」：广播 file_open，受理即返回不等客户端回执。
func (c *Client) UIOpen(path string) (json.RawMessage, error) {
	return c.CallWithTimeout("ui.open", UIOpenParams{Path: path})
}

// UIOpenLink 受理「客户端打开链接」：广播 link_open，受理即返回不等客户端回执。
func (c *Client) UIOpenLink(url string) (json.RawMessage, error) {
	return c.CallWithTimeout("ui.open_link", UIOpenLinkParams{URL: url})
}

// UIRun 受理「客户端终端执行命令」：广播 terminal_run，受理即返回不等客户端回执。
func (c *Client) UIRun(command, cwd string) (json.RawMessage, error) {
	return c.CallWithTimeout("ui.run", UIRunParams{Command: command, Cwd: cwd})
}
