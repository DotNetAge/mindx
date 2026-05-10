# MindX 命令机制说明与开发指南

## 一、命令架构概览

MindX 采用**统一的命令元数据模型**（`CommandMeta`），在 Server 和 Client 之间共享命令定义。所有命令都集中在 `internal/commands/` 目录下定义，Server 和 Client 各自引用。

```
┌─────────────────────────────────────────────────────────────┐
│                统一命令定义 (internal/commands/)             │
│                                                             │
│  Meta{ Name, Description, Category, Scope, Example, Params }│
│                                                             │
│  Scope:                                                     │
│    local   = 仅客户端执行（如 /switch, /help）               │
│    remote  = 仅服务端执行（如 /agents, /models）             │
│    both    = 两端都有实现（如 /clear）                       │
└─────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │                           │
         服务端注册                   客户端注册
     ┌─────────────────────┐     ┌──────────────────────┐
     │ svc/wiring.go       │     │ client/registry.go   │
     │ → RegisterAll(gw)   │     │ → BuiltinCommands()  │
     │ → SetCatalogDeps()  │     │ → SyncRemoteCommands()│
     │ → SetSchedulerDeps()│     │                      │
     └─────────────────────┘     └──────────────────────┘
```

---

## 二、CommandMeta 字段说明

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `Name` | string | 命令名（不含前缀 /） | `"agents"` |
| `Description` | string | 简短描述 | `"显示智能体列表"` |
| `Category` | string | 分类标签，用于分组展示 | `"agent"` / `"system"` / `"ui"` |
| `Scope` | CommandScope | 执行范围 | `gateway.ScopeRemote` |
| `Example` | string | 完整使用示例 | `"/agents"` |
| `Params` | string | 参数格式说明 | `"@<agent-name> <content>"` |

### Scope 枚举

```go
const (
    ScopeLocal  CommandScope = "local"   // 仅客户端执行
    ScopeRemote CommandScope = "remote"  // 仅服务端执行
    ScopeBoth   CommandScope = "both"    // 两端都有实现
)
```

---

## 三、命令执行流程

### 3.1 远程命令（ScopeRemote）

```
用户在 TUI 输入 /agents
         │
         ▼
client/registry.go: SlashCommandRegistry.Find("agents")
         │
         ▼ cmd.Scope == ScopeRemote
         │
client/handlers.go: handleRemoteCommand("agents", args)
         │
         ▼ client.SendCommand("agents", "")
         │
gort/client.go: Call("agents", {"args": ""})
         │  JSON-RPC over WebSocket
         ▼
gort/server.go: methods["agents"](ctx, params)
         │
commands/catalog.go: handleAgents(ctx)
         │
ctx.RespondWithType(RespTable, "Available Agents", data)
         │  JSON-RPC Notification: {"method":"table",...}
         ▼
client/handlers.go: On("table") handler
         │
client/display.go: renderTableEnvelope(data)
         │
         ▼
TUI 显示表格
```

### 3.2 本地命令（ScopeLocal）

```
用户在 TUI 输入 /switch
         │
         ▼
client/registry.go: SlashCommandRegistry.Find("switch")
         │
         ▼ cmd.Scope == ScopeLocal
         │
client/handlers.go: handleCommand(cmd, args)
         │
         ▼ cmd.Run(args)
         │
client/registry.go: BuiltinCommands() 中定义的函数
         │
         ▼
TUI 执行结果（消息或清空聊天等）
```

### 3.3 双端命令（ScopeBoth）

```
用户在 TUI 输入 /clear
         │
         ▼
client/handlers.go: cmd.Scope == ScopeBoth → handleRemoteCommand
         │
         ▼ 服务端返回 "__clear__" 哨兵值
         │
client/handlers.go: result.Message == "__clear__" → ClearChat = true
         │
         ▼
TUI 清空聊天历史
```

---

## 四、命令注册中心

### 4.1 服务端注册表（commands.Registry）

定义在 `internal/commands/commands.go`：

```go
type Registry struct {
    commands []struct {
        meta    Meta
        handler Handler
    }
}
```

核心方法：

| 方法 | 说明 |
|------|------|
| `Register(meta, handler)` | 注册一条命令 |
| `Metas()` | 返回所有命令元数据（供客户端同步） |
| `RegisterAll(gw)` | 将所有命令注册到 Gateway Server |

### 4.2 客户端注册表（client.SlashCommandRegistry）

定义在 `internal/client/registry.go`：

```go
type SlashCommandRegistry struct {
    commands []Command
}

type Command struct {
    gateway.CommandMeta           // 嵌入统一元数据
    Hidden      bool              // 是否在建议列表中隐藏
    Run         func(args string) *CommandResult  // 本地执行器
    SubCommands []Command         // 嵌套子命令
}
```

核心方法：

| 方法 | 说明 |
|------|------|
| `Register(cmd)` | 注册一条命令 |
| `All()` | 返回所有命令 |
| `Visible()` | 返回非隐藏的命令 |
| `Filter(prefix)` | 按前缀过滤（Tab 补全用） |
| `Find(name)` | 精确查找 |
| `SyncRemoteCommands(metas)` | 同步服务端命令 |

