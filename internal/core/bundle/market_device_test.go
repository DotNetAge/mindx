package bundle

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 设备授权登录（device flow）客户端测试：假市场服务按路由返回可控行为，
// 覆盖 start 响应解析与 poll 三态（pending/authorized/过期）。

func newDeviceFixture(t *testing.T, pollHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/device/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"dev_abc123","verification_url":"http://localhost/authorize/device?code=dev_abc123","expires_in":900,"interval":2}`))
	})
	mux.HandleFunc("/api/v1/auth/device/poll", pollHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDeviceStart_ParsesVerificationURL(t *testing.T) {
	srv := newDeviceFixture(t, func(w http.ResponseWriter, r *http.Request) {})
	m := NewMarketClient(srv.URL, "")

	start, err := m.DeviceStart()
	if err != nil {
		t.Fatalf("DeviceStart() error = %v", err)
	}
	if start.DeviceCode != "dev_abc123" {
		t.Errorf("device_code = %q", start.DeviceCode)
	}
	if start.VerificationURL == "" || start.Interval != 2 || start.ExpiresIn != 900 {
		t.Errorf("start 响应字段异常：%+v", start)
	}
}

func TestDevicePoll_PendingAndAuthorized(t *testing.T) {
	// pending：不签发
	srv := newDeviceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"pending"}`))
	})
	m := NewMarketClient(srv.URL, "")

	res, err := m.DevicePoll("dev_abc123")
	if err != nil {
		t.Fatalf("DevicePoll() error = %v", err)
	}
	if res.Status != "pending" || res.AuthToken != "" {
		t.Errorf("pending 轮询结果异常：%+v", res)
	}

	// authorized：带 AuthToken
	srv2 := newDeviceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"authorized","auth_token":"mk_token123"}`))
	})
	m2 := NewMarketClient(srv2.URL, "")

	res2, err := m2.DevicePoll("dev_abc123")
	if err != nil {
		t.Fatalf("DevicePoll() error = %v", err)
	}
	if res2.Status != "authorized" || res2.AuthToken != "mk_token123" {
		t.Errorf("authorized 轮询结果异常：%+v", res2)
	}
}

func TestDevicePoll_ExpiredMapsToError(t *testing.T) {
	// 服务端 expired_token：HTTP 404 → postJSON 映射为错误，调用方终止轮询
	srv := newDeviceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"expired_token","message":"设备授权会话不存在或已过期"}}`))
	})
	m := NewMarketClient(srv.URL, "")

	if _, err := m.DevicePoll("dev_gone"); err == nil {
		t.Fatal("过期会话轮询应返回错误")
	}
}
