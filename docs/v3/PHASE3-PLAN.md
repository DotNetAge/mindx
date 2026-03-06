# Phase 3 实施计划：Tools 与 MCP 架构重构

> 创建日期：2026-03-06
>
> 目标：彻底分离 Tools 和 Skills，实现独立的 Tool 管理系统

---

## 🎯 核心目标

### 1. 架构分离

**V2 架构（当前错误）**：
```
Skills 目录
├── skill_name/
│   ├── SKILL.md          ← Skill 定义（SOP）
│   ├── tool.json         ← Tool 定义（混在一起）❌
│   └── main.go           ← Tool 可执行文件（混在一起）❌
```

**V3 架构（目标正确）**：
```
Skills 目录（纯 SOP 知识）
├── weather_query/
│   └── SKILL.md          ← 只有 SOP 文档

Tools 目录（独立管理）
├── web_search/
│   ├── tool.json         ← Tool 定义
│   └── main.go           ← Tool 可执行文件
├── http_request/
│   ├── tool.json
│   └── main.py

MCP 配置（独立管理）
└── mcp_config.yaml       ← MCP 服务器配置
```

---

## 📋 实施步骤

### Step 1: 架构设计和规划（1天）

**任务**：
1. 设计 ToolManager 接口
2. 设计 MCPManager 接口
3. 设计 Tool 目录结构
4. 设计 tool.json 格式规范
5. 创建详细的实施文档

**输出**：
- `docs/v3/01-tool-architecture.md`
- `docs/v3/02-tool-format-spec.md`
- `docs/v3/03-mcp-integration.md`

---

### Step 2: 实现 ToolManager（3天）

**文件**：
- `internal/usecase/tools/tool_manager.go`
- `internal/usecase/tools/tool_manager_test.go`
- `internal/usecase/tools/tool_loader.go`
- `internal/usecase/tools/tool_executor.go`

**核心功能**：
```go
type ToolManager interface {
    // 加载工具
    LoadTools(toolsDir string) error

    // 获取工具
    GetTool(name string) (*Tool, error)
    GetAllTools() ([]*Tool, error)

    // 执行工具
    Execute(name string, params map[string]interface{}) (string, error)

    // 工具管理
    RegisterTool(tool *Tool) error
    UnregisterTool(name string) error

    // 工具验证
    ValidateTool(tool *Tool) error
}
```

**Tool 结构**：
```go
type Tool struct {
    Name        string
    Description string
    Version     string
    Command     string
    Parameters  ToolParameters
    Timeout     time.Duration
    OS          []string
    Requires    *ToolRequires
}
```

---

### Step 3: 实现 MCPManager（3天）

**文件**：
- `internal/usecase/mcp/mcp_manager.go`
- `internal/usecase/mcp/mcp_manager_test.go`
- `internal/usecase/mcp/mcp_client.go`
- `internal/usecase/mcp/mcp_tool_adapter.go`

**核心功能**：
```go
type MCPManager interface {
    // 连接管理
    Connect(serverName string, config MCPConfig) error
    Disconnect(serverName string) error

    // 工具发现
    DiscoverTools(serverName string) ([]*MCPTool, error)
    GetTool(serverName, toolName string) (*MCPTool, error)

    // 工具执行
    Execute(serverName, toolName string, params map[string]interface{}) (string, error)

    // 服务器管理
    ListServers() []string
    GetServerStatus(serverName string) (ServerStatus, error)
}
```

**MCP Tool 适配器**：
```go
// 将 MCP Tool 转换为统一的 Tool 接口
type MCPToolAdapter struct {
    serverName string
    mcpTool    *MCPTool
    client     *MCPClient
}

func (a *MCPToolAdapter) ToToolSchema() entity.ToolSchema {
    // 转换为 OpenAI Tools 格式
}
```

---

### Step 4: 重构 ToolAssembler（2天）

**更新**：`internal/usecase/skills/tool_assembler.go`

