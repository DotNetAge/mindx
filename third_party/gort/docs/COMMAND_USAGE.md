# Command 用法说明

## 一、设计概览

Gort 采用 **统一的 Command 元数据模型**（`CommandMeta`），在 Server 和 Client 之间共享命令定义，消除旧版本中两套割裂的 Command 结构带来的混淆。

```
┌─────────────────────────────────────────────────────────────┐
│              统一的 Command 模型 (CommandMeta)                │
│                                                             │
│  Name        string    // 命令名（唯一）                      │
│  Description string    // 描述                               │
│  Category    string    // 分类：agent | system | ui           │
│  Scope       CommandScope  // local | remote | both          │
│  Example     string    // 使用示例                           │
│  Params      string    // 参数格式说明                        │
└─────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │                           │
         服务端注册                   客户端注册
     ┌─────────────────────┐     ┌──────────────────────┐
     │ gateway.CommandMeta │     │ client.Command        │
     │ + handler 函数       │     │  { gateway.CommandMeta │
     │ → JSON-RPC method    │     │    + Run 函数         │
     └─────────────────────┘     │    + Hidden           │
                                 │    + SubCommands }    │
                                 └──────────────────────┘
```

---

## 二、CommandMeta 结构

### 2.1 字段定义

```go
type CommandMeta struct {
    Name        string         `json:"name"`         // 命令名（不含前缀 /）
    Description string         `json:"description"`  // 简短描述
    Category    string         `json:"category,omitempty"`   // 分类标签
    Scope       CommandScope   `json:"scope"`        // 执行范围
    Example     string         `json:"example,omitempty"`    // 完整使用示例
    Params      string         `json:"params,omitempty"`     // 参数格式说明
}
```

### 2.2 Scope 枚举

```go
type CommandScope string

const (
    ScopeLocal  CommandScope = "local"   // 仅客户端执行
    ScopeRemote CommandScope = "remote"  // 仅服务端执行
    ScopeBoth   CommandScope = "both"    // 两端都有实现
)
```

### 2.3 Category 约定

| Category | 说明 | 示例 |
|----------|------|------|
| `agent` | 智能体相关 | agents, models, skills |
| `system` | 系统功能 | help, clear, init, compress, job-* |
| `ui` | 纯界面操作 | switch, theme（仅客户端） |

---

## 三、服务端用法

### 3.1 注册命令

```go
gw.RegisterCommand(gateway.CommandMeta{
    Name:        "agents",
    Description: "显示智能体列表",
    Category:    "agent",
    Scope:       gateway.ScopeRemote,
    Example:     "/agents",
}, func(ctx *gateway.CommandContext) (any, error) {
    // 执行业务逻辑
    agents := listAgents()
    
    // 推送类型化响应（通知）
    ctx.RespondWithType(gateway.RespTable, "Available Agents", map[string]interface{}{
        "headers": []string{"Name", "Role", "Description"},
        "rows":    toAgentTableRows(agents),
    })
    return nil, nil // handler 已推送响应，无需返回值
})
```

### 3.2 带参数的命令

```go
gw.RegisterCommand(gateway.CommandMeta{
    Name:        "job-add",
    Description: "添加计划任务",
    Category:    "system",
    Scope:       gateway.ScopeRemote,
    Example:     `/job-add @assistant 每日晨会提醒 expr="0 0 9 * * *"`,
    Params:      `@<agent-name> <content> expr="<cron表达式>"`,
}, func(ctx *gateway.CommandContext) (any, error) {
    args := ctx.Args // 获取命令参数字符串
    // 解析并执行
    return result, nil
})
```

### 3.3 直接返回结果（不推送通知）

```go
gw.RegisterCommand(gateway.CommandMeta{
    Name:        "about",
    Description: "关于 MindX",
    Category:    "system",
    Scope:       gateway.ScopeRemote,
}, func(ctx *gateway.CommandContext) (any, error) {
    return "MindX Agent Chat v0.1", nil
})
```

### 3.4 CommandContext

```go
type CommandContext struct {
    ClientID string // 调用方客户端 ID
    Args     string // 命令参数字符串
}

// RespondWithType 向调用方推送类型化响应
func (ctx *CommandContext) RespondWithType(t ResponseType, title string, data interface{})

// Server 获取 Server 实例引用
func (ctx *CommandContext) Server() *Server
```

---

## 四、客户端用法

### 4.1 注册本地命令

客户端的 `Command` 嵌入 `gateway.CommandMeta`，保持元数据对齐，同时添加本地执行器 `Run`。

```go
r.Register(Command{
    CommandMeta: gateway.CommandMeta{
        Name:        "help",
        Description: "显示所有可用命令",
        Category:    "ui",
        Scope:       gateway.ScopeLocal,
    },
    Run: func(args string) *CommandResult {
        return &CommandResult{Message: "帮助文本"}
    },
})
```

### 4.2 同步远程命令

客户端连接后，从服务端获取 `CommandMeta` 列表并同步：

```go
metas, err := client.GetCommands()
// metas 类型为 []gateway.CommandMeta
registry.SyncRemoteCommands(metas)
```

