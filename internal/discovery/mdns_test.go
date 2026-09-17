package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

// TestStartAndDiscover 验证广播注册后本机能通过 mDNS 发现该服务（回环验证）。
func TestStartAndDiscover(t *testing.T) {
	// 注册在临时端口上，避免与真实 daemon（1314）冲突
	b, err := Start(18134, "MindX-Test@local")
	if err != nil {
		t.Fatalf("注册广播失败: %v", err)
	}
	defer b.Shutdown()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		t.Fatalf("创建解析器失败: %v", err)
	}
	entries := make(chan *zeroconf.ServiceEntry, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := resolver.Browse(ctx, ServiceType, Domain, entries); err != nil {
		t.Fatalf("浏览服务失败: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			t.Fatal("5 秒内未发现服务，广播可能未生效")
		case entry := <-entries:
			if entry == nil {
				continue
			}
			if entry.Port != 18134 {
				t.Fatalf("发现的端口不符: %d", entry.Port)
			}
			t.Logf("发现服务: 实例=%s 端口=%d TXT=%v", entry.Instance, entry.Port, entry.Text)
			return
		}
	}
}
