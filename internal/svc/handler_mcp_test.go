package svc

import (
	"context"
	"testing"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/mcp"
	"github.com/DotNetAge/mindx/pkg/logging"
)

// withMCPManager 为 daemon 装配真实 MCPManager（配置落在隔离 HOME 的 mcp.json，
// add/remove 仅写文件不拉起进程），并保证测试后还原与关闭。
func withMCPManager(t *testing.T) *Daemon {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)

	logger := logging.DefaultZapLogger(&logging.ZapConfig{Console: false})
	d.mcpMgr = mcp.NewManager(logger, core.NewCredentialStore(d.app.Settings().UserPreferences()), d.app.Settings().McpFile(), mcp.DefaultMarketMCPURL)
	return d
}

// withNilMCPManager 临时置空 mcpMgr 测服务不可用分支，结束恢复原指针。
func withNilMCPManager(t *testing.T, d *Daemon, fn func()) {
	t.Helper()
	orig := d.mcpMgr
	d.mcpMgr = nil
	defer func() { d.mcpMgr = orig }()
	fn()
}

func TestHandleMCPServerLifecycle(t *testing.T) {
	d := withMCPManager(t)
	ctx := context.Background()

	if d.MCPManager() != d.mcpMgr {
		t.Fatal("MCPManager() should return the manager")
	}

	// add：stdio 推断（不传 type，按 command 推断）
	addParams := mustParams(t, map[string]any{"name": "echo", "command": "/bin/echo"})
	if _, err := d.handleMCPServerAdd(ctx, addParams); err != nil {
		t.Fatalf("add error = %v", err)
	}

	// list：应包含刚添加的条目
	result, err := d.handleMCPServerList(ctx, nil)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	servers := result.([]mcp.ServerListEntry)
	if len(servers) != 1 || servers[0].Name != "echo" {
		t.Fatalf("servers = %#v, want [echo]", result)
	}
	if servers[0].Type != mcp.ServerTypeStdio {
		t.Errorf("type = %v, want stdio（按 command 推断）", servers[0].Type)
	}

	// set_enabled
	if _, err := d.handleMCPServerSetEnabled(ctx, mustParams(t, map[string]any{"name": "echo", "enabled": false})); err != nil {
		t.Fatalf("set_enabled error = %v", err)
	}
	result, err = d.handleMCPServerList(ctx, nil)
	if err != nil {
		t.Fatalf("list after set_enabled error = %v", err)
	}
	servers = result.([]mcp.ServerListEntry)
	if servers[0].Enabled {
		t.Errorf("enabled = true, want false")
	}

	// update 缺 type → 校验失败（update 与 add 不同，type 必填）
	if _, err := d.handleMCPServerUpdate(ctx, mustParams(t, map[string]any{"name": "echo", "command": "/bin/echo", "args": []string{"hi"}})); err == nil {
		t.Error("update without type: expected error")
	}
	// update 成功（type 必填）
	if _, err := d.handleMCPServerUpdate(ctx, mustParams(t, map[string]any{"name": "echo", "type": "stdio", "command": "/bin/echo", "args": []string{"hi"}})); err != nil {
		t.Fatalf("update error = %v", err)
	}

	// remove
	if _, err := d.handleMCPServerRemove(ctx, mustParams(t, map[string]any{"name": "echo"})); err != nil {
		t.Fatalf("remove error = %v", err)
	}
	result, err = d.handleMCPServerList(ctx, nil)
	if err != nil {
		t.Fatalf("list after remove error = %v", err)
	}
	if servers = result.([]mcp.ServerListEntry); len(servers) != 0 {
		t.Errorf("servers after remove = %#v, want empty", servers)
	}
}

func TestHandleMCPServerErrors(t *testing.T) {
	d := withMCPManager(t)
	ctx := context.Background()

	// update 不存在的 server → 报错引导 add
	if _, err := d.handleMCPServerUpdate(ctx, mustParams(t, map[string]any{"name": "ghost", "command": "/bin/echo"})); err == nil {
		t.Error("update missing server: expected error")
	}
	// add 缺 command/url → 校验失败
	if _, err := d.handleMCPServerAdd(ctx, mustParams(t, map[string]any{"name": "empty"})); err == nil {
		t.Error("add without command/url: expected error")
	}
	// remove 不存在的 server → 幂等成功（RemoveServerByName 找不到不报错）
	if _, err := d.handleMCPServerRemove(ctx, mustParams(t, map[string]any{"name": "ghost"})); err != nil {
		t.Errorf("remove missing server should be idempotent, got %v", err)
	}
	// set_enabled 不存在的 server
	if _, err := d.handleMCPServerSetEnabled(ctx, mustParams(t, map[string]any{"name": "ghost", "enabled": true})); err == nil {
		t.Error("set_enabled missing server: expected error")
	}
	// pending 列表：空清单
	result, err := d.handleMCPPendingList(ctx, nil)
	if err != nil {
		t.Fatalf("pending list error = %v", err)
	}
	_ = result
}

func TestHandleMCPServiceUnavailable(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	withNilMCPManager(t, d, func() {
		if _, err := d.handleMCPServerAdd(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("add = %v, want errMCPServiceUnavailable", err)
		}
		if _, err := d.handleMCPServerUpdate(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("update = %v", err)
		}
		if _, err := d.handleMCPServerRemove(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("remove = %v", err)
		}
		if _, err := d.handleMCPServerList(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("list = %v", err)
		}
		if _, err := d.handleMCPServerSetEnabled(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("set_enabled = %v", err)
		}
		if _, err := d.handleMCPServerTest(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("test = %v", err)
		}
		if _, err := d.handleMCPServerDiscover(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("discover = %v", err)
		}
		if _, err := d.handleMCPToolCall(context.Background(), nil); err != errMCPServiceUnavailable {
			t.Errorf("tool.call = %v", err)
		}
	})
}

func TestJSONErrorMessage(t *testing.T) {
	if errMCPServiceUnavailable.Error() == "" {
		t.Error("jsonError.Error() should not be empty")
	}
}
