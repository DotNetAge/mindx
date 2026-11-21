# Gateway 包 API 文档

## 概述

`gateway` 包提供基于 **JSON-RPC 2.0 over WebSocket** 的双向通信服务端（Server）和客户端（Client）。采用标准化的 JSON-RPC 协议，替代了早期的管道协议（BEGN/TEXT/JSON/CMD）。

**核心设计原则：**
- Server/Client 均使用 Option 模式构造
- JSON-RPC 2.0 标准：Request（有 `id`）/ Response（匹配 `id`）/ Notification（无 `id`）
- 对等通信：Server 和 Client 地位完全对等，可互相发起调用和推送

---

## 目录

1. [JSON-RPC 协议](#1-json-rpc-协议)
2. [Server API](#2-server-api)
3. [Client API](#3-client-api)
4. [消息与类型定义](#4-消息与类型定义)
5. [Channel 集成](#5-channel-集成)
6. [错误定义](#6-错误定义)

---

## 1. JSON-RPC 协议

### 1.1 请求（Request）

有 `id`，期望获得匹配的 Response：

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "method": "agents",
  "params": {"args": ""}
}
```

### 1.2 响应（Response）

`id` 必须与请求相同：

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "result": [...]
}
```

### 1.3 通知（Notification）

无 `id`，不期望响应（用于服务端主动推送）：

```json
{
  "jsonrpc": "2.0",
  "method": "table",
  "params": {
    "type": "table",
    "title": "Agents",
    "data": {"headers": ["Name"], "rows": [["writer"]]}
  }
}
```

### 1.4 错误（Error）

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "error": {
    "code": -32601,
    "message": "Method not found: unknown_method"
  }
}
```

### 1.5 标准错误码

| 错误码 | 描述 | 场景 |
|--------|------|------|
| `-32700` | Parse error | JSON 解析失败 |
| `-32600` | Invalid request | 不符合 JSON-RPC 规范 |
| `-32601` | Method not found | 方法未注册 |
| `-32602` | Invalid params | 参数类型/格式错误 |
| `-32603` | Internal error | 服务器内部错误 |

### 1.6 完整交互流程

```
客户端                                    服务端
  │                                        │
  │  Request: {"id":"1","method":"agents"}  │
  │ ──────────────────────────────────────►│
  │                                        │ → 执行 handlers["agents"]
  │  Response: {"id":"1","result":[...]}    │
  │ ◄──────────────────────────────────────│
  │                                        │
  │  Notification: {"method":"table",...}   │ ← 主动推送
  │ ◄──────────────────────────────────────│
  │                                        │
```

---

## 2. Server API

### 2.1 构造函数

```go
func New(opts ...Option) *Server
```

**Option 列表：**

| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `WithAddr(addr)` | string | `:8081` | 监听地址（如 `0.0.0.0:9090`） |
| `WithPort(port)` | int | `8081` | 端口号（自动拼接为 `:{port}`），与 WithAddr 二选一 |
| `WithPath(path)` | string | `/ws` | WebSocket 端点路径 |
| `WithHandler(h)` | MessageHandler | nil | 通知消息处理器（收到 Notification 时触发） |
| `WithSessionTimeout(d)` | Duration | 30 分钟 | 会话超时时间 |
| `WithHeartbeat(cfg)` | *HeartbeatConfig | nil | 心跳监控配置 |
| `WithWSConfig(cfg)` | *WSConfig | localhost-only | WebSocket 来源白名单 |
| `WithChannels(chs)` | []channel.Channel | nil | 启动时批量注册渠道 |

**示例：**
```go
server := gateway.New(
    gateway.WithPort(8080),
    gateway.WithPath("/ws"),
    gateway.WithSessionTimeout(10 * time.Minute),
)
```

### 2.2 生命周期

#### Start()

```go
func (s *Server) Start() error
```

- **行为：** 阻塞式启动 WebSocket 服务器，内部会进行 TCP 可达性探测（最长 2 秒）
- **返回：** `ErrAlreadyRunning` 如果已在运行；否则阻塞直到服务器停止或出错
- **注意：** 通常在 goroutine 中调用

#### Shutdown()

```go
func (s *Server) Shutdown(ctx context.Context) error
```

- **行为：** 优雅关闭服务器，清理所有客户端的 session
- **参数：** `ctx` — 关闭超时上下文
- **返回：** `ErrNotRunning` 如果未在运行

#### IsRunning()

```go
func (s *Server) IsRunning() bool
```

返回当前运行状态。

### 2.3 方法注册

#### RegisterMethod()

```go
func (s *Server) RegisterMethod(method string, handler MethodHandler)
```

注册一个 JSON-RPC 方法处理器。

```go
type MethodHandler func(ctx context.Context, params json.RawMessage) (any, error)
```

**示例：**
```go
server.RegisterMethod("users.list", func(ctx context.Context, params json.RawMessage) (any, error) {
    return users, nil
})
```

#### RegisterCommand()

```go
func (s *Server) RegisterCommand(name string, handler func(ctx *CommandContext) (any, error), description string)
```

注册命令，**自动将其注册为同名的 JSON-RPC 方法**（不再通过 `command.execute` 代理）。

`CommandContext` 结构：

```go
type CommandContext struct {
    ClientID string  // 调用方客户端 ID（仅服务端回调时可用）
    Args     string  // 命令参数字符串
}
```

`CommandContext` 方法：

| 方法 | 说明 |
|------|------|
| `RespondWithType(type, title, data)` | 向调用方推送类型化响应（Notification） |
| `Server() *Server` | 获取 Server 实例引用 |

**示例：**
```go
server.RegisterCommand("agents", func(ctx *gateway.CommandContext) (any, error) {
    agents := listAgents()
    ctx.RespondWithType(gateway.RespTable, "Available Agents", map[string]interface{}{
        "headers": []string{"Name", "Role", "Description"},
        "rows":    toRows(agents),
    })
    return nil, nil
}, "显示智能体列表")
```

#### CommandList()

```go
func (s *Server) CommandList() map[string]string
```

返回所有已注册命令的名称和描述。

### 2.4 内置方法

| 方法名 | 说明 | 参数 | 返回 |
|--------|------|------|------|
| `command.list` | 列出所有已注册命令 | 无 | `[]CommandInfo` |

`command.list` 在 `New()` 时自动注册，无需手动注册。

### 2.5 客户端操作

#### Notify()

```go
func (s *Server) Notify(clientID, method string, params any) error
```

向指定客户端推送通知。

- **clientID:** 目标客户端 ID
- **method:** 通知方法名
- **params:** 通知参数（任意可序列化对象）

#### BroadcastNotification()

```go
func (s *Server) BroadcastNotification(method string, params any)
```

向所有已连接客户端广播通知。

#### Call()

```go
func (s *Server) Call(ctx context.Context, clientID, method string, params any) (json.RawMessage, error)
```

向客户端发起请求并等待响应（服务端发起的 RPC）。

- **ctx:** 超时控制上下文
- **clientID:** 目标客户端 ID
- **method:** 方法名
- **返回:** `json.RawMessage` 结果，或错误

### 2.6 旧版发送方法（便捷包装）

以下方法均为 `Notify` 的便捷包装，内部转换为 JSON-RPC Notification：

| 方法 | 等价调用 |
|------|----------|
| `Send(to, message)` | `Notify(to, "message", {"text": message})` |
| `Broadcast(message)` | `BroadcastNotification("message", {"text": message})` |
| `BroadcastMessage(message)` | 同上 |
| `SendJSON(to, v)` | `Notify(to, "json", v)` |
| `BroadcastJSON(v)` | `BroadcastNotification("json", v)` |
| `SendFile(to, filename)` | 读取文件后发送 |
| `BroadcastFile(filename)` | 广播文件 |
| `SendBatch(to, msgs)` | 逐条 Notify |
| `BroadcastBatch(msgs)` | 逐条 Broadcast |

### 2.7 连接管理

#### ClientCount()

```go
func (s *Server) ClientCount() int
```

返回当前在线客户端数量。

#### GetClient()

```go
func (s *Server) GetClient(clientID string) *client
```

获取指定客户端的引用。

### 2.8 通知注册（针对单个客户端）

#### OnNotification()

```go
func (s *Server) OnNotification(clientID, method string, handler NotificationHandler)
```

为指定客户端注册服务端发起的调用通知处理器（用于双向 RPC）。

```go
type NotificationHandler func(ctx context.Context, params json.RawMessage)
```

---

## 3. Client API

### 3.1 构造函数

```go
func NewClient(addr string) *Client
```

| 参数 | 说明 |
|------|------|
| `addr` | 完整 WebSocket URL（如 `ws://localhost:8081/ws`） |

**示例：**
```go
client := gateway.NewClient("ws://localhost:8081/ws")
```

### 3.2 连接管理

#### Connect()

```go
func (c *Client) Connect(ctx context.Context) error
```

建立 WebSocket 连接。内部启动 readLoop 和 writeLoop 协程。

- **返回：** 连接失败时的 error

#### ConnectSync()

```go
func (c *Client) ConnectSync() error
```

不使用 context 建立连接（向后兼容）。

#### Close()

```go
func (c *Client) Close() error
```

关闭连接，清理所有 pending 请求。

#### IsConnected()

```go
func (c *Client) IsConnected() bool
```

返回连接状态。

### 3.3 JSON-RPC 方法

#### Call()

```go
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error)
```

发送请求并等待响应。

- 自动生成 UUID 作为 `id`
- 在 `pending` map 中注册等待 channel
- 收到匹配 `id` 的 Response 后返回
- 超时由 `ctx` 控制

**示例：**
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := client.Call(ctx, "agents", nil)
if err != nil {
    log.Fatalf("call failed: %v", err)
}
```

#### Notify()

```go
func (c *Client) Notify(method string, params any) error
```

发送通知（无 `id`，不期望响应）。

**示例：**
```go
client.Notify("user.message", map[string]string{"text": "Hello"})
```

#### On()

```go
func (c *Client) On(method string, handler NotificationHandler)
```

注册服务端通知的处理器。

**示例：**
```go
client.On("table", func(ctx context.Context, params json.RawMessage) {
    var env gateway.ResponseEnvelope
    json.Unmarshal(params, &env)
    renderTable(&env)
})
```

### 3.4 旧版兼容方法

#### SendCommand()

```go
func (c *Client) SendCommand(name string, args string) (string, error)
```

便捷包装：调用与 `name` 同名的 JSON-RPC method，传递 `{"args": args}`。内部使用 30 秒超时。

**示例：**
```go
resp, err := client.SendCommand("agents", "")
```

#### OnResponse()

```go
func (c *Client) OnResponse(responseType ResponseType, handler func(env *ResponseEnvelope, orig *Message))
```

注册类型化响应处理器。内部映射到 `client.On(string(responseType), ...)`。

#### OnReceived()

```go
func (c *Client) OnReceived(handler func(message string))
```

注册通用消息接收回调。内部映射到 `client.On("message", ...)`。

#### GetCommands()

```go
func (c *Client) GetCommands() ([]CommandInfo, error)
```

获取服务端已注册命令列表。内部调用 `client.Call(ctx, "command.list", nil)`。

#### OnResponseFallback()

```go
func (c *Client) OnResponseFallback(handler func(env *ResponseEnvelope, orig *Message))
```

注册回退响应处理器（JSON-RPC 模式下为 no-op）。

### 3.5 连接状态

```go
type ConnectionState string

const (
    StateDisconnected ConnectionState = "disconnected"
    StateConnected    ConnectionState = "connected"
)

func (c *Client) OnStateChange(fn func(oldState, newState ConnectionState))
```

---

## 4. 消息与类型定义

### 4.1 JSON-RPC 消息类型

```go
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id"`              // string/number, 通知为 nil
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
    JSONRPC string          `json:"jsonrpc"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Error struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

### 4.2 Message 类型（网关消息）

```go
type Message struct {
    ID          string    `json:"id"`           // 消息唯一标识（UUID）
    ChannelID   string    `json:"channel_id"`   // 关联的 Channel 名称
    ClientID    string    `json:"client_id"`    // 发送方/目标客户端 ID
    SessionID   string    `json:"session_id"`   // 会话 ID
    Direction   Direction `json:"direction"`    // inbound/outbound
    Data        []byte    `json:"data"`         // 消息体（原始字节）
    ContentType string    `json:"content_type"` // 内容类型
    Timestamp   time.Time `json:"timestamp"`    // 创建时间
}

func (m *Message) Text() string  // 将 Data 作为字符串返回
```

### 4.3 ResponseEnvelope

用于类型化通知的响应信封：

```go
type ResponseEnvelope struct {
    Type  ResponseType          `json:"type"`
    Title string                `json:"title"`
    Data  interface{}           `json:"data"`
    Meta  map[string]interface{} `json:"meta,omitempty"`
}

type ResponseType string

const (
    RespTable  ResponseType = "table"
    RespOptions ResponseType = "options"
    RespText   ResponseType = "text"
    RespTodo   ResponseType = "todo"
)
```

### 4.4 CommandInfo

```go
type CommandInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
}
```

### 4.5 辅助函数

```go
// 判断消息是否为 Notification（无 id）
func IsNotification(raw json.RawMessage) bool

// 判断消息是否为 Response（有 result 或 error）
func HasResultOrError(raw json.RawMessage) bool
```

---

## 5. Channel 集成

Gateway 通过 `RegisterChannel` 注册 Channel 适配器后，可在 handler 内通过 `msg.ChannelID` 判断消息来源渠道：

```go
server.RegisterChannel(dingTalkCh)

server.OnMessage(func(msg *gateway.Message) {
    if ch, ok := server.GetChannel(msg.ChannelID); ok {
        ch.SendMessage(ctx, replyMsg)
    }
})
```

---

## 6. 错误定义

| 错误变量 | 说明 |
|----------|------|
| `ErrAlreadyRunning` | Server 已在运行时再次调用 Start() |
| `ErrNotRunning` | Server 未运行时调用 Shutdown() |
| `ErrSessionNotFound` | 向不存在的 Session 发送数据 |
| `ErrSessionLimitReached` | Session 数量达到上限 |

---

## 注意事项

1. **Start() 是阻塞调用** — 应在 goroutine 中启动或作为程序入口的最后一步
2. **Session 超时默认 30 分钟** — 未结束的会话会被自动清理
3. **Notify 不保证送达** — 当客户端 send buffer 满时会静默丢弃
4. **消息大小限制 10MB** — WebSocket 读取限制
5. **所有命令注册为独立 JSON-RPC method** — 不再通过 `command.execute` 代理，每个命令名直接映射为 method
6. **CommandContext.ClientID 仅服务端回调时可用** — 客户端调用命令时 ClientID 为空
7. **心跳默认 54s Ping 间隔** — 可通过 `WithHeartbeat` 配置
8. **WebSocket 来源白名单默认仅 localhost** — 生产环境需配置 `WithWSConfig`