---

## 五、命令分类与职责

### 5.1 目录类命令（catalog.go）

列出系统资源，返回表格数据。

| 命令 | 说明 | 参数 | 返回 |
|------|------|------|------|
| `/agents` | 显示智能体列表 | 无 | RespTable (Name, Role, Description) |
| `/models` | 列出所有可用模型 | 无 | RespTable (名称, 描述) |
| `/skills` | 列出所有可用技能 | 无 | RespTable (名称, 描述) |

依赖注入：

```go
commands.SetCatalogDeps(commands.CatalogDeps{
    ListAgents: func() ([]map[string]string, error) { ... },
    ListModels: func() ([]map[string]string, error) { ... },
    ListSkills: func() ([]map[string]string, error) { ... },
})
```

### 5.2 调度类命令（scheduler.go）

管理定时任务。

| 命令 | 说明 | 参数 | 返回 |
|------|------|------|------|
| `/job-add` | 添加计划任务 | `@<agent> <content> expr="<cron>"` | 成功消息 |
| `/job-list` | 列出所有计划任务 | 无 | RespTable (ID, Agent, Content, Rule, Status) |
| `/job-del` | 删除计划任务 | `id=<任务ID>` | 成功消息 |

依赖注入：

```go
commands.SetSchedulerDeps(commands.SchedulerDeps{
    SchedulerDB: func() *scheduler.FileSchedulerStore { ... },
    Scheduler:   func() *scheduler.Scheduler { ... },
})
```

### 5.3 系统类命令（system.go）

系统功能，不涉及外部依赖。

| 命令 | Scope | 说明 | 返回 |
|------|-------|------|------|
| `/help` | Remote | 显示所有可用命令 | 文本列表 |
| `/about` | Remote | 关于 MindX | 文本 |
| `/init` | Remote | 初始化会话 | 文本 |
| `/clear` | Both | 清理上下文 | `"__clear__"` 哨兵值 |
| `/compress` | Remote | 压缩上下文 | 文本 |

### 5.4 客户端本地命令（local.go）

仅客户端执行，不发送到服务端。

| 命令 | 说明 |
|------|------|
| `/help` | 本地帮助（显示客户端注册的命令列表） |
| `/clear` | 本地清理（直接清空 TUI 聊天历史） |
| `/switch` | 切换当前对话的 agent |

---

## 六、开发指南

### 6.1 添加新的远程命令

#### 步骤 1：在 commands 包中定义

根据命令类型选择对应文件：

```go
// internal/commands/catalog.go  ← 如果是资源列表
// internal/commands/scheduler.go ← 如果是调度相关
// internal/commands/system.go    ← 如果是系统功能
// 或创建新文件：internal/commands/weather.go

package commands

import "github.com/DotNetAge/gort/pkg/gateway"

func registerWeatherCommands(r *Registry) {
    r.Register(Meta{
        Name:        "weather",
        Description: "查询天气",
        Category:    "agent",
        Scope:       gateway.ScopeRemote,
        Example:     "/weather 北京",
        Params:      "<城市名>",
    }, handleWeather)
}

func handleWeather(ctx *gateway.CommandContext) (any, error) {
    city := ctx.Args
    if city == "" {
        return nil, fmt.Errorf("请指定城市名，如 /weather 北京")
    }
    info := queryWeather(city)
    ctx.RespondWithType(gateway.RespText, fmt.Sprintf("%s 天气", city), info)
    return nil, nil
}
```

#### 步骤 2：在 New() 中注册

```go
// internal/commands/commands.go
func New() *Registry {
    r := &Registry{}
    registerSystemCommands(r)
    registerCatalogCommands(r)
    registerSchedulerCommands(r)
    registerWeatherCommands(r)  // ← 新增这一行
    return r
}
```

#### 步骤 3：注入依赖（如果需要）

```go
// internal/commands/weather.go
type WeatherDeps struct {
    Client func() *WeatherAPI
}

var weatherDeps WeatherDeps

func SetWeatherDeps(deps WeatherDeps) {
    weatherDeps = deps
}

func handleWeather(ctx *gateway.CommandContext) (any, error) {
    if weatherDeps.Client == nil {
        return nil, fmt.Errorf("天气服务未配置")
    }
    client := weatherDeps.Client()
    info := client.Query(ctx.Args)
    return info, nil
}
```

然后在 `svc/wiring.go` 中注入：

```go
func RegisterBuiltinCommands(gw *gateway.Server, app *App) {
    // ... 其他依赖注入 ...
    
    commands.SetWeatherDeps(commands.WeatherDeps{
        Client: func() *WeatherAPI { return app.WeatherClient() },
    })
    commands.New().RegisterAll(gw)
}
```

### 6.2 添加新的客户端本地命令

#### 步骤 1：在 local.go 中注册元数据

```go
// internal/commands/local.go
func registerLocalCommands(r *LocalRegistry) {
    // ... 现有命令 ...

    r.Register(Meta{
        Name:        "theme",
        Description: "切换主题",
        Category:    "ui",
        Scope:       gateway.ScopeLocal,
    })
}
```

