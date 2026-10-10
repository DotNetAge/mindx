package svc

import (
	"context"
	"encoding/json"
	"testing"
)

// 市场账号 RPC 测试纪律：不触碰 credStore 写路径（darwin 下是真实钥匙串）。
// 本组用例只覆盖参数校验与匿名形态；身份链路（懒登记/绑定/评分读写）由
// core 与 bundle 包的 mock 服务端测试覆盖。

func TestHandleMarketRate_RequiresKindAndName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]any{"kind": "", "name": "", "score": 5})
	if _, err := d.handleMarketRate(context.Background(), params); err == nil {
		t.Fatal("kind/name 为空应报错")
	}
}

func TestHandleMarketRate_RequiresScore(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]any{"kind": "agent", "name": "demo"})
	if _, err := d.handleMarketRate(context.Background(), params); err == nil {
		t.Fatal("缺 score 应报错（bundle.Rate 越界校验）")
	}
}

func TestHandleMarketRatingGet_RequiresKindAndName(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]any{"kind": "agent", "name": ""})
	if _, err := d.handleMarketRatingGet(context.Background(), params); err == nil {
		t.Fatal("name 为空应报错")
	}
}

// TestHandleMarketAccountGet_Anonymous 匿名形态：has_token=false、user=nil。
// 不触发 Market() 之外的凭据写路径；Market() 懒创建在测试模式下仅用内存身份。
func TestHandleMarketAccountGet_Anonymous(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleMarketAccountGet(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleMarketAccountGet error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["has_token"] != false {
		t.Errorf("has_token = %v, want false", m["has_token"])
	}
	if m["user"] != nil {
		t.Errorf("user = %v, want nil", m["user"])
	}
}
