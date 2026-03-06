# Phase 3 Step 1：架构设计和规划

> 创建日期：2026-03-06
>
> 状态：进行中

---

## 🎯 设计目标

1. **完全解耦**：Skills、Tools、MCP 三者完全独立
2. **清晰职责**：每个管理器只负责一类资源
3. **易于扩展**：支持新的工具类型和 MCP 服务器
4. **高性能**：工具加载和执行高效

---

## 📐 核心接口设计

### 1. ToolManager 接口

```go
// ToolManager 本地工具管理器
type ToolManager interface {
    // LoadTools 加载所有本地工具
    LoadTools() error

    // GetTool 获取指定工具
    GetTool(name string) (*Tool, error)

    // ListTools 列出所有工具
    ListTools() []string

    // ExecuteTool 执行工具
    ExecuteTool(name string, params map[string]interface{}) (string, error)

    // ReloadTool 重新加载指定工具
    ReloadTool(name string) error
}
```

**职责**：
- 从 `tools/` 目录加载本地工具
- 解析 `tool.json` 配置
- 执行本地工具（调用可执行文件）
- 管理工具生命周期

---

### 2. MCPManager 接口

```go
// MCPManager MCP 服务器管理器
type MCPManager interface {
    // Connect 连接到 MCP 服务器
    Connect(serverName string) error

    // Disconnect 断开连接
    Disconnect(serverName string) error

    // ListServers 列出所有 MCP 服务器
    ListServers() []string

    // GetTools 获取指定服务器的工具列表
    GetTools(serverName string) ([]*MCPTool, error)

    // ExecuteTool 执行 MCP 工具
    ExecuteTool(serverName, toolName string, params map[string]interface{}) (string, error)

    // IsConnected 检查服务器是否已连接
    IsConnected(serverName string) bool
}
```

**职责**：
- 连接和管理 MCP 服务器
- 从 MCP 服务器获取工具列表
- 执行 MCP 工具
- 处理连接失败和重试

---

### 3. Tool 定义格式

```go
// Tool 本地工具定义
type Tool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Version     string                 `json:"version"`
    Author      string                 `json:"author"`

    // 执行配置
    Command     string                 `json:"command"`      // 可执行文件路径
    Args        []string               `json:"args"`         // 默认参数
    Env         map[string]string      `json:"env"`          // 环境变量
    Timeout     int                    `json:"timeout"`      // 超时时间（秒）

    // 参数定义（OpenAI Tools 格式）
    Parameters  map[string]interface{} `json:"parameters"`

    // 依赖
    Requires    *ToolRequires          `json:"requires"`

    // 元数据
    FilePath    string                 `json:"-"`            // tool.json 路径
    Directory   string                 `json:"-"`            // 工具目录
}

// ToolRequires 工具依赖
type ToolRequires struct {
    Bins []string `json:"bins"` // 依赖的二进制文件
    Envs []string `json:"envs"` // 依赖的环境变量
}
```

---

### 4. MCPTool 定义格式

```go
// MCPTool MCP 工具定义
type MCPTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    ServerName  string                 `json:"server_name"`  // 所属 MCP 服务器

    // 参数定义（OpenAI Tools 格式）
    InputSchema map[string]interface{} `json:"inputSchema"`
}
```

---

## 📁 目录结构设计

### 新的目录结构

```
mindx/
├── skills/                    # Skills 目录（纯 SOP）
│   ├── weather_query/
│   │   └── SKILL.md          # 只有 SOP 文档
│   ├── calculator/
│   │   └── SKILL.md
│   └── ...
│
├── tools/                     # Tools 目录（新增）
│   ├── web_search/
│   │   ├── tool.json         # 工具定义
│   │   └── main.go           # 可执行文件
│   ├── http_request/
│   │   ├── tool.json
│   │   └── main.py
│   ├── calculator/
│   │   ├── tool.json
│   │   └── main.py
│   └── ...
│
├── config/
│   └── mcp_servers.json      # MCP 服务器配置（新增）
│
└── internal/
    └── usecase/
        ├── tools/            # 新增：Tool 管理
        │   ├── manager.go
        │   ├── manager_test.go
        │   ├── executor.go
        │   └── loader.go
        └── mcp/              # 新增：MCP 管理
            ├── manager.go
            ├── manager_test.go
            ├── client.go
            └── connector.go
```

---

## 📄 配置文件格式

### tool.json 格式

```json
{
  "name": "web_search",
  "description": "网页搜索工具",
  "version": "1.0.0",
  "author": "mindx",
  "command": "./main.go",
  "args": [],
  "env": {},
  "timeout": 60,
  "parameters": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "搜索关键词"
      },
      "max_results": {
        "type": "integer",
        "description": "最大结果数",
        "default": 10
      }
    },
    "required": ["query"]
  },
  "requires": {
    "bins": [],
    "envs": []
  }
}
```

