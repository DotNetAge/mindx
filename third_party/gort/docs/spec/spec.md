# Gort 重构规格说明 (Spec)

> 基于 2026-04-22 代码审计报告，定义重构的详细技术规格。

## 1. 范围与目标

### 目标
1. **恢复可编译性**: 修复所有编译错误，使 `go build ./...` 和 `go test ./...` 通过
2. **安全加固**: 修复所有 P1 级别安全漏洞
3. **代码质量**: 消除重复代码，统一错误处理
4. **测试保障**: 确保核心模块测试覆盖率 >= 85%

### 不在范围内
- 新功能开发
- 架构重新设计（保持现有架构）
- 第三方依赖版本升级（除非安全修复需要）

---

## 2. P0: channel.Message 接口修复规格

### 2.1 Message 结构体增强

**文件**: `pkg/channel/channel.go`

当前 `Message` 结构体需增加以下字段和方法：

```go
type Message struct {
    ID        string
    ChannelID string          // 新增: 渠道标识符
    Type      MessageType
    Direction Direction
    From      UserInfo
    To        UserInfo
    Content   string
    Data      []byte
    Raw       []byte
    Timestamp string          // 保持 string 类型以兼容现有代码
    Metadata  map[string]interface{}
}
```

### 2.2 UserInfo 结构体增强

```go
type UserInfo struct {
    ID       string
    Name     string
    Avatar   string
    Language string
    Platform string         // 新增: 平台标识符
}
```

### 2.3 新增 MessageType 常量

```go
const (
    MessageTypeText        MessageType = "text"
    MessageTypeImage       MessageType = "image"
    MessageTypeFile        MessageType = "file"
    MessageTypeAudio       MessageType = "audio"
    MessageTypeVideo       MessageType = "video"
    MessageTypeEvent       MessageType = "event"
    MessageTypeMarkdown    MessageType = "markdown"     // 新增
    MessageTypeNews        MessageType = "news"         // 新增
    MessageTypeVoice       MessageType = "voice"        // 新增
    MessageTypeTemplateCard MessageType = "template_card" // 新增
)
```

### 2.4 新增方法

```go
// GetMetadata 安全地获取 metadata 值
func (m *Message) GetMetadata(key string) (interface{}, bool) {
    if m.Metadata == nil {
        return nil, false
    }
    v, ok := m.Metadata[key]
    return v, ok
}

// SetMetadata 安全地设置 metadata 值
func (m *Message) SetMetadata(key string, value interface{}) {
    if m.Metadata == nil {
        m.Metadata = make(map[string]interface{})
    }
    m.Metadata[key] = value
}
```

### 2.5 NewMessage 构造函数

```go
func NewMessage(id, channelID string, direction Direction, from UserInfo, content string, msgType MessageType) *Message {
    return &Message{
        ID:        id,
        ChannelID: channelID,
        Type:      msgType,
        Direction: direction,
        From:      from,
        Content:   content,
        Metadata:  make(map[string]interface{}),
    }
}
```

### 2.6 各 Channel 文件修复清单

每个 channel 实现文件需要确认以下修改：

| 文件 | 需要修改的内容 |
|------|---------------|
| `wechat/wechat.go` | `ChannelID` 字段赋值、`Platform` 字段、`SetMetadata/GetMetadata` 调用、`Timestamp` 使用字符串 |
| `dingtalk/dingtalk.go` | 同上 |
| `telegram/telegram.go` | 同上 + `Timestamp` 从 time.Time 改为 string |
| `feishu/feishu.go` | 同上 + `Timestamp` 从 time.Time 改为 string |
| `slack/slack.go` | `NewMessage` 调用、`Platform` 字段、`GetMetadata/SetMetadata`、`MessageTypeMarkdown` |
| `discord/discord.go` | 同 slack |
| `wecom/wecom.go` | `MessageTypeMarkdown/News/Voice/TemplateCard`、`GetMetadata/SetMetadata` |
| `whatsapp/whatsapp.go` | `NewMessage` |
| `messenger/messenger.go` | `NewMessage`、`Platform` |
| `channel_test.go` | 更新引用不存在的类型 |
| `channel_bench_test.go` | 更新包引用 |

---

## 3. P1: WebSocket 安全加固规格

### 3.1 Origin 白名单验证

**文件**: `pkg/gateway/gateway.go`

将全局 `upgrader` 改为支持配置的 Origin 验证：

