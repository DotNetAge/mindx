package middleware

import (
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"strings"

	"github.com/gin-gonic/gin"
)

// Auth Gateway 防护中间件（插件化设计）
//
// 采用 AuthProvider 接口实现插件化 Gateway 防护，不与核心层耦合。
// 设计目标：保护 Gateway 不被外部入侵，而不是要求用户登录。
// 当 AuthProvider 为 nil 或未启用时，所有请求直接放行。
func Auth(provider core.AuthProvider) gin.HandlerFunc {
	if provider == nil || !provider.Enabled() {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	authMiddleware := provider.Middleware()
	if authMiddleware == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	publicPaths := provider.PublicPaths()

	return func(c *gin.Context) {
		// 检查是否为公开路径（不需要认证）
		reqPath := c.Request.URL.Path
		for _, pp := range publicPaths {
			if strings.HasPrefix(reqPath, pp) {
				c.Next()
				return
			}
		}

		// 将 i18n 翻译后的未授权消息注入上下文，供认证插件使用
		c.Set("auth.unauthorized_message", i18n.T("auth.unauthorized"))

		// 执行认证插件
		authMiddleware(c)
	}
}
