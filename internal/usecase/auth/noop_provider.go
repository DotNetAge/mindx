package auth

import "github.com/gin-gonic/gin"

// NoopProvider 默认 Gateway 防护提供者（不启用防护）
//
// 契合 MindX「轻量化仿生大脑」的设计理念，默认不开启 Gateway 防护。
// 不会产生任何登录提示——防护模块的初衷是保护 Gateway 不被外部入侵，
// 而不是要求用户登录。
//
// 当用户需要外网防护时，可通过前端面板开启，或实现 core.AuthProvider 接口替换此默认提供者。
type NoopProvider struct{}

// NewNoopProvider 创建默认的无认证提供者
func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) Name() string {
	return "noop"
}

func (p *NoopProvider) Enabled() bool {
	return false
}

func (p *NoopProvider) Middleware() gin.HandlerFunc {
	return nil
}

func (p *NoopProvider) PublicPaths() []string {
	return nil
}