```go
type WSConfig struct {
    AllowedOrigins []string // 允许的 Origin 列表，支持通配符
    AllowAllInDev  bool     // 开发环境允许全部
}

var upgrader *websocket.Upgrader

func initUpgrader(cfg *WSConfig) {
    upgrader = &websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            origin := r.Header.Get("Origin")
            
            // 开发环境放行
            if cfg.AllowAllInDev && os.Getenv("GO_ENV") == "development" {
                return true
            }
            
            for _, allowed := range cfg.AllowedOrigins {
                if matchOrigin(origin, allowed) {
                    return true
                }
            }
            
            slog.Warn("rejected WebSocket connection", "origin", origin)
            return false
        },
    }
}

func matchOrigin(origin, pattern string) bool {
    if strings.HasSuffix(pattern, "*") {
        prefix := strings.TrimSuffix(pattern, "*")
        return strings.HasPrefix(origin, prefix)
    }
    return origin == pattern
}
```

### 3.2 Server 结构体扩展

```go
type Server struct {
    // ... 现有字段 ...
    wsConfig   *WSConfig
}
```

新增 Option:

```go
func WithWSConfig(cfg *WSConfig) Option {
    return func(s *Server) { s.wsConfig = cfg }
}
```

---

## 4. P1: 敏感信息保护规格

### 4.1 凭证传递方式变更

**原则**: 所有 Token、Secret、Key 等敏感参数必须通过 HTTP Body 传递，禁止拼接到 URL query string。

**受影响位置**:
- `pkg/channel/wechat/wechat.go` - `refreshToken()` 方法
- 其他可能存在类似问题的 channel

**修改模式**:

```go
// Before (不安全):
url := fmt.Sprintf("%s%s?appid=%s&secret=%s", base, endpoint, appID, secret)

// After (安全):
url := fmt.Sprintf("%s%s", base, endpoint)
body := map[string]string{"appid": appID, "secret": secret}
reqBody, _ := json.Marshal(body)
req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
```

### 4.2 日志脱敏规则

在日志输出中自动屏蔽敏感字段：
- 包含 `token`, `secret`, `key`, `password` 的值应显示为 `[REDACTED]`
- URL 中的 query 参数应过滤敏感字段

---

## 5. P1: 输入验证增强规格

### 5.1 消息大小限制

**常量定义** (`pkg/gateway/constants.go`):

```go
const (
    MaxMessageSize    = 1024 * 1024  // 1MB 单条消息最大长度
    MaxSessionParts   = 1000         // 单个 session 最大分片数
    MaxSessionTotal   = 1000         // Session 总数上限（从 10000 降低）
    MaxCommandLength  = 20           // 命令名称最大长度
)
```

### 5.2 命令白名单

```go
var validCommands = map[Command]bool{
    CmdSessionStart: true,
    CmdData:         true,
    CmdSessionEnd:   true,
}
```

### 5.3 dispatchCommand 增强

```go
func (s *Server) dispatchCommand(c *client, text string) {
    if len(text) > MaxMessageSize {
        slog.Warn("message exceeds max size", "id", c.id, "size", len(text))
        return
    }

    parts := strings.SplitN(text, "|", 5)
    if len(parts) < 1 {
        return
    }

    cmd := Command(parts[0])
    if !validCommands[cmd] {
        slog.Warn("unknown command rejected", "id", c.id, "cmd", cmd)
        return
    }

    switch cmd {
    case CmdSessionStart:
        s.handleSessionStart(c, parts)
    case CmdData:
        s.handleData(c, parts)
    case CmdSessionEnd:
        s.handleSessionEnd(c, parts)
    }
}
```

### 5.4 handleSessionStart 边界检查

```go
func (s *Server) handleSessionStart(c *client, parts []string) {
    total := 1
    if len(parts) >= 4 && parts[3] != "" {
        if _, err := fmt.Sscanf(parts[3], "%d", &total); err != nil {
            slog.Warn("invalid total format", "id", c.id, "value", parts[3])
            total = 1
        }
    }

    if total < 1 { total = 1 }
    if total > MaxSessionTotal { total = MaxSessionTotal }

    sess := c.sessions.create(c.id, total)
    resp := fmt.Sprintf("%s|%s|%s||", CmdSessionStart, sess.id, CmdOK)
    s.sendText(c, resp)
}
```

---

## 6. P2: HTTP 客户端统一规格

### 6.1 扩展 httpclient.Client

**文件**: `pkg/channel/httpclient/httpclient.go`

新增方法：

```go
// PostJSON 发送 JSON POST 请求并解析响应
func (c *Client) PostJSON(ctx context.Context, endpoint string, payload interface{}, result interface{}) error {
    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    respBody, err := c.Post(ctx, endpoint, map[string]string{
        "Content-Type": "application/json",
    }, body)
    if err != nil {
        return err
    }

    if result != nil {
        return json.Unmarshal(respBody, result)
    }
    return nil
}

// SetDefaultHeaders 设置默认请求头
func (c *Client) SetDefaultHeaders(headers map[string]string) {
    c.defaultHeaders = headers
}

// WithAuth 设置认证头
func (c *Client) WithAuth(tokenType, token string) *Client {
    c.defaultHeaders["Authorization"] = tokenType + " " + token
    return c
}
```

