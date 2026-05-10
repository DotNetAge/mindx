# MindX 命名机制与开发指南

## 一、项目架构总览

```
mindx/
├── cmd/                      ← 入口层（CLI 命令）
├── internal/                 ← 核心业务逻辑（禁止外部引用）
│   ├── client/               ←    TUI 客户端（终端界面）
│   ├── commands/             ←    命令定义与执行中心
│   └── svc/                  ←    服务端（Gateway + Agent 路由）
├── pkg/                      ← 可复用的公共库（可被外部引用）
│   ├── logging/              ←    日志封装
│   ├── memory/               ←    记忆管理
│   ├── scheduler/            ←    定时任务调度
│   └── session/              ←    会话存储
├── runtime/                  ← 运行时资源（配置文件 + Agent + Skills）
│   ├── agents/               ←    Agent 定义（Markdown）
│   ├── settings/             ←    模型配置（YAML）
│   └── skills/               ←    Skills 集合
└── docs/                     ← 项目文档
```

---

## 二、命名哲学

**名正言顺** — 每个文件、类型、函数的名称必须准确反映其职责，名称与职责不匹配就是设计缺陷。

### 核心原则

1. **职责决定名称** — 看名称就知道这个文件/类型做什么
2. **名称不称职就改名** — 不要容忍名不副实的命名
3. **一个文件一个职责** — 文件内的所有代码围绕同一个主题

---

## 三、包级命名

### 3.1 顶层包

| 包名 | 路径 | 职责 | 命名规则 |
|------|------|------|----------|
| `cmd` | `cmd/` | CLI 入口命令 | 动词优先（start, tui, whisper） |
| `client` | `internal/client/` | TUI 客户端 | UI 组件、事件处理、数据获取 |
| `commands` | `internal/commands/` | 命令定义中心 | 命令元数据、处理器、注册表 |
| `svc` | `internal/svc/` | 服务端业务 | 路由、调度、事件分发 |
| `logging` | `pkg/logging/` | 日志库 | 工具函数、封装 |
| `scheduler` | `pkg/scheduler/` | 定时任务 | 调度器、存储 |
| `session` | `pkg/session/` | 会话存储 | Store 接口与实现 |

### 3.2 internal/ 下的文件命名规则

#### client/ 包

| 文件名 | 职责 | 命名依据 |
|--------|------|----------|
| `model.go` | Bubble Tea Model 核心 | 框架约定（Model = 状态 + Update + View） |
| `registry.go` | 命令注册表（SlashCommandRegistry） | 职责是管理命令列表，不是单个命令 |
| `handlers.go` | UI 事件处理（键盘、Tab、命令执行） | handler = 事件处理器 |
| `display.go` | 渲染助手（表格、选项、分隔线） | display = 渲染/展示逻辑 |
| `fetch.go` | 数据获取（从服务端拉取数据） | fetch = 拉取数据 |
| `connect.go` | 连接管理（WebSocket 连接、重连、发送） | connect = 连接操作 |
| `types.go` | 类型定义（消息类型、样式常量） | types = 类型集合 |

#### commands/ 包

| 文件名 | 职责 | 命名依据 |
|--------|------|----------|
| `commands.go` | Registry + LocalRegistry 核心结构 | 核心注册表 |
| `catalog.go` | 目录类命令（agents, models, skills） | catalog = 目录/列表 |
| `scheduler.go` | 调度类命令（job-add, job-list, job-del） | scheduler = 调度 |
| `system.go` | 系统类命令（help, clear, about, init, compress） | system = 系统功能 |
| `local.go` | 客户端本地命令（help, clear, switch） | local = 仅客户端 |

#### svc/ 包

| 文件名 | 职责 | 命名依据 |
|--------|------|----------|
| `app.go` | 应用主结构体与生命周期 | app = 应用程序 |
| `wiring.go` | 依赖注入与路由注册 | wiring = 接线/组装 |
| `dispatch.go` | Agent 事件分发与格式化 | dispatch = 分发/转发 |
| `settings.go` | 配置管理 | settings = 配置 |

