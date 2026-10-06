package svc

import (
	"context"
	"encoding/json"
	"testing"
)

// 决策量表出网授权（metric.consent.get/set）handler 测试：
// 默认未授权（granted=false）→ 授权 → 撤回的全链状态机。
//
// 注意：kvStore 由 NewDaemon 构造函数直接初始化（bbolt 已持锁），
// 测试内不得再调 initKVStore（同进程二次打开同一 kv.db 会 flock 超时）。

func mustConsentGet(t *testing.T, d *Daemon) metricConsentState {
	t.Helper()
	raw, err := d.handleMetricConsentGet(context.Background(), nil)
	if err != nil {
		t.Fatalf("consent.get error = %v", err)
	}
	blob, _ := json.Marshal(raw)
	var state metricConsentState
	if err := json.Unmarshal(blob, &state); err != nil {
		t.Fatalf("unmarshal consent state: %v", err)
	}
	return state
}

func TestMetricConsentDefaultDenied(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	if d.kvStore == nil {
		t.Fatal("NewDaemon 应已初始化 kvStore")
	}

	state := mustConsentGet(t, d)
	if state.Granted {
		t.Fatal("从未设置授权时应默认拒绝（granted=false）")
	}
	if state.UpdatedAt != nil {
		t.Fatalf("从未设置时 updated_at 应省略，got %v", state.UpdatedAt)
	}
}

func TestMetricConsentSetAndRevoke(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 授权
	params, _ := json.Marshal(map[string]bool{"granted": true})
	if _, err := d.handleMetricConsentSet(context.Background(), params); err != nil {
		t.Fatalf("consent.set(true) error = %v", err)
	}
	state := mustConsentGet(t, d)
	if !state.Granted {
		t.Fatal("授权后 get 应返回 granted=true")
	}
	if state.UpdatedAt == nil {
		t.Fatal("授权后 updated_at 应有值")
	}

	// 撤回（幂等覆盖）
	params, _ = json.Marshal(map[string]bool{"granted": false})
	if _, err := d.handleMetricConsentSet(context.Background(), params); err != nil {
		t.Fatalf("consent.set(false) error = %v", err)
	}
	if state := mustConsentGet(t, d); state.Granted {
		t.Fatal("撤回后 get 应返回 granted=false")
	}
}
