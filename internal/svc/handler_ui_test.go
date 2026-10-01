package svc

import (
	"context"
	"testing"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// TestUIHandlersAcceptance 验收：ui.* 三方法受理语义——缺参报错、过闸参数受理；
// newTestDaemon 的 gw 为 nil，broadcastUI 应静默跳过（不炸）。
func TestUIHandlersAcceptance(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 缺参必须报错
	if _, err := d.handleUIOpen(context.Background(), mustJSON(t, map[string]any{})); err == nil {
		t.Fatal("缺 path 应报错")
	}
	if _, err := d.handleUIOpenLink(context.Background(), mustJSON(t, map[string]any{})); err == nil {
		t.Fatal("缺 url 应报错")
	}
	if _, err := d.handleUIRun(context.Background(), mustJSON(t, map[string]any{})); err == nil {
		t.Fatal("缺 command 应报错")
	}

	// 过闸参数必须受理（gw 为 nil：广播静默跳过，受理路径不炸）
	if _, err := d.handleUIOpen(context.Background(), mustJSON(t, map[string]any{"path": "/tmp/a.md"})); err != nil {
		t.Fatalf("过闸 open 应受理: %v", err)
	}
	if _, err := d.handleUIOpenLink(context.Background(), mustJSON(t, map[string]any{"url": "https://example.com"})); err != nil {
		t.Fatalf("过闸 open_link 应受理: %v", err)
	}
	if _, err := d.handleUIRun(context.Background(), mustJSON(t, map[string]any{"command": "pnpm test", "cwd": "/tmp"})); err != nil {
		t.Fatalf("过闸 run 应受理: %v", err)
	}

	// RPC 参数结构默认值兜底（pkg/rpc 定义的形状与 handler 解析字段一致）
	if _, err := d.handleUIOpen(context.Background(), mustJSON(t, rpc.UIOpenParams{})); err == nil {
		t.Fatal("空 path 结构体应报错")
	}
}