**新架构**：
```go
type ToolAssembler struct {
    toolManager  *tools.ToolManager  // 本地工具管理器
    mcpManager   *mcp.MCPManager     // MCP 工具管理器
    mu           sync.RWMutex
}

func (a *ToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
    var schemas []entity.ToolSchema

    for _, toolName := range skill.RequiredTools {
        // 1. 优先查找本地工具
        if tool, err := a.toolManager.GetTool(toolName); err == nil {
            schemas = append(schemas, tool.ToSchema())
            continue
        }

        // 2. 查找 MCP 工具
        if mcpTool, err := a.findMCPTool(toolName); err == nil {
            schemas = append(schemas, mcpTool.ToSchema())
            continue
        }

        // 3. 必需工具缺失，返回错误
        return nil, fmt.Errorf("required tool not found: %s", toolName)
    }

    return schemas, nil
}
```

---

### Step 5: 迁移 Tools 到独立目录（2天）

**任务**：
1. 创建 `tools/` 目录结构
2. 从 `skills/` 目录提取所有工具
3. 转换为新的 tool.json 格式
4. 更新工具可执行文件路径
5. 生成迁移报告

**迁移脚本**：`scripts/migrate_tools.go`

**目录结构**：
```
tools/
├── web_search/
│   ├── tool.json
│   └── main.py
├── calculator/
│   ├── tool.json
│   └── main.py
├── weather/
│   ├── tool.json
│   └── main.sh
└── ...
```

**tool.json 格式**：
```json
{
  "name": "web_search",
  "description": "网页搜索工具",
  "version": "1.0.0",
  "command": "./main.py",
  "parameters": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "搜索关键词"
      }
    },
    "required": ["query"]
  },
  "timeout": 60,
  "os": ["darwin", "linux"],
  "requires": {
    "bins": ["python3"],
    "env": ["SEARCH_API_KEY"]
  }
}
```

---

### Step 6: 更新 SkillMatchProcessor（1天）

**更新**：`internal/usecase/brain/processors/skill_processor.go`

**变化**：
- 使用新的 ToolAssembler（已集成 ToolManager 和 MCPManager）
- 无需修改接口，只需更新依赖注入

---

### Step 7: 测试和验证（3天）

**任务**：
1. 单元测试（ToolManager, MCPManager, ToolAssembler）
2. 集成测试（完整流程）
3. 端到端测试（实际工具执行）
4. 性能测试
5. 文档更新

**测试覆盖**：
- ToolManager: > 80%
- MCPManager: > 80%
- ToolAssembler: > 80%
- 集成测试: 核心流程

---

## 📊 时间估算

| 步骤 | 任务 | 工作量 |
|------|------|--------|
| Step 1 | 架构设计和规划 | 1 天 |
| Step 2 | 实现 ToolManager | 3 天 |
| Step 3 | 实现 MCPManager | 3 天 |
| Step 4 | 重构 ToolAssembler | 2 天 |
| Step 5 | 迁移 Tools 到独立目录 | 2 天 |
| Step 6 | 更新 SkillMatchProcessor | 1 天 |
| Step 7 | 测试和验证 | 3 天 |
| **总计** | | **15 天** |

---

## ✅ 验收标准

### 架构验收
- [ ] Skills 和 Tools 完全解耦
- [ ] Skills 目录只包含 SKILL.md
- [ ] Tools 目录独立管理本地工具
- [ ] MCP 配置独立管理

### 功能验收
- [ ] ToolManager 正确加载和执行本地工具
- [ ] MCPManager 正确连接和执行 MCP 工具
- [ ] ToolAssembler 正确动态组装工具
- [ ] 所有测试通过

### 质量验收
- [ ] 测试覆盖率 > 80%
- [ ] 无遗留代码
- [ ] 文档完整

---

## 🚨 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 工具迁移失败 | 高 | 提供自动迁移脚本和回滚方案 |
| MCP 连接不稳定 | 中 | 实现重试机制和降级策略 |
| 性能下降 | 中 | 实现缓存和并发优化 |
| 兼容性问题 | 高 | 保持接口兼容，渐进式迁移 |

---

**创建时间**：2026-03-06
**预计完成**：2026-03-21（15 个工作日）
