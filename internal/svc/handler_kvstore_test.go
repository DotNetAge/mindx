package svc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"go.etcd.io/bbolt"
)

// withNilKVStore 临时把 d.kvStore 置 nil 跑 fn，结束后恢复原指针再交还 cleanup。
// stopBackgroundServices 会 Close kvStore，必须恢复原指针避免 nil panic / 误关。
func withNilKVStore(t *testing.T, d *Daemon, fn func()) {
	t.Helper()
	orig := d.kvStore
	d.kvStore = nil
	defer func() { d.kvStore = orig }()
	fn()
}

// putRawKVItem 绕过 handler 直接写一条 kvItem（测试预置数据用）。
func putRawKVItem(t *testing.T, d *Daemon, item kvItem) {
	t.Helper()
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal kvItem: %v", err)
	}
	err = d.kvStore.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		return b.Put([]byte(item.Key), raw)
	})
	if err != nil {
		t.Fatalf("put raw kv item: %v", err)
	}
}

func kvFound(t *testing.T, result any) (*kvItem, bool) {
	t.Helper()
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	found, _ := m["found"].(bool)
	item, _ := m["item"].(*kvItem)
	return item, found
}

func TestHandleKVGet_Validation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: ""})); err == nil {
		t.Error("empty key: expected error, got nil")
	}

	withNilKVStore(t, d, func() {
		if _, err := d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: "k"})); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestHandleKVGet_NotFound(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: "missing"}))
	if err != nil {
		t.Fatalf("handleKVGet error = %v", err)
	}
	if _, found := kvFound(t, result); found {
		t.Error("expected found=false for missing key")
	}
}

func TestHandleKVGet_ExpiredTTL(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 直接落一条已过期的条目，避免测试 sleep 等真过期
	putRawKVItem(t, d, kvItem{
		Key: "stale", Value: "old",
		CreatedAt: time.Now().Unix() - 100,
		ExpiresAt: time.Now().Unix() - 50,
	})

	result, err := d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: "stale"}))
	if err != nil {
		t.Fatalf("handleKVGet error = %v", err)
	}
	if _, found := kvFound(t, result); found {
		t.Error("expected found=false for expired key")
	}
	// 过期条目应被惰性删除
	if got := d.kvGetStringInternal("stale"); got != "" {
		t.Errorf("expired entry should be deleted lazily, got %q", got)
	}
}

func TestHandleKVSetGetRoundTrip(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVSet(context.Background(), mustJSON(t, rpc.KVSetParams{
		Key: "greeting", Value: map[string]any{"lang": "zh"}, TTL: 3600,
	})); err != nil {
		t.Fatalf("handleKVSet error = %v", err)
	}

	result, err := d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: "greeting"}))
	if err != nil {
		t.Fatalf("handleKVGet error = %v", err)
	}
	item, found := kvFound(t, result)
	if !found {
		t.Fatal("expected found=true after set")
	}
	if item.Key != "greeting" {
		t.Errorf("item.Key = %q, want greeting", item.Key)
	}
	val, _ := item.Value.(map[string]interface{})
	if val == nil || val["lang"] != "zh" {
		t.Errorf("item.Value = %#v, want map with lang=zh", item.Value)
	}
	if item.ExpiresAt <= time.Now().Unix() {
		t.Errorf("item.ExpiresAt = %d, want in the future (ttl=3600)", item.ExpiresAt)
	}
}