### 6.2 各 Channel 迁移模式

```go
// 迁移前:
type Channel struct {
    *channel.BaseChannel
    config     Config
    httpClient *http.Client  // 标准 net/http
}

// 迁移后:
type Channel struct {
    *channel.BaseChannel
    config     Config
    httpClient *httpclient.Client  // 统一 HTTP 客户端
    baseURL    string
}
```

### 6.3 统一 API 请求封装

每个 channel 的 `sendAPIRequest` / `apiRequest` 方法应统一使用 `httpclient.Client.PostJSON()`：

```go
func (c *Channel) sendAPIRequest(ctx context.Context, endpoint string, payload interface{}, result interface{}) error {
    return c.httpClient.PostJSON(ctx, endpoint, payload, result)
}
```

---

## 7. P2: Session 管理优化规格

### 7.1 资源限制

```go
type SessionManager struct {
    mu           sync.RWMutex
    sessions     map[string]*session
    timeout      time.Duration
    done         chan struct{}
    once         sync.Once
    maxSessions  int              // 新增: 最大 session 数量
    currentCount int32            // 新增: 原子计数器
}
```

### 7.2 创建时检查限制

```go
func (sm *SessionManager) create(clientID string, total int) (*session, error) {
    count := atomic.AddInt32(&sm.currentCount, 1)
    if int(count) > sm.maxSessions {
        atomic.AddInt32(&sm.currentCount, -1)
        return nil, fmt.Errorf("session limit reached (%d)", sm.maxSessions)
    }
    
    id := uuid.New().String()
    s := &session{...}
    sm.mu.Lock()
    sm.sessions[id] = s
    sm.mu.Unlock()
    return s, nil
}
```

### 7.3 清理间隔缩短

清理间隔从 30 秒改为 10 秒。

---

## 8. P2: 错误处理统一规格

### 8.1 统一 Sentinel Errors

建议创建 `pkg/errors/` 包或直接在 `pkg/channel/` 中扩展：

```go
// 通用错误
var (
    ErrInvalidArgument  = errors.New("invalid argument")
    ErrNotFound         = errors.New("not found")
    ErrPermissionDenied = errors.New("permission denied")
    ErrRateLimited      = errors.New("rate limited")
    ErrTimeout          = errors.New("timeout")
    ErrUnavailable      = errors.New("service unavailable")
)

// Channel 专用错误
var (
    ErrChannelNotRunning   = errors.New("channel is not running")
    ErrChannelAlreadyRunning = errors.New("channel is already running")
    ErrChannelNotFound     = errors.New("channel not found")
    ErrMessageTooLarge     = errors.New("message exceeds size limit")
    ErrUnsupportedType     = errors.New("unsupported message type")
    ErrTokenExpired        = errors.New("token expired or not available")
    ErrAuthenticationFailed = errors.New("authentication failed")
)

// 验证错误
var (
    ErrRequiredField = errors.New("required field is missing")
    ErrInvalidFormat = errors.New("invalid format")
    ErrInvalidRange  = errors.New("value out of valid range")
)
```

### 8.2 错误包装规范

```go
// 推荐格式: 使用 fmt.Errorf + %w 进行错误链包装
return fmt.Errorf("%w: channel=%s", ErrChannelNotRunning, ch.Name())

// 不推荐: 直接 errors.New 不带上下文
return errors.New("channel is not running")

// 不推荐: 仅用 fmt.Errorf 但不用 sentinel error
return fmt.Errorf("channel %s is not running", ch.Name())
```

---

## 9. 验收标准

### 编译验收
```bash
go build ./...          # 必须通过，无警告无错误
go vet ./...            # 必须通过
golangci-lint run...    # 无新引入的问题
```

### 测试验收
```bash
go test ./... -count=1 -race  # 全部通过，无竞态
go test ./... -cover          # 核心模块覆盖率 >= 85%
```

### 安全验收
- [ ] `CheckOrigin` 在非开发环境拒绝未知 Origin
- [ ] 无凭证出现在 URL query string 中
- [ ] 消息大小有明确上限
- [ ] 命令经过白名单验证

### 代码质量验收
- [ ] 无重复的 HTTP 客户端实现（各 channel 使用 httpclient）
- [ ] 错误处理使用统一的 sentinel errors
- [ ] Session 有数量上限保护
