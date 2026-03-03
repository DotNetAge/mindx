package core

import "github.com/gin-gonic/gin"

// AuthProvider 认证插件接口
//
// 设计为可插拔的认证模块，与核心层完全解耦。
// 默认使用 NoopAuthProvider（不启用认证），避免不必要的登录提示。
// 初衷：保护 Gateway 不被外部入侵，而非要求用户每次都登录。
//
// 用户可通过实现此接口添加自定义认证插件（如 JWT、API Key、OAuth 等），
// 并通过 NewServer 的可选参数注入，无需修改核心代码。
type AuthProvider interface {
	// Name 返回认证提供者名称
	Name() string

	// Enabled 返回认证是否启用
	// 返回 false 时中间件直接放行所有请求
	Enabled() bool

	// Middleware 返回 Gin 中间件，用于保护 API 路由
	// 返回 nil 表示不需要认证中间件
	// 插件可通过 c.Get("auth.unauthorized_message") 获取 i18n 翻译后的未授权消息
	Middleware() gin.HandlerFunc

	// PublicPaths 返回不需要认证的路径列表（如健康检查、登录等）
	PublicPaths() []string
}