func TestHandleKVSet_Validation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVSet(context.Background(), mustJSON(t, rpc.KVSetParams{Key: ""})); err == nil {
		t.Error("empty key: expected error, got nil")
	}
	withNilKVStore(t, d, func() {
		if _, err := d.handleKVSet(context.Background(), mustJSON(t, rpc.KVSetParams{Key: "k"})); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestHandleKVDelete(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVSet(context.Background(), mustJSON(t, rpc.KVSetParams{Key: "gone", Value: "x"})); err != nil {
		t.Fatalf("handleKVSet error = %v", err)
	}

	// 参数校验
	if _, err := d.handleKVDelete(context.Background(), mustJSON(t, rpc.KVDeleteParams{Key: ""})); err == nil {
		t.Error("empty key: expected error, got nil")
	}

	result, err := d.handleKVDelete(context.Background(), mustJSON(t, rpc.KVDeleteParams{Key: "gone"}))
	if err != nil {
		t.Fatalf("handleKVDelete error = %v", err)
	}
	m, ok := result.(map[string]string)
	if !ok || m["status"] != "ok" || m["deleted_key"] != "gone" {
		t.Errorf("unexpected delete result: %#v", result)
	}

	result, err = d.handleKVGet(context.Background(), mustJSON(t, rpc.KVGetParams{Key: "gone"}))
	if err != nil {
		t.Fatalf("handleKVGet after delete error = %v", err)
	}
	if _, found := kvFound(t, result); found {
		t.Error("expected found=false after delete")
	}
}

func TestHandleKVDelete_NotInitialized(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	withNilKVStore(t, d, func() {
		if _, err := d.handleKVDelete(context.Background(), mustJSON(t, rpc.KVDeleteParams{Key: "k"})); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestKVInternalHelpers(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// nil store 行为
	withNilKVStore(t, d, func() {
		if err := d.kvSetInternal("a", "b"); err == nil {
			t.Error("kvSetInternal with nil store: expected error")
		}
		if err := d.kvDeleteInternal("a"); err == nil {
			t.Error("kvDeleteInternal with nil store: expected error")
		}
		if got := d.kvGetStringInternal("a"); got != "" {
			t.Errorf("kvGetStringInternal with nil store = %q, want empty", got)
		}
	})

	if err := d.kvSetInternal("flag", "on"); err != nil {
		t.Fatalf("kvSetInternal error = %v", err)
	}
	if got := d.kvGetStringInternal("flag"); got != "on" {
		t.Errorf("kvGetStringInternal = %q, want on", got)
	}
	// 非字符串值返回空串
	putRawKVItem(t, d, kvItem{Key: "num", Value: 42, CreatedAt: time.Now().Unix()})
	if got := d.kvGetStringInternal("num"); got != "" {
		t.Errorf("kvGetStringInternal on non-string value = %q, want empty", got)
	}
	if err := d.kvDeleteInternal("flag"); err != nil {
		t.Fatalf("kvDeleteInternal error = %v", err)
	}
	if got := d.kvGetStringInternal("flag"); got != "" {
		t.Errorf("after delete kvGetStringInternal = %q, want empty", got)
	}
}

func TestHandleKVList(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	entries := []rpc.KVBatchSetEntry{
		{Key: "app/a", Value: "1"},
		{Key: "app/b", Value: "2"},
		{Key: "other/c", Value: "3", TTL: 3600},
	}
	if _, err := d.handleKVBatchSet(context.Background(), mustJSON(t, rpc.KVBatchSetParams{Entries: entries})); err != nil {
		t.Fatalf("handleKVBatchSet error = %v", err)
	}

	// keys-only（默认）
	result, err := d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "app/"}))
	if err != nil {
		t.Fatalf("handleKVList error = %v", err)
	}
	m := result.(map[string]interface{})
	if m["count"].(int) != 2 {
		t.Errorf("count = %v, want 2", m["count"])
	}
	keys, _ := m["keys"].([]string)
	if len(keys) != 2 || keys[0] != "app/a" || keys[1] != "app/b" {
		t.Errorf("keys = %#v, want [app/a app/b]", keys)
	}
	if _, hasItems := m["items"]; hasItems {
		t.Error("keys-only list should not include items")
	}

	// with_values
	result, err = d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "app/", WithValues: true}))
	if err != nil {
		t.Fatalf("handleKVList with_values error = %v", err)
	}
	m = result.(map[string]interface{})
	items, _ := m["items"].([]kvItem)
	if len(items) != 2 || items[0].Value != "1" || items[1].Value != "2" {
		t.Errorf("items = %#v, want values 1,2", items)
	}

	// limit 生效
	result, err = d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "app/", Limit: 1}))
	if err != nil {
		t.Fatalf("handleKVList limit error = %v", err)
	}
	if m := result.(map[string]interface{}); m["count"].(int) != 1 {
		t.Errorf("count with limit=1 = %v, want 1", m["count"])
	}

	// 过期条目跳过
	putRawKVItem(t, d, kvItem{Key: "app/dead", Value: "x", CreatedAt: time.Now().Unix(), ExpiresAt: time.Now().Unix() - 10})
	result, err = d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "app/"}))
	if err != nil {
		t.Fatalf("handleKVList expired error = %v", err)
	}
	if m := result.(map[string]interface{}); m["count"].(int) != 2 {
		t.Errorf("count with expired entry = %v, want 2", m["count"])
	}
}

