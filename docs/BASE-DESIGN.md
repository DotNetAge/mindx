
## Cli 要求

### 起动 

启动命令:

```
mindx start
```

启动 gateway 服务，监听 WebSocket 连接（默认 `ws://localhost:1314/ws`）。

### tui

基本用法: 

```
mindx tui
```

tui 为交互式界面，进入后通过 `gort.gateway.Client` 连接到 `mindx start` 建立的 gateway 服务。

界面功能:

- 支持彩色输出；
- 该界面是一个 Agent 的聊天界面应用；
- 服务器将收到的 Message 序列化为 JSON 字符串返回给客户端（MVP 阶段临时方案，后续替换为真实 Agent 逻辑）；
- 界面直接显示服务器返回的信息；
- 发送信息给服务器后界面要显示 Loading 动画效果，等待服务器返回信息后显示返回信息；


## 技术栈

- **CLI 框架**: cobra
- **TUI 框架**: bubbletea (Elm 架构，搭配 bubbletea/lipgloss 做样式)
- **Gateway**: gort.gateway（Server + Client，WebSocket 通信）

## API 对齐说明

使用 gort.gateway 新版本 API：

- Server 端：`gateway.New()` → `gw.Start()` / `gw.Shutdown(ctx)`
- Handler 签名：`func(g *Server, msg *Message)` （非旧版 `func(clientId string, msg *Message)`）
- Client 端：`gateway.NewClient()` → `c.Connect()` / `c.Send()` / `c.OnMessage()`
- 服务端通过 `gw.Send(clientId, message)` 向指定客户端推送消息

## MVP 范围

- 先实现核心链路：start 启动服务 → tui 连接 → 收发消息 → 展示效果
- 连接地址硬编码 `localhost:1314`
- 不处理断线重连、多会话、持久化等边缘场景
