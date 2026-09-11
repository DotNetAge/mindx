package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/goharness/tools"
	mindxtools "github.com/DotNetAge/mindx/internal/tools"
)

// BuildTools 把 MCP 动态发现的 toolDef 列表转成 goharness FuncTool 实例。
// 每个 toolDef 变成一个 DynamicTool，info 来自 MCP inputSchema，
// execute 闭包通过 pool.Call 路由到对应的 MCP server。
//
// 工具命名规则：mcp:<server>:<toolName>
// 这样即使多个 server 暴露同名工具也不会在 ToolRegistry 里冲突。
//
// Schema 透传策略：
//   - RawSchema: 放完整 MCP inputSchema（嵌套 / 联合类型 / $ref 全保留），
//     goharness buildAllToolDefinitions 优先用它序列化给 LLM
//   - Parameters: 仍保留扁平 []Parameter 版本，供 UI 展示 / fallback
func BuildTools(defs []toolDef, pool *ConnectionPool) []tools.FuncTool {
	out := make([]tools.FuncTool, 0, len(defs))
	for _, d := range defs {
		goharnessName := fmt.Sprintf("mcp:%s:%s", d.server, d.Name)

		info := &tools.ToolInfo{
			Name:        goharnessName,
			Description: d.Description,
			Parameters:  convertSchema(d.InputSchema),
			RawSchema:   normalizeInputSchema(d.InputSchema),
		}

		out = append(out, mindxtools.NewDynamic(info, func(ctx context.Context, params map[string]any) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			result, err := pool.Call(ctx, d.server, d.Name, params)
			if err != nil {
				return nil, fmt.Errorf("mcp tool %q: %w", goharnessName, err)
			}
			return result, nil
		}))
	}
	return out
}

// normalizeInputSchema 处理 MCP inputSchema 给 RawSchema 用：
//   - nil → 返回 {type:"object", properties:{}}（LLM 工具零参数合法 schema）
//   - 没有 type:"object" 外层 → 补上
//   - type 不是 object 但有 properties → 修正 type 为 object
//
// 不做深度修改，完整保留嵌套结构。
func normalizeInputSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	// 拷贝一份，不改 MCP client 拿回来的原始 map
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		out[k] = v
	}
	typ, _ := out["type"].(string)
	if typ != "object" {
		if _, hasProps := out["properties"]; hasProps {
			out["type"] = "object"
		} else if typ == "" {
			out["type"] = "object"
		}
	}
	if _, ok := out["properties"]; !ok {
		out["properties"] = map[string]any{}
	}
	return out
}

// convertSchema 把 MCP JSON Schema inputSchema 转成 goharness []Parameter（扁平版本）。
// 仅保留顶层 properties 的 type/description/enum 等简单字段，嵌套结构会丢失。
// 给 UI 表单生成 / 文档展示用，不发给 LLM（LLM 用 RawSchema）。
func convertSchema(schema map[string]any) []tools.Parameter {
	if schema == nil {
		return nil
	}

	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return nil
	}

	reqSet := make(map[string]bool)
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				reqSet[name] = true
			}
		}
	}

	var params []tools.Parameter
	for name, propRaw := range props {
		prop, _ := propRaw.(map[string]any)
		if prop == nil {
			continue
		}
		typ, _ := prop["type"].(string)
		desc, _ := prop["description"].(string)

		p := tools.Parameter{
			Name:        name,
			Type:        typ,
			Description: desc,
			Required:    reqSet[name],
		}
		if def, ok := prop["default"]; ok {
			p.Default = def
		}
		if enum, ok := prop["enum"].([]any); ok {
			p.Enum = enum
		}
		params = append(params, p)
	}
	return params
}
