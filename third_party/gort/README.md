<div align="center">

# Gort

**A lightweight JSON-RPC 2.0 WebSocket gateway for real-time bidirectional communication between desktop apps and AI agents.**

Gort's JSON-RPC gateway also extends different communication methods through the **Channel** mechanism, achieving seamless conversion between IM platform message formats and JSON-RPC data.

[![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gort.svg)](https://pkg.go.dev/github.com/DotNetAge/gort)
[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gort)](https://goreportcard.com/report/github.com/DotNetAge/gort)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[**English**](./README.md) | [**中文说明**](./README_zh-CN.md)

</div>

---

## Overview

Gort is a WebSocket-based JSON-RPC 2.0 communication library that provides a Server and Client for real-time bidirectional messaging. Each IM platform (WeChat, DingTalk, Feishu, Telegram, etc.) implements the `channel.Channel` interface to complete:

1. **Inbound**: IM platform message → `channel.Message` → converted to JSON-RPC → pushed to WebSocket Client
2. **Outbound**: JSON-RPC Notification → converted to `channel.Message` → sent to IM platform

This achieves seamless conversion between different communication formats and JSON-RPC data.

### Key Principles

- **JSON-RPC 2.0** — Standard protocol with Request/Response/Notification
- **Orthogonality** — Requests have `id`, notifications have no `id`, errors carry `code`+`message`
- **Peer-to-Peer** — Server and Client can both initiate calls and push notifications
- **Channel Extensibility** — IM platform adapters implement `channel.Channel`, automatically injected via `RegisterChannel`

### Architecture

```
┌──────────────────┐   JSON-RPC over WebSocket   ┌──────────────────┐
│  Client (mindx)  │ ◄─────────────────────────► │  Server (mindx)  │
│                  │                             │                  │
│  Call("agents")  │ ──── request ──────────────►│  methods["agents"]│
│  ◄────────────── │ ──── response ───────────── │                  │
│                  │                             │  Notify("table")  │
│  On("table") ◄── │ ──── notification ───────── │                  │
└──────────────────┘                             └──────────────────┘
```

## Installation

```bash
go get github.com/DotNetAge/gort
```

### Prerequisites

- Go 1.23 or higher

## Quick Start

### Server

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

    // Register a JSON-RPC method
    server.RegisterMethod("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
        log.Printf("Received: %s", params)
        return map[string]string{"echo": string(params)}, nil
    })

    // Register a command (convenience wrapper that auto-registers as JSON-RPC method)
    server.RegisterCommand("agents", func(ctx *gateway.CommandContext) (any, error) {
        agents := listAgents()
        ctx.RespondWithType(gateway.RespTable, "Available Agents", map[string]interface{}{
            "headers": []string{"Name", "Role"},
            "rows":    toRows(agents),
        })
        return nil, nil
    }, "List available agents")

    // Start the server (blocking, use goroutine in production)
    go server.Start()
    defer server.Shutdown(context.Background())

    select {}
}
```

### Client

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

    // Connect
    if err := client.Connect(context.Background()); err != nil {
        log.Fatal(err)
    }

    // Call a server method (request-response)
    result, err := client.Call(context.Background(), "agents", nil)
    if err != nil {
        log.Fatalf("call failed: %v", err)
    }
    log.Printf("Result: %s", result)

    // Register a notification handler (server push)
    client.On("table", func(ctx context.Context, params json.RawMessage) {
        var table gateway.ResponseEnvelope
        json.Unmarshal(params, &table)
        log.Printf("Table pushed: %s", table.Title)
    })

    // Or use the convenience wrapper for commands
    resp, err := client.SendCommand("agents", "")
    if err != nil {
        log.Fatalf("command failed: %v", err)
    }
    log.Printf("Command response: %s", resp)

    select {}
}
```

## Server API

### Constructor