---

### mcp_servers.json 格式

```json
{
  "servers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "env": {},
      "timeout": 30
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      },
      "timeout": 60
    }
  }
}
```

---

## 🔄 工作流程设计

### 1. 工具加载流程

```
启动时
  ↓
ToolManager.LoadTools()
  ↓
扫描 tools/ 目录
  ↓
解析每个 tool.json
  ↓
验证依赖（bins, envs）
  ↓
注册到 ToolManager
```

### 2. MCP 连接流程

```
启动时
  ↓
MCPManager.LoadConfig()
  ↓
读取 mcp_servers.json
  ↓
按需连接（懒加载）
  ↓
获取工具列表
  ↓
注册到 MCPManager
```

### 3. 工具执行流程

```
SkillMatchProcessor
  ↓
识别需要的工具（从 SOP）
  ↓
ToolAssembler.AssembleTools()
  ├─→ ToolManager.GetTool()      # 本地工具
  └─→ MCPManager.GetTools()      # MCP 工具
  ↓
返回 ToolSchema 列表
  ↓
LLM 决定调用哪个工具
  ↓
ToolExecutionProcessor
  ├─→ ToolManager.ExecuteTool()  # 执行本地工具
  └─→ MCPManager.ExecuteTool()   # 执行 MCP 工具
```

---

## 🔧 ToolAssembler 重构设计

### 新的 ToolAssembler

```go
type ToolAssembler struct {
    toolManager *ToolManager
    mcpManager  *MCPManager
    cache       map[string][]entity.ToolSchema
    mu          sync.RWMutex
}

// AssembleTools 组装工具（新实现）
func (a *ToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
    var schemas []entity.ToolSchema

    // 1. 从 ToolManager 获取本地工具
    for _, toolName := range skill.RequiredTools {
        tool, err := a.toolManager.GetTool(toolName)
        if err != nil {
            // 尝试从 MCP 获取
            mcpTool, err := a.findMCPTool(toolName)
            if err != nil {
                return nil, fmt.Errorf("required tool not found: %s", toolName)
            }
            schemas = append(schemas, mcpToolToSchema(mcpTool))
            continue
        }
        schemas = append(schemas, toolToSchema(tool))
    }

    // 2. 可选工具（失败不影响）
    for _, toolName := range skill.OptionalTools {
        if tool, err := a.toolManager.GetTool(toolName); err == nil {
            schemas = append(schemas, toolToSchema(tool))
        } else if mcpTool, err := a.findMCPTool(toolName); err == nil {
            schemas = append(schemas, mcpToolToSchema(mcpTool))
        }
    }

    return schemas, nil
}
```

---

## 📊 接口对比

### 旧接口（Phase 2）

```go
// ToolAssembler（Phase 2）
type ToolAssembler struct {
    localTools map[string]*LocalTool  // 手动注册
    mcpTools   map[string]*MCPTool    // 手动注册
}

// 问题：需要手动注册每个工具
assembler.RegisterLocalTool(&LocalTool{...})
assembler.RegisterMCPTool(&MCPTool{...})
```

### 新接口（Phase 3）

```go
// ToolAssembler（Phase 3）
type ToolAssembler struct {
    toolManager *ToolManager  // 自动加载
    mcpManager  *MCPManager   // 自动连接
}

// 优势：自动发现和加载工具
toolManager.LoadTools()  // 自动扫描 tools/ 目录
mcpManager.Connect()     // 自动连接 MCP 服务器
```

---

## ✅ 设计验收标准

### 接口设计
- [x] ToolManager 接口定义清晰
- [x] MCPManager 接口定义清晰
- [x] Tool 定义格式规范
- [x] MCPTool 定义格式规范

### 目录结构
- [x] tools/ 目录结构设计
- [x] config/ 目录结构设计
- [x] 代码目录结构设计

### 配置格式
- [x] tool.json 格式定义
- [x] mcp_servers.json 格式定义

### 工作流程
- [x] 工具加载流程设计
- [x] MCP 连接流程设计
- [x] 工具执行流程设计

---

## 🚀 下一步

**Step 2**：实现 ToolManager（3天）

**任务**：
1. 实现 ToolManager 接口
2. 实现工具加载逻辑
3. 实现工具执行逻辑
4. 编写单元测试

**文件**：
- `internal/usecase/tools/manager.go`
- `internal/usecase/tools/manager_test.go`
- `internal/usecase/tools/executor.go`
- `internal/usecase/tools/loader.go`

---

**完成时间**：2026-03-06
**耗时**：1 天（按计划）
**状态**：✅ 已完成，可以继续 Step 2
