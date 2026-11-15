package mcp

import "testing"

// TestResolveSSEEndpoint endpoint 是服务端下发的数据，后续 POST 会带凭据，
// 因此只允许指向与 SSE 连接相同的主机。
//
// 回归点（P1 §4.10）：以前直接 ResolveReference 后存为 postURL，
// 恶意 MCP 服务回一个第三方地址即可把 Authorization / x-api-key 发出去。
func TestResolveSSEEndpoint(t *testing.T) {
	base := "https://mcp.example.com/sse"

	t.Run("相对路径按 SSE 地址解析", func(t *testing.T) {
		got, err := resolveSSEEndpoint(base, "/message?sessionId=1")
		if err != nil {
			t.Fatalf("相对路径应被接受: %v", err)
		}
		if want := "https://mcp.example.com/message?sessionId=1"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("同主机绝对地址放行", func(t *testing.T) {
		if _, err := resolveSSEEndpoint(base, "https://mcp.example.com:443/msg"); err != nil {
			t.Fatalf("同主机绝对地址应被接受: %v", err)
		}
	})

	t.Run("异主机拒绝", func(t *testing.T) {
		if _, err := resolveSSEEndpoint(base, "https://evil.example.net/msg"); err == nil {
			t.Fatal("异主机 endpoint 应被拒绝")
		}
		// 子域也属于异主机：凭据不能被引到兄弟域
		if _, err := resolveSSEEndpoint(base, "https://api.mcp.example.com/msg"); err == nil {
			t.Fatal("子域 endpoint 应被拒绝")
		}
	})

	t.Run("协议降级拒绝", func(t *testing.T) {
		if _, err := resolveSSEEndpoint(base, "http://mcp.example.com/msg"); err == nil {
			t.Fatal("https 降级到 http 应被拒绝")
		}
	})

	t.Run("空地址拒绝", func(t *testing.T) {
		if _, err := resolveSSEEndpoint(base, ""); err == nil {
			t.Fatal("空 endpoint 应被拒绝")
		}
	})
}
