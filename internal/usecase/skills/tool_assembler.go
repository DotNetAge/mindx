package skills

import (
	"fmt"
	"mindx/internal/entity"
	"mindx/internal/usecase/mcp"
	"mindx/internal/usecase/tools"
	"sync"
)

// ToolAssembler 工具组装器（Phase 3 重构版）
// 从 ToolManager 和 MCPManager 自动获取工具，不再需要手动注册
type ToolAssembler struct {
	toolManager *tools.ToolManager
	mcpManager  *mcp.MCPManager
	mu          sync.RWMutex
}

// NewToolAssembler 创建工具组装器
func NewToolAssembler(toolManager *tools.ToolManager, mcpManager *mcp.MCPManager) *ToolAssembler {
	return &ToolAssembler{
		toolManager: toolManager,
		mcpManager:  mcpManager,
	}
}

// AssembleTools 组装工具（根据 Skill 的工具依赖）
func (a *ToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var schemas []entity.ToolSchema
	var missingRequired []string

	// 1. 组装必需工具
	for _, toolName := range skill.RequiredTools {
		schema, err := a.findTool(toolName)
		if err != nil {
			missingRequired = append(missingRequired, toolName)
			continue
		}
		schemas = append(schemas, schema)
	}

	// 2. 检查必需工具是否都找到
	if len(missingRequired) > 0 {
		return nil, fmt.Errorf("required tools not found: %v", missingRequired)
	}

	// 3. 组装可选工具（失败不影响）
	for _, toolName := range skill.OptionalTools {
		schema, err := a.findTool(toolName)
		if err != nil {
			// 可选工具失败只记录，不返回错误
			continue
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// findTool 查找工具（优先本地工具，回退到 MCP 工具）
func (a *ToolAssembler) findTool(name string) (entity.ToolSchema, error) {
	// 1. 优先查找本地工具
	if a.toolManager != nil && a.toolManager.HasTool(name) {
		tool, err := a.toolManager.GetTool(name)
		if err == nil {
			return a.localToolToSchema(tool), nil
		}
	}

	// 2. 查找 MCP 工具
	if a.mcpManager != nil && a.mcpManager.HasTool(name) {
		mcpTool, err := a.mcpManager.GetTool(name)
		if err == nil {
			return a.mcpToolToSchema(mcpTool), nil
		}
	}

	return entity.ToolSchema{}, fmt.Errorf("tool not found: %s", name)
}

// localToolToSchema 将本地工具转换为 ToolSchema
func (a *ToolAssembler) localToolToSchema(tool *tools.Tool) entity.ToolSchema {
	return entity.ToolSchema{
		Type: "function",
		Function: entity.ToolFunctionSchema{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		},
	}
}

// mcpToolToSchema 将 MCP 工具转换为 ToolSchema
func (a *ToolAssembler) mcpToolToSchema(tool *mcp.MCPTool) entity.ToolSchema {
	return entity.ToolSchema{
		Type: "function",
		Function: entity.ToolFunctionSchema{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Schema,
		},
	}
}

// AssembleToolsByNames 根据工具名称列表组装工具
func (a *ToolAssembler) AssembleToolsByNames(toolNames []string) ([]entity.ToolSchema, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var schemas []entity.ToolSchema

	for _, name := range toolNames {
		schema, err := a.findTool(name)
		if err != nil {
			// 跳过未找到的工具
			continue
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// HasTool 检查工具是否存在
func (a *ToolAssembler) HasTool(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.hasToolUnsafe(name)
}

// ListTools 列出所有工具
func (a *ToolAssembler) ListTools() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var tools []string

	if a.toolManager != nil {
		tools = append(tools, a.toolManager.ListTools()...)
	}

	if a.mcpManager != nil {
		tools = append(tools, a.mcpManager.ListTools()...)
	}

	return tools
}

// GetToolCount 获取工具数量
func (a *ToolAssembler) GetToolCount() (local, mcp, total int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.toolManager != nil {
		local = a.toolManager.GetToolCount()
	}

	if a.mcpManager != nil {
		mcp = a.mcpManager.GetToolCount()
	}

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
	if a.toolManager != nil && a.toolManager.HasTool(name) {
		return true
	}
	if a.mcpManager != nil && a.mcpManager.HasTool(name) {
		return true
	}
	return false
}
