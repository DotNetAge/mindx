<div align="center">

# Gort

**一个轻量级的 JSON-RPC 2.0 WebSocket 网关，用于桌面应用与 AI 代理之间的实时双向通信。**

Gort 的 JSON-RPC 网关还通过 **Channel** 机制对不同通信方式进行扩展，实现不同 IM 平台的消息格式与 JSON-RPC 数据之间的平滑转换。

[![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gort.svg)](https://pkg.go.dev/github.com/DotNetAge/gort)
[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gort)](https://goreportcard.com/report/github.com/DotNetAge/gort)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[**English**](./README.md) | [**中文说明**](./README_zh-CN.md)

</div>

---

## 概述

Gort 是一个基于 WebSocket 的 JSON-RPC 2.0 通信库，提供 Server 和 Client 用于实时双向消息传递。每个 IM 平台（微信、钉钉、飞书、Telegram 等）实现 `channel.Channel` 接口，完成以下转换：

1. **入站**：IM 平台消息 → `channel.Message` → 转换为 JSON-RPC 格式 → 推送给 WebSocket Client
2. **出站**：JSON-RPC Notification → 转换为 `channel.Message` → 发送到 IM 平台

这样实现了不同通信格式与 JSON-RPC 数据之间的平滑转换。

### 核心设计原则

- **JSON-RPC 2.0** — 标准协议，包含 Request/Response/Notification
- **正交性** — 请求有 `id`，通知无 `id`，错误携带 `code`+`message`
- **对等通信** — Server 和 Client 均可发起调用和推送通知
- **Channel 可扩展性** — IM 平台适配器实现 `channel.Channel`，通过 `RegisterChannel` 自动注入

### 架构

```
┌──────────────────┐   JSON-RPC over WebSocket   ┌──────────────────┐
│  Client (mindx)  │ ◄─────────────────────────► │  Server (mindx)  │
│                  │                             │                  │
│  Call("agents")  │ ──── 请求 ─────────────────►│  methods["agents"]│
│  ◄────────────── │ ──── 响应 ───────────────── │                  │
│                  │                             │  Notify("table")  │
│  On("table") ◄── │ ──── 通知推送 ───────────── │                  │
└──────────────────┘                             └──────────────────┘
```

## 安装

```bash
go get github.com/DotNetAge/gort
```

### 前置要求

- Go 1.23 或更高版本

## 快速开始

### 服务端

```go
package main

import (
    "context"
    "log"

    "github.com/DotNetAge/gort/pkg/gateway"
)

func main() {
    server := gateway.New(
        gateway.WithAddr(":8081"),
        gateway.WithPath("/ws"),
    )

    // 注册 JSON-RPC 方法
    server.RegisterMethod("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
        log.Printf("收到: %s", params)
        return map[string]string{"echo": string(params)}, nil
    })

    // 注册命令（便捷包装，自动注册为 JSON-RPC method）
    server.RegisterCommand("agents", func(ctx *gateway.CommandContext) (any, error) {
        agents := listAgents()
        ctx.RespondWithType(gateway.RespTable, "可用智能体", map[string]interface{}{
            "headers": []string{"名称", "角色"},
            "rows":    toRows(agents),
        })
        return nil, nil
    }, "显示智能体列表")

    // 启动服务（阻塞的，生产环境用 goroutine）
    go server.Start()
    defer server.Shutdown(context.Background())

    select {}
}
```

### 客户端

```go
package main

import (
    "context"
    "log"

    "github.com/DotNetAge/gort/pkg/gateway"
)

func main() {
    client := gateway.NewClient("ws://localhost:8081/ws")
    defer client.Close()

    // 连接
    if err := client.Connect(context.Background()); err != nil {
        log.Fatal(err)
    }

    // 调用服务端方法（请求-响应）
    result, err := client.Call(context.Background(), "agents", nil)
    if err != nil {
        log.Fatalf("调用失败: %v", err)
    }
    log.Printf("结果: %s", result)

    // 注册通知处理器（服务端推送）
    client.On("table", func(ctx context.Context, params json.RawMessage) {
        var table gateway.ResponseEnvelope
        json.Unmarshal(params, &table)
        log.Printf("推送表格: %s", table.Title)
    })

    // 或使用便捷命令包装
    resp, err := client.SendCommand("agents", "")
    if err != nil {
        log.Fatalf("命令失败: %v", err)
    }
    log.Printf("命令响应: %s", resp)

    select {}
}
```

## 服务端 API

### 构造函数

```go
func New(opts ...Option) *Server
```

| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `WithAddr(addr)` | string | `:8081` | 监听地址（如 `0.0.0.0:9090`） |
| `WithPort(port)` | int | `8081` | 仅端口号（自动拼接为 `:{port}`） |
| `WithPath(path)` | string | `/ws` | WebSocket 端点路径 |
| `WithHandler(h)` | MessageHandler | nil | 通知消息处理器 |
| `WithSessionTimeout(d)` | Duration | 30m | 会话超时时间 |
| `WithHeartbeat(cfg)` | *HeartbeatConfig | nil | 心跳监控 |
| `WithWSConfig(cfg)` | *WSConfig | localhost-only | WebSocket 来源白名单 |
| `WithChannels(chs)` | []channel.Channel | nil | 启动时注册渠道 |

### 生命周期

```go
func (s *Server) Start() error
func (s *Server) Shutdown(ctx context.Context) error
func (s *Server) IsRunning() bool
```

`Start()` 是阻塞调用，大多数场景下应在 goroutine 中启动。

### 方法注册

```go
func (s *Server) RegisterMethod(method string, handler MethodHandler)
```

注册 JSON-RPC 方法处理器。处理器接收 `context.Context` 和 `json.RawMessage` 参数，返回 `(any, error)`。

```go
server.RegisterMethod("users.list", func(ctx context.Context, params json.RawMessage) (any, error) {
    return users, nil
})
```

### 命令注册（便捷方式）

```go
func (s *Server) RegisterCommand(name string, handler func(ctx *CommandContext) (any, error), description string)
```

注册命令，自动将其注册为同名的 JSON-RPC 方法。`CommandContext` 提供：

- `ctx.Args` — 命令参数字符串
- `ctx.ClientID` — 调用该命令的客户端 ID
- `ctx.RespondWithType(type, title, data)` — 向客户端推送类型化响应
- `ctx.Server()` — Server 实例引用

```go
server.RegisterCommand("models", func(ctx *gateway.CommandContext) (any, error) {
    ctx.RespondWithType(gateway.RespTable, "可用模型", data)
    return nil, nil
}, "列出所有可用模型")
```

内置方法 `command.list` 在 `New()` 时自动注册 — 返回所有已注册命令及描述。

### 客户端操作

```go
func (s *Server) Notify(clientID, method string, params any) error
func (s *Server) BroadcastNotification(method string, params any)
func (s *Server) Call(ctx context.Context, clientID, method string, params any) (json.RawMessage, error)
```

- `Notify` — 向指定客户端推送通知
- `BroadcastNotification` — 向所有已连接客户端广播通知
- `Call` — 调用客户端上的方法并等待响应（服务端发起的 RPC）

### 旧版发送方法

```go
func (s *Server) Send(to string, message string)
func (s *Server) Broadcast(message string)
func (s *Server) BroadcastMessage(message string)
func (s *Server) SendJSON(to string, v interface{}) error
func (s *Server) BroadcastJSON(v interface{}) error
func (s *Server) SendFile(to string, filename string) error
func (s *Server) BroadcastFile(filename string) error
func (s *Server) SendBatch(to string, msgs []*Message)
func (s *Server) BroadcastBatch(msgs []*Message)
```

这些是 `Notify` 的便捷包装。例如 `Send(to, msg)` 发送 `Notify(to, "message", {"text": msg})`。

### 客户端管理

```go
func (s *Server) ClientCount() int
func (s *Server) GetClient(clientID string) *client
```

### 渠道集成

```
┌──────────────────┐     入站         ┌──────────────────────┐
│  外部 IM 平台     │ ──(webhook/轮询)→ │  Channel 适配器         │
│  (微信/钉钉/飞书) │                  │  (channel.Channel)      │
└──────────────────┘                  └──────┬───────────────┘
                                           │ GatewaySender
                                           ↓ (广播/发送)
┌──────────────────┐   JSON-RPC WS   ┌─────┴─────────────────┐
│  浏览器/App 客户端  │ ←─请求/通知 ────→ │  Gateway Server        │
│  (WebSocket)      │                │  (gateway.Server)       │
└──────────────────┘                └──────┬───────────────┘
                                           │ 通知/调用
                                           ↓ 出站
                                    ┌──────┴───────────────┐
                                    │  Channel 适配器         │
                                    │  (SendMessage)          │
                                    └──────┬───────────────┘
                                           │
                                    ┌──────┴───────────────┐
                                    │  外部 IM 平台            │
                                    └───────────────────────┘
```

```go
// 1. 创建网关服务
server := gateway.New(gateway.WithAddr(":8081"))

// 2. 创建钉钉渠道
dingCh, _ := dingtalk.NewChannel("my-dingtalk", config)

// 3. 注册渠道到网关
server.RegisterChannel(dingCh)

// 4. 启动渠道并绑定消息处理器
dingCh.Start(ctx, func(ctx context.Context, msg *channel.Message) error {
    // IM 收到消息 → 广播给所有 WebSocket 客户端
    server.Broadcast(msg.Content)
    return nil
})
```

**支持的渠道类型：**

| ChannelType | 平台 | 说明 |
|-------------|------|------|
| `ChannelTypeWeChat` | 微信公众号 | 公众号 + Token |
| `ChannelTypeDingTalk` | 钉钉 | Webhook 机器人 |
| `ChannelTypeFeishu` | 飞书 | 自建应用 + Token |
| `ChannelTypeTelegram` | Telegram | Bot Token |
| `ChannelTypeSlack` | Slack | Bot Token |
| `ChannelTypeDiscord` | Discord | Bot Token |
| `ChannelTypeWhatsApp` | WhatsApp | Business API |
| `ChannelTypeMessenger` | Facebook Messenger | Page Access Token |
| `ChannelTypeWeCom` | 企业微信 | Webhook 机器人 |
| `ChannelTypeIMessage` | iMessage | macOS + imsg CLI |

```go
func (s *Server) RegisterChannel(ch channel.Channel)
func (s *Server) GetChannel(name string) (channel.Channel, bool)
func (s *Server) Channels() map[string]channel.Channel
```

## 客户端 API

### 构造函数

```go
func NewClient(addr string) *Client
```

| 参数 | 说明 |
|------|------|
| `addr` | 完整 WebSocket URL（如 `ws://localhost:8081/ws`） |

### 连接管理

```go
func (c *Client) Connect(ctx context.Context) error
func (c *Client) ConnectSync() error
func (c *Client) Close() error
func (c *Client) IsConnected() bool
```

`ConnectSync()` 不需要 context（向后兼容）。

### JSON-RPC 方法

```go
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error)
```

发送请求并等待响应。使用 `context.WithTimeout` 控制超时。

```go
func (c *Client) Notify(method string, params any) error
```

发送通知（无需响应）。

```go
func (c *Client) On(method string, handler NotificationHandler)
```

注册服务端通知的处理器。

### 旧版兼容

```go
func (c *Client) SendCommand(name string, args string) (string, error)
```

便捷包装：调用与 `name` 同名的 JSON-RPC method，传递 `{"args": args}`。

```go
func (c *Client) OnResponse(responseType ResponseType, handler func(env *ResponseEnvelope, orig *Message))
func (c *Client) OnReceived(handler func(message string))
func (c *Client) GetCommands() ([]CommandInfo, error)
```

## JSON-RPC 协议

### 请求（Request）

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "method": "agents",
  "params": {"args": ""}
}
```

### 响应（Response）

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "result": [...]
}
```

