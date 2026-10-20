package svc

import (
	"context"
	"testing"
)

// 设备授权登录编排测试纪律：不打真实网络（login 的服务端交互由 bundle 层
// mock 测试与服务端集成测试覆盖），本组只覆盖状态机与查询形态。

func TestHandleMarketDeviceStatus_InitiallyIdle(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleMarketDeviceStatus(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleMarketDeviceStatus error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["status"] != "idle" {
		t.Errorf("status = %v, want idle", m["status"])
	}
	if m["user"] != nil {
		t.Errorf("user = %v, want nil", m["user"])
	}
}

func TestMarketDeviceLoginState_SetClearsUser(t *testing.T) {
	var s marketDeviceLoginState
	s.mu.Lock()
	s.phase, s.user = "done", "octo"
	s.mu.Unlock()

	// 非 done 状态切换应清空 user 快照与授权页 URL
	s.set("error", "超时", "")
	phase, errMsg, authURL, user := s.snapshot()
	if phase != "error" || errMsg != "超时" || authURL != "" || user != nil {
		t.Errorf("set 后状态异常：phase=%q err=%q url=%q user=%v", phase, errMsg, authURL, user)
	}

	// pending 状态携带授权页 URL（重复点「重新打开」时重新广播）
	s.set("pending", "", "https://community.example/auth/login/github")
	if _, _, authURL, _ := s.snapshot(); authURL == "" {
		t.Error("pending 状态应缓存授权页 URL")
	}
}