```go
func New(opts ...Option) *Server
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithAddr(addr)` | string | `:8081` | Listen address (e.g., `0.0.0.0:9090`) |
| `WithPort(port)` | int | `8081` | Port only (auto-formatted as `:{port}`) |
| `WithPath(path)` | string | `/ws` | WebSocket endpoint path |
| `WithHandler(h)` | MessageHandler | nil | Message handler for notifications |
| `WithSessionTimeout(d)` | Duration | 30m | Session timeout |
| `WithHeartbeat(cfg)` | *HeartbeatConfig | nil | Heartbeat monitoring |
| `WithWSConfig(cfg)` | *WSConfig | localhost-only | WebSocket origin whitelist |
| `WithChannels(chs)` | []channel.Channel | nil | Register channels at startup |

### Lifecycle

```go
func (s *Server) Start() error
func (s *Server) Shutdown(ctx context.Context) error
func (s *Server) IsRunning() bool
```

`Start()` is blocking. Run it in a goroutine for most use cases.

### Method Registration

```go
func (s *Server) RegisterMethod(method string, handler MethodHandler)
```

Registers a JSON-RPC method handler. The handler receives `context.Context` and `json.RawMessage` params, and returns `(any, error)`.

```go
server.RegisterMethod("users.list", func(ctx context.Context, params json.RawMessage) (any, error) {
    return users, nil
})
```

### Command Registration (Convenience)

```go
func (s *Server) RegisterCommand(name string, handler func(ctx *CommandContext) (any, error), description string)
```

Registers a command that automatically becomes a JSON-RPC method with the same name. The `CommandContext` provides:

- `ctx.Args` — Command arguments as string
- `ctx.ClientID` — The client ID that invoked the command
- `ctx.RespondWithType(type, title, data)` — Push typed response to client
- `ctx.Server()` — Reference to the Server instance

```go
server.RegisterCommand("models", func(ctx *gateway.CommandContext) (any, error) {
    ctx.RespondWithType(gateway.RespTable, "Models", data)
    return nil, nil
}, "List available models")
```

Built-in method `command.list` is auto-registered — returns all registered commands with descriptions.

### Client Operations

```go
func (s *Server) Notify(clientID, method string, params any) error
func (s *Server) BroadcastNotification(method string, params any)
func (s *Server) Call(ctx context.Context, clientID, method string, params any) (json.RawMessage, error)
```

- `Notify` — Push a notification to a specific client
- `BroadcastNotification` — Push to all connected clients
- `Call` — Call a method on a client and wait for response (server-initiated RPC)

### Legacy Send Methods

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

These are convenience wrappers around `Notify`. For example, `Send(to, msg)` sends `Notify(to, "message", {"text": msg})`.

### Client Management

```go
func (s *Server) ClientCount() int
func (s *Server) GetClient(clientID string) *client
```

### Channel Integration

```
┌──────────────────┐     inbound      ┌──────────────────────┐
│  External IM     │ ──(webhook/poll)→ │  Channel Adapter       │
│  (WeChat/DingTalk)│                 │  (channel.Channel)      │
└──────────────────┘                  └──────┬───────────────┘
                                           │ GatewaySender
                                           ↓ (Broadcast/Send)
┌──────────────────┐   JSON-RPC WS   ┌─────┴─────────────────┐
│  Browser/App     │ ←─Request/Notif─→ │  Gateway Server        │
│  (WebSocket)     │                │  (gateway.Server)       │
└──────────────────┘                └──────┬───────────────┘
                                           │ Notify/Call
                                           ↓ outbound
                                    ┌──────┴───────────────┐
                                    │  Channel Adapter       │
                                    │  (SendMessage)          │
                                    └──────┬───────────────┘
                                           │
                                    ┌──────┴───────────────┐
                                    │  External IM Platform   │
                                    └───────────────────────┘
```

```go
// 1. Create the gateway server
server := gateway.New(gateway.WithAddr(":8081"))

// 2. Create a DingTalk channel
dingCh, _ := dingtalk.NewChannel("my-dingtalk", config)

// 3. Register the channel to the gateway
server.RegisterChannel(dingCh)

// 4. Start the channel with a message handler
dingCh.Start(ctx, func(ctx context.Context, msg *channel.Message) error {
    // IM message received → broadcast to all WebSocket clients
    server.Broadcast(msg.Content)
    return nil
})
```