---

## 四、类型命名规则

### 4.1 结构体

**规则：使用名词，首字母大写（导出），准确描述职责**

```go
// ✅ 好：名称反映职责
type SlashCommandRegistry struct { ... }   // 注册表，不是"命令"
type CommandMeta struct { ... }            // 元数据，不是"信息"
type CommandResult struct { ... }          // 结果，明确是执行结果
type ScheduleEntry struct { ... }          // 条目，明确是计划任务条目

// ❌ 坏：名称模糊或误导
type Command struct { ... }                // 太泛，不知道是定义还是执行器
type Info struct { ... }                   // 太泛
type Data struct { ... }                   // 无意义
```

### 4.2 接口

**规则：使用 -er 后缀或描述性名词**

```go
// ✅ 好
type MessageHandler func(ctx context.Context, msg *Message) error
type CommandExecutor interface { Execute(cmd string) error }

// ❌ 坏
type HandlerInterface interface { ... }    // 冗余的 Interface 后缀
type Doer interface { ... }                // 太泛
```

### 4.3 类型别名

**规则：当需要重命名外部类型时使用**

```go
// ✅ 好：为外部类型提供领域特定的别名
type Meta = gateway.CommandMeta   // 在 commands 包中使用更短的名字
```

### 4.4 枚举/常量

**规则：使用有意义的分组前缀**

```go
// ✅ 好
type CommandScope string

const (
    ScopeLocal  CommandScope = "local"   // 仅客户端执行
    ScopeRemote CommandScope = "remote"  // 仅服务端执行
    ScopeBoth   CommandScope = "both"    // 两端都有
)

// ❌ 坏
const (
    Local  = "local"   // 无类型安全
    Remote = "remote"
    Both   = "both"
)
```

---

## 五、函数命名规则

### 5.1 构造函数

**规则：`New` + 类型名**

```go
// ✅ 好
func NewSlashCommandRegistry() *SlashCommandRegistry
func NewCommandRegistry() *CommandRegistry
func New(opts ...Option) *Server
```

### 5.2 依赖注入

**规则：`Set` + 依赖名 + `Deps`**

```go
// ✅ 好
func SetCatalogDeps(deps CatalogDeps)
func SetSchedulerDeps(deps SchedulerDeps)
```

### 5.3 注册方法

**规则：`Register` + 对象名**

```go
// ✅ 好
func (r *Registry) Register(meta Meta, handler Handler)
func (r *Registry) RegisterAll(gw *gateway.Server)
```

### 5.4 查询方法

**规则：动词 + 对象，返回复数用复数名词**

```go
// ✅ 好
func (r *Registry) All() []Command          // 返回所有
func (r *Registry) Visible() []Command      // 返回可见的
func (r *Registry) Filter(prefix string) []Command  // 过滤
func (r *Registry) Find(name string) *Command       // 查找单个
```

### 5.5 事件处理

**规则：`handle` + 事件名 或 `forward` + 对象名**

```go
// ✅ 好（服务端）
func (a *App) defaultHandler(msg *gateway.Message)   // 处理默认消息
func (a *App) forwardEvent(clientID string, event)   // 转发事件
func (a *App) sendEvent(clientID string, ...)        // 发送事件

// ✅ 好（客户端）
func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd    // 处理按键
func (m *model) handleSlashCommand(raw string)       // 处理斜杠命令
func (m *model) handleRemoteCommand(name string)     // 处理远程命令
```

---

## 六、开发指南

### 6.1 如何添加新命令

#### 步骤 1：在 `internal/commands/` 下定义

根据命令类型选择对应文件：

```
目录类（列表查询）→ catalog.go
调度类（定时任务）→ scheduler.go
系统类（帮助/清理）→ system.go
纯客户端（UI操作）→ local.go
```

如果命令不属于以上任何一类，创建新文件：

