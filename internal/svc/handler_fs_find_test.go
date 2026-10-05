package svc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

// TestFSFind 验证 fs.find：递归匹配、剪枝名单生效、上限截断、参数校验。
func TestFSFind(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("<board></board>"), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	wantA := mk("a.dash")                    // 根
	wantB := mk(".agents/dashboards/b.dash") // 隐藏目录（不在剪枝名单，应命中）
	mk("docs/c.dash")                        // 普通子目录
	mk("node_modules/pkg/d.dash")            // 剪枝目录内，不应命中
	mk(".git/hooks/e.dash")                  // 剪枝目录内，不应命中
	mk("notes/readme.md")                    // 不匹配 pattern
	for i := 0; i < 5; i++ {
		mk("many/f" + string(rune('0'+i)) + ".dash") // 供上限截断用
	}

	find := func(t *testing.T, params rpc.FSFindParams) []FSEntry {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		d, cleanup := newTestDaemon(t)
		defer cleanup()
		result, err := d.handleFSFind(context.Background(), raw)
		if err != nil {
			t.Fatalf("fs.find error = %v", err)
		}
		return result.([]FSEntry)
	}

	t.Run("递归命中含隐藏目录并剪枝依赖目录", func(t *testing.T) {
		entries := find(t, rpc.FSFindParams{Path: root, Pattern: "*.dash"})
		paths := map[string]bool{}
		for _, e := range entries {
			paths[e.Path] = true
			if e.IsDir {
				t.Errorf("结果不应含目录: %s", e.Path)
			}
		}
		for _, want := range []string{wantA, wantB} {
			if !paths[want] {
				t.Errorf("应命中 %s，实际: %v", want, paths)
			}
		}
		if len(paths) != 8 {
			t.Errorf("期望 8 条（2 + docs + 5 个 many），实际 %d: %v", len(paths), paths)
		}
	})

	t.Run("上限截断", func(t *testing.T) {
		entries := find(t, rpc.FSFindParams{Path: root, Pattern: "*.dash", MaxResults: 3})
		if len(entries) != 3 {
			t.Errorf("max_results=3 期望 3 条，实际 %d", len(entries))
		}
	})

	t.Run("参数校验", func(t *testing.T) {
		d, cleanup := newTestDaemon(t)
		defer cleanup()
		if raw, _ := json.Marshal(rpc.FSFindParams{Pattern: "*.dash"}); true {
			if _, err := d.handleFSFind(context.Background(), raw); err == nil {
				t.Error("空 path 应报错")
			}
		}
		if raw, _ := json.Marshal(rpc.FSFindParams{Path: root}); true {
			if _, err := d.handleFSFind(context.Background(), raw); err == nil {
				t.Error("空 pattern 应报错")
			}
		}
		if raw, _ := json.Marshal(rpc.FSFindParams{Path: root, Pattern: "["}); true {
			if _, err := d.handleFSFind(context.Background(), raw); err == nil {
				t.Error("非法 pattern 应报错")
			}
		}
	})
}