### 通知（Notification）

```json
{
  "jsonrpc": "2.0",
  "method": "table",
  "params": {
    "type": "table",
    "title": "Agents",
    "data": {"headers": ["名称"], "rows": [["writer"]]}
  }
}
```

### 错误（Error）

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "error": {
    "code": -32601,
    "message": "Method not found: unknown_method"
  }
}
```

## 响应信封（Response Envelope）

用于服务端到客户端的类型化通知（table、options、todo、text）：

```go
type ResponseEnvelope struct {
    Type  ResponseType          `json:"type"`
    Title string                `json:"title"`
    Data  interface{}           `json:"data"`
    Meta  map[string]interface{} `json:"meta,omitempty"`
}

const (
    RespTable  ResponseType = "table"
    RespOptions ResponseType = "options"
    RespText    ResponseType = "text"
    RespTodo   ResponseType = "todo"
)
```

## 消息类型

```go
type Message struct {
    ID          string    `json:"id"`
    ChannelID   string    `json:"channel_id"`
    ClientID    string    `json:"client_id"`
    SessionID   string    `json:"session_id"`
    Direction   Direction `json:"direction"`
    Data        []byte    `json:"data"`
    ContentType string    `json:"content_type"`
    Timestamp   time.Time `json:"timestamp"`
}
```

## 连接状态

```go
type ConnectionState string