```go
// internal/commands/weather.go
package commands

func registerWeatherCommands(r *Registry) {
    r.Register(Meta{
        Name:        "weather",
        Description: "查询天气",
        Category:    "agent",
        Scope:       gateway.ScopeRemote,
        Example:     "/weather 北京",
    }, handleWeather)
}
```

然后在 `commands.go` 的 `New()` 中调用：

```go
func New() *Registry {
    r := &Registry{}
    registerSystemCommands(r)
    registerCatalogCommands(r)
    registerSchedulerCommands(r)
    registerWeatherCommands(r)  // ← 新增
    return r
}
```

#### 步骤 2：注入依赖（如果需要）

如果命令需要访问 App 的资源，在 `commands/` 中定义依赖结构体：

```go
// internal/commands/weather.go
type WeatherDeps struct {
    Client func() *WeatherClient
}

var weatherDeps WeatherDeps

func SetWeatherDeps(deps WeatherDeps) {
    weatherDeps = deps
}
```

然后在 `svc/wiring.go` 中注入：

```go
func RegisterBuiltinCommands(gw *gateway.Server, app *App) {
    commands.SetWeatherDeps(commands.WeatherDeps{
        Client: func() *WeatherClient { return app.WeatherClient() },
    })
    commands.New().RegisterAll(gw)
}
```

### 6.2 如何添加新 TUI 本地命令

```go
// internal/commands/local.go
func registerLocalCommands(r *LocalRegistry) {
    r.Register(Meta{
        Name:        "theme",
        Description: "切换主题",
        Category:    "ui",
        Scope:       gateway.ScopeLocal,
    })
}
```

然后在 `client/registry.go` 的 `BuiltinCommands()` 中添加处理逻辑：

```go
case "theme":
    cmd.Run = func(args string) *CommandResult {
        return &CommandResult{Message: "主题已切换"}
    }
```

### 6.3 如何添加新的 pkg/ 库

1. 在 `pkg/` 下创建新目录
2. 包名使用目录名（小写，无连字符）
3. 导出类型使用大写字母开头
4. 提供 `New` 构造函数

```
pkg/
└── cache/
    ├── cache.go          // 核心类型
    ├── cache_test.go     // 测试
    └── lru.go            // LRU 实现
```

### 6.4 如何添加新的 Agent

在 `runtime/agents/` 下创建 Markdown 文件：

```markdown
# weather-agent

Role: weather

Description: 天气查询助手

System prompt:
你是一个专业的天气查询助手...
```

### 6.5 如何添加新的 Skill

在 `runtime/skills/` 下创建目录和 `SKILL.md`：

```
runtime/skills/
└── weather/
    ├── SKILL.md          ← 必需：技能定义
    └── scripts/          ← 可选：辅助脚本
        └── fetch.sh
```

---

## 七、文件命名红线

**以下命名方式严禁使用：**

| 禁止命名 | 理由 | 替代方案 |
|----------|------|----------|
| `utils.go` | 太泛，什么都往里塞 | 按职责拆分（display.go, fetch.go） |
| `common.go` | 同上 | 按职责拆分 |
| `helpers.go` | 同上 | 按职责拆分 |
| `handler.go`（当文件内容是分发时） | 名不副实 | `dispatch.go` |
| `commands.go`（当文件内容是注册表时） | 名不副实 | `registry.go` |
| `data.go` | 无意义 | 按数据类型命名 |
| `types.go`（当文件超过 200 行） | 太大，职责不清 | 拆分为多个文件 |

---

## 八、代码组织清单

提交代码前自检：

- [ ] 文件名是否准确反映其职责？
- [ ] 类型名是否准确反映其用途？
- [ ] 函数名是否清晰表达其行为？
- [ ] 是否把不相关的代码放到了错误的文件？
- [ ] 是否有 `utils.go` / `common.go` / `helpers.go`？
- [ ] 新命令是否在 `internal/commands/` 下定义？
- [ ] 依赖注入是否在 `svc/wiring.go` 中完成？
