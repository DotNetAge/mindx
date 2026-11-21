# Gort 通信协议规格说明书

## 一、协议概述

Gort 采用 **JSON-RPC 2.0 over WebSocket** 协议，实现双向实时通信：

```
┌─────────────────────────────────────────────────────────────┐
│                   应用层 (Application Layer)                  │
│  Server / Client - 面向对象API，高层抽象                      │
│  Call() / Notify() / On() / RegisterMethod()                │
│  RegisterCommand() / SendResponse() / Broadcast()           │
│  Channel 扩展：IM 平台 ↔ JSON-RPC 数据转换                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   传输层 (Transport Layer)                   │
│  JSON-RPC 2.0 over WebSocket                                │
│  Request / Response / Notification                          │
└─────────────────────────────────────────────────────────────┘
```

Gort 的 JSON-RPC 网关还通过 **Channel** 机制对不同通信方式进行扩展，实现不同 IM 平台的消息格式与 JSON-RPC 数据之间的平滑转换。

---

## 二、JSON-RPC 2.0 消息格式

### 2.1 请求（Request）

客户端或服务端发起的请求，**必须包含 `"jsonrpc": "2.0"`**：

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "method": "agents",
  "params": {"args": ""}
}
```

### 2.2 响应（Response）

对请求的响应，**必须包含 `"jsonrpc": "2.0"`** 且 `id` 必须与请求相同：

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "result": [
    {"name": "writer", "role": "writer", "description": "Professional writer"}
  ]
}
```

### 2.3 通知（Notification）

服务端主动推送的消息，**必须包含 `"jsonrpc": "2.0"`**，无 `id`，不期望响应：

```json
{
  "jsonrpc": "2.0",
  "method": "table",
  "params": {
    "title": "Available Agents",
    "type": "table",
    "data": {
      "headers": ["Name", "Role", "Description"],
      "rows": [["writer", "writer", "Professional writer"]]
    }
  }
}
```

### 2.4 错误（Error）

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "error": {
    "code": -32601,
    "message": "Method not found: agents"
  }
}
```

### 2.5 标准错误码

| 错误码 | 描述 | 场景 |
|--------|------|------|
| `-32700` | Parse error | JSON 解析失败 |
| `-32600` | Invalid request | 不符合 JSON-RPC 规范 |
| `-32601` | Method not found | 方法未注册 |
| `-32602` | Invalid params | 参数错误 |
| `-32603` | Internal error | 服务器内部错误 |

---

## 三、通信模式

### 3.1 请求-响应模式（同步）

```
Client                              Server
   |                                   |
   |--- {"id":"1","method":"agents"} -->|
   |                                   |
   |<-- {"id":"1","result":[...]} -----|
   |                                   |
```

**特点**：
- 请求方设置 `id`，等待匹配 `id` 的响应
- 被调用方处理请求，返回结果
- 适用于：命令查询、数据获取

### 3.2 通知模式（异步推送）

```
Server                              Client
   |                                   |
   |-- {"method":"table",...} ------->|
   |                                   |
```

**特点**：
- 无 `id`，不期望响应
- 适用于：主动推送表格、选项、思考过程等

### 3.3 双向调用（对等通信）

```
Server                              Client
   |                                   |
   |-- {"id":"5","method":"client.show"} ->|
   |                                   |
   |<-- {"id":"5","result":{"ack":true}} -|
   |                                   |
```

**特点**：
- 服务端可以调用客户端方法
- 客户端可以调用服务端方法
- 完全对等，无主从之分

---

## 四、API 设计

### 4.1 Server 端

```go
// 创建服务器
server := gateway.New(
    gateway.WithAddr(":8081"),
    gateway.WithPath("/ws"),
)

// 注册 JSON-RPC 方法
server.RegisterMethod("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
    return map[string]string{"echo": string(params)}, nil
})

// 注册命令（自动注册为同名 JSON-RPC method）
server.RegisterCommand("agents", func(ctx *gateway.CommandContext) (any, error) {
    agents := listAgents()
    ctx.RespondWithType(gateway.RespTable, "Available Agents", map[string]interface{}{
        "headers": []string{"Name", "Role", "Description"},
        "rows":    toRows(agents),
    })
    return nil, nil
}, "显示智能体列表")

// 发送通知
server.Notify(clientID, "table", tableData)

// 广播通知
server.BroadcastNotification("update", updateData)

// 调用客户端方法
result, err := server.Call(ctx, clientID, "client.show", params)

// 启动
go server.Start()
defer server.Shutdown(context.Background())
```

### 4.2 Client 端

```go
// 创建客户端
client := gateway.NewClient("ws://localhost:8081/ws")

// 连接
client.Connect(context.Background())
defer client.Close()

// 发送请求
result, err := client.Call(ctx, "agents", nil)

