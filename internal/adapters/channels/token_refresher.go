package channels

import (
	"context"
	"mindx/pkg/logging"
	"sync"
	"time"
)

// TokenRefresher 统一的 Token 刷新组件
// 适用于飞书、微信、钉钉等需要 OAuth token 刷新的渠道
type TokenRefresher struct {
	mu           sync.RWMutex
	accessToken  string
	tokenExpires time.Time
	refreshFunc  func(ctx context.Context) (token string, expiresIn int, err error)
	logger       logging.Logger
}

// NewTokenRefresher 创建 Token 刷新器
// refreshFunc 负责调用各平台 API 获取新 token，返回 token 字符串和过期秒数
func NewTokenRefresher(refreshFunc func(ctx context.Context) (string, int, error), logger logging.Logger) *TokenRefresher {
	return &TokenRefresher{
		refreshFunc: refreshFunc,
		logger:      logger,
	}
}

// GetToken 获取有效的 access token（双重检查锁）
func (tr *TokenRefresher) GetToken(ctx context.Context) (string, error) {
	tr.mu.RLock()
	if tr.accessToken != "" && time.Now().Before(tr.tokenExpires) {
		token := tr.accessToken
		tr.mu.RUnlock()
		return token, nil
	}
	tr.mu.RUnlock()

	tr.mu.Lock()
	defer tr.mu.Unlock()

	// 双重检查：可能其他 goroutine 已经刷新
	if tr.accessToken != "" && time.Now().Before(tr.tokenExpires) {
		return tr.accessToken, nil
	}

	token, expiresIn, err := tr.refreshFunc(ctx)
	if err != nil {
		return "", err
	}

	tr.accessToken = token
	tr.tokenExpires = time.Now().Add(time.Duration(expiresIn) * time.Second)
	tr.logger.Info("token 刷新成功", logging.Int("expires_in", expiresIn))

	return tr.accessToken, nil
}
