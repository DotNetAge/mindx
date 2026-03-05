package skills

import (
	"fmt"
	"mindx/internal/entity"
	"sync"
)

// ToolAssembler 工具组装器
// 根据 Skill 的 RequiredTools 和 OptionalTools 动态查找和组装工具
type ToolAssembler struct {
	localTools map[string]*LocalTool // 本地工具注册表
	mcpTools   map[string]*MCPTool   // MCP 工具注册表
	mu         sync.RWMutex
}

// LocalTool 本地工具
type LocalTool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Execute     func(params map[string]interface{}) (string, error)
}

// MCPTool MCP 工具
type MCPTool struct {
	Name        string
	Description string
	ServerName  string // MCP 服务器名称
	Schema      entity.ToolSchema
}

// NewToolAssembler 创建工具组装器
func NewToolAssembler() *ToolAssembler {
	return &ToolAssembler{
		localTools: make(map[string]*LocalTool),
		mcpTools:   make(map[string]*MCPTool),
	}
}

// RegisterLocalTool 注册本地工具
func (a *ToolAssembler) RegisterLocalTool(tool *LocalTool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.localTools[tool.Name] = tool
}

// RegisterMCPTool 注册 MCP 工具
func (a *ToolAssembler) RegisterMCPTool(tool *MCPTool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.mcpTools[tool.Name] = tool
}

// AssembleTools 组装工具
// 根据 Skill 的 RequiredTools 和 OptionalTools 查找并组装工具
func (a *ToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var schemas []entity.ToolSchema
	var missingRequired []string

	// 1. 处理必需工具
	for _, toolName := range skill.RequiredTools {
		schema, err := a.findTool(toolName)
		if err != nil {
			missingRequired = append(missingRequired, toolName)
			continue
		}
		schemas = append(schemas, schema)
	}

	// 2. 如果有必需工具缺失，返回错误
	if len(missingRequired) > 0 {
		return nil, fmt.Errorf("required tools not found: %v", missingRequired)
	}

	// 3. 处理可选工具（失败不影响）
	for _, toolName := range skill.OptionalTools {
		schema, err := a.findTool(toolName)
		if err != nil {
			// 可选工具未找到，只记录日志，不影响流程
			continue
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// AssembleToolsByNames 根据工具名称列表组装工具
func (a *ToolAssembler) AssembleToolsByNames(toolNames []string) ([]entity.ToolSchema, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var schemas []entity.ToolSchema
	var missing []string

	for _, toolName := range toolNames {
		schema, err := a.findTool(toolName)
		if err != nil {
			missing = append(missing, toolName)
			continue
		}
		schemas = append(schemas, schema)
	}

	if len(missing) > 0 {
		return schemas, fmt.Errorf("tools not found: %v", missing)
	}

	return schemas, nil
}

// findTool 查找工具（本地 + MCP）
func (a *ToolAssembler) findTool(name string) (entity.ToolSchema, error) {
	// 1. 优先查找本地工具
	if tool, ok := a.localTools[name]; ok {
		return a.localToolToSchema(tool), nil
	}

	// 2. 查找 MCP 工具
	if tool, ok := a.mcpTools[name]; ok {
		return tool.Schema, nil
	}

	return entity.ToolSchema{}, fmt.Errorf("tool %s not found", name)
}

// localToolToSchema 将本地工具转换为 ToolSchema
func (a *ToolAssembler) localToolToSchema(tool *LocalTool) entity.ToolSchema {
	return entity.ToolSchema{
		Type: "function",
		Function: entity.ToolFunctionSchema{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		},
	}
}

// GetTool 获取工具（本地或 MCP）
func (a *ToolAssembler) GetTool(name string) (interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 查找本地工具
	if tool, ok := a.localTools[name]; ok {
		return tool, nil
	}

	// 查找 MCP 工具
	if tool, ok := a.mcpTools[name]; ok {
		return tool, nil
	}

	return nil, fmt.Errorf("tool %s not found", name)
}

// HasTool 检查工具是否存在
func (a *ToolAssembler) HasTool(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, hasLocal := a.localTools[name]
	_, hasMCP := a.mcpTools[name]

	return hasLocal || hasMCP
}

// ListTools 列出所有工具
func (a *ToolAssembler) ListTools() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]string, 0, len(a.localTools)+len(a.mcpTools))

	for name := range a.localTools {
		tools = append(tools, name)
	}

	for name := range a.mcpTools {
		tools = append(tools, name)
	}

	return tools
}

// ListLocalTools 列出所有本地工具
func (a *ToolAssembler) ListLocalTools() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]string, 0, len(a.localTools))
	for name := range a.localTools {
		tools = append(tools, name)
	}

	return tools
}

// ListMCPTools 列出所有 MCP 工具
func (a *ToolAssembler) ListMCPTools() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]string, 0, len(a.mcpTools))
	for name := range a.mcpTools {
		tools = append(tools, name)
	}

	return tools
}

// UnregisterLocalTool 注销本地工具
func (a *ToolAssembler) UnregisterLocalTool(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.localTools, name)
}

// UnregisterMCPTool 注销 MCP 工具
func (a *ToolAssembler) UnregisterMCPTool(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.mcpTools, name)
}

// Clear 清空所有工具
func (a *ToolAssembler) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.localTools = make(map[string]*LocalTool)
	a.mcpTools = make(map[string]*MCPTool)
}

// GetToolCount 获取工具数量
func (a *ToolAssembler) GetToolCount() (local, mcp, total int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	local = len(a.localTools)
	mcp = len(a.mcpTools)
	total = local + mcp

	return
}

// ValidateSkillTools 验证 Skill 的工具是否都可用
func (a *ToolAssembler) ValidateSkillTools(skill *entity.Skill) (missing []string, optional []string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 检查必需工具
	for _, toolName := range skill.RequiredTools {
		if !a.hasToolUnsafe(toolName) {
			missing = append(missing, toolName)
		}
	}

	// 检查可选工具
	for _, toolName := range skill.OptionalTools {
		if !a.hasToolUnsafe(toolName) {
			optional = append(optional, toolName)
		}
	}

	return
}

// hasToolUnsafe 检查工具是否存在（不加锁，内部使用）
func (a *ToolAssembler) hasToolUnsafe(name string) bool {
	_, hasLocal := a.localTools[name]
	_, hasMCP := a.mcpTools[name]
	return hasLocal || hasMCP
}