#### 步骤 2：在 client/registry.go 中添加执行逻辑

```go
// internal/client/registry.go
func BuiltinCommands() *SlashCommandRegistry {
    r := NewSlashCommandRegistry()

    for _, meta := range commands.NewLocal().Metas() {
        m := meta
        cmd := Command{
            CommandMeta: m,
        }

        switch m.Name {
        // ... 现有命令 ...
        case "theme":
            cmd.Run = func(args string) *CommandResult {
                return &CommandResult{Message: "主题已切换"}
            }
        default:
            cmd.Run = func(args string) *CommandResult {
                return &CommandResult{Message: fmt.Sprintf("/%s: %s", m.Name, m.Description)}
            }
        }

        r.Register(cmd)
    }

    return r
}
```

### 6.3 添加带参数的命令

参数通过 `ctx.Args` 传递，自行解析：

```go
r.Register(Meta{
    Name:        "search",
    Description: "搜索知识库",
    Category:    "agent",
    Scope:       gateway.ScopeRemote,
    Params:      "<关键词> [--source=web|local]",
    Example:     `/search AI agent --source=web`,
}, func(ctx *gateway.CommandContext) (any, error) {
    args := ctx.Args  // "AI agent --source=web"
    
    // 解析参数
    parts := strings.Fields(args)
    keyword := parts[0]
    source := "local"
    for _, part := range parts {
        if strings.HasPrefix(part, "--source=") {
            source = strings.TrimPrefix(part, "--source=")
        }
    }
    
    results := search(keyword, source)
    return results, nil
})
```

### 6.4 返回不同类型的响应

```go
// 返回文本
return "操作成功", nil

// 返回表格（通过 ctx.RespondWithType）
ctx.RespondWithType(gateway.RespTable, "标题", map[string]interface{}{
    "headers": []string{"列1", "列2"},
    "rows":    [][]string{{"值1", "值2"}},
})
return nil, nil

// 返回选项列表
ctx.RespondWithType(gateway.RespOptions, "请选择", map[string]interface{}{
    "options": []string{"选项A", "选项B", "选项C"},
})
return nil, nil

// 返回待办事项
ctx.RespondWithType(gateway.RespTodo, "待完成", map[string]interface{}{
    "todos": []string{"任务1", "任务2", "任务3"},
})
return nil, nil
```

### 6.5 命令测试

由于命令逻辑集中在 `internal/commands/` 中，可以直接对 handler 函数进行单元测试：

```go
// internal/commands/catalog_test.go
func TestHandleAgents(t *testing.T) {
    // 注入测试依赖
    SetCatalogDeps(CatalogDeps{
        ListAgents: func() ([]map[string]string, error) {
            return []map[string]string{
                {"name": "writer", "role": "writer", "description": "Writer"},
            }, nil
        },
        ListModels: nil,
        ListSkills: nil,
    })

    ctx := &gateway.CommandContext{}
    _, err := handleAgents(ctx)
    if err != nil {
        t.Fatalf("handleAgents failed: %v", err)
    }
}
```

---

## 七、命令生命周期

### 7.1 启动时

```
mindx start
    │
    ▼
svc/app.go: Start()
    │
    ▼
svc/wiring.go: RegisterBuiltinCommands(gw, app)
    │
    ├── SetCatalogDeps(...)     ← 注入依赖
    ├── SetSchedulerDeps(...)   ← 注入依赖
    └── commands.New().RegisterAll(gw)  ← 注册所有命令到 Gateway
```

### 7.2 客户端连接时

```
Client Connect
    │
    ▼
client/fetch.go: fetchCommands()
    │
    ▼
client.GetCommands() → server "command.list"
    │
    ▼
client/fetch.go: m.registry.SyncRemoteCommands(metas)
    │
    ▼
客户端获得完整的命令列表（本地 + 远程）
```

### 7.3 命令执行时

```
用户输入 /cmd args
    │
    ▼
client/handlers.go: handleSlashCommand()
    │
    ├── cmd.Scope == Local  → handleCommand(cmd, args) → cmd.Run()
    ├── cmd.Scope == Remote → handleRemoteCommand() → SendCommand()
    └── cmd.Scope == Both   → handleRemoteCommand() → 返回哨兵值 → 本地执行
```

---

## 八、注意事项

1. **不要在 `svc/` 或 `client/` 中定义命令元数据** — 所有定义都在 `internal/commands/`
2. **不要在命令 handler 中直接访问 App** — 通过依赖注入传递
3. **`ScopeLocal` 的命令必须有 `Run` 函数** — 否则点击无效
4. **`ScopeRemote` 的命令 `Run` 为 nil** — 由 `handleRemoteCommand` 处理
5. **`ScopeBoth` 的命令需要两端协调** — 通常服务端返回哨兵值，客户端识别后执行本地逻辑
6. **命令名不含前缀** — 注册时用 `"agents"` 不是 `"/agents"`
7. **分类(Category)影响展示** — 客户端可利用 Category 分组展示
8. **隐藏命令** — 客户端中设置 `Hidden: true` 可从建议列表隐藏但仍可执行