func TestHandleKVList_NotInitialized(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	withNilKVStore(t, d, func() {
		if _, err := d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{})); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestHandleKVBatchSet(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVBatchSet(context.Background(), mustJSON(t, rpc.KVBatchSetParams{})); err == nil {
		t.Error("empty entries: expected error, got nil")
	}
	withNilKVStore(t, d, func() {
		p := rpc.KVBatchSetParams{Entries: []rpc.KVBatchSetEntry{{Key: "k", Value: "v"}}}
		if _, err := d.handleKVBatchSet(context.Background(), mustJSON(t, p)); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestHandleKVClear(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleKVBatchSet(context.Background(), mustJSON(t, rpc.KVBatchSetParams{Entries: []rpc.KVBatchSetEntry{
		{Key: "tmp/1", Value: "a"},
		{Key: "tmp/2", Value: "b"},
		{Key: "keep/3", Value: "c"},
	}})); err != nil {
		t.Fatalf("handleKVBatchSet error = %v", err)
	}

	result, err := d.handleKVClear(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "tmp/"}))
	if err != nil {
		t.Fatalf("handleKVClear error = %v", err)
	}
	m := result.(map[string]interface{})
	if m["deleted"].(int) != 2 || m["status"] != "ok" {
		t.Errorf("clear result = %#v, want deleted=2", m)
	}

	result, err = d.handleKVList(context.Background(), mustJSON(t, rpc.KVListParams{}))
	if err != nil {
		t.Fatalf("handleKVList error = %v", err)
	}
	if m := result.(map[string]interface{}); m["count"].(int) != 1 {
		t.Errorf("remaining count = %v, want 1 (keep/3)", m["count"])
	}

	withNilKVStore(t, d, func() {
		if _, err := d.handleKVClear(context.Background(), mustJSON(t, rpc.KVListParams{Prefix: "tmp/"})); err == nil {
			t.Error("nil kvstore: expected error, got nil")
		}
	})
}

func TestDecodeKVItem(t *testing.T) {
	// 标准 kvItem 包装
	item := decodeKVItem("k", []byte(`{"key":"k","value":"v","created_at":123}`))
	if item.Value != "v" || item.CreatedAt != 123 {
		t.Errorf("decodeKVItem wrapped = %#v", item)
	}
	// 裸 JSON 回退：与 kvItem struct 类型不匹配的合法 JSON（数组）走 raw 回退
	item = decodeKVItem("k", []byte(`[1,2,3]`))
	if item.Key != "k" {
		t.Errorf("decodeKVItem raw key = %q", item.Key)
	}
	if v, ok := item.Value.([]interface{}); !ok || len(v) != 3 {
		t.Errorf("decodeKVItem raw value = %#v, want [1,2,3]", item.Value)
	}
	// 非 JSON 回退为字符串
	item = decodeKVItem("k", []byte("plain text"))
	if item.Value != "plain text" {
		t.Errorf("decodeKVItem plain = %#v, want %q", item.Value, "plain text")
	}
}

func TestInitKVStore(t *testing.T) {
	db, err := initKVStore(t.TempDir())
	if err != nil {
		t.Fatalf("initKVStore error = %v", err)
	}
	defer db.Close()
	// 默认 bucket 应已就绪
	err = db.View(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte(kvStoreBucket)) == nil {
			t.Error("default bucket missing after initKVStore")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("view: %v", err)
	}

	// 非法数据目录（路径是文件）应报错
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := initKVStore(f); err == nil {
		t.Error("initKVStore with file as dir: expected error, got nil")
	}
}
