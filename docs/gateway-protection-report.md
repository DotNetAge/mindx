# MindX Gateway 防护实现报告

## 一、设计理念

MindX 的 Gateway 防护模块以**保护 Gateway 不被外部入侵**为核心目标，而非要求用户登录。这完全契合 MindX「轻量化仿生大脑」的设计理念：

- **默认零负担**：不启用防护时，不会产生任何登录提示或认证开销
- **按需开启**：仅当 Gateway 暴露到外部网络时，才需要通过前端面板一键开启防护
- **插件化架构**：防护逻辑与核心业务完全解耦，不影响仿生大脑的运行效率

---

## 二、架构设计

### 分层架构

```
┌─────────────────────────────────────────────────┐
│              前端面板 (GeneralSettings)            │
│    Gateway 防护开关 → /api/config/general          │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│          core.AuthProvider 接口                   │
│  internal/core/auth.go                           │
│  - Name() / Enabled() / Middleware() / PublicPaths()│
└──────────────────────┬──────────────────────────┘
                       │ 实现
          ┌────────────┼────────────┐
          │            │            │
    ┌─────▼────┐ ┌─────▼────┐ ┌────▼─────┐
    │ NoopProvider│ │ TokenProvider│ │ 自定义插件 │
    │ (默认关闭)  │ │ (内置防护)   │ │ (可扩展)  │
    └──────────┘ └──────────┘ └──────────┘
```

### 关键文件

| 文件 | 职责 |
|------|------|
| `internal/core/auth.go` | 定义 `AuthProvider` 接口（核心层，无实现） |
| `internal/adapters/http/middleware/auth.go` | Gateway 防护中间件 |
| `internal/usecase/auth/noop_provider.go` | 默认空实现（不启用防护） |
| `internal/config/global.go` | `GatewayProtectionConfig` 配置结构体 |
| `internal/infrastructure/bootstrap/server.go` | 服务器启动时注入防护插件 |
| `dashboard/src/components/GeneralSettings.tsx` | 前端防护开关面板 |

---

## 三、防护实现方式

### 3.1 插件化接口 (`core.AuthProvider`)

```go
type AuthProvider interface {
    Name() string              // 防护插件名称
    Enabled() bool             // 是否启用（false = 全部放行）
    Middleware() gin.HandlerFunc  // 防护中间件
    PublicPaths() []string     // 白名单路径（如 /health）
}
```

**设计要点**：
- 接口定义在核心层，不包含任何具体实现
- `Enabled()` 返回 `false` 时，中间件直接放行所有请求，零开销
- `PublicPaths()` 允许声明不需要防护的路径（如健康检查）

### 3.2 中间件处理流程

```
请求到达 → 检查 Provider 是否启用
            ├── 未启用 → 直接放行（零开销）
            └── 已启用 → 检查是否公开路径
                         ├── 是公开路径 → 直接放行
                         └── 非公开路径 → 注入 i18n 拒绝消息
                                          → 执行防护插件验证
                                          → 通过 → 放行
                                          → 拒绝 → 返回 401
```

### 3.3 默认行为 (`NoopProvider`)

```go
type NoopProvider struct{}

func (p *NoopProvider) Name() string    { return "noop" }
func (p *NoopProvider) Enabled() bool   { return false }  // 默认关闭
func (p *NoopProvider) Middleware() gin.HandlerFunc { return nil }
func (p *NoopProvider) PublicPaths() []string { return nil }
```

**效果**：系统默认不启用任何防护，不会产生登录提示。

### 3.4 前端面板开关

通过 GeneralSettings 面板，用户可以一键开关 Gateway 防护：

- **配置路径**：`server.yml` → `gateway_protection.enabled`
- **API 端点**：`GET/POST /api/config/general`
- **前端组件**：Toggle 开关 + 状态描述

```typescript
// 前端配置接口
interface GeneralConfig {
  gateway_protection: {
    enabled: boolean;  // 防护开关
    mode: string;      // 防护模式（预留扩展）
  };
}
```

---

## 四、防护策略对比

| 特性 | 传统登录方式 | MindX Gateway 防护 |
|------|------------|-------------------|
| 用户体验 | 每次访问需登录 | 默认无感知，按需开启 |
| 资源消耗 | 需要 session/token 管理 | 未启用时零开销 |
| 防护目标 | 用户身份认证 | 阻止外部入侵 |
| 启用方式 | 强制开启 | 前端面板可选开关 |
| 可扩展性 | 固定实现 | 插件化，支持自定义 |

---

## 五、具体防护方式建议

### 方式 A：API Key 验证（推荐，最轻量）

适合本地部署暴露到公网场景：

```go
type APIKeyProvider struct {
    apiKey string
}

func (p *APIKeyProvider) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        key := c.GetHeader("X-API-Key")
        if key != p.apiKey {
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid api key"})
            return
        }
        c.Next()
    }
}
```

**优势**：无需 session 管理，无登录页面，纯 API 级防护。

### 方式 B：Bearer Token 验证

适合需要动态 token 的场景：

```go
func (p *TokenProvider) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if !strings.HasPrefix(token, "Bearer ") || !p.validateToken(token[7:]) {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        c.Next()
    }
}
```

### 方式 C：IP 白名单

适合固定网络环境：

```go
func (p *IPWhitelistProvider) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        clientIP := c.ClientIP()
        if !p.isAllowed(clientIP) {
            c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
            return
        }
        c.Next()
    }
}
```

---

## 六、测试覆盖

| 测试场景 | 文件 | 状态 |
|---------|------|------|
| nil Provider → 全部放行 | `auth_test.go` | ✅ |
| NoopProvider → 全部放行 | `auth_test.go` | ✅ |
| 启用防护 → 保护路由拦截 | `auth_test.go` | ✅ |
| 启用防护 → 公开路由放行 | `auth_test.go` | ✅ |
| 禁用 Provider → 全部放行 | `auth_test.go` | ✅ |
| i18n 拒绝消息注入 | `auth_test.go` | ✅ |
| NoopProvider 属性验证 | `noop_provider_test.go` | ✅ |

---

## 七、总结

MindX 的 Gateway 防护实现完全遵循「轻量化仿生大脑」的设计理念：

1. **轻量**：默认不启用，零运行时开销
2. **可插拔**：通过 `AuthProvider` 接口实现完全解耦
3. **可控**：前端面板一键开关，配置持久化到 `server.yml`
4. **不侵入**：与核心业务逻辑（大脑、记忆、技能）完全隔离
5. **不打扰**：不产生任何登录提示，纯后台防护