// 使用便捷命令包装
resp, err := client.SendCommand("agents", "")

// 注册通知处理器
client.On("table", func(ctx context.Context, params json.RawMessage) {
    var env gateway.ResponseEnvelope
    json.Unmarshal(params, &env)
    // 处理表格数据
})
```

---

## 五、ResponseEnvelope 设计

服务端通过 ResponseEnvelope 推送类型化通知：

```go
type ResponseEnvelope struct {
    Type  ResponseType          `json:"type"`
    Title string                `json:"title"`
    Data  interface{}           `json:"data"`
    Meta  map[string]interface{} `json:"meta,omitempty"`
}

type ResponseType string

const (
    RespTable   ResponseType = "table"
    RespOptions ResponseType = "options"
    RespText    ResponseType = "text"
    RespTodo    ResponseType = "todo"
    RespFile    ResponseType = "file"
    RespError   ResponseType = "error"
    RespMarkdown ResponseType = "markdown"
    // ... 更多类型见 response_types.go
)
```

**传输示例**：
```json
{
  "jsonrpc": "2.0",
  "method": "table",
  "params": {
    "type": "table",
    "title": "Available Agents",
    "data": {
      "headers": ["Name", "Role"],
      "rows": [["writer", "writer"]]
    }
  }
}
```

---

## 六、Channel 扩展机制

### 6.1 设计理念

Gort 的 JSON-RPC 网关通过 **Channel** 机制对不同通信方式进行扩展。每个 IM 平台（微信、钉钉、飞书、Telegram 等）实现 `channel.Channel` 接口，完成以下转换：

1. **入站**：IM 平台消息 → `channel.Message` → 转换为 JSON-RPC 格式 → 推送给 WebSocket Client
2. **出站**：JSON-RPC Notification → 转换为 `channel.Message` → 发送到 IM 平台

这样实现了不同通信格式与 JSON-RPC 数据之间的平滑转换。

### 6.2 Channel 接口

```go
type Channel interface {
    Name() string                              // 渠道实例唯一标识
    Type() ChannelType                         // 平台类型（wechat/dingtalk/telegram 等）
    Start(ctx context.Context, handler MessageHandler) error  // 启动并绑定消息处理器
    Stop(ctx context.Context) error            // 优雅关闭
    IsRunning() bool                           // 运行状态
    SendMessage(ctx context.Context, msg *Message) error     // 发送消息到 IM 平台
    GetStatus() Status                         // 详细状态
}

type MessageHandler func(ctx context.Context, msg *Message) error

type Message struct {
    ID        string                 // 消息唯一标识
    ChannelID string                 // 关联的 Channel 名称
    Type      MessageType            // 消息类型（text/image/file 等）
    Direction Direction              // inbound/outbound
    From      UserInfo               // 发送者信息
    To        UserInfo               // 接收者信息
    Content   string                 // 消息内容
    Metadata  map[string]interface{} // 扩展元数据
}
```

### 6.3 完整用法

```go
package main

import (
    "context"
    "log"

    "github.com/DotNetAge/gort/pkg/channel"
    "github.com/DotNetAge/gort/pkg/channel/dingtalk"
    "github.com/DotNetAge/gort/pkg/gateway"
)

func main() {
    // 1. 创建 JSON-RPC Gateway Server
    server := gateway.New(
        gateway.WithAddr(":8081"),
        gateway.WithPath("/ws"),
    )

    // 2. 注册业务命令
    server.RegisterCommand("agents", func(ctx *gateway.CommandContext) (any, error) {
        return listAgents(), nil
    }, "显示智能体列表")

    // 3. 启动 Gateway
    go server.Start()
    defer server.Shutdown(context.Background())

    // 4. 创建 DingTalk Channel
    dingCh, err := dingtalk.NewChannel("my-dingtalk", dingtalk.Config{
        AppKey:    "your-app-key",
        AppSecret: "your-app-secret",
    })
    if err != nil {
        log.Fatal(err)
    }

    // 5. 注册 Channel 到 Gateway
    server.RegisterChannel(dingCh)

    // 6. 启动 Channel 并绑定消息处理器
    ctx := context.Background()
    handler := func(ctx context.Context, msg *channel.Message) error {
        // 收到 IM 平台消息后，广播给所有 WebSocket Client
        server.Broadcast(msg.Content)

        // 或通过 Channel 回复
        return dingCh.SendMessage(ctx, &channel.Message{
            ID:        "reply-001",
            ChannelID: "my-dingtalk",
            Direction: channel.DirectionOutbound,
            Content:   "已收到你的消息",
        })
    }

    if err := dingCh.Start(ctx, handler); err != nil {
        log.Fatal(err)
    }

    select {}
}
```

### 6.4 支持的 Channel 类型

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

### 6.5 Channel 与 JSON-RPC 的集成

```
┌──────────────────┐     inbound      ┌──────────────────────┐
│  外部平台         │ ──(webhook/poll)→ │  Channel Adapter       │
│  (微信/钉钉/飞书) │                  │  (channel.Channel)      │
└──────────────────┘                  └──────┬───────────────┘
                                           │ GatewaySender
                                           ↓ (Broadcast/Send)
