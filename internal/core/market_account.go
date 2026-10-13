// MindX 账号域（App 辅助方法，独立文件）：设备身份、用户登录令牌（AuthToken）的
// 存取与绑定流程装配。凭据纪律与商户 PAT 一致：仅存密钥存储与内存，严禁落明文或写日志。
package core

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
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
			return fmt.Errorf("社区登录凭据保存失败：%w", err)
		}
	}
	a.marketAuthToken = token
	if a.market != nil {
		a.market.SetAuthToken(token)
	}
	return nil
}

// MarketUsername / MarketPassword 登录凭据读取（仅内存值；严禁写日志）。
func (a *App) MarketUsername() string {
	a.marketMu.RLock()
	defer a.marketMu.RUnlock()
	return a.marketUsername
}

// MarketPassword 同上（仅内存值；严禁写日志）。
func (a *App) MarketPassword() string {
	a.marketMu.RLock()
	defer a.marketMu.RUnlock()
	return a.marketPassword
}

// SetMarketCredentials 录入/清除登录凭据（username+password 一组写）：
// 写密钥存储（重装恢复用）与内存。清除（空 username）时三件套一并清。
func (a *App) SetMarketCredentials(username, password string) error {
	a.marketMu.Lock()
	defer a.marketMu.Unlock()
	if a.credStore != nil {
		if err := a.credStore.Set("market:username", username); err != nil {
			return fmt.Errorf("登录凭据保存失败：%w", err)
		}
		if err := a.credStore.Set("market:password", password); err != nil {
			return fmt.Errorf("登录凭据保存失败：%w", err)
		}
	}
	a.marketUsername, a.marketPassword = username, password
	return nil
}

// MarketDeviceMeta 设备元数据上报值（设备列表渲染用）：主机名 + 平台。
func MarketDeviceMeta() (deviceName, platform string) {
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "未命名设备"
	}
	return name, runtime.GOOS
}

// HasMarketCredentials 是否已录入登录凭据（静默重登判据）。
func (a *App) HasMarketCredentials() bool {
	a.marketMu.RLock()
	defer a.marketMu.RUnlock()
	return a.marketUsername != "" && a.marketPassword != ""
}

// GenerateMarketCredentials 生成首次自动建号的随机凭据：用户名 = "mindx" + 10 位
// 随机小写字母数字（满足服务端 ^[a-zA-Z][a-zA-Z0-9_-]{2,31}$，冲突概率可忽略），
// 密码 = 24 位 base62 随机串（拒绝采样消除模偏差）。凭据仅经返回值交调用方
// 持久化到密钥存储，严禁写日志。
func GenerateMarketCredentials() (username, password string, err error) {
	const lowerAlpha = "abcdefghijklmnopqrstuvwxyz"
	const alphaNum = lowerAlpha + "0123456789"
	const pwChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + alphaNum

	userSuffix, err := randString(10, alphaNum)
	if err != nil {
		return "", "", fmt.Errorf("随机凭据生成失败：%w", err)
	}
	pass, err := randString(24, pwChars)
	if err != nil {
		return "", "", fmt.Errorf("随机凭据生成失败：%w", err)
	}
	return "mindx" + userSuffix, pass, nil
}

// randString 密码学随机字符串（rejection sampling 消除模偏差）。
func randString(n int, chars string) (string, error) {
	out := make([]byte, 0, n)
	max := 256 - (256 % len(chars))
	for len(out) < n {
		buf := make([]byte, n-len(out))
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= max {
				continue
			}
			out = append(out, chars[int(b)%len(chars)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}
