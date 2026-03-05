package entity

import (
	"time"
)

// ThinkContext 共享上下文对象
// 在处理器管线中传递和丰富，每个处理器修改其负责的部分
type ThinkContext struct {
	// ========== 输入 ==========
	Input     string // 用户原始输入
	SessionID string // 会话 ID

	// ========== 意图理解 ==========
	Intent *IntentContext // 意图上下文

	// ========== 知识检索 ==========
	Memories      []*MemoryPoint // 检索到的记忆点
	MatchedSkills []*SkillSOP    // 匹配的技能 SOP

	// ========== 工具执行 ==========
	Tools       []ToolSchema     // 可用工具列表
	ToolResults []ToolExecResult // 工具执行结果

	// ========== 输出 ==========
	Response string // 最终响应
	SendTo   string // 发送目标（用于消息转发）

	// ========== 错误处理 ==========
	Errors []ProcessorError // 各处理器的错误记录

	// ========== 元数据 ==========
	StartTime time.Time
	Metadata  map[string]interface{}
}

// IntentContext 意图上下文
type IntentContext struct {
	Type       string   // 意图类型（weather_query, schedule_create, etc.）
	Keywords   []string // 关键词列表
	Confidence float64  // 置信度 [0.0, 1.0]
}

// MemoryPoint 记忆点
type MemoryPoint struct {
	ID        string
	Content   string
	Keywords  []string
	Timestamp time.Time
}

// SkillSOP 技能标准操作程序
type SkillSOP struct {
	Name          string   // 技能名称
	Description   string   // 技能描述
	Keywords      []string // 关键词列表
	RequiredTools []string // 所需工具列表
	SOPContent    string   // SOP 正文内容
}

// ToolSchema 工具描述（OpenAI Tools 格式）
type ToolSchema struct {
	Type     string             `json:"type"`
	Function ToolFunctionSchema `json:"function"`
}

// ToolFunctionSchema 工具函数描述
type ToolFunctionSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolExecResult 工具执行结果
type ToolExecResult struct {
	ToolCallID   string                 // 工具调用 ID
	FunctionName string                 // 函数名称
	Arguments    map[string]interface{} // 调用参数
	Result       string                 // 执行结果
	Error        string                 // 错误信息（如果有）
}

// ProcessorError 处理器错误
type ProcessorError struct {
	ProcessorName string
	Error         error
	Timestamp     time.Time
}

// NewThinkContext 创建新的思考上下文
func NewThinkContext(input, sessionID string) *ThinkContext {
	return &ThinkContext{
		Input:     input,
		SessionID: sessionID,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
		Errors:    make([]ProcessorError, 0),
	}
}

// AddError 添加处理器错误
func (tc *ThinkContext) AddError(processorName string, err error) {
	tc.Errors = append(tc.Errors, ProcessorError{
		ProcessorName: processorName,
		Error:         err,
		Timestamp:     time.Now(),
	})
}

// HasErrors 是否有错误
func (tc *ThinkContext) HasErrors() bool {
	return len(tc.Errors) > 0
}

// Duration 计算执行时长
func (tc *ThinkContext) Duration() time.Duration {
	return time.Since(tc.StartTime)
}
