# MindX 插件化功能添加建议方案

## 一、Gateway 防护模块架构回顾

### 当前设计确认

MindX 的 Gateway 防护模块已经采用了**插件化（Plugin）**设计，核心要点如下：

1. **接口解耦**：`core.AuthProvider` 接口定义在核心层（`internal/core/auth.go`），但不包含任何具体实现
2. **默认不启用**：`NoopProvider`（`internal/usecase/auth/noop_provider.go`）作为默认提供者，`Enabled()` 返回 `false`，所有请求直接放行
3. **可选注入**：`NewServer()` 通过可变参数 `authProvider ...core.AuthProvider` 接受防护插件，不传入则不启用
4. **公开路径白名单**：`PublicPaths()` 允许插件声明不需要防护的路径（如 `/health`）
5. **i18n 支持**：中间件将 i18n 翻译后的拒绝消息注入到请求上下文，防护插件可通过 `c.Get("auth.unauthorized_message")` 获取
6. **前端面板开关**：通过 GeneralSettings 面板可选地开启/关闭 Gateway 防护，配置持久化到 `server.yml` 的 `gateway_protection` 字段

### 设计初衷

防护模块的初衷是**保护 Gateway 不被外部入侵**，而不是要求用户登录。这完全规避了以下问题：

> "在这个阶段是否真的需要增加登录？每次进入都可能会提示，虽然可以用记住密码跳过，但就变得意义不大。"

通过插件化设计，防护功能仅在需要时按需启用（通过前端面板或配置文件），不影响默认使用体验，也不会产生任何登录提示。

---

## 二、插件化功能添加建议方案

以下是在 MindX 中添加新插件化功能的建议方案，遵循现有的认证模块设计模式。

### 方案概览

```
                    ┌─────────────────────┐
                    │   core (接口定义)     │
                    │   - XxxProvider      │
                    └──────────┬──────────┘
                               │ 实现
               ┌───────────────┼───────────────┐
               │               │               │
    ┌──────────▼──┐  ┌────────▼────────┐  ┌──▼──────────┐
    │ NoopProvider │  │ BuiltinProvider │  │ CustomPlugin │
    │  (默认空实现) │  │  (内置实现)      │  │  (用户自定义) │
    └─────────────┘  └─────────────────┘  └─────────────┘
```

### 步骤 1：在核心层定义接口

在 `internal/core/` 中定义插件接口，只包含行为契约，不包含实现细节：

```go
// internal/core/rate_limiter.go（示例）
package core

import "github.com/gin-gonic/gin"

// RateLimiterProvider 限流插件接口
// 设计为可插拔模块，默认不启用，需要时通过注入方式添加
type RateLimiterProvider interface {
    Name() string
    Enabled() bool
    Middleware() gin.HandlerFunc
}
```

**关键原则**：
- 接口定义简洁，只暴露必要的方法
- 包含 `Enabled()` 方法，允许运行时开关
- 返回 `nil` 时表示不执行任何操作

### 步骤 2：提供默认的 Noop 实现

在 `internal/usecase/` 下创建默认实现，保证系统在没有插件时正常运行：

```go
// internal/usecase/ratelimit/noop_provider.go（示例）
package ratelimit

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider { return &NoopProvider{} }
func (p *NoopProvider) Name() string               { return "noop" }
func (p *NoopProvider) Enabled() bool              { return false }
func (p *NoopProvider) Middleware() gin.HandlerFunc { return nil } // 中间件层已处理 nil 情况
```

### 步骤 3：在中间件层集成

在 `internal/adapters/http/middleware/` 中创建中间件包装器：

```go
// internal/adapters/http/middleware/rate_limiter.go（示例）
func RateLimiter(provider core.RateLimiterProvider) gin.HandlerFunc {
    if provider == nil || !provider.Enabled() {
        return func(c *gin.Context) { c.Next() }
    }
    return provider.Middleware()
}
```

### 步骤 4：通过可选参数注入

在 `NewServer()` 或 `App` 初始化时，通过可选参数或配置注入插件：

```go
// 方式一：可选参数（适合少量插件）
func NewServer(port int, staticDir string, opts ...ServerOption) (*Server, error)

// 方式二：选项模式（推荐，适合多插件场景）
type ServerOption func(*Server)

func WithAuth(provider core.AuthProvider) ServerOption {
    return func(s *Server) { s.authProvider = provider }
}

func WithRateLimiter(provider core.RateLimiterProvider) ServerOption {
    return func(s *Server) { s.rateLimiter = provider }
}
```

### 步骤 5：添加 i18n 支持

在 `pkg/i18n/locales/` 的 JSON 文件中添加插件相关的翻译键：

```json
{
  "ratelimit.exceeded": "请求频率过高，请稍后再试",
  "ratelimit.plugin.enabled": "限流插件已启用: {{.Name}}"
}
```

### 步骤 6：编写测试

测试应覆盖以下场景：
- `nil` 提供者 → 所有请求通过
- `NoopProvider` → 所有请求通过
- 启用的自定义提供者 → 正确执行插件逻辑
- 禁用的自定义提供者 → 所有请求通过

---

## 三、推荐可插件化的功能列表

| 功能模块 | 接口名称 | 说明 |
|---------|---------|------|
| Gateway 防护 | `AuthProvider` | ✅ 已实现，保护 Gateway 不被外部入侵，可在前端面板开关 |
| 限流 | `RateLimiterProvider` | 防止 API 被恶意大量调用 |
| 审计日志 | `AuditLogProvider` | 记录关键操作便于审计追踪 |
| 通知 | `NotificationProvider` | 异常告警通知（邮件、Webhook 等） |
| 缓存 | `CacheProvider` | 响应缓存，减少重复推理开销 |
| CORS | `CORSProvider` | 跨域策略，按需配置 |

---

## 四、核心设计原则

1. **默认关闭，按需启用**：所有插件默认 `Enabled() == false`，避免不必要的功能干扰
2. **接口在核心层，实现在外围**：保持 `internal/core/` 的纯净，实现放在 `internal/usecase/` 或用户自定义包中
3. **零侵入注入**：通过可选参数或选项模式注入，不修改已有函数签名
4. **i18n 友好**：所有用户可见消息使用 i18n 键，支持多语言
5. **可测试性**：每个插件提供 Mock 实现，测试覆盖启用/禁用/公开路径等场景