`SyncRemoteCommands` 的行为：
- 远程命令不存在于本地时 → **添加**新命令（`Run = nil`）
- 远程命令已存在于本地时 → **更新**元数据（`Description`、`Scope` 等），清除 `Run`

### 4.3 Command 结构

```go
type Command struct {
    gateway.CommandMeta          // 嵌入统一元数据
    Hidden      bool             // 是否在建议列表中隐藏
    Run         func(args string) *CommandResult // 本地执行器（仅 ScopeLocal）
    SubCommands []Command        // 嵌套子命令
}

type CommandResult struct {
    Message   string  // 显示消息
    ClearChat bool    // 是否清空聊天历史
}
```

### 4.4 命令执行路由

客户端根据 `Scope` 决定执行方式：

```go
if cmd.Scope == gateway.ScopeRemote || cmd.Scope == gateway.ScopeBoth {
    // 发送到服务端执行
    return m.handleRemoteCommand(name, args, raw)
}
// 本地执行
return m.handleCommand(cmd, args)
```

---

## 五、内置方法 command.list

`command.list` 是 gort 在 `New()` 时自动注册的内置方法，返回所有已注册命令的 `CommandMeta` 列表：

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "method": "command.list",
  "params": null
}

// 响应：
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "result": [
    {
      "name": "agents",
      "description": "显示智能体列表",
      "category": "agent",
      "scope": "remote",
      "example": "/agents"
    }
  ]
}
```

客户端可通过 `client.GetCommands()` 便捷调用：

```go
metas, err := client.GetCommands()
// 返回 []gateway.CommandMeta
```

---

## 六、完整示例

### 6.1 服务端完整注册

```go
package svc

import "github.com/DotNetAge/gort/pkg/gateway"

func RegisterCommands(gw *gateway.Server) {
    // Agent 查询命令
    gw.RegisterCommand(gateway.CommandMeta{
        Name:        "agents",
        Description: "显示智能体列表",
        Category:    "agent",
        Scope:       gateway.ScopeRemote,
        Example:     "/agents",
    }, func(ctx *gateway.CommandContext) (any, error) {
        agents := listAgents()
        ctx.RespondWithType(gateway.RespTable, "Agents", map[string]interface{}{
            "headers": []string{"Name", "Role"},
            "rows":    agentsToRows(agents),
        })
        return nil, nil
    })

    // 带参数的命令
    gw.RegisterCommand(gateway.CommandMeta{
        Name:        "search",
        Description: "搜索知识库",
        Category:    "agent",
        Scope:       gateway.ScopeRemote,
        Example:     "/search 关键词",
        Params:      "<关键词>",
    }, func(ctx *gateway.CommandContext) (any, error) {
        keyword := ctx.Args
        results := search(keyword)
        return results, nil
    })
}
```

### 6.2 客户端完整注册

```go
package client

import "github.com/DotNetAge/gort/pkg/gateway"

func BuiltinCommands() *CommandRegistry {
    r := NewCommandRegistry()

    // 纯本地命令
    r.Register(Command{
        CommandMeta: gateway.CommandMeta{
            Name:        "help",
            Description: "显示所有可用命令",
            Category:    "ui",
            Scope:       gateway.ScopeLocal,
        },
        Run: func(args string) *CommandResult {
            return &CommandResult{Message: r.helpText()}
        },
    })

    // 隐藏的内部命令
    r.Register(Command{
        CommandMeta: gateway.CommandMeta{
            Name:        "debug",
            Description: "显示调试信息",
            Category:    "system",
            Scope:       gateway.ScopeLocal,
        },
        Hidden: true,
        Run: func(args string) *CommandResult {
            return &CommandResult{Message: showDebug()}
        },
    })

    return r
}
```

---

## 七、Category 分类展示

客户端可利用 `Category` 字段对命令进行分组展示：

```
System Commands:
  /help          显示所有可用命令
  /clear         清理当前所有上下文
  /init          初始化会话

Agent Commands:
  /agents        显示智能体列表
  /models        列出所有可用模型
  /skills        列出所有可用技能

Schedule Commands:
  /job-add       添加计划任务
  /job-list      列出所有计划任务
  /job-del       删除计划任务
```

---

## 八、迁移指南（从旧 API 到新 API）

### 旧写法（已删除）

```go
// ❌ 旧 API
gw.RegisterCommand("agents", handler, "显示智能体列表")
```

### 新写法

```go
// ✅ 新 API
gw.RegisterCommand(gateway.CommandMeta{
    Name:        "agents",
    Description: "显示智能体列表",
    Category:    "agent",
    Scope:       gateway.ScopeRemote,
}, handler)
```

### 主要变化

| 旧 API | 新 API | 说明 |
|--------|--------|------|
| `RegisterCommand(name, handler, desc)` | `RegisterCommand(meta, handler)` | 用 `CommandMeta` 替代字符串参数 |
| `CommandInfo{Name, Description}` | `CommandMeta{Name, Description, Category, Scope, Example, Params}` | 新增分类、范围、示例、参数 |
| `Command.Remote bool` | `Command.Scope CommandScope` | 布尔值 → 三态枚举 |
| `client.GetCommands() → []CommandInfo` | `client.GetCommands() → []CommandMeta` | 返回完整元数据 |
