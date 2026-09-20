package conv

import "time"

// ActionStep 是对话流中一次工具调用的展示单元，由 JSON-RPC 工具事件驱动：
//
//   - tool_use_delta：流式拼接参数预览；
//   - tool_exec_start：显示「⏺ 工具名(参数…) | 计时」，计时随 Tick 实时跳动；
//   - tool_exec_end：补齐「耗时 | Token」（Token 为服务器计算的真实消耗 =
//     输入 + 输出 - 缓存），并在下方渲染输出结果；执行失败时结果区切换为
//     错误组件（红框）呈现。
type ActionStep struct {
	ToolCallID    string
	ArgIndex      int // tool_use_delta 流式序号（与服务端 index 对应）
	ToolName      string
	Status        ActionStepStatus
	Params        map[string]any
	StreamingArgs string // 参数流式预览（start 后被 Params 取代）

	StartTime time.Time // 执行开始时刻（计时来源）
	Duration  time.Duration
	Tokens    int // 真实 token 消耗（in + out - cached）

	Result    string // 成功为输出结果；失败为错误信息
	DiffText  string
	DiffAdds  int
	DiffDels  int
	DiffFile  string
	Collapsed bool
}

// ResultFormatter 按工具名定制结果区的渲染。
// 不同工具返回的内容格式各异（JSON / diff / 表格 / 长文本…），
// 通过 RegisterResultFormatter 注册即可替换默认渲染，无需改动组件本身。
type ResultFormatter func(step ActionStep, result string, width int) string

var resultFormatters = map[string]ResultFormatter{}

// RegisterResultFormatter 注册某工具的结果渲染器（建议在包 init 中调用）。
func RegisterResultFormatter(toolName string, f ResultFormatter) {
	resultFormatters[toolName] = f
}
