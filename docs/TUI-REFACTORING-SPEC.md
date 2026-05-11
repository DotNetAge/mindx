# MindX TUI 独立化重构 — Design Specification

> **版本**: v1.0  
> **日期**: 2026-05-10  
> **状态**: Draft  
> **作者**: Architecture Team  

---

## 📋 目录

1. [项目背景与目标](#1-项目背景与目标)
2. [当前架构分析](#2-当前架构分析)
3. [目标架构设计](#3-目标架构设计)
4. [模块详细规范](#4-模块详细规范)
5. [TUI 改造方案](#5-tui-改造方案)
6. [迁移策略](#6-迁移策略)
7. [接口契约](#7-接口契约)
8. [风险评估与缓解](#8-风险评估与缓解)
9. [实施计划](#9-实施计划)
10. [验收标准](#10-验收标准)

---

## 1. 项目背景与目标

### 1.1 业务背景

MindX 是一个 AI-native 的多 Agent 对话平台，支持三种 UI 形态：
- **TUI** (Terminal UI): 技术人员的工具箱，也是默认入口
- **WebUI** (Browser): 浏览器访问，零安装
- **MacUI** (Native App): macOS 原生应用

### 1.2 当前痛点

#### 问题 1: 双进程启动负担
```
❌ 用户启动流程:
   mindx tui → 必须先启动 mindx start (gateway server) → 再连接 WebSocket
   
✅ 期望的用户体验:
   mindx → 回车即用，单进程启动
```

**影响**:
- 普通用户无法使用（需要同时管理两个进程）
- 需要额外的 launcher/installer 程序
- 进程间依赖导致部署复杂度指数级增长

#### 问题 2: 架构过度工程
```
当前消息流 (绕大圈):
  用户输入 → TUI (Bubble Tea) 
    → JSON-RPC encode 
      → WebSocket write (localhost:1314) 
        → Gateway readPump 
          → handleNotification("user.message") 
            → defaultHandler() 
              → resolveAgent() 
                → agent.Ask() 
                  → ReactEvent channel 
                    → forwardEvent() 映射 30+ 种事件类型 
                      → JSON-RPC Notification 
                        → WebSocket write 
                          → TUI readLoop 
                            → JSON-RPC decode 
                              → Bubble Tea Msg 
                                → 渲染
```

**开销来源**:
- JSON-RPC 编解码（序列化/反序列化）
- WebSocket 连接管理（重连、心跳、健康检查）
- 事件类型双重映射（core.ReactEvent ↔ gateway.ResponseType）
- 跨进程调试困难

#### 问题 3: 维护成本高
每次 GoReact 新增事件类型，需要同步修改三层代码：
1. `forwardEvent()` — 服务端映射逻辑
2. `session.go` — 客户端 handler 注册
3. `component_*.go` — TUI 渲染逻辑

### 1.3 重构目标

| 目标 | 描述 | 优先级 |
|------|------|--------|
| **G1: 单进程启动** | TUI 可独立运行，不依赖 gateway server | P0 |
| **G2: 消息短路** | TUI 直接调用 Agent，跳过网络层 | P0 |
| **G3: 架构清晰** | 引擎层与服务层分离，职责明确 | P1 |
| **G4: 向后兼容** | Daemon 模式保留，WebUI/MacUI 不受影响 | P1 |
| **G5: 易于维护** | 减少跨层同步，降低维护成本 | P2 |

### 1.4 设计原则

1. **引擎是心脏，不是服务**
   - Agent + Session + Scheduler 是纯 library，不感知网络
   - TUI 和 Daemon 都是引擎的"消费者"

2. **统一在事件层，而非传输层**
   ```
   ❌ 错误: 统一传输协议 (所有 UI 都走 WebSocket)
   ✅ 正确: 统一事件源 (core.ReactEvent channel)
        - TUI: 直接读 channel (零开销)
        - WebUI/MacUI: 通过 gateway 桥接 (localhost 或远程)
   ```

3. **会话共享靠文件系统**
   - Session store 已落盘 (`runtime/sessions/*.yml`)
   - 多进程指向同一目录即可天然共享
   - 无需进程间通信协议

4. **渐进式迁移，风险可控**
   - 先验证 TUI 独立运行可行性
   - 再考虑 Daemon 化细节
   - 保持向后兼容

---

## 2. 当前架构分析

### 2.1 现有文件结构

```
mindx/
├── main.go                          # 入口点
├── cmd/
│   └── tui.go                       # TUI 命令入口
├── internal/
│   ├── client/                      # TUI 组件层 (~15 个文件)
│   │   ├── component_root.go        # 根组件 (依赖 gateway.Client)
│   │   ├── component_inputbox.go    # 输入框 (已有 /command 处理)
│   │   ├── session.go               # 会话管理 (WebSocket handler 注册)
│   │   ├── fetch.go                 # 远程命令封装
│   │   └── registry.go              # SlashCommand 注册表
│   ├── commands/                    # 命令定义
│   │   ├── commands.go              # 命令注册表 (依赖 gateway.CommandMeta)
│   │   ├── system.go                # 系统命令 (/help, /clear)
│   │   ├── catalog.go               # 目录命令 (/models, /agents)
│   │   ├── scheduler.go             # 调度命令 (/job-*)
│   │   └── local.go                 # 本地命令
│   └── svc/                         # 服务层 (上帝类 App)
│       ├── app.go                   # ⚠️ App 结构体 (混合引擎+服务)
│       ├── dispatch.go              # ⚠️ 386 行事件转发逻辑
│       ├── settings.go              # 配置 (含网络配置 Addr/WSPath)
│       └── wiring.go                # 命令注入
├── pkg/
│   ├── session/                     # 会话存储 (FileSessionStore)
│   ├── scheduler/                   # 调度器 (CronScheduler)
│   └── logging/                     # 日志
```

### 2.2 核心问题：App 是上帝类

[internal/svc/app.go:24-41](../internal/svc/app.go#L24-L41) 的 `App` 结构体混合了两种完全不同的职责：

```go
type App struct {
    // ===== 引擎层 (Engine Layer) =====
    agents     *goreact.AgentRegistry      // Agent 注册表
    models     *goreact.ModelRegistry      // Model 注册表
    master     *goreact.Agent              // Master Agent 实例
    rules      core.RuleRegistry           // 规则引擎
    sessDB     *session.FileSessionStore   // 会话存储
    agentCache map[string]*goreact.Agent   // Agent 缓存
    
    // ===== 服务层 (Service Layer) =====
    gw          *gateway.Server            // ← 网络依赖！
    scheduler   *scheduler.Scheduler       // ← 调度器！
    schedulerDB *scheduler.FileSchedulerStore
}
```

**问题表现**:

| 方法 | 层级 | 应该属于 |
|------|------|---------|
| `getMaster()` / `resolveAgent()` | ✅ 引擎层 | core.App |
| `defaultHandler()` / `forwardEvent()` | ❌ 服务层 | daemon |
| `Start()` / `initGateway()` | ❌ 服务层 | daemon |
| `RegisterBuiltinCommands()` | ❌ 服务层 | daemon/server |
| `executeScheduleCommand()` | ❌ 服务层 | daemon |

### 2.3 依赖关系图（当前）

```
client (TUI 组件)
  ↓ import
  gort/pkg/gateway  ← 强依赖！
  ↓
svc.App
  ↓ import
  goreact (Agent 引擎)
  pkg/session
  pkg/scheduler
  gort/pkg/gateway  ← 循环依赖风险！
```

**问题**:
- TUI 直接依赖 `gateway.Client`，无法独立运行
- `svc` 包同时被 `client` 和 `commands` 导入，容易形成循环依赖
- 网络层侵入到渲染层，违反关注点分离原则

---

## 3. 目标架构设计

### 3.1 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                    MindX 产品矩阵                            │
├─────────────┬──────────────┬────────────────────────────────┤
│   mindx     │  mindx       │     mindx daemon               │
│   (TUI)     │  daemon      │     (--gateway)                 │
│             │              │                                 │
│  单进程     │  后台进程     │  后台进程 + 网络服务             │
│  零配置     │  调度常驻     │  支持 WebUI/MacUI 接入         │
└──────┬──────┴──────┬───────┴──────────────┬─────────────────┘
       │             │                       │
       ▼             ▼                       ▼
┌─────────────────────────────────────────────────────────────┐
│                  core.App (纯引擎)                           │
│  ┌─────────┬──────────┬──────────┬──────────┬────────────┐  │
│  │ Agents  │ Models   │ Sessions │ Rules    │ AgentCache │  │
│  └─────────┴──────────┴──────────┴──────────┴────────────┘  │
│                                                                 │
│  核心方法:                                                       │
│  - ResolveAgent(name) → *goreact.Agent                          │
│  - GetMaster() → *goreact.Agent                                 │
│  - IsModelAvailable(name...) bool                                │
│  - Agents()/Models()/SessionDB()/RuleRegistry()                  │
└─────────────────────────────────────────────────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
     ┌────────────────┐ ┌─────────────┐ ┌──────────────┐
     │  TUI (直接嵌入)  │ │   Daemon     │ │  未来扩展...  │
     │                │ │             │ │              │
     │ - 读 ReactEvent│ │ - Gateway    │ │ - gRPC       │
     │ - 本地渲染     │ │ - Scheduler  │ │ - MQTT       │
     │ - /command    │ │ - 事件桥接   │ │ - HTTP API   │
     └────────────────┘ └─────────────┘ └──────────────┘
```

### 3.2 三层分离模型

| 层级 | 包名 | 职责 | 依赖 | 使用者 |
|------|------|------|------|--------|
| **引擎层** | `internal/core` | Agent/Model/Session/Rules 管理 | `goreact`, `pkg/session`, `pkg/logging` | TUI, Daemon |
| **服务层** | `internal/svc` | Gateway + Scheduler + 事件转发 | `core.App`, `gort/gateway`, `pkg/scheduler` | Server, main |
| **合成层** | `internal/svc` | 启动/停止/生命周期编排 | `core.App`, `Daemon` | main |

### 3.3 消息流对比

#### 重构前（绕大圈）

```
TUI InputBox
  → sendToServerWithSession(client, text, sessionID)
    → client.Notify("user.message", payload)  [JSON-RPC]
      → WebSocket write (localhost:1314)
        → Gateway readPump
          → handleNotification("user.message")
            → defaultHandler()
              → resolveAgent(agentName)
                → agent.EventsFiltered(filter)
                → agent.Ask(sessionID, content)
                  → [ReactEvent stream]
                    → forwardEvent(event)  [映射 30+ 种事件]
                      → gw.SendResponse()  [JSON-RPC]
                        → WebSocket write
                          → TUI readLoop
                            → On("event_type", handler)
                              → outputCh ← tea.Msg
                                → Root.Update()
                                  → routeToAnswer()
                                    → ContentPanel 渲染
```

**开销统计**:
- 4 次序列化/反序列化 (JSON-RPC × 2 + WebSocket frame × 2)
- 2 次跨进程 IPC (localhost TCP)
- 1 次 30+ 分支的 switch-case (forwardEvent)
- ~200 行胶水代码 (session.go, fetch.go)

#### 重构后（短路）

```
TUI InputBox
  → handleSend(msg)
    → engine.ResolveAgent(currentAgent)
      → agent.EventsFiltered(filter)
        → go agent.Ask(sessionID, content)
          → [ReactEvent stream]
            → consumeEvents(eventCh, sessionID)
              → trySend(outputCh, agentAnswerUpdateMsg)
                → Root.Update()
                  → routeToAnswer()
                    → ContentPanel 渲染
```

**收益**:
- ✅ 0 次序列化（内存 channel 直传）
- ✅ 0 次跨进程调用（同 goroutine）
- ✅ 0 次事件映射（直接用 core.ReactEvent.Type）
- ✅ 删除 ~150 行 WebSocket 管理代码

---

## 4. 模块详细规范

### 4.1 internal/core — 引擎层

#### 4.1.1 文件结构

```
internal/
└── core/
    ├── app.go          # App 主结构体 + 工厂方法
    ├── settings.go     # 配置（不含网络参数）
    └── app_test.go     # 单元测试
```

#### 4.1.2 core.App 结构体

```go
package core

import (
    "sync"
    
    "github.com/DotNetAge/goreact"
    "github.com/DotNetAge/goreact/core"
    "github.com/DotNetAge/mindx/pkg/logging"
    "github.com/DotNetAge/mindx/pkg/session"
)

// App is the MindX engine — manages agents, models, sessions, and rules.
// It has ZERO network dependencies and can be embedded directly in TUI or Daemon.
//
// Design Principles:
//   - Pure library: no HTTP/WebSocket/gRPC dependencies
//   - Stateful: maintains agent cache and master agent instance
//   - Thread-safe: uses RWMutex for concurrent access
type App struct {
    settings *Settings
    logger   logging.Logger
    
    agents   *goreact.AgentRegistry
    models   *goreact.ModelRegistry
    master   *goreact.Agent
    masterMu sync.RWMutex

    rules      core.RuleRegistry
    sessDB     *session.FileSessionStore
    agentCache map[string]*goreact.Agent
    agentMu    sync.RWMutex
}
```

**不变量 (Invariants)**:
1. `master` 为 nil 表示尚未初始化（lazy init）
2. `agentCache` 中的 key 是 agent name，value 是已实例化的 Agent
3. `sessDB` 不为 nil（构造时必选初始化，失败只 warn 不 error）

#### 4.1.3 公开方法签名

```go
// ===== 工厂方法 =====

func DefaultApp() (*App, error)
// 从环境变量 + 默认值创建 App 实例
// 加载顺序: .env → Agents → Models → Rules → Sessions

// ===== 访问器 (Accessors) =====

func (a *App) Settings() *Settings
func (a *App) Agents() *goreact.AgentRegistry
func (a *App) Models() *goreact.ModelRegistry
func (a *App) SessionDB() *session.FileSessionStore
func (a *App) RuleRegistry() core.RuleRegistry
func (a *App) SetLogger(l logging.Logger)

// ===== 核心 Agent 管理 =====

func (a *App) GetMaster() (*goreact.Agent, error)
// 返回 Master Agent（懒加载 + 缓存）
// 首次调用时从 registry 创建实例，后续返回缓存

func (a *App) ResolveAgent(name string) (*goreact.Agent, error)
// 按 name 解析 Agent（懒加载 + 缓存）
// name 为空时等同于 GetMaster()

// ===== 查询方法 =====

func (a *App) IsModelAvailable(name ...string) bool
// 检查 Model 是否可用（发送 Hello 测试请求）
// name 为空时检查 Master Agent 的 model
```

#### 4.1.4 内部方法（非导出）

```go
func (a *App) getMaster() (*goreact.Agent, error)
// GetMaster 的实际实现（加锁版本）

func (a *App) resolveAgent(name string) (*goreact.Agent, error)
// ResolveAgent 的实际实现（加锁版本）
// 逻辑:
//   1. name 为空 → getMaster()
//   2. 检查 agentCache 命中 → 返回缓存
//   3. 从 agents.Get(name) 获取配置
//   4. 用 models.Get(cfg.Model) 获取 Model 配置
//   5. 构建 goreact.NewAgent(opts...) 并缓存
```

#### 4.1.5 core.Settings 结构体

```go
package core

import "path/filepath"

type Settings struct {
    Workspace   string // 工作区根目录 (e.g., ~/.mindx)
    Path        string // PWD 路径
    MasterAgent string // 默认 Agent 名称
}

// 目录解析方法（无副作用，纯计算）
func (s *Settings) SkillsDir() string
func (s *Settings) ModelsFile() string
func (s *Settings) ProgramDir() string
func (s *Settings) DocumentDir() string
func (s *Settings) DataDir() string
func (s *Settings) AgentsDir() string
func (s *Settings) RulesFile() string
func (s *Settings) SessionsDir() string
func (s *Settings) SchedulesDir() string
```

**关键变化**:
- ❌ 移除 `Addr string` (ws_addr) — 属于网络层
- ❌ 移除 `WSPath string` (ws_path) — 属于网络层
- ✅ 保留所有路径解析方法 — 引擎需要知道文件位置

---

### 4.2 internal/svc — 服务层

#### 4.2.1 文件结构

```
internal/
└── svc/
    ├── daemon.go       # Daemon 结构体 + 网关管理
    ├── dispatch.go     # 事件转发逻辑（从原 app.go 拆出）
    ├── server.go       # Server 合成器
    ├── wiring.go       # 命令注册（调整依赖）
    └── settings.go     # ~~删除~~ (移至 core)
```

#### 4.2.2 Daemon 结构体

```go
package svc

import (
    "context"
    
    "github.com/DotNetAge/gort/pkg/gateway"
    "github.com/DotNetAge/mindx/internal/core"
    "github.com/DotNetAge/mindx/pkg/logging"
    "github.com/DotNetAge/mindx/pkg/scheduler"
)

// Daemon wraps the MindX engine with network capabilities.
// It provides WebSocket gateway and scheduler services for remote UI clients.
//
// Usage:
//   - WebUI/MacUI connect via WebSocket to Daemon
//   - Scheduler executes timed tasks through Daemon
//   - TUI does NOT need Daemon (uses core.App directly)
type Daemon struct {
    app         *core.App
    gw          *gateway.Server
    scheduler   *scheduler.Scheduler
    schedulerDB *scheduler.FileSchedulerStore
    
    addr   string
    wsPath string
    logger logging.Logger
}

func NewDaemon(app *core.App, addr, wsPath string) *Daemon
// 创建 Daemon 实例（不自动启动 gateway）

func (d *Daemon) Start(ctx context.Context) error
// 启动 Gateway + Scheduler，阻塞直到 ctx 取消

func (d *Daemon) Gateway() *gateway.Server
func (d *Daemon) App() *core.App
func (d *Daemon) Scheduler() *scheduler.Scheduler
func (d *Daemon) SchedulerDB() *scheduler.FileSchedulerStore
```

#### 4.2.3 Daemon 核心方法

```go
// initGateway 懒创建 gateway server
func (d *Daemon) initGateway() {
    d.gw = gateway.New(
        gateway.WithAddr(d.addr),
        gateway.WithPath(d.wsPath),
        gateway.WithHandler(d.defaultHandler),
    )
}

// defaultHandler 处理 WebSocket 消息（从 dispatch.go:20 搬来）
func (d *Daemon) defaultHandler(msg *gateway.Message) {
    // 1. 解析 user.message notification
    // 2. parseAgentTarget(text) → (agentName, content)
    // 3. d.app.ResolveAgent(agentName)
    // 4. d.resolveSessionID(msg.SessionID)
    // 5. agent.EventsFiltered(filterReactEvents)
    // 6. go agent.Ask(sessionID, content)
    // 7. for event := range eventCh { d.forwardEvent(...) }
}

// forwardEvent 将 core.ReactEvent 转发为 gateway response（从 dispatch.go:124 搬来）
func (d *Daemon) forwardEvent(clientID string, event core.ReactEvent) {
    // 30+ 种事件类型的 switch-case 映射
    // e.g., ThinkingDelta → RespThinkingDelta
}

// executeScheduleCommand 执行调度任务
func (d *Daemon) executeScheduleCommand(ctx context.Context, agent, content string) error
```

#### 4.2.4 Server 合成器

```go
package svc

import (
    "context"
    
    "github.com/DotNetAge/mindx/internal/core"
)

// Server combines engine (App) with network services (Daemon).
// Use this when you need both local AI capabilities AND remote access.
//
// Typical usage:
//   server, _ := svc.NewServer(":1314", "/ws")
//   server.RegisterBuiltinCommands()
//   server.Start(ctx)
type Server struct {
    app    *core.App
    daemon *Daemon
}

func NewServer(addr, wsPath string) (*Server, error)
// 创建完整的服务实例（engine + daemon）

func (s *Server) Start(ctx context.Context) error
// 启动 daemon（阻塞）

func (s *Server) App() *core.App
func (s *Server) Daemon() *Daemon

func (s *Server) RegisterBuiltinCommands()
// 注册内置命令到 gateway（复用 wiring.go 逻辑）
```

---

## 5. TUI 改造方案

### 5.1 改造范围

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| [cmd/tui.go](../cmd/tui.go) | **修改** | 使用 `core.DefaultApp()` 替代空参数 |
| [component_root.go](../internal/client/component_root.go) | **大幅修改** | 移除 `gateway.Client`，改用 `core.App` |
| [session.go](../internal/client/session.go) | **简化** | 删除 WebSocket handler 注册（~50% 代码） |
| [fetch.go](../internal/client/fetch.go) | **删除** | 远程命令封装不再需要 |
| [registry.go](../internal/client/registry.go) | **修改** | 移除远程命令同步回调 |
| [component_inputbox.go](../internal/client/component_inputbox.go) | **微调** | 已有 `/command` 处理，基本不变 |

### 5.2 rootModel 结构变化

#### Before

```go
type rootModel struct {
    // 子组件
    contentPanel *ContentPanel
    statusBar    StatusBar
    inputBox     InputBox
    
    // 网络 ← 要删除
    client *gateway.Client  // ❌ WebSocket 客户端
    
    // 会话管理
    sessionReg *sessionRegistry
    outputCh   chan tea.Msg
    
    // 持久化
    chatManager *chatSessionManager
    
    // 共享状态
    registry         *SlashCommandRegistry
    currentAgent     string
    currentSessionID string
    
    executing bool
}
```

#### After

```go
type rootModel struct {
    // 子组件（不变）
    contentPanel *ContentPanel
    statusBar    StatusBar
    inputBox     InputBox
    
    // 引擎 ← 新增
    app *core.App  // ✅ 直接持有引擎实例
    
    // 会话管理（不变）
    sessionReg *sessionRegistry
    outputCh   chan tea.Msg
    
    // 持久化（不变）
    chatManager *chatSessionManager
    
    // 共享状态（不变）
    registry         *SlashCommandRegistry
    currentAgent     string
    currentSessionID string
    
    executing bool
    currentCancel context.CancelFunc  // 新增：用于取消进行中的 Ask()
}
```

### 5.3 Init() 方法改造

#### Before ([component_root.go:60-87](../internal/client/component_root.go#L60-L87))

```go
func (m *rootModel) Init() tea.Cmd {
    addr := os.Getenv("MINDX_WS_ADDR")
    if addr == "" {
        addr = "localhost:1314"
    }
    wsPath := os.Getenv("MINDX_WS_PATH")
    if wsPath == "" {
        wsPath = "/ws"
    }
    wsURL := fmt.Sprintf("ws://%s%s", addr, wsPath)
    m.client = gateway.NewClient(wsURL)  // ❌ 创建 WebSocket 客户端
    
    m.registry.SetCommandSender(func(name, args string) (string, error) {
        return m.client.SendCommand(name, args)  // ❌ 远程命令回调
    })
    
    RegisterHandlers(m.client, m.sessionReg, m.outputCh)  // ❌ 注册 WS handler
    
    return tea.Batch(
        connectWithRetry(m.client, connectAttempt{}),  // ❌ 连接服务器
        m.loadOrInitSession(),
    )
}
```

#### After

```go
func (m *rootModel) Init() tea.Cmd {
    var err error
    m.app, err = core.DefaultApp()  // ✅ 直接创建引擎实例
    if err != nil {
        return func() tea.Msg { return errMsg(err) }
    }
    
    // 本地命令查询函数（替代远程 SendCommand）
    m.registry.SetQueryFunc(func(queryType, name string) (string, error) {
        switch queryType {
        case "agents":
            return listAgentsLocal(m.app)
        case "models":
            return listModelsLocal(m.app)
        case "skills":
            return listSkillsLocal(m.app)
        default:
            return "", fmt.Errorf("unknown query type: %s", queryType)
        }
    })
    
    return m.loadOrInitSession()  // ✅ 不再连接 WebSocket
}
```

### 5.4 handleSend() 方法改造（核心短路逻辑）

#### Before ([component_root.go:351-418](../internal/client/component_root.go#L351-L418))

```go
func (m *rootModel) handleSend(msg sendMsg) (tea.Model, tea.Cmd) {
    if !m.client.IsConnected() {  // ❌ 检查连接状态
        return m, /* 显示错误 */
    }
    
    // ... 解析 @agent_name ...
    
    return m, tea.Batch(
        sendToServerWithSession(m.client, text, sessionID),  // ❌ 发送到服务器
        waitEvent(m.outputCh),  // 等待 WebSocket 事件
    )
}
```

#### After

```go
func (m *rootModel) handleSend(msg sendMsg) (tea.Model, tea.Cmd) {
    m.executing = true
    
    // 解析 @agent_name（本地处理）
    text := msg.text
    if strings.HasPrefix(text, "@") {
        parts := strings.SplitN(text, " ", 2)
        if len(parts) >= 2 {
            targetAgent := parts[0][1:]
            for _, a := range m.inputBox.suggestAg.agents {
                if a.name == targetAgent {
                    m.currentAgent = targetAgent
                    m.currentModel = a.model
                    m.statusBar.SetAgent(m.currentAgent, m.currentModel)
                    break
                }
            }
            text = strings.TrimSpace(parts[1])
        }
    }
    
    if strings.TrimSpace(text) == "" {
        return m, nil
    }
    
    sessionID := m.getOrCreateSessionID()
    answer := m.contentPanel.CreateAnswer(sessionID, m.currentAgent)
    m.sessionReg.add(sessionID, answer)
    answer.AppendResult(msg.text)
    
    // ✅ 直接调用引擎，短路网络层
    agent, err := m.app.ResolveAgent(m.currentAgent)
    if err != nil {
        return m, func() tea.Msg { return errMsg(err) }
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    m.currentCancel = cancel
    
    eventCh, cancelEvents := agent.EventsFiltered(func(e core.ReactEvent) bool {
        return filterReactEvents(e)  // 复用原有过滤逻辑
    })
    
    go func() {
        defer cancelEvents()
        _, err = agent.Ask(sessionID, text)
        if err != nil {
            trySend(m.outputCh, errMsg{err})
        }
    }()
    
    go m.consumeEvents(eventCh, sessionID)  // ✅ 直接消费事件流
    
    return m, waitEvent(m.outputCh)
}

func (m *rootModel) consumeEvents(eventCh <-chan core.ReactEvent, sessionID string) {
    for event := range eventCh {
        trySend(m.outputCh, agentAnswerUpdateMsg{
            sessionID:   sessionID,
            contentType: string(event.Type),  // ✅ 直接用 ReactEvent.Type
            content:     stringifyEventData(event.Data),
        })
    }
    trySend(m.outputCh, agentAnswerDoneMsg{sessionID: sessionID})
}
```

### 5.5 指令系统处理

#### 现有优势（无需大改）

[component_inputbox.go:118-147](../internal/client/component_inputbox.go#L118-L147) 已经实现了本地 `/command` 处理：

```go
if strings.HasPrefix(text, "/" ) {
    cmdName := parts[0]  // "/models"
    searchName := strings.TrimPrefix(cmdName, "/")
    cmd := b.registry.Find(searchName)
    if cmd != nil && cmd.Run != nil {
        result := cmd.Run(args)  // ✅ 直接执行本地函数
    }
}
```

#### 指令分类体系

| 类型 | 示例 | 执行方式 | 实现 |
|------|------|----------|------|
| **纯本地** | `/clear`, `/quit`, `/help` | 直接执行 | `system.go` |
| **引擎查询** | `/models`, `/agents` | 调用 `app.Agents()/app.Models()` | `catalog.go` (改造) |
| **引擎操作** | `/model gpt-4` | 更新 `currentAgent/currentModel` | 新增 |
| **对话内容** | 其他输入 | 发送给 `agent.Ask()` | handleSend() |

#### 本地查询函数示例

```go
func listAgentsLocal(app *core.App) (string, error) {
    registry := app.Agents()
    agents := registry.List()
    
    var result []map[string]string
    for _, agent := range agents {
        result = append(result, map[string]string{
            "name":        agent.Name,
            "role":        agent.Role,
            "description": agent.Description,
            "model":       agent.Model,
        })
    }
    
    data, _ := json.Marshal(result)
    return string(data), nil
}
```

### 5.6 会话恢复机制（保持不变）

[component_root.go:90-103](../internal/client/component_root.go#L90-L103) 的会话恢复逻辑基本不变：

```go
func (m *rootModel) loadOrInitSession() tea.Cmd {
    return func() tea.Msg {
        if m.chatManager.Exists() {
            session, err := m.chatManager.Load()
            if err == nil && session.AgentName != "" && session.SessionID != "" {
                m.currentAgent = session.AgentName
                m.currentSessionID = session.SessionID
                m.statusBar.SetAgent(m.currentAgent, m.currentModel)
                
                // ✅ 可选：从引擎加载历史消息用于显示
                msgs, _ := m.app.SessionDB().Get(context.Background(), session.SessionID)
                return sessionLoadedMsg{
                    agentName: session.AgentName,
                    sessionID: session.SessionID,
                    messages:  msgs,
                }
            }
        }
        return sessionInitRequiredMsg{}
    }
}
```

---

## 6. 迁移策略

### 6.1 渐进式迁移路线图

```
Phase 1: 引擎提取 (2-3天)
├── 创建 internal/core/app.go
├── 创建 internal/core/settings.go
├── 从 svc/app.go 提取引擎逻辑
└── 单元测试: DefaultApp(), ResolveAgent(), GetMaster()
         ↓
Phase 2: Daemon 拆分 (2-3天)
├── 创建 internal/svc/daemon.go
├── 从 svc/app.go + dispatch.go 提取服务逻辑
├── 创建 internal/svc/server.go (合成器)
└── 集成测试: Daemon.Start() + WebSocket 连接
         ↓
Phase 3: TUI 改造 (3-4天)
├── 修改 component_root.go (移除 gateway 依赖)
├── 简化 session.go (删除 ~50% 代码)
├── 删除 fetch.go
├── 改造 registry.go (本地查询)
└── 端到端测试: 完整对话流程
         ↓
Phase 4: 清理与优化 (1-2天)
├── 删除 svc/app.go 中的废弃代码
├── 更新 tests (app_integration_test.go)
├── 性能测试: 长对话、高频输出
└── 文档更新: README, ARCHITECTURE.md
```

### 6.2 向后兼容保证

#### 兼容性矩阵

| 功能 | 重构前 | 重构后 | 兼容？ |
|------|--------|--------|--------|
| `mindx tui` | 需要 gateway | 独立运行 | ✅ 行为更好 |
| `mindx start` | 启动 gateway | 启动 daemon | ✅ 接口不变 |
| WebUI 连接 | 通过 gateway | 通过 daemon | ✅ 无感知 |
| 会话持久化 | 文件系统 | 文件系统 | ✅ 完全兼容 |
| `/command` | 混合（本地+远程） | 纯本地 | ✅ 功能增强 |
| 调度任务 | gateway 内置 | daemon 内置 | ✅ 无变化 |

#### 废弃标记策略

```go
// Phase 1-2: 标记为 Deprecated
// svc/app.go
// Deprecated: Use core.DefaultApp() instead. This method will be removed in v3.0.
func DefaultApp() (*App, error) {
    // 内部委托给 core.DefaultApp()
}

// Phase 4: 完全删除
```

### 6.3 回滚方案

如果 Phase 3 (TUI 改造) 出现问题：

```bash
# 方案 A: 回退到旧版 TUI（保留 gateway 依赖）
git checkout HEAD~1 -- cmd/tui.go internal/client/

# 方案 B: 双模式启动（通过环境变量切换）
export MINDX_MODE=gateway  # 使用旧版（走 WebSocket）
export MINDX_MODE=local    # 使用新版（直连引擎）
```

---

## 7. 接口契约

### 7.1 core.App 公开 API

```go
package core

// ===== 构造 =====
func DefaultApp() (*App, error)
// Precondition: MINDX_WORKSPACE 环境变量可访问（或使用默认值 ~/.mindx）
// Postcondition: 返回的 App 实例可安全并发使用
// Error conditions:
//   - 无法加载 agents 目录 → error
//   - 无法加载 models 文件 → error
//   - 无法创建 session store → warn (不返回 error)

// ===== Agent 管理 =====
func (a *App) GetMaster() (*goreact.Agent, error)
// Returns: Master Agent 实例（懒加载 + 缓存）
// Errors:
//   - MasterAgent 未配置且无可用 Agent → error
//   - Agent Model 未配置 → error
//   - Model 不存在 → error
//   - goreact.NewAgent() 失败 → error
// Thread-safety: 安全（内部 RWMutex 保护）

func (a *App) ResolveAgent(name string) (*goreact.Agent, error)
// Parameters:
//   - name: Agent 名称（空字符串 = Master Agent）
// Returns: 对应的 Agent 实例（懒加载 + 缓存）
// Errors: 同 GetMaster()
// Thread-safety: 安全

// ===== 查询 =====
func (a *App) IsModelAvailable(name ...string) bool
// Parameters:
//   - name: Model 名称（可选，空 = Master Agent 的 model）
// Returns: Model 是否可用（发送 Hello 测试）
// Side-effects: 发送真实 LLM 请求（耗时 ~1-3秒）

// ===== 访问器 =====
func (a *App) Agents() *goreact.AgentRegistry
func (a *App) Models() *goreact.ModelRegistry
func (a *App) SessionDB() *session.FileSessionStore
func (a *App) RuleRegistry() core.RuleRegistry
func (a *App) Settings() *Settings
func (a *App) SetLogger(l logging.Logger)
```

### 7.2 Daemon 公开 API

```go
package svc

// ===== 构造 =====
func NewDaemon(app *core.App, addr, wsPath string) *Daemon
// Parameters:
//   - app: 引擎实例（必须非 nil）
//   - addr: 监听地址 (e.g., ":1314", "0.0.0.0:8080")
//   - wsPath: WebSocket 路径 (e.g., "/ws")
// Postcondition: Daemon 实例已就绪但未启动（需显式调用 Start()）

// ===== 生命周期 =====
func (d *Daemon) Start(ctx context.Context) error
// Behavior:
//   1. 懒初始化 gateway server
//   2. 启动 scheduler
//   3. 启动 gateway（阻塞）
//   4. 等待 ctx.Done()
//   5. 优雅关闭（stop channels → stop scheduler → shutdown gateway）
// Errors:
//   - gateway.Start() 失败 → error
//   - 关闭过程中的错误 → warn (仍返回 nil)

// ===== 访问器 =====
func (d *Daemon) Gateway() *gateway.Server
func (d *Daemon) App() *core.App
func (d *Daemon) Scheduler() *scheduler.Scheduler
func (d *Daemon) SchedulerDB() *scheduler.FileSchedulerStore
```

### 7.3 Server 公开 API

```go
package svc

func NewServer(addr, wsPath string) (*Server, error)
// Behavior:
//   1. 调用 core.DefaultApp() 创建引擎
//   2. 调用 NewDaemon(app, addr, wsPath) 创建守护进程
//   3. 组合成 Server 实例
// Errors: core.DefaultApp() 失败 → error

func (s *Server) Start(ctx context.Context) error
// Delegates to s.daemon.Start(ctx)

func (s *Server) RegisterBuiltinCommands()
// Registers /help, /agents, /models, /skills to gateway
// Precondition: Gateway 已初始化（或懒初始化）
```

### 7.4 TUI 与引擎交互协议

#### 消息格式

TUI 通过 `agentAnswerUpdateMsg` 消费引擎事件：

```go
type agentAnswerUpdateMsg struct {
    sessionID   string
    contentType string  // 对应 core.ReactEvent.Type
    content     string  // 序列化后的 event.Data
}
```

**contentType 映射表**:

| core.ReactEvent.Type | contentType | TUI 处理方式 |
|---------------------|-------------|--------------|
| `ThinkingDelta` | `"thinking"` | AppendThinking() |
| `ThinkingDone` | `"thinking_done"` | SetThinkingDone() |
| `ActionStart` | `"action_start"` | AppendAction() |
| `ActionProgress` | `"action_progress"` | SetActionProgress() |
| `ActionResult` | `"action_result"` | parseActionResult() |
| `FinalAnswer` | `"result"` | AppendResult() |
| `Error` | `"error"` | AppendError() |
| `ExecutionSummary` | (特殊) | agentAnswerDoneMsg |

#### 生命周期

```
用户输入
  ↓
handleSend()
  ↓
agent.Ask(sessionID, content)  [goroutine]
  ↓
[ReactEvent stream]
  ↓
consumeEvents()  [goroutine]
  ↓
outputCh ← agentAnswerUpdateMsg × N
  ↓
Root.Update() → routeToAnswer()
  ↓
ContentPanel 渲染
  ↓
[最后一个事件]
  ↓
outputCh ← agentAnswerDoneMsg
  ↓
handleSessionDone()
  ↓
executing = false  (等待下一次输入)
```

---

## 8. 风险评估与缓解

### 8.1 高风险项

#### 🔴 Risk 1: 流式输出性能瓶颈

**场景**: Agent 输出高频事件（每秒数十个 thinking_delta）

**影响**: TUI 渲染卡顿、内存占用飙升

**概率**: 中 (30%)

**缓解措施**:
```go
// 方案 A: 带缓冲 Channel
eventCh := make(chan core.ReactEvent, 256)

// 方案 B: 批量消费（推荐）
func (m *rootModel) consumeEvents(eventCh <-chan core.ReactEvent, sessionID string) {
    ticker := time.NewTicker(16 * time.Millisecond)  // ~60fps
    defer ticker.Stop()
    
    batch := []agentAnswerUpdateMsg{}
    
    for {
        select {
        case event, ok := <-eventCh:
            if !ok {
                flush(batch)
                trySend(m.outputCh, agentAnswerDoneMsg{sessionID: sessionID})
                return
            }
            batch = append(batch, convertEvent(event))
            
        case <-ticker.C:
            flush(batch)
            batch = batch[:0]
        }
    }
}
```

**回退方案**: 如果批量消费引入延迟 > 100ms，回退到逐事件模式

---

#### 🔴 Risk 2: 会话并发冲突

**场景**: TUI 和 Daemon 同时写入同一 session 文件

**影响**: 数据损坏、会话丢失

**概率**: 低 (10%) — 大多数用户不会同时运行两者

**缓解措施**:
```go
// 方案 A: TUI 只读模式检测（推荐）
func (m *rootModel) Init() tea.Cmd {
    if isDaemonRunning() {
        m.mode = ReadOnly  // 只读模式：可查看但不能执行调度任务
    } else {
        m.mode = Active   // 独立模式：完整功能
    }
    // ...
}

func isDaemonRunning() bool {
    // 检查 PID 文件或端口占用
    conn, err := net.DialTimeout("tcp", "localhost:1314", 100*time.Millisecond)
    if err == nil {
        conn.Close()
        return true
    }
    return false
}

// 方案 B: 文件锁（备选）
lock := flock.New(filepath.Join(settings.SessionsDir(), ".lock"))
locked, _ := lock.TryLock()
if !locked {
    // Daemon 正在运行，TUI 退让
}
```

---

### 8.2 中风险项

#### 🟡 Risk 3: 事件类型映射遗漏

**场景**: GoReact 新增 ReactEvent 类型，但 TUI 未及时更新

**影响**: 某些事件被静默丢弃

**概率**: 中 (40%)

**缓解措施**:
```go
// 在 consumeEvents() 中添加 fallback
func (m *rootModel) consumeEvents(eventCh <-chan core.ReactEvent, sessionID string) {
    for event := range eventCh {
        msg, ok := convertEvent(event)
        if !ok {
            // ⚠️ 未知事件类型，记录日志但不崩溃
            m.logger.Warn("unhandled event type", "type", event.Type)
            continue
        }
        trySend(m.outputCh, msg)
    }
}

func convertEvent(event core.ReactEvent) (agentAnswerUpdateMsg, bool) {
    switch event.Type {
    case core.ThinkingDelta:
        return agentAnswerUpdateMsg{..., contentType: "thinking", ...}, true
    // ... 其他已知类型 ...
    default:
        return agentAnswerUpdateMsg{}, false  // 未知类型
    }
}
```

---

#### 🟡 Risk 4: 测试覆盖率下降

**场景**: 重构后部分集成测试失效

**影响**: 回归缺陷未被发现

**概率**: 中 (35%)

**缓解措施**:
```go
// 1. 保留原有集成测试（调整为使用 Server）
func TestServerIntegration(t *testing.T) {
    server, err := svc.NewServer(":0", "/ws")
    require.NoError(t, err)
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    go func() {
        err := server.Start(ctx)
        require.ErrorIs(t, err, context.Canceled)
    }()
    
    // 等待服务器就绪
    time.Sleep(100 * time.Millisecond)
    
    // 执行测试...
}

// 2. 新增 TUI 单元测试（mock core.App）
func TestHandleSend(t *testing.T) {
    mockApp := &MockApp{}
    m := &rootModel{app: mockApp}
    
    msg := sendMsg{text: "Hello"}
    model, cmd := m.handleSend(msg)
    
    assert.True(t, model.(*rootModel).executing)
    assert.NotNil(t, cmd)
}
```

---

### 8.3 低风险项

#### 🟢 Risk 5: 环境变量兼容性

**场景**: 用户已配置 `MINDX_WS_ADDR`, `MINDX_WS_PATH` 等环境变量

**影响**: TUI 启动时报未知变量警告

**概率**: 低 (10%)

**缓解措施**:
```go
// core.DefaultApp() 中忽略网络相关环境变量
func DefaultApp() (*App, error) {
    settings := &Settings{
        Workspace:   os.Getenv("MINDX_WORKSPACE"),
        Path:        os.Getenv("MINDX_PWD_PATH"),
        MasterAgent: os.Getenv("MINDX_MASTER"),
        // ❌ 不读取 MINDX_WS_ADDR, MINDX_WS_PATH
    }
    // ...
}
```

---

## 9. 实施计划

### 9.1 任务分解与时间估算

| Phase | 任务 | 预估工时 | 优先级 | 依赖 |
|-------|------|---------|--------|------|
| **P1** | 创建 `internal/core/app.go` | 4h | P0 | - |
| | 创建 `internal/core/settings.go` | 1h | P0 | - |
| | 从 `svc/app.go` 提取引擎逻辑 | 3h | P0 | core/app.go |
| | 编写单元测试 (DefaultApp, ResolveAgent) | 3h | P0 | core/app.go |
| | **小计** | **11h** | | |
| **P2** | 创建 `internal/svc/daemon.go` | 4h | P1 | P1 完成 |
| | 从 `dispatch.go` 提取事件转发逻辑 | 3h | P1 | daemon.go |
| | 创建 `internal/svc/server.go` | 2h | P1 | daemon.go |
| | 更新 `wiring.go` 依赖 | 1h | P1 | server.go |
| | 编写集成测试 (Daemon + WebSocket) | 3h | P1 | daemon.go |
| | **小计** | **13h** | | |
| **P3** | 修改 `component_root.go` (Init/handleSend) | 6h | P0 | P1 完成 |
| | 简化 `session.go` (删除 WS handlers) | 2h | P0 | - |
| | 删除 `fetch.go` | 0.5h | P0 | - |
| | 改造 `registry.go` (本地查询) | 2h | P1 | - |
| | 实现 `consumeEvents()` 事件消费 | 3h | P0 | component_root.go |
| | 端到端测试 (完整对话流程) | 4h | P0 | 以上全部 |
| | **小计** | **17.5h** | | |
| **P4** | 清理 `svc/app.go` 废弃代码 | 2h | P2 | P3 完成 |
| | 更新 `app_integration_test.go` | 2h | P2 | P3 完成 |
| | 性能测试 (长对话、高频输出) | 3h | P2 | P3 完成 |
| | 文档更新 (README, ARCHITECTURE) | 2h | P2 | 全部完成 |
| | **小计** | **9h** | | |
| **总计** | | **~50.5h (~6个工作日)** | | |

### 9.2 里程碑

| 里程碑 | 交付物 | 验收标准 | 时间点 |
|--------|--------|-----------------|
| **M1: 引擎就绪** | `internal/core/` 包 | 所有单元测试通过 | Day 2 |
| **M2: 服务拆分** | `internal/svc/daemon.go` + `server.go` | Daemon 可接受 WebSocket 连接 | Day 4 |
| **M3: TUI 独立** | TUI 可单进程运行 | 完整对话流程正常（无 gateway） | Day 7 |
| **M4: 生产就绪** | 代码清理 + 文档 | 所有测试通过 + 性能达标 | Day 9 |

### 9.3 并行开发策略

```
开发者 A (架构师)          开发者 B (TUI 专家)
     │                          │
     ├─ P1: core 包开发         │
     │   (Day 1-2)              │
     │                          │
     ├─ P2: daemon 拆分         ├─ 准备工作:
     │   (Day 3-4)              │   阅读 TUI 代码
     │                          │   理解 component_root.go
     ├─ P4: 清理与优化          │
     │   (Day 8-9)              ├─ P3: TUI 改造
     │                          │   (Day 5-7)
     └──────────────────────────┴─ P4: 协助测试
                                │   (Day 8-9)
```

---

## 10. 验收标准

### 10.1 功能验收

#### AC1: TUI 单进程启动

```bash
# Given: 用户在终端执行
$ mindx

# Then: TUI 界面正常显示（无 gateway 依赖）
# And: 状态栏显示 "Connected (local)"
# And: 可以正常输入并发送消息
# And: Agent 响应正确渲染
```

**自动化测试**:
```go
func TestTUILocalMode(t *testing.T) {
    p := client.NewProgram()  // 内部使用 core.DefaultApp()
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    go func() {
        _, err := p.Run()
        assert.NoError(t, err)
    }()
    
    // 模拟用户输入
    p.Send(tea.KeyPressMsg{Key: 'H'})
    p.Send(tea.KeyPressMsg{Key: 'i'})
    // ...
    p.Send(tea.KeyPressMsg{Key: tea.KeyEnter})
    
    // 验证响应
    select {
    case <-ctx.Done():
        t.Fatal("timeout waiting for response")
    case <-time.After(5 * time.Second):
        // 检查 UI 状态
    }
}
```

---

#### AC2: 消息短路生效

```bash
# Given: TUI 运行中（无 gateway 进程）
# When: 用户发送消息 "Hello"

# Then: 日志中无 WebSocket 相关输出
# And: 响应延迟 < 500ms（相比之前降低 50%+）
```

**性能基准**:

| 指标 | 重构前 (Gateway) | 重构后 (Local) | 目标改善 |
|------|------------------|----------------|---------|
| 首次响应延迟 | 800ms - 1200ms | 300ms - 600ms | **≥50%↓** |
| thinking_delta 延迟 | 50ms - 100ms | < 10ms | **≥80%↓** |
| 内存占用 (空闲) | 45MB - 60MB | 30MB - 40MB | **≥25%↓** |
| CPU 占用 (对话中) | 15% - 25% | 8% - 15% | **≥40%↓** |

---

#### AC3: /command 指令正常

```bash
# Given: TUI 运行中
# When: 用户输入以下指令

/models     → 显示本地 Model 列表（不从服务器获取）
/agents     → 显示本地 Agent 列表
/clear      → 清屏
/help       → 显示帮助信息
/model gpt4 → 切换模型（更新 currentModel）
```

---

#### AC4: 会话恢复正常

```bash
# Given: 用户之前有对话历史
# When: 重启 TUI

# Then: 自动恢复上次的 Agent 和 SessionID
# And: 可以继续之前的对话上下文
```

---

#### AC5: Daemon 模式兼容

```bash
# Terminal 1: 启动 Daemon
$ mindx start
# Output: MindX daemon starting on ws://localhost:1314/ws

# Terminal 2: 启动 WebUI (模拟)
$ curl -i -N -H "Connection: Upgrade" \
       -H "Upgrade: websocket" \
       -H "Sec-WebSocket-Key: test" \
       -H "Sec-WebSocket-Version: 13" \
       http://localhost:1314/ws
# Response: 101 Switching Protocols (WebSocket established)

# When: 通过 WebSocket 发送消息
# Then: Daemon 正常处理并返回响应
```

---

### 10.2 代码质量验收

#### CQ1: 零循环依赖

```bash
# 执行依赖分析
$ go mod graph | grep -E "(svc.*→.*core|core.*→.*svc)"
# Expected: 只有 svc → core (单向)
# Forbidden: core → svc (循环!)
```

**工具验证**:
```bash
# 使用 golangci-lint 检查
$ golangci-lint run --enable=cyclop ./...

# 或使用 depguard
$ go install github.com/OpenPeeDeeP/depguard@latest
$ depguard check --config=.depguard.yml
```

---

#### CQ2: 测试覆盖率

| 包 | 最低覆盖率 | 目标覆盖率 |
|----|-----------|-----------|
| `internal/core` | 80% | 90% |
| `internal/svc/daemon` | 70% | 85% |
| `internal/client` | 60% | 75% |

```bash
# 生成覆盖率报告
$ go test -coverprofile=coverage.out ./...
$ go tool cover -html=coverage.out -o coverage.html
```

---

#### CQ3: 性能回归测试

```bash
# 基准测试
$ go test -bench=. -benchmem ./internal/core/
# Expected:
# BenchmarkResolveAgent-8    1000000    1200 ns/op    256 B/op    5 allocs
# BenchmarkGetMaster-8       500000    2100 ns/op    512 B/op    8 allocs

# 对比重构前后
$ benchstat old.txt new.txt
# Expected: 无显著退化 (< 10%)
```

---

### 10.3 文档验收

#### D1: 架构文档更新

- [ ] `ARCHITECTURE.md` 反映新的三层架构
- [ ] `README.md` 更新启动方式（`mindx` vs `mindx start`）
- [ ] 代码注释覆盖所有公开 API（godoc 友好）

#### D2: 迁移指南

- [ ] `MIGRATION.md`: 从旧版升级的步骤
- [ ] Breaking Changes 列表（如有）
- [ ] 环境变量变更说明

---

## 附录

### A. 术语表

| 术语 | 定义 |
|------|------|
| **Engine (引擎)** | Agent + Model + Session + Rules 的集合，纯 library |
| **Daemon (守护进程)** | 后台服务进程，提供 Gateway + Scheduler |
| **Server (服务器)** | Engine + Daemon 的组合，完整服务实例 |
| **TUI (Terminal UI)** | 基于 Bubble Tea 的终端界面 |
| **短路 (Short-circuit)** | TUI 直接调用 Engine，跳过网络层 |
| **ReactEvent** | GoReact 的事件类型（`core.ReactEvent`） |
| **Gateway** | WebSocket JSON-RPC 服务器（gort） |

### B. 参考资源

- [GoReact Documentation](https://github.com/DotNetAge/goreact)
- [Bubble Tea Framework](https://charm.sh/bubbletea/)
- [gort (Gateway)](https://github.com/DotNetAge/gort)
- [Design Discussion Log](./TUI-Final.md) — 本次重构的决策记录

### C. 变更日志

| 版本 | 日期 | 作者 | 变更内容 |
|------|------|------|---------|
| v1.0 | 2026-05-10 | Architecture Team | 初稿完成 |

---

## ✅ 审批确认

- [ ] 架构师审批
- [ ] Tech Lead 审批
- [ ] 产品经理确认（如有业务影响）

**下一步行动**: 
- 审批通过后进入 **Phase 1: 引擎提取**
- 预计开始时间: _______________
- 预计完成时间: _______________
