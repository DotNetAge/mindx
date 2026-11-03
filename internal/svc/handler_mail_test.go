package svc

import (
	"context"
	"testing"
)

// TestHandleMailDeliver_LocalOnly 直投口闸：非本机直通连接一律拒绝
// （无 client_id / 外接设备 / 闸未装配均走拒绝分支，防伪造发信人投信）。
// 收信合法通道是 daemon 轮询（mailLoopOnce），直投口仅本机直通可用。
func TestHandleMailDeliver_LocalOnly(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleMailDeliver(context.Background(), []byte(`{"uid":"u1","body":"hi"}`))
	if err == nil {
		t.Fatal("expected rejection for non-local-verified connection, got nil")
	}
}