**Supported Channel Types:**

| ChannelType | Platform | Description |
|-------------|----------|-------------|
| `ChannelTypeWeChat` | WeChat Official | Official Account + Token |
| `ChannelTypeDingTalk` | DingTalk | Webhook Bot |
| `ChannelTypeFeishu` | Feishu | Self-built App + Token |
| `ChannelTypeTelegram` | Telegram | Bot Token |
| `ChannelTypeSlack` | Slack | Bot Token |
| `ChannelTypeDiscord` | Discord | Bot Token |
| `ChannelTypeWhatsApp` | WhatsApp | Business API |
| `ChannelTypeMessenger` | Facebook Messenger | Page Access Token |
| `ChannelTypeWeCom` | WeCom | Webhook Bot |
| `ChannelTypeIMessage` | iMessage | macOS + imsg CLI |

```go
func (s *Server) RegisterChannel(ch channel.Channel)
func (s *Server) GetChannel(name string) (channel.Channel, bool)
func (s *Server) Channels() map[string]channel.Channel
```

## Client API

### Constructor

```go
func NewClient(addr string) *Client
```

| Parameter | Description |
|-----------|-------------|
| `addr` | Full WebSocket URL (e.g., `ws://localhost:8081/ws`) |

### Connection

```go
func (c *Client) Connect(ctx context.Context) error
func (c *Client) ConnectSync() error
func (c *Client) Close() error
func (c *Client) IsConnected() bool
```

`ConnectSync()` connects without context (for backward compatibility).

### JSON-RPC Methods

```go
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error)
```

Send a request and wait for response. Uses `context.WithTimeout` for cancellation.

```go
func (c *Client) Notify(method string, params any) error
```

Send a notification (no response expected).

```go
func (c *Client) On(method string, handler NotificationHandler)
```

Register a handler for server-initiated notifications.

### Legacy Compatibility

```go
func (c *Client) SendCommand(name string, args string) (string, error)
```

Convenience wrapper: calls the JSON-RPC method with the same name as `name`, passing `{"args": args}`.

```go
func (c *Client) OnResponse(responseType ResponseType, handler func(env *ResponseEnvelope, orig *Message))
func (c *Client) OnReceived(handler func(message string))
func (c *Client) GetCommands() ([]CommandInfo, error)
```

## JSON-RPC Protocol

### Request

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "method": "agents",
  "params": {"args": ""}
}
```

### Response

```json
{
  "jsonrpc": "2.0",
  "id": "abc123",
  "result": [...]
}
```

### Notification

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

### Error

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

## Response Envelope

For typed server-to-client notifications (table, options, todo, text):

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

## Message Types

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

## Connection State

```go
type ConnectionState string

const (
    StateDisconnected ConnectionState = "disconnected"
    StateConnected    ConnectionState = "connected"
)

func (c *Client) OnStateChange(fn func(oldState, newState ConnectionState))
```

## Project Structure

```
gort/
├── pkg/
│   ├── gateway/
│   │   ├── server.go          # Server: lifecycle, WebSocket, read/write pump
│   │   ├── client.go          # Client: connect, Call, Notify, On
│   │   ├── types.go           # JSON-RPC Request/Response/Notification/Error
│   │   ├── message.go         # Message type + Direction enum
│   │   ├── response_types.go  # ResponseEnvelope, ResponseType, CommandInfo
│   │   ├── session.go         # SessionManager for message aggregation
│   │   └── heartbeat.go       # Heartbeat monitoring
│   └── channel/               # IM platform adapters (WeChat, DingTalk, etc.)
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
│   ├── PROTOCOL_SPEC.md       # JSON-RPC 2.0 protocol specification
│   └── gateway.md             # API documentation
├── go.mod
├── README.md
└── README_zh-CN.md
```

## License

MIT License — see LICENSE file for details.
