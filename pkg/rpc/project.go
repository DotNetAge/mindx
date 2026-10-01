package rpc

import "encoding/json"

// ProjectListParams are the params for project.list（当前无过滤参数，预留扩展位）。
type ProjectListParams struct{}

// ProjectList 列出系统中的全部项目：枚举所有工作目录分片的会话后按 project_dir 去重聚合。
func (c *Client) ProjectList() (json.RawMessage, error) {
	return c.CallWithTimeout("project.list", ProjectListParams{})
}
