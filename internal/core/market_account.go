// 市场账号域（App 辅助方法，独立文件）：设备身份、用户登录令牌（AuthToken）的
// 存取与绑定流程装配。凭据纪律与商户 PAT 一致：仅存密钥存储与内存，严禁落明文或写日志。
package core

import (
	"crypto/rand"
	"fmt"
)

// marketClientUUID 生成 UUIDv4 格式的设备身份（RFC 4122 随机版，128bit 不可猜测——
// 匿名身份唯一性降维为设备级的防伪造依据，§2.1）。
func marketClientUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// MarketClientID 返回设备身份 UUID（凭据存储持久化；UI 显示短码由前端截取）。
func (a *App) MarketClientID() string {
	a.marketMu.RLock()
	defer a.marketMu.RUnlock()
	return a.marketClientID
}

// MarketAuthToken 返回当前用户登录令牌。仅内存值，严禁写日志。
func (a *App) MarketAuthToken() string {
	a.marketMu.RLock()
	defer a.marketMu.RUnlock()
	return a.marketAuthToken
}

// SetMarketAuthToken 更新用户登录令牌：写入密钥存储（持久）与内存（运行时），
// 已建市场客户端立即生效（身份语义变化触发重新登记）。空串 = 清除令牌。
func (a *App) SetMarketAuthToken(token string) error {
	a.marketMu.Lock()
	defer a.marketMu.Unlock()
	if a.credStore != nil {
		if err := a.credStore.Set("market:auth_token", token); err != nil {
			return fmt.Errorf("市场登录令牌保存失败：%w", err)
		}
	}
	a.marketAuthToken = token
	if a.market != nil {
		a.market.SetAuthToken(token)
	}
	return nil
}
