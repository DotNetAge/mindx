package tools

import (
	"context"

	"github.com/DotNetAge/goharness/tools"
)

// ExecuteFunc 是动态工具的执行签名，与 tools.FuncTool.Execute 完全对齐。
type ExecuteFunc func(ctx context.Context, params map[string]any) (any, error)

// DynamicTool 是一个闭包容器，实现 tools.FuncTool 接口。
//
// 它不固化任何属性或行为：ToolInfo 和 Execute 均由外部注入，
// 用于把运行时动态发现的工具（如 MCP server 暴露的工具、插件远程工具等）
// 注册进 goharness 的 ToolRegistry，而无需为每种来源写一个新的 FuncTool 实现。
//
// Usage:
//
//	tool := tools.NewDynamic(info, func(ctx, p) (any, error) {
//	    return mcpClient.Call(ctx, "get_weather", p)
//	})
//	rt.RegisterTool(tool)
type DynamicTool struct {
	info *tools.ToolInfo
	fn   ExecuteFunc
}

// NewDynamic 构建一个动态工具。info 必须提供 Name 和 Description；fn 为 nil 时
// Execute 返回 "not implemented" 错误（适合仅暴露 schema 但暂不可调用的占位工具）。
func NewDynamic(info *tools.ToolInfo, fn ExecuteFunc) *DynamicTool {
	return &DynamicTool{info: info, fn: fn}
}

var _ tools.FuncTool = (*DynamicTool)(nil)

// Info 返回外部注入的 ToolInfo。
func (t *DynamicTool) Info() *tools.ToolInfo { return t.info }

// Execute 转发给外部注入的执行闭包。
func (t *DynamicTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.fn == nil {
		return nil, ErrNotImplemented
	}
	return t.fn(ctx, params)
}

// ErrNotImplemented 是 fn 为 nil 时 Execute 返回的哨兵错误。
var ErrNotImplemented = errNotImplemented{}

type errNotImplemented struct{}

func (errNotImplemented) Error() string { return "dynamic tool has no execute handler" }
