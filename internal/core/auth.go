package core

import "github.com/gin-gonic/gin"

// AuthProvider Gateway 防护插件接口
//
// 设计为可插拔的 Gateway 防护模块，与核心层完全解耦。
// 初衷：保护 Gateway 不被外部入侵，而不是要求用户登录。
// 默认使用 NoopAuthProvider（不启用防护），不会产生任何登录提示。
//
// 当 Gateway 需要暴露到外部网络时，可通过前端面板开启防护，
// 或实现此接口添加自定义防护插件（如 API Key、Token 验证等）。
type AuthProvider interface {
	// Name 返回防护插件名称
	Name() string

	// Enabled 返回 Gateway 防护是否启用
	// 返回 false 时中间件直接放行所有请求
	Enabled() bool

	// Middleware 返回 Gin 中间件，用于保护 Gateway API 路由
	// 返回 nil 表示不需要防护中间件
	// 插件可通过 c.Get("auth.unauthorized_message") 获取 i18n 翻译后的拒绝消息
	Middleware() gin.HandlerFunc

	// PublicPaths 返回不需要防护的路径列表（如健康检查等）
	PublicPaths() []string
}