const (
    StateDisconnected ConnectionState = "disconnected"
    StateConnected    ConnectionState = "connected"
)

func (c *Client) OnStateChange(fn func(oldState, newState ConnectionState))
```

## 项目结构

```
gort/
├── pkg/
│   ├── gateway/
│   │   ├── server.go          # 服务端：生命周期、WebSocket、读写泵
│   │   ├── client.go          # 客户端：连接、Call、Notify、On
│   │   ├── types.go           # JSON-RPC Request/Response/Notification/Error
│   │   ├── message.go         # Message 类型 + Direction 枚举
│   │   ├── response_types.go  # ResponseEnvelope、ResponseType、CommandInfo
│   │   ├── session.go         # SessionManager 消息聚合
│   │   └── heartbeat.go       # 心跳监控
│   └── channel/               # IM 平台适配器（微信、钉钉等）
│       ├── channel.go
│       ├── dingtalk/
│       ├── discord/
│       ├── feishu/
│       ├── imessage/
│       ├── messenger/
│       ├── slack/
│       ├── telegram/
│       ├── wechat/
│       ├── wecom/
│       ├── whatsapp/
│       ├── httpclient/
│       └── tokenmanager/
├── docs/
│   ├── PROTOCOL_SPEC.md       # JSON-RPC 2.0 协议规格说明
│   └── gateway.md             # API 文档
├── go.mod
├── README.md
└── README_zh-CN.md
```

## 许可证

MIT License — 详见 LICENSE 文件
