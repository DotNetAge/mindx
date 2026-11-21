### 功能包名称： gateway

#### 1. 核心目标

实现基于WebSocket的阻塞式服务器，支持带会话管理的文本指令格式数据传输，用于高效处理客户端请求与响应。

#### 2. 技术栈

- 服务端语言：Go
- 会话存储：内存映射（带超时清理）

#### 3. 核心模块设计

##### 3.1 会话管理器（SessionManager）

- 功能：生成唯一会话ID，维护会话状态，超时自动销毁（默认30分钟无活动）。
- 数据结构：`map[string]Session`，带读写锁保证并发安全。
- 会话状态：包含客户端连接、创建时间、最后活动时间、未完成分片数据缓存。

##### 3.2 协议解析器（ProtocolParser）

- 文本指令格式：`指令类型|会话ID|参数1|参数2|数据体`，使用`|`作为分隔符。
- 支持指令类型：
  - `SESSION_START`：初始化新会话，返回会话ID。
  - `DATA`：传输分片数据，参数为分片序号、总分片数。
  - `SESSION_END`：结束会话，触发服务端数据处理。
- 解析逻辑：按分隔符切割字符串，校验指令合法性与参数完整性。

##### 3.3 WebSocket服务器（WSServer）

- 阻塞式主循环：使用gorilla/websocket库，为每个连接启动独立读写协程。
- 连接处理流程：
  1. HTTP升级为WebSocket连接。
  2. 接收客户端消息，调用协议解析器处理。
  3. 根据指令类型分发至会话管理器执行对应操作。
  4. 返回处理结果（如会话ID、分片确认）。
- 错误处理：连接中断时自动清理关联会话。

#### 4. 关键交互流程

1. **会话建立**：客户端发送`SESSION_START|||`，服务端生成会话ID返回`SESSION_START|ID|OK||`。
2. **数据传输**：客户端发送`DATA|ID|1|5|分片1数据`，服务端缓存并返回`DATA|ID|1|OK||`。
3. **会话结束**：客户端发送`SESSION_END|ID|||`，服务端合并分片数据，触发业务处理回调，销毁会话。

#### 5. 扩展接口

- 提供`OnSessionCompleted(sessionID string, data []byte)`回调函数，供上层业务处理完整数据。
- 提供 AddMessageHandler(g *gateway.Gateway, msg *Message) 方法，用于添加消息处理函数。
- 支持通过配置文件修改端口、会话超时时间、最大分片大小。


```go
type Message struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
  ClientID    string    `json:"client_id"`
	SessionID    string   `json:"session_id"`
	Direction   Direction `json:"direction"`
	Data        []byte    `json:"data,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}
```

### 提供客户端的通信接口

采用与服务端相同的会话式通信机制，与一至的数据格式保持一致。

### 要求

- 服务端只与Gateway通信，Session, SessionManager 这些类只应该被封装于包内部由Gateway去构造，持有与销毁，开发人员不应该直接使用这些类。
- 同样地客户端也只与唯的一Client的通信。

Gateway 与 Client 的构造函数采用 WithXXX Option 模式，尽量避免复杂的、多余的构造参数。

用法必须简单：

如（但不限于）以下用法：

```go
g := gateway.New(
  WithPort(8080), // 可选
  WithPath("/ws"), // 可选
  WithMessageHandler(myMessageHandler)
)

g.Start()
```

消息处理
```go

func myMessageHandler(g *gateway.Gateway, msg *Message) {
  // 处理消息
  fmt.Println(msg)
  // 回复消息给当前客户端
  g.Send("Message to current client")
  // 广播消息给所有客户端
  g.Broadcast("Message to all clients")

  // 发送文件给当前客户端
  g.SendFile("test.txt")
  // 广播文件给所有客户端
  g.BroadcastFile("test.txt")

  // 发送JSON数据给当前客户端
  g.SendJSON(map[string]string{"key": "value"})
  // 广播JSON数据给所有客户端
  g.BroadcastJSON(map[string]string{"key": "value"})
}
```

客户端：

```go
client := gateway.Client(
  WithPort(8080), // 可选
  WithPath("/ws"), // 可选
)

client.OnMessage(func(msg *Message) {
  fmt.Println(msg)
})

client.Send("Message to current client")
client.SendFile(io.Reader)
client.SendJSON(map[string]string{"key": "value"})
```

而与Channel之间的通信是隐形的，只是从 msg.ChannelID 来判断是哪个Channel的通信。但仍然会尊崇上述的通信机制。只是Channel可以用自身的客户端与Gateway通信。