┌──────────────────┐   JSON-RPC WS   ┌─────┴─────────────────┐
│  浏览器/App 客户端  │ ←─Request/Notif─→ │  Gateway Server        │
│  (WebSocket)      │                │  (gateway.Server)       │
└──────────────────┘                └──────┬───────────────┘
                                           │ Notify/Call
                                           ↓ outbound
                                    ┌──────┴───────────────┐
                                    │  Channel Adapter       │
                                    │  (SendMessage)          │
                                    └──────┬───────────────┘
                                           │
                                    ┌──────┴───────────────┐
                                    │  外部平台               │
                                    └───────────────────────┘
```

---

## 七、核心优势

### 7.1 正交性

| 通信模式 | JSON-RPC 特征 | 路由方式 |
|----------|---------------|----------|
| 请求-响应 | 有 `id` | 匹配 `pending[id]` |
| 主动推送 | 无 `id` | 触发 `On(method)` handler |
| 错误 | 有 `error` 字段 | 返回错误 |

### 7.2 对等性

- 客户端和服务端地位完全对等
- 双方都可以发起请求和推送通知
- 打破了传统的"客户端请求-服务端响应"模式

### 7.3 标准化

- 符合 JSON-RPC 2.0 规范
- 与 LLM Tools 调用格式一致
- 工具生态成熟（调试器、mock 服务器等）

### 7.4 Channel 可扩展性

- 每个 IM 平台独立实现 `channel.Channel` 接口
- 通过 `RegisterChannel` 自动注入到 Gateway
- Channel 消息通过 `Broadcast`/`Notify` 转换为 JSON-RPC 推送

---

## 八、错误处理

### 8.1 连接错误

| 场景 | 处理方法 |
|------|----------|
| 连接失败 | `Connect()` 返回错误 |
| 连接断开 | 自动关闭所有 pending 请求 |
| 超时 | `context.WithTimeout` 控制 |

### 8.2 请求错误

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-123",
  "error": {
    "code": -32603,
    "message": "agents failed: database connection lost"
  }
}
```

### 8.3 通知丢失

通知不保证送达，适用于实时 UI 更新场景。

---

## 九、数据格式约束

### 9.1 消息大小限制

| 类型 | 最大值 | 备注 |
|------|--------|------|
| 单条消息 | 10MB | WebSocket 帧限制 |
| JSON 序列化后 | 无限制 | 建议 < 1MB |

### 9.2 字符编码

- 所有文本使用 **UTF-8** 编码
- JSON 序列化使用标准 `encoding/json`
- `"jsonrpc": "2.0"` 为 JSON-RPC 2.0 规范的**必选字段**

### 9.3 时限约束

| 操作 | 超时时间 | 备注 |
|------|----------|------|
| 连接建立 | 10s | |
| 请求-响应 | 30s | 可配置 context |
| 心跳间隔 | 54s | 自动 ping/pong |

---

## 十、最佳实践

### 10.1 使用请求-响应模式

```go
// 推荐：使用 Call 获取数据
result, err := client.Call(ctx, "agents", nil)

// 便捷方式：SendCommand 直接调用同名 method
resp, err := client.SendCommand("agents", "")
```

### 10.2 使用通知模式推送

```go
// 服务端：推送表格更新
server.Notify(clientID, "table", tableData)

// 客户端：接收并处理
client.On("table", func(ctx context.Context, params json.RawMessage) {
    renderTable(params)
})
```

### 10.3 Channel 集成

```go
// 创建 Channel 并注册到 Gateway
ch := dingtalk.NewChannel("dingtalk", config)
server.RegisterChannel(ch)

// 启动 Channel，消息自动广播给 WebSocket Client
ch.Start(ctx, func(ctx context.Context, msg *channel.Message) error {
    server.Broadcast(msg.Content)
    return nil
})
```

### 10.4 资源清理

```go
// 确保连接关闭
defer client.Close()

// 使用 context 控制超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

---

## 十一、与 LLM Tools 调用的对应关系

```
LLM Tools 调用格式：
{
  "tool_calls": [{
    "id": "call_abc123",
    "function": {
      "name": "get_weather",
      "arguments": "{\"location\": \"Beijing\"}"
    }
  }]
}

对应 JSON-RPC：
{
  "jsonrpc": "2.0",
  "id": "call_abc123",
  "method": "get_weather",
  "params": {"location": "Beijing"}
}
```

**优势**：整个系统从 LLM → Gateway → Client 使用同一种通信范式！
