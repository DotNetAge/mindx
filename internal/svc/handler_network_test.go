package svc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newAuthTestDaemon 最小认证测试装置：networkAuthenticator 只依赖
// 本机令牌落点（~/.mindx/.token，随 HOME 隔离到临时目录）与 devGate
// （此处 nil，设备令牌分支由 devgate 包自测覆盖）。
func newAuthTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // 令牌唯一落点跟随真实用户根，必须隔离 HOME
	return &Daemon{}
}

// publicReq 构造公网来源的升级请求（authenticator 的私网分支直接放行，
// Bearer/URL 凭证逻辑只在公网来源生效）。
func publicReq(tokenURL string, headers map[string]string) *http.Request {
	r := httptest.NewRequest("GET", tokenURL, nil)
	r.RemoteAddr = "203.0.113.7:44300" // TEST-NET-3 公网段
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestNetworkAuthenticator(t *testing.T) {
	d := newAuthTestDaemon(t)
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		t.Fatalf("生成本机令牌失败: %v", err)
	}
	auth := d.networkAuthenticator()

	t.Run("公网无凭证拒绝", func(t *testing.T) {
		if err := auth(publicReq("ws://example.com/ws", nil)); err == nil {
			t.Fatal("公网来源无凭证应被拒绝")
		}
	})

	t.Run("Bearer 头本机令牌放行", func(t *testing.T) {
		if err := auth(publicReq("ws://example.com/ws", map[string]string{
			"Authorization": "Bearer " + token,
		})); err != nil {
			t.Fatalf("Bearer 头携带本机令牌应放行: %v", err)
		}
	})

	t.Run("Bearer 头大小写不敏感", func(t *testing.T) {
		if err := auth(publicReq("ws://example.com/ws", map[string]string{
			"Authorization": "bearer " + token,
		})); err != nil {
			t.Fatalf("scheme 小写应放行: %v", err)
		}
	})

	t.Run("URL token 过渡兼容放行", func(t *testing.T) {
		if err := auth(publicReq("ws://example.com/ws?token="+token, nil)); err != nil {
			t.Fatalf("?token= 过渡兼容应放行: %v", err)
		}
	})

	t.Run("错误令牌拒绝", func(t *testing.T) {
		err := auth(publicReq("ws://example.com/ws?token=wrong-token", nil))
		if err == nil {
			t.Fatal("错误令牌应被拒绝")
		}
		err = auth(publicReq("ws://example.com/ws", map[string]string{
			"Authorization": "Bearer wrong-token",
		}))
		if err == nil {
			t.Fatal("Bearer 错误令牌应被拒绝")
		}
	})

	t.Run("Bearer 头优先于 URL token", func(t *testing.T) {
		// 头携带正确令牌、URL 携带垃圾值：应按头放行（头优先语义）
		if err := auth(publicReq("ws://example.com/ws?token=garbage", map[string]string{
			"Authorization": "Bearer " + token,
		})); err != nil {
			t.Fatalf("Bearer 头有效时应忽略 URL 参数: %v", err)
		}
	})

	t.Run("私网来源无凭证放行", func(t *testing.T) {
		r := httptest.NewRequest("GET", "ws://example.com/ws", nil)
		r.RemoteAddr = "192.168.1.24:51000"
		if err := auth(r); err != nil {
			t.Fatalf("私网来源应直接放行: %v", err)
		}
	})
}

// TestBearerTokenParsing bearerToken 提取边界：空头/非 Bearer scheme/裸值。
func TestBearerTokenParsing(t *testing.T) {
	cases := []struct {
		name string
		auth string
		want string
	}{
		{"空头", "", ""},
		{"标准 Bearer", "Bearer abc123", "abc123"},
		{"小写 scheme", "bearer abc123", "abc123"},
		{"非 Bearer scheme", "Basic dXNlcjpwYXNz", ""},
		{"裸值无 scheme", "abc123", ""},
		{"Bearer 带尾随空格", "Bearer abc123  ", "abc123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tc.auth != "" {
				r.Header.Set("Authorization", tc.auth)
			}
			if got := bearerToken(r); got != tc.want {
				t.Fatalf("bearerToken() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestConstantTimeEqual 常量时间比较语义：等值 1、不等 0。
func TestConstantTimeEqual(t *testing.T) {
	if !constantTimeEqual("abc", "abc") {
		t.Fatal("等值应为 true")
	}
	if constantTimeEqual("abc", "abd") {
		t.Fatal("不等应为 false")
	}
	if constantTimeEqual("abc", "abcd") {
		t.Fatal("长度不同应为 false")
	}
	if constantTimeEqual("", "") {
		t.Fatal("双空串应为 false（空凭证永不匹配）")
	}
}
